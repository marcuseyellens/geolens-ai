// Package growth 提供 WP-6.3 可分享检查结果与 WP-6.6 转化漏斗事件的最小化持久化。
// 独立于业务仓储，便于单独清理与审计；不采集任何个人信息。
package growth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxEvents = 20000

var allowedEvents = map[string]bool{
	"visit":       true,
	"check":       true,
	"result":      true,
	"share":       true,
	"signup":      true,
	"checkout_39": true,
	"subscribe":   true,
}

// Steps 是漏斗的固定顺序，前端与看板共用。
var Steps = []string{"visit", "check", "result", "share", "signup", "checkout_39", "subscribe"}

type sharedAudit struct {
	Token     string          `json:"token"`
	Score     json.RawMessage `json:"score"`
	CreatedAt time.Time       `json:"created_at"`
}

type FunnelEvent struct {
	Event     string    `json:"event"`
	Path      string    `json:"path,omitempty"`
	TS        int64     `json:"ts,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type fileData struct {
	Shares map[string]sharedAudit `json:"shares"`
	Events []FunnelEvent          `json:"events"`
}

// FunnelStats 是看板数据；转化率按 WBS §WP-8.2 的两个基线口径给出。
type FunnelStats struct {
	Steps       []string           `json:"steps"`
	Counts      map[string]int     `json:"counts"`
	Conversions map[string]float64 `json:"conversions"`
	Baselines   map[string]float64 `json:"baselines"`
	TotalEvents int                `json:"total_events"`
	UniqueDays  int                `json:"unique_days"`
	Since       string             `json:"since"`
}

type Store struct {
	mu   sync.Mutex
	path string
	data fileData
}

func NewStore(path string) (*Store, error) {
	s := &Store{path: path, data: fileData{
		Shares: map[string]sharedAudit{},
		Events: []FunnelEvent{},
	}}
	b, err := os.ReadFile(path)
	if err == nil && len(b) > 0 {
		if err := json.Unmarshal(b, &s.data); err != nil {
			return nil, err
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if s.data.Shares == nil {
		s.data.Shares = map[string]sharedAudit{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) saveLocked() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// SaveShare 保存一次检查结果并返回短 token。
func (s *Store) SaveShare(score json.RawMessage) (string, error) {
	if len(score) == 0 || !json.Valid(score) {
		return "", errors.New("invalid score payload")
	}
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Shares[token] = sharedAudit{
		Token:     token,
		Score:     append(json.RawMessage(nil), score...),
		CreatedAt: time.Now().UTC(),
	}
	return token, s.saveLocked()
}

// GetShare 返回免登录可访问的检查结果。
func (s *Store) GetShare(token string) (json.RawMessage, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.data.Shares[token]
	if !ok {
		return nil, false
	}
	return append(json.RawMessage(nil), a.Score...), true
}

// RecordEvent 记录匿名漏斗事件，非白名单事件直接忽略。
func (s *Store) RecordEvent(e FunnelEvent) error {
	if !allowedEvents[e.Event] {
		return nil
	}
	e.CreatedAt = time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Events = append(s.data.Events, e)
	if len(s.data.Events) > maxEvents {
		s.data.Events = s.data.Events[len(s.data.Events)-maxEvents:]
	}
	return s.saveLocked()
}

func (s *Store) Funnel() FunnelStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	counts := map[string]int{}
	for _, step := range Steps {
		counts[step] = 0
	}
	days := map[string]bool{}
	for _, e := range s.data.Events {
		counts[e.Event]++
		days[e.CreatedAt.Format("2006-01-02")] = true
	}

	ratio := func(num, den string) float64 {
		if counts[den] == 0 {
			return 0
		}
		return float64(counts[num]) / float64(counts[den])
	}

	conversions := map[string]float64{
		"visit_to_check":       ratio("check", "visit"),
		"check_to_result":      ratio("result", "check"),
		"result_to_share":      ratio("share", "result"),
		"check_to_checkout_39": ratio("checkout_39", "check"),
		"checkout_39_to_subscribe": ratio("subscribe", "checkout_39"),
	}

	since := ""
	var earliest time.Time
	for _, e := range s.data.Events {
		if earliest.IsZero() || e.CreatedAt.Before(earliest) {
			earliest = e.CreatedAt
		}
	}
	if !earliest.IsZero() {
		since = earliest.Format("2006-01-02")
	}

	return FunnelStats{
		Steps:       append([]string(nil), Steps...),
		Counts:      counts,
		Conversions: conversions,
		Baselines: map[string]float64{
			"check_to_checkout_39":     0.08,
			"checkout_39_to_subscribe": 0.15,
		},
		TotalEvents: len(s.data.Events),
		UniqueDays:  len(days),
		Since:       since,
	}
}

func randomToken() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
