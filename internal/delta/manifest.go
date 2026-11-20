package delta

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/answerlens/answerlens/domain"
)

// ---- EX-09 ToolManifest 版本表 + tools/list 固定包装（M6 后小版本）----

// ToolDescriptor 是清单里的一个工具（含自检工具）。
type ToolDescriptor struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SelfCheckToolNames 是 M6 后新增的自检工具（EX-09）。
var SelfCheckToolNames = []string{"self_check.retrieval_locale", "self_check.suspect_quarantine"}

// ManifestStore 管理工具清单版本、保留最近 N 版、破坏性变更检测。
type ManifestStore struct {
	mu        sync.RWMutex
	manifests []domain.ToolManifest
	keepN     int
}

func NewManifestStore(cfg *Config) *ManifestStore {
	return &ManifestStore{
		keepN:     int(cfg.Get(domain.ThresholdManifestKeepVersions)),
		manifests: []domain.ToolManifest{},
	}
}

// Publish 发布一个版本。version 必须是 major.minor；破坏性变更由调用方声明
// 并触发 tool_manifest_changed 事件，不兼容变更才升 major（TC-16-14）。
func (s *ManifestStore) Publish(version string, tools []ToolDescriptor, breaking bool, cfg *Config) (domain.ToolManifest, string, error) {
	if !validMajorMinor(version) {
		return domain.ToolManifest{}, "", fmt.Errorf("manifest_version 必须形如 major.minor，收到 %q", version)
	}
	b, err := json.Marshal(tools)
	if err != nil {
		return domain.ToolManifest{}, "", err
	}
	s.mu.Lock()
	prev := ""
	if len(s.manifests) > 0 {
		prev = s.manifests[len(s.manifests)-1].ManifestVersion
	}
	m := domain.ToolManifest{
		ManifestID:       int64(len(s.manifests) + 1),
		ManifestVersion:  version,
		PublishedAt:      fixedTime(),
		ToolsJSON:        string(b),
		IsBreakingChange: breaking,
		PreviousVersion:  prev,
	}
	s.manifests = append(s.manifests, m)
	// 保留最近 keepN 版（TC-16-13：发布 7 版只保留 5 版）。
	if len(s.manifests) > s.keepN {
		s.manifests = s.manifests[len(s.manifests)-s.keepN:]
	}
	s.mu.Unlock()

	event := ""
	if breaking {
		event = "tool_manifest_changed"
		cfg.Audit("manifest", event, version)
	}
	return m, event, nil
}

func (s *ManifestStore) Versions() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []string{}
	for _, m := range s.manifests {
		out = append(out, m.ManifestVersion)
	}
	return out
}

func (s *ManifestStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.manifests)
}

func validMajorMinor(v string) bool {
	parts := strings.Split(v, ".")
	if len(parts) != 2 {
		return false
	}
	if _, err := strconv.Atoi(parts[0]); err != nil {
		return false
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
		return false
	}
	return true
}

// ---- EX-10 tools/list 固定包装 + 幂等 + 裁切提示 ----

// ToolsListResponse 是 tools/list 的固定包装（§3.2）。
type ToolsListResponse struct {
	ManifestVersion string           `json:"manifest_version"`
	Tools           []ToolDescriptor `json:"tools"`
	ReturnedCount   int              `json:"returned_count"`
	TotalCount      int              `json:"total_count"`
	Truncated       bool             `json:"truncated"`
}

// BuildToolsList 生成固定包装：按 name 稳定排序后裁切到 limit；
// truncated=true 时客户端必须显示「能力集不完整」（§6 CI 断言）。
func BuildToolsList(version string, tools []ToolDescriptor, limit int) ToolsListResponse {
	sorted := append([]ToolDescriptor{}, tools...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	total := len(sorted)
	truncated := false
	if limit > 0 && total > limit {
		sorted = sorted[:limit]
		truncated = true
	}
	return ToolsListResponse{
		ManifestVersion: version,
		Tools:           sorted,
		ReturnedCount:   len(sorted),
		TotalCount:      total,
		Truncated:       truncated,
	}
}

// ClientNotice 是 truncated 时客户端必须展示的提示。
func ClientNotice(resp ToolsListResponse) string {
	if resp.Truncated {
		return domain.ToolsetIncomplete
	}
	return ""
}

// ---- Webhook 幂等（§3.2 tool_manifest_changed / suspect_marked）----

// WebhookDedup 按 (event_type, payload_hash) 去重，重复投递只入库一次
// （TC-16-15 / F-903）。
type WebhookDedup struct {
	mu      sync.Mutex
	seen    map[string]bool
	entries []WebhookEntry
}

type WebhookEntry struct {
	EventType   string `json:"event_type"`
	PayloadHash string `json:"payload_hash"`
}

func NewWebhookDedup() *WebhookDedup {
	return &WebhookDedup{seen: map[string]bool{}, entries: []WebhookEntry{}}
}

// Deliver 返回 true 表示首次入库，false 表示重复投递被忽略。
func (d *WebhookDedup) Deliver(eventType, payloadHash string) bool {
	key := eventType + "|" + payloadHash
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.seen[key] {
		return false
	}
	d.seen[key] = true
	d.entries = append(d.entries, WebhookEntry{EventType: eventType, PayloadHash: payloadHash})
	return true
}

func (d *WebhookDedup) Count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.entries)
}
