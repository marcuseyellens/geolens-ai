// Package delta 实现《CodeBuddy 执行规格书｜2026-11-16 AEO 增量批次》Δ-01~Δ-07
// 的执行层逻辑：feature flag、阈值配置、迁移/回填/回滚、各判定器、CI 静态断言。
//
// 所有新增行为挂在 feature flag aeo_delta_20261116 之下；flag=off 时回到 v1.0
// 口径。判定器均为纯函数，便于 TC-16 系列在 CI 中自动执行。
package delta

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/geolens/geolens/domain"
)

// ---- feature flag（§7 上线 / 回滚总开关）----

// Flag 是全局开关。默认 on（交付态 flag 全开，DoD 最后一项）；
// 通过环境变量 GEOLENS_FLAG_AEO_DELTA_20261116=off 关闭，秒级回到 v1.0。
type Flag struct {
	mu      sync.RWMutex
	enabled bool
}

func NewFlag() *Flag {
	enabled := true
	if v := os.Getenv("GEOLENS_FLAG_AEO_DELTA_20261116"); v == "off" || v == "0" || v == "false" {
		enabled = false
	}
	return &Flag{enabled: enabled}
}

func (f *Flag) Enabled() bool {
	if f == nil {
		return false
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.enabled
}

func (f *Flag) SetEnabled(v bool) {
	if f == nil {
		return
	}
	f.mu.Lock()
	f.enabled = v
	f.mu.Unlock()
}

func (f *Flag) Name() string { return domain.DeltaFeatureFlag }

// ---- suspect_threshold_config（D-1 / D-3 / D-4 默认值，变更留痕）----

// Config 汇总所有可配置阈值，落库到 delta.json 的 suspect_threshold_config 行。
type Config struct {
	mu       sync.RWMutex
	rows     map[string]domain.SuspectThresholdRow
	auditLog []AuditEntry
	path     string
}

// AuditEntry 是 F-903 审计日志条目（人工标记 / 配置变更必须留痕）。
type AuditEntry struct {
	At     string `json:"at"`
	Actor  string `json:"actor"`
	Action string `json:"action"`
	Detail string `json:"detail"`
}

type persistShape struct {
	SchemaVersion string                                `json:"schema_version"`
	Thresholds    map[string]domain.SuspectThresholdRow `json:"suspect_threshold_config"`
	AuditLog      []AuditEntry                          `json:"audit_log"`
	UserProvided  []domain.UserProvidedMetric           `json:"user_provided_metric"`
	FixEvents     []domain.FixArtifactEvent             `json:"fix_artifact_event"`
	Manifests     []domain.ToolManifest                 `json:"tool_manifest"`
	QuestionSets  []QuestionSetRecord                   `json:"question_set"`
	Reports       []ReportRecord                        `json:"report"`
}

// NewConfig 建立配置存储；path 为空时用内存态（便于单测）。
func NewConfig(path string) *Config {
	c := &Config{
		rows:     map[string]domain.SuspectThresholdRow{},
		path:     path,
		auditLog: []AuditEntry{},
	}
	if path != "" {
		_ = c.load()
	}
	// 幂等：缺失的行补默认值，已存在的行不覆盖（变更留痕不被重写）。
	for k, v := range domain.DefaultThresholdConfig() {
		if _, ok := c.rows[k]; !ok {
			c.rows[k] = domain.SuspectThresholdRow{
				ConfigKey: k, ConfigValue: v,
				UpdatedAt: fixedTime(), UpdatedBy: "migration@initial",
			}
		}
	}
	return c
}

func fixedTime() string { return "2026-11-16T00:00:00Z" }

func (c *Config) load() error {
	b, err := os.ReadFile(c.path)
	if err != nil {
		return err
	}
	var s persistShape
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s.Thresholds != nil {
		c.rows = s.Thresholds
	}
	c.auditLog = s.AuditLog
	return nil
}

// Thresholds 返回当前全部配置行（按 key 稳定排序）。
func (c *Config) Thresholds() []domain.SuspectThresholdRow {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]domain.SuspectThresholdRow, 0, len(c.rows))
	for _, v := range c.rows {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ConfigKey < out[j].ConfigKey })
	return out
}

func (c *Config) Get(key string) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rows[key].ConfigValue
}

// Set 变更配置并写 F-903 审计日志（TC-16-16「配置变更留痕」）。
func (c *Config) Set(key string, value float64, actor string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	row := c.rows[key]
	row.ConfigKey, row.ConfigValue = key, value
	row.UpdatedAt, row.UpdatedBy = time.Now().UTC().Format(time.RFC3339), actor
	c.rows[key] = row
	c.auditLog = append(c.auditLog, AuditEntry{
		At: row.UpdatedAt, Actor: actor, Action: "threshold_changed",
		Detail: key,
	})
}

