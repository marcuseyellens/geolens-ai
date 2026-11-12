// Package metrics 实现 PRD §5 的强制统计口径。
//
// 这里的函数全部是纯函数：给定同一批不可变 RunAnswer，永远得到同一组指标。
// 任何聚合指标都必须经过这里，页面不得自行用分子分母相除。
package metrics

import (
	"sort"

	"github.com/geolens/geolens/domain"
)

// Filter 决定哪些答案进入一个指标集合。BrandCheck 为 nil 表示不按该维度过滤。
type Filter struct {
	BrandCheck   *bool
	Platform     domain.AIModelID
	Intent       domain.QueryIntent
	ObservedFrom string
}

// BoolPtr 方便构造 Filter.BrandCheck。
func BoolPtr(v bool) *bool { return &v }

func (f Filter) match(a domain.RunAnswer) bool {
	if f.BrandCheck != nil && a.IsBrandCheck != *f.BrandCheck {
		return false
	}
	if f.Platform != "" && a.Platform != f.Platform {
		return false
	}
	if f.Intent != "" && a.Intent != f.Intent {
		return false
	}
	if f.ObservedFrom != "" && a.ObservedOn < f.ObservedFrom {
		return false
	}
	return true
}

// Effective 返回真正计入分母的答案：有效、非强制搜索、且通过过滤条件。
func Effective(all []domain.RunAnswer, f Filter) []domain.RunAnswer {
	out := make([]domain.RunAnswer, 0, len(all))
	for _, a := range all {
		if a.Valid && !a.ForcedSearch && f.match(a) {
			out = append(out, a)
		}
	}
	return out
}

func rate(num, den int) float64 {
	if den == 0 {
		return 0
	}
	return float64(num) / float64(den)
}

// Compute 从候选答案计算一组指标。
//
// §5.1 失败/超时/限流/解析失败/脏数据一律排除，不记为「未提及」；
//
//	planned / usable / failed 三个数都返回，分母恒为 usable。
//
// §5.2 mentioned / recommended / linked 三态独立计数。
// §5.3 Brand Check 与 Buyer Question 用不同 Filter 分别调用，绝不合并分母。
// §5.6 forced search 结果排除出主指标，单独计数标注。
// §5.8 名称混淆不算品牌提及，单独计数。
// DoD 3 分母为 0 时给出「样本不足」，不把无数据显示成 0%。
func Compute(all []domain.RunAnswer, f Filter) domain.MetricSet {
	m := domain.MetricSet{}
	effective := Effective(all, f)

	for _, a := range all {
		if !f.match(a) {
			// 统计被单列出去的 brand check 有效答案，让报告能说明「另有 N 条单独统计」。
			if f.BrandCheck != nil && !*f.BrandCheck && a.IsBrandCheck && a.Valid && !a.ForcedSearch {
				m.BrandCheckExcluded++
			}
			continue
		}
		m.Planned++
		if !a.Valid {
			m.Failed++
			continue
		}
		if a.ForcedSearch {
			m.ForcedExcluded++
			continue
		}
	}

	m.Usable = len(effective)
	var brandMentions, allBrandMentions, cited int
	for _, a := range effective {
		// §5.8 名称混淆的答案仍是有效分母，但不计入品牌提及。
		if a.Mention.ConfusedWith != "" {
			m.ConfusionCount++
			continue
		}
		if a.Mention.Mentioned {
			m.Mentioned++
			brandMentions++
		}
		if a.Mention.Recommended {
			m.Recommended++
		}
		if a.Mention.Linked {
			m.Linked++
		}
		if len(a.Citations) > 0 {
			cited++
		}
		allBrandMentions += a.TotalBrandMentions
		for _, n := range a.CompetitorMentions {
			allBrandMentions += n
		}
	}
	// 适配层未填 TotalBrandMentions 时，用本品牌 + 竞品明细兜底。
	if allBrandMentions == 0 {
		allBrandMentions = brandMentions
	}

	m.MentionRate = rate(m.Mentioned, m.Usable)
	m.RecommendRate = rate(m.Recommended, m.Usable)
	m.LinkRate = rate(m.Linked, m.Usable)
	m.CitationRate = rate(cited, m.Usable)
	m.ShareOfVoice = rate(brandMentions, allBrandMentions)
	if m.Usable == 0 {
		m.SampleNote = "样本不足：无有效答案，指标不计算"
	}
	return m
}

// Buyer 是 Buyer Question 集合（不含 brand check、不含强制搜索）。
func Buyer(all []domain.RunAnswer) domain.MetricSet {
	return Compute(all, Filter{BrandCheck: BoolPtr(false)})
}

// BrandCheck 是 brand check 的独立集合，分母与 Buyer Question 完全隔离。
func BrandCheck(all []domain.RunAnswer) domain.MetricSet {
	return Compute(all, Filter{BrandCheck: BoolPtr(true)})
}

// ByModel 逐模型给出指标，刻意不提供「综合 AI 分数」（F-305 / DoD 6）。
func ByModel(all []domain.RunAnswer, f Filter) map[domain.AIModelID]domain.ModelMetricSet {
	out := make(map[domain.AIModelID]domain.ModelMetricSet, len(domain.AllModels()))
	for _, p := range domain.AllModels() {
		pf := f
		pf.Platform = p
		row := domain.ModelMetricSet{Platform: p, Metrics: Compute(all, pf)}
		for _, a := range Effective(all, pf) {
			switch a.Mention.Sentiment {
			case domain.SentimentPositive:
				row.Positive++
			case domain.SentimentNegative:
				row.Negative++
			default:
				row.Neutral++
			}
			switch a.SearchActivity {
			case domain.SearchOn:
				row.SearchOn++
			case domain.SearchOff:
				row.SearchOff++
			case domain.SearchAuto:
				row.SearchAuto++
			default:
				row.SearchUnknown++
			}
		}
		out[p] = row
	}
	return out
}

