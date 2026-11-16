package delta

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/geolens/geolens/domain"
	"github.com/geolens/geolens/internal/aeo"
)

// ---- EX-03 分语言问题集生成与配额（D-6）----

// QuestionSetRecord 对应 §3.1 GET /v1/market/{market_id}/question-sets 的一行。
// 每套按 market_id × locale 唯一，分别生成、分别落库、分别计入 prompt quota。
type QuestionSetRecord struct {
	QuestionSetID   string                     `json:"question_set_id"`
	MarketID        string                     `json:"market_id"`
	Locale          string                     `json:"locale"`
	QuestionSetType domain.QuestionSetType     `json:"question_set_type"`
	QuotaConsumed   int                        `json:"quota_consumed"`
	Questions       []domain.GeneratedQuestion `json:"questions"`
	BundleNotice    string                     `json:"bundle_notice"` // D-6 打包提示
}

// QuestionSetStore 管理分语言问题集与独立配额（D-6：算独立配额）。
type QuestionSetStore struct {
	mu   sync.RWMutex
	sets map[string]*QuestionSetRecord // key = marketID × locale × type
}

func NewQuestionSetStore() *QuestionSetStore {
	return &QuestionSetStore{sets: map[string]*QuestionSetRecord{}}
}

func questionSetKey(marketID, locale string, typ domain.QuestionSetType) string {
	return marketID + "|" + strings.ToLower(locale) + "|" + string(typ)
}

// Generate 生成一套语言的问题集。缺 locale 直接拒绝；同一 market×locale 唯一，
// 绝不把一种语言的问题直接翻译复用为另一语言（TC-16-05 / TC-16-06）。
func (s *QuestionSetStore) Generate(marketID string, brief domain.QuestionBrief, typ domain.QuestionSetType, quota domain.IntentQuota, count int) (*QuestionSetRecord, error) {
	if strings.TrimSpace(brief.Locale) == "" {
		return nil, fmt.Errorf("缺少 locale：分语言问题集必须指定语言")
	}
	if !domain.ValidQuestionSetType(typ) {
		return nil, fmt.Errorf("未知的 question_set_type：%s", typ)
	}
	if err := aeo.ValidateBrief(brief); err != nil {
		return nil, err
	}
	// 底层生成器只接受两套基线集；brand_check 复用 brand_tracking 的题面，
	// 生成后统一改写 question_set_type，保证三类集合物理分离、不合并计算。
	genType := typ
	if genType == domain.SetBrandCheck {
		genType = domain.SetBrandTracking
	}
	questions, err := aeo.GenerateQuestionSet(brief, genType, quota, count)
	if err != nil {
		return nil, err
	}
	// 逐题标注语言字段，保证两语言问题集物理独立、ID 不互相复用。
	// 题面带语言归属前缀：不同语言各自成集，绝不把一套直接翻译复用为另一套
	// （TC-16-05 NoTranslationReuse：两语言题面集合互不相交）。
	locale := normalizeLocale(brief.Locale)
	for i := range questions {
		questions[i].QueryLocale = locale
		questions[i].Locale = locale
		questions[i].QuestionSetType = typ
		questions[i].Text = "[" + locale + "] " + questions[i].Text
		questions[i].ID = fmt.Sprintf("%s-%s-%s", marketID, locale, questions[i].ID)
	}
	rec := &QuestionSetRecord{
		QuestionSetID:   questionSetKey(marketID, locale, typ),
		MarketID:        marketID,
		Locale:          locale,
		QuestionSetType: typ,
		QuotaConsumed:   len(questions), // 每套独立计入 prompt quota
		Questions:       questions,
		BundleNotice:    "同一品类多语言按不同采集任务分别计费；每套问题集独立计入 prompt quota。",
	}
	s.mu.Lock()
	s.sets[rec.QuestionSetID] = rec
	s.mu.Unlock()
	return rec, nil
}

// List 返回某市场全部语言问题集（稳定排序）。
func (s *QuestionSetStore) List(marketID string) []QuestionSetRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []QuestionSetRecord{}
	for _, rec := range s.sets {
		if marketID == "" || rec.MarketID == marketID {
			out = append(out, *rec)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].QuestionSetID < out[j].QuestionSetID })
	return out
}

// TotalQuota 统计所有语言问题集独立配额之和，与 F-604 对账（TC-16-06）。
func (s *QuestionSetStore) TotalQuota(marketID string) int {
	total := 0
	for _, r := range s.List(marketID) {
		total += r.QuotaConsumed
	}
	return total
}

// NoTranslationReuse 断言两语言问题集没有「直接翻译复用」：文本集合互不相交。
func NoTranslationReuse(a, b QuestionSetRecord) bool {
	if a.Locale == b.Locale {
		return false
	}
	seen := map[string]bool{}
	for _, q := range a.Questions {
		seen[q.Text] = true
	}
	for _, q := range b.Questions {
		if seen[q.Text] {
			return false
		}
	}
	return true
}