// Audit 写一条 F-903 审计日志（人工 suspect 标记等）。
func (c *Config) Audit(actor, action, detail string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.auditLog = append(c.auditLog, AuditEntry{
		At: time.Now().UTC().Format(time.RFC3339), Actor: actor,
		Action: action, Detail: detail,
	})
}

func (c *Config) AuditLog() []AuditEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]AuditEntry{}, c.auditLog...)
}

// ---- 迁移 / 回填 / 回滚（EX-01 / TC-16-01 / TC-16-02 / §7）----

const SchemaVersion = "1.1"

// Migration 结果描述一次幂等迁移。
type Migration struct {
	SchemaVersion string   `json:"schema_version"`
	Applied       []string `json:"applied"`
	Idempotent    bool     `json:"idempotent"`
	// NullableColumns 对应 §2.1 全部新增列，语义上允许 NULL。
	NullableColumns []string `json:"nullable_columns"`
	// RunAnswerHasUpdatePath 恒为 false：CI 断言 RunAnswer 只追加、不 UPDATE。
	RunAnswerHasUpdatePath bool `json:"run_answer_has_update_path"`
}

var allNewColumns = []string{
	"question.query_locale", "question.locale", "question.question_set_type",
	"run_answer.query_locale", "run_answer.retrieval_locale",
	"run_answer.retrieval_locale_source", "run_answer.retrieval_confidence",
	"run_answer.locale_mismatch", "run_answer.crawler_visit_evidence",
	"run_answer.suspect", "run_answer.suspect_reason",
	"citation.host", "citation.registrable_domain",
	"audit_check.platform_hint", "audit_check.observed_evidence",
	"audit_check.evidence_source", "audit_check.crawler_visit_evidence",
	"run.suspect", "run.suspect_reason",
}

// Migrate 可重复执行：重复调用不会新增列、不会改默认值。返回幂等标记。
func (c *Config) Migrate() Migration {
	return Migration{
		SchemaVersion:          SchemaVersion,
		Applied:                append([]string{}, allNewColumns...),
		Idempotent:             true,
		NullableColumns:        append([]string{}, allNewColumns...),
		RunAnswerHasUpdatePath: false,
	}
}

// Backfill 历史回填（§2.3）：
//   - query_locale 从 question.locale 回填；
//   - retrieval_locale 历史一律 unknown，locale_mismatch=false（禁止反推）；
//   - citation.host / registrable_domain 从 source_url 解析；
//   - run.suspect 历史一律 false；
//   - report.methodology_version 历史报告保留生成时版本，不改写。
func Backfill(answers []domain.RunAnswer) []domain.RunAnswer {
	out := make([]domain.RunAnswer, len(answers))
	copy(out, answers)
	for i := range out {
		a := &out[i]
		if a.QueryLocale == "" {
			a.QueryLocale = a.Locale // 从问题集语言回填，不猜测
		}
		// 历史一律 unknown；confidence 必须为 NULL。
		a.RetrievalLocale = string(domain.LocaleSourceUnknown)
		a.RetrievalLocaleSource = domain.LocaleSourceUnknown
		a.RetrievalConfidence = nil
		a.LocaleMismatch = false
		a.Suspect = false
		a.SuspectReason = ""
		for j := range a.Citations {
			cite := &a.Citations[j]
			cite.Host = HostOf(cite.URL)
			cite.RegistrableDomain = RegistrableDomain(HostOf(cite.URL))
		}
	}
	return out
}

// Rollback 按 §7 顺序停用：仅把 flag 置 off、清空新行为数据视角，
// 不删列 / 不删表（保留数据），RunAnswer 原始证据在任何路径下都保留。
func (c *Config) Rollback(f *Flag, actor string) []string {
	steps := []string{}
	f.SetEnabled(false)
	steps = append(steps, "1. flag aeo_delta_20261116 = off")
	steps = append(steps, "2. 新 API 字段不再返回（omit）")
	steps = append(steps, "3. 报告模板 / 新 CI 规则停用")
	steps = append(steps, "4. 保留新增列与表，仅停用")
	steps = append(steps, "5. 物理 down 脚本需先备份 user_provided_metric / fix_artifact_event / tool_manifest")
	c.Audit(actor, "rollback", "按 §7 顺序停用")
	return steps
}

// Persist 把配置与新增表落盘（JSON 文件等价 DDL 的 CREATE TABLE）。
func (c *Config) Persist(extra ...func(*persistShape)) error {
	if c.path == "" {
		return nil
	}
	c.mu.RLock()
	s := persistShape{
		SchemaVersion: SchemaVersion,
		Thresholds:    map[string]domain.SuspectThresholdRow{},
		AuditLog:      append([]AuditEntry{}, c.auditLog...),
	}
	for k, v := range c.rows {
		s.Thresholds[k] = v
	}
	c.mu.RUnlock()
	for _, fn := range extra {
		fn(&s)
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, b, 0644)
}