// ByIntent 按提问意图细分 Buyer Question 指标（F-501 报告分节）。
func ByIntent(all []domain.RunAnswer, f Filter) map[domain.QueryIntent]domain.MetricSet {
	out := make(map[domain.QueryIntent]domain.MetricSet)
	seen := map[domain.QueryIntent]bool{}
	for _, a := range all {
		if a.Intent == "" || seen[a.Intent] {
			continue
		}
		seen[a.Intent] = true
		intentFilter := f
		intentFilter.Intent = a.Intent
		out[a.Intent] = Compute(all, intentFilter)
	}
	return out
}

// CompetitorSOV 计算 Share of Voice，每行都带 run_id 证据，可下钻（F-304）。
func CompetitorSOV(all []domain.RunAnswer, brand string, competitors []string, f Filter) []domain.SOVRow {
	names := append([]string{brand}, competitors...)
	counts := make(map[string]int, len(names))
	runIDs := make(map[string][]string, len(names))
	for _, a := range Effective(all, f) {
		if a.Mention.ConfusedWith != "" {
			continue
		}
		if a.Mention.Mentioned {
			counts[brand]++
			runIDs[brand] = append(runIDs[brand], a.RunID)
		}
		for name, n := range a.CompetitorMentions {
			counts[name] += n
			for i := 0; i < n; i++ {
				runIDs[name] = append(runIDs[name], a.RunID)
			}
		}
	}
	total := 0
	for _, c := range counts {
		total += c
	}
	rows := make([]domain.SOVRow, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		rows = append(rows, domain.SOVRow{
			Brand: name, Mentions: counts[name], Share: rate(counts[name], total), RunIDs: runIDs[name],
		})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Mentions > rows[j].Mentions })
	return rows
}

// CitationMap 把答案展示的链接按来源类型归一，保留可下钻的 run_id（F-108 / §5.7）。
func CitationMap(all []domain.RunAnswer, f Filter) []domain.CitationGroup {
	byClass := map[domain.SourceClass]*domain.CitationGroup{}
	domainSeen := map[domain.SourceClass]map[string]bool{}
	for _, a := range Effective(all, f) {
		for _, c := range a.Citations {
			g := byClass[c.SourceClass]
			if g == nil {
				g = &domain.CitationGroup{SourceClass: c.SourceClass}
				byClass[c.SourceClass] = g
				domainSeen[c.SourceClass] = map[string]bool{}
			}
			g.Count++
			if !domainSeen[c.SourceClass][c.Domain] {
				domainSeen[c.SourceClass][c.Domain] = true
				g.Domains = append(g.Domains, c.Domain)
			}
			if len(g.Evidence) < 3 {
				g.Evidence = append(g.Evidence, domain.CitationSample{
					RunID: a.RunID, URL: c.URL, Intent: string(a.Intent), IsDirect: c.DirectCitation,
				})
			}
		}
	}
	out := make([]domain.CitationGroup, 0, len(byClass))
	for _, sc := range domain.AllSourceClasses() {
		if g := byClass[sc]; g != nil {
			sort.Strings(g.Domains)
			out = append(out, *g)
		}
	}
	return out
}

// Confusions 单独列出名称混淆，供报告单列一节（F-202 / §5.8）。
func Confusions(all []domain.RunAnswer) []domain.NameConfusion {
	out := []domain.NameConfusion{}
	for _, a := range all {
		if !a.Valid || a.Mention.ConfusedWith == "" {
			continue
		}
		out = append(out, domain.NameConfusion{
			RunID: a.RunID, ConfusedWith: a.Mention.ConfusedWith, Excerpt: excerpt(a.RawAnswerText, 160),
		})
	}
	return out
}

func excerpt(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// MatchedComparison 只比较两次都 valid、且 question_id + platform 相同的子集。
// 复扫一律附 ObservationNote，绝不写「修复带来 +X%」（F-208 / §5.4 / §5.5）。
func MatchedComparison(before, after []domain.RunAnswer) domain.Comparison {
	type key struct {
		q string
		p domain.AIModelID
	}
	index := func(list []domain.RunAnswer) map[key]domain.RunAnswer {
		m := make(map[key]domain.RunAnswer, len(list))
		for _, a := range list {
			if a.Valid && !a.ForcedSearch && !a.IsBrandCheck {
				m[key{a.QuestionID, a.Platform}] = a
			}
		}
		return m
	}
	b, a := index(before), index(after)
	c := domain.Comparison{Note: domain.ObservationNote}
	for k, bAns := range b {
		aAns, ok := a[k]
		if !ok {
			continue
		}
		c.MatchedQuestions++
		c.BeforeUsable++
		c.AfterUsable++
		if bAns.Mention.Mentioned && bAns.Mention.ConfusedWith == "" {
			c.BeforeMentioned++
		}
		if aAns.Mention.Mentioned && aAns.Mention.ConfusedWith == "" {
			c.AfterMentioned++
		}
	}
	c.BeforeRate = rate(c.BeforeMentioned, c.BeforeUsable)
	c.AfterRate = rate(c.AfterMentioned, c.AfterUsable)
	return c
}
