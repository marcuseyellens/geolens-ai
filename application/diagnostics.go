package application

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/geolens/geolens/domain"
	"github.com/geolens/geolens/internal/metrics"
	"github.com/geolens/geolens/internal/siteaudit"
)

var linkRE = regexp.MustCompile(`https?://[^\s)"'<>\]]+`)

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func classifySource(host, brandHost string, competitors []string) domain.SourceClass {
	if host == "" {
		return domain.SourceOther
	}
	if brandHost != "" && (host == brandHost || strings.HasSuffix(host, "."+brandHost)) {
		return domain.SourceBrandSite
	}
	for _, c := range competitors {
		key := strings.ToLower(c)
		if key != "" && strings.Contains(host, key) {
			return domain.SourceCompetitorSite
		}
	}
	for _, k := range []string{"reddit.com", "quora.com", "forum", "community", "discuss"} {
		if strings.Contains(host, k) {
			return domain.SourceCommunity
		}
	}
	for _, k := range []string{"g2.com", "capterra", "trustradius", "producthunt", "directory"} {
		if strings.Contains(host, k) {
			return domain.SourceDirectory
		}
	}
	for _, k := range []string{"news", "research", "report", "journal", "study"} {
		if strings.Contains(host, k) {
			return domain.SourceNewsResearch
		}
	}
	for _, k := range []string{"best", "top", "roundup", "guide", "review"} {
		if strings.Contains(host, k) {
			return domain.SourceIndependentRoundup
		}
	}
	return domain.SourceOther
}

func extractCitations(text, brandHost string, competitors []string) []domain.Citation {
	seen := map[string]bool{}
	out := []domain.Citation{}
	for _, raw := range linkRE.FindAllString(text, 20) {
		raw = strings.TrimRight(raw, ".,;:!?)")
		if seen[raw] {
			continue
		}
		seen[raw] = true
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		host := strings.ToLower(u.Hostname())
		out = append(out, domain.Citation{
			URL: raw, Domain: host, PagePath: u.Path,
			SourceClass:    classifySource(host, brandHost, competitors),
			DirectCitation: brandHost != "" && (host == brandHost || strings.HasSuffix(host, "."+brandHost)),
		})
	}
	return out
}

// AdaptLegacy 把现有的 DetectionResult 映射为不可变的 RunAnswer 证据。
//
// 旧存储只保存成功结果，失败运行从未落库，因此这里一律 Valid=true；
// 旧数据没有搜索开关记录，一律记为 unknown。三态中的 recommended 由位置近似，
// linked 由答案中指向品牌域名的链接判断。这些适配假设会写进报告的口径说明。
func AdaptLegacy(brand domain.Brand, queries []*domain.DetectionQuery, results []*domain.DetectionResult) []domain.RunAnswer {
	byID := make(map[string]*domain.DetectionQuery, len(queries))
	for _, q := range queries {
		byID[q.ID] = q
	}
	brandHost := hostOf(brand.Website)
	answers := make([]domain.RunAnswer, 0, len(results))
	for _, r := range results {
		q := byID[r.QueryID]
		text, intent := "", domain.QueryIntent("")
		if q != nil {
			text, intent = q.Text, q.Intent
		}
		citations := extractCitations(r.ResponseText, brandHost, brand.Competitors)
		linked := false
		for _, c := range citations {
			if c.DirectCitation {
				linked = true
			}
		}
		competitorMentions := map[string]int{}
		for _, c := range r.CompetitorsMentioned {
			if c != "" {
				competitorMentions[c]++
			}
		}
		brandCount := 0
		if r.BrandMentioned {
			brandCount = 1
		}
		total := brandCount
		for _, n := range competitorMentions {
			total += n
		}
		recommended := r.BrandMentioned && (r.MentionPosition == domain.PositionFirst || r.MentionPosition == domain.PositionTop3)
		isBrandCheck := brand.Name != "" && strings.Contains(strings.ToLower(text), strings.ToLower(brand.Name))
		observed := r.DetectedAt.Format("2006-01-02")

		answers = append(answers, domain.RunAnswer{
			RunID: r.ID, QuestionID: r.QueryID, QuestionText: text,
			IsBrandCheck: isBrandCheck, Intent: intent, QuestionCategory: string(intent),
			Platform: r.Model, ModelVersion: "unrecorded", Region: "CN", Locale: "zh-CN",
			SearchActivity: domain.SearchUnknown, ForcedSearch: false,
			RawAnswerText: r.ResponseText, Citations: citations,
			Valid: true, FailureType: "",
			Mention: domain.Mention{
				BrandID: brand.ID, Mentioned: r.BrandMentioned,
				Recommended: recommended, Linked: linked,
				RankInList: positionRank(r.MentionPosition), InShortlist: recommended,
				Sentiment: r.Sentiment,
			},
			CompetitorMentions: competitorMentions, TotalBrandMentions: total,
			ObservedOn: observed, CollectedAt: r.DetectedAt, LatencyMS: r.ResponseTime,
		})
	}
	return answers
}

func positionRank(p domain.MentionPosition) int {
	switch p {
	case domain.PositionFirst:
		return 1
	case domain.PositionTop3:
		return 3
	case domain.PositionTop5:
		return 5
	case domain.PositionMentioned:
		return 0
	default:
		return 0
	}
}

func buildComparison(answers []domain.RunAnswer) *domain.Comparison {
	byDate := map[string][]domain.RunAnswer{}
	for _, a := range answers {
		byDate[a.ObservedOn] = append(byDate[a.ObservedOn], a)
	}
	if len(byDate) < 2 {
		return nil
	}
	dates := make([]string, 0, len(byDate))
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	c := metrics.MatchedComparison(byDate[dates[len(dates)-2]], byDate[dates[len(dates)-1]])
	return &c
}

func methodologyNotes() []string {
	return []string{
		"planned / usable / failed 三数并列展示，分母恒为有效答案数。",
		"失败、超时、限流、解析失败与脏数据一律排除，不记为「未提及」。",
		"mentioned / recommended / linked 三态独立计数，彼此不互相补算。",
		"Brand Check 与 Buyer Question 分母完全隔离，绝不合并。",
		"forced search 结果单独标注，排除出主指标。",
		"citation 是答案实际展示的链接，不代表答案依据的全部，也不代表训练数据。",
		"名称混淆单独报告，不计入品牌提及。",
		"逐模型并列展示，不提供单一合并 AI 分数。",
		"站点就绪度 /100 与 AI 可见性分栏并列，不合成综合分。技术就绪 ≠ 被推荐。",
		"复扫只显示变化，不证明修复导致变化。",
		"旧检测记录未记录搜索开关与失败运行，适配时视为有效、搜索状态 unknown；recommended 由位置近似，linked 由品牌域名链接判断。",
	}
}

// buildExecutiveSummary 生成报告第一节（F-501.1）：现状、差距、下一步。
// 只陈述可度量的事实，不用"技术就绪≠被推荐"之外的空话。
func buildExecutiveSummary(v domain.MetricSet, audit *domain.SiteScore, fixes []domain.FixItem) domain.ExecutiveSummary {
	current := "暂无有效 Buyer Question 检测记录，尚无可报告结论。"
	if v.Usable > 0 {
		current = "Buyer Question 提及率 " + pct(v.MentionRate) +
			"，推荐率 " + pct(v.RecommendRate) + "（有效样本 " + itoa(v.Usable) + " 条）。"
	}
	if audit != nil && audit.Source != "unreachable" {
		current += " 站点就绪度 " + score(audit.Score) + "/100。"
	}

	gap := "暂无数据，无法判断差距。"
	switch {
	case v.Usable > 0 && v.MentionRate < 0.5:
		gap = "多数 Buyer Question 答案未出现本品牌，站点缺少可被 AI 直接引用的公开事实。"
	case audit != nil && audit.Source != "unreachable" && audit.Score < 70:
		gap = "站点就绪度 " + score(audit.Score) + "/100，低于 70 分基准，缺少关键 AI 可见性元素。"
	case v.Usable > 0:
		gap = "提及率已达标，差距集中在推荐与链接转化，以及站点结构化数据覆盖。"
	case audit != nil && audit.TopFix != "":
		gap = audit.TopFix
	}

	next := "先补充检测数据，再生成修复建议。"
	if len(fixes) > 0 {
		next = "优先执行 " + fixes[0].Priority + " 项：" + fixes[0].Change
	} else if audit != nil && audit.TopFix != "" {
		next = audit.TopFix
	}

	return domain.ExecutiveSummary{Current: current, Gap: gap, Next: next}
}

func pct(f float64) string { return fmt.Sprintf("%.1f%%", f*100) }

func score(f float64) string { return fmt.Sprintf("%.1f", f) }

func itoa(n int) string { return strconv.Itoa(n) }

// BuildDiagnostics 组装报告固定 7 节（F-501）。所有指标都由 RunAnswer 经口径引擎派生，可重算。
func (s *DetectionService) BuildDiagnostics(ctx context.Context, brandID string, audit *domain.SiteScore) (*domain.DiagnosticReport, error) {
	brand, err := s.brands.GetByID(ctx, brandID)
	if err != nil {
		return nil, ErrNotFound
	}
	queries, err := s.queries.ListByBrand(ctx, brandID)
	if err != nil {
		return nil, err
	}
	results, err := s.results.ListByBrand(ctx, brandID)
	if err != nil {
		return nil, err
	}
	answers := AdaptLegacy(*brand, queries, results)
	buyerFilter := metrics.Filter{BrandCheck: metrics.BoolPtr(false)}

	report := &domain.DiagnosticReport{
		MethodologyVersion: domain.MethodologyVersion,
		BrandID:            brand.ID,
		BrandName:          brand.Name,
		GeneratedAt:        time.Now(),
		SiteReadiness:      audit,
		AIVisibility:       metrics.Buyer(answers),
		BuyerQuestions:     metrics.ByIntent(answers, buyerFilter),
		BrandCheck:         metrics.BrandCheck(answers),
		ModelBreakdown:     metrics.ByModel(answers, buyerFilter),
		CompetitorSOV:      metrics.CompetitorSOV(answers, brand.Name, brand.Competitors, buyerFilter),
		CitationMap:        metrics.CitationMap(answers, buyerFilter),
		NameConfusion:      metrics.Confusions(answers),
		Artifacts:          siteaudit.GenerateArtifacts(*brand),
		Evidence:           answers,
		Comparison:         buildComparison(answers),
		MethodologyNotes:   methodologyNotes(),
	}
	report.FixList = siteaudit.FixList(audit, report.AIVisibility)
	report.ExecutiveSummary = buildExecutiveSummary(report.AIVisibility, audit, report.FixList)
	return report, nil
}

// GetVisibility 返回品牌可见性（F-305）：Buyer Question 汇总 + 逐模型指标，刻意不合并单一分数。
func (s *DetectionService) GetVisibility(ctx context.Context, brandID string) (*domain.Visibility, error) {
	brand, err := s.brands.GetByID(ctx, brandID)
	if err != nil {
		return nil, ErrNotFound
	}
	queries, err := s.queries.ListByBrand(ctx, brandID)
	if err != nil {
		return nil, err
	}
	results, err := s.results.ListByBrand(ctx, brandID)
	if err != nil {
		return nil, err
	}
	answers := AdaptLegacy(*brand, queries, results)
	buyerFilter := metrics.Filter{BrandCheck: metrics.BoolPtr(false)}
	return &domain.Visibility{
		BrandID: brand.ID,
		Buyer:   metrics.Buyer(answers),
		Models:  metrics.ByModel(answers, buyerFilter),
	}, nil
}