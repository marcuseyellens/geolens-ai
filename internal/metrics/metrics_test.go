package metrics

import (
	"strings"
	"testing"

	"github.com/geolens/geolens/domain"
)

func ans(runID, questionID string, p domain.AIModelID) domain.RunAnswer {
	return domain.RunAnswer{
		RunID: runID, QuestionID: questionID, Platform: p, Valid: true,
		ObservedOn: "2026-11-12", SearchActivity: domain.SearchOff,
		RawAnswerText: "示例答案原文", Mention: domain.Mention{BrandID: "b1", Sentiment: domain.SentimentNeutral},
	}
}

func mention(a domain.RunAnswer, m, r, l bool) domain.RunAnswer {
	a.Mention.Mentioned = m
	a.Mention.Recommended = r
	a.Mention.Linked = l
	return a
}

// §5.1 失败运行排除：超时/限流/解析失败/脏数据不得记为「未提及」，且三数都要显示。
func TestFailedRunsExcluded(t *testing.T) {
	all := []domain.RunAnswer{
		mention(ans("r1", "q1", domain.ModelChatGPT), true, true, false),
		func() domain.RunAnswer {
			a := ans("r2", "q2", domain.ModelClaude)
			a.Valid = false
			a.FailureType = domain.FailureTimeout
			return a
		}(),
		func() domain.RunAnswer {
			a := ans("r3", "q3", domain.ModelGemini)
			a.Valid = false
			a.FailureType = domain.FailureRateLimit
			return a
		}(),
		func() domain.RunAnswer {
			a := ans("r4", "q4", domain.ModelPerplexity)
			a.Valid = false
			a.FailureType = domain.FailureParse
			return a
		}(),
	}
	m := Buyer(all)
	if m.Planned != 4 || m.Usable != 1 || m.Failed != 3 {
		t.Fatalf("planned/usable/failed = %d/%d/%d, want 4/1/3", m.Planned, m.Usable, m.Failed)
	}
	if m.MentionRate != 1 {
		t.Fatalf("mention_rate = %v, want 1 (分母只能是有效答案)", m.MentionRate)
	}
}

// DoD 3 分母为 0 显示「样本不足」，而不是把无数据显示成 0%。
func TestZeroDenominatorShowsInsufficientSample(t *testing.T) {
	m := Buyer(nil)
	if m.Usable != 0 || m.MentionRate != 0 {
		t.Fatalf("usable=%d rate=%v", m.Usable, m.MentionRate)
	}
	if !strings.Contains(m.SampleNote, "样本不足") {
		t.Fatalf("SampleNote = %q, want 样本不足", m.SampleNote)
	}
}

// §5.2 三态分离：recommended 与 linked 独立计数，不得把 recommended 当成 mentioned。
func TestThreeStatesIndependent(t *testing.T) {
	all := []domain.RunAnswer{
		mention(ans("r1", "q1", domain.ModelChatGPT), true, true, true),
		mention(ans("r2", "q2", domain.ModelClaude), true, false, false),
		mention(ans("r3", "q3", domain.ModelGemini), false, false, false),
		// 脏数据：recommended=true 但 mentioned=false，不能补算成提及。
		mention(ans("r4", "q4", domain.ModelPerplexity), false, true, false),
	}
	m := Buyer(all)
	if m.Mentioned != 2 || m.Recommended != 2 || m.Linked != 1 {
		t.Fatalf("mentioned/recommended/linked = %d/%d/%d, want 2/2/1", m.Mentioned, m.Recommended, m.Linked)
	}
	if m.MentionRate != 0.5 || m.RecommendRate != 0.5 || m.LinkRate != 0.25 {
		t.Fatalf("rates = %v/%v/%v", m.MentionRate, m.RecommendRate, m.LinkRate)
	}
}

// §5.3 Brand Check 与 Buyer Question 分母完全隔离。
func TestBrandCheckSeparated(t *testing.T) {
	all := []domain.RunAnswer{
		mention(ans("r1", "q1", domain.ModelChatGPT), true, true, false),
		func() domain.RunAnswer {
			a := mention(ans("r2", "q2", domain.ModelClaude), true, true, false)
			a.IsBrandCheck = true
			return a
		}(),
		func() domain.RunAnswer {
			a := mention(ans("r3", "q3", domain.ModelGemini), true, true, false)
			a.IsBrandCheck = true
			return a
		}(),
	}
	buyer := Buyer(all)
	check := BrandCheck(all)
	if buyer.Usable != 1 || buyer.Mentioned != 1 || buyer.BrandCheckExcluded != 2 {
		t.Fatalf("buyer usable=%d mentioned=%d excluded=%d, want 1/1/2", buyer.Usable, buyer.Mentioned, buyer.BrandCheckExcluded)
	}
	if check.Usable != 2 || check.Mentioned != 2 {
		t.Fatalf("brand check usable=%d mentioned=%d, want 2/2", check.Usable, check.Mentioned)
	}
	if buyer.MentionRate != 1 || check.MentionRate != 1 {
		t.Fatalf("两个集合不得合并分母: %v / %v", buyer.MentionRate, check.MentionRate)
	}
}

// §5.4 matched comparison 只比较两次都 valid 的同题同平台子集。
func TestMatchedComparisonOnlyValidPairs(t *testing.T) {
	before := []domain.RunAnswer{
		mention(ans("b1", "q1", domain.ModelChatGPT), true, false, false),
		mention(ans("b2", "q2", domain.ModelChatGPT), false, false, false),
		func() domain.RunAnswer { a := ans("b3", "q3", domain.ModelClaude); a.Valid = false; return a }(),
	}
	after := []domain.RunAnswer{
		mention(ans("a1", "q1", domain.ModelChatGPT), true, false, false),
		func() domain.RunAnswer { a := ans("a2", "q2", domain.ModelChatGPT); a.Valid = false; return a }(),
		mention(ans("a3", "q3", domain.ModelClaude), true, false, false),
	}
	c := MatchedComparison(before, after)
	if c.MatchedQuestions != 1 || c.BeforeUsable != 1 || c.AfterUsable != 1 {
		t.Fatalf("matched=%d before=%d after=%d, want 1/1/1", c.MatchedQuestions, c.BeforeUsable, c.AfterUsable)
	}
	if c.BeforeRate != 1 || c.AfterRate != 1 {
		t.Fatalf("rates = %v/%v", c.BeforeRate, c.AfterRate)
	}
}

// §5.5 复扫结果必须带「观察不等于因果」措辞，禁止写「修复带来」。
func TestObservationNotCausal(t *testing.T) {
	c := MatchedComparison(
		[]domain.RunAnswer{mention(ans("b1", "q1", domain.ModelChatGPT), false, false, false)},
		[]domain.RunAnswer{mention(ans("a1", "q1", domain.ModelChatGPT), true, false, false)},
	)
	if c.AfterRate <= c.BeforeRate {
		t.Fatal("fixture 应呈现上升，才能验证仍然不做因果归因")
	}
	if !strings.Contains(c.Note, "不证明修复导致变化") {
		t.Fatalf("note = %q, want 观察≠因果措辞", c.Note)
	}
}

// §5.6 forced search 单独标注，排除出主指标。
func TestForcedSearchExcluded(t *testing.T) {
	all := []domain.RunAnswer{
		mention(ans("r1", "q1", domain.ModelChatGPT), true, false, false),
		func() domain.RunAnswer {
			a := mention(ans("r2", "q2", domain.ModelClaude), true, false, false)
			a.ForcedSearch = true
			a.SearchActivity = domain.SearchOn
			return a
		}(),
	}
	m := Buyer(all)
	if m.Usable != 1 || m.ForcedExcluded != 1 || m.MentionRate != 1 {
		t.Fatalf("usable=%d forced=%d rate=%v, want 1/1/1", m.Usable, m.ForcedExcluded, m.MentionRate)
	}
}

// §5.7 citation 按来源类型归一，保留 run_id，且统计的是「答案展示的链接」。
func TestCitationClassification(t *testing.T) {
	all := []domain.RunAnswer{
		func() domain.RunAnswer {
			a := ans("r1", "q1", domain.ModelChatGPT)
			a.Citations = []domain.Citation{
				{URL: "https://competitor-a.com/best", Domain: "competitor-a.com", SourceClass: domain.SourceCompetitorSite},
				{URL: "https://roundup.io/top", Domain: "roundup.io", SourceClass: domain.SourceIndependentRoundup},
			}
			return a
		}(),
		func() domain.RunAnswer {
			a := ans("r2", "q2", domain.ModelClaude)
			a.Citations = []domain.Citation{{URL: "https://competitor-a.com/x", Domain: "competitor-a.com", SourceClass: domain.SourceCompetitorSite, DirectCitation: true}}
			return a
		}(),
	}
	groups := CitationMap(all, Filter{BrandCheck: BoolPtr(false)})
	if len(groups) != 2 || groups[0].SourceClass != domain.SourceCompetitorSite || groups[0].Count != 2 {
		t.Fatalf("groups = %+v", groups)
	}
	if len(groups[0].Domains) != 1 || groups[0].Domains[0] != "competitor-a.com" {
		t.Fatalf("domains = %+v", groups[0].Domains)
	}
	if groups[0].Evidence[0].RunID != "r1" || groups[1].Evidence[0].RunID != "r1" {
		t.Fatalf("evidence 应保留 run_id: %+v / %+v", groups[0].Evidence, groups[1].Evidence)
	}
	if m := Buyer(all); m.CitationRate != 1 {
		t.Fatalf("citation_rate = %v, want 1", m.CitationRate)
	}
}

// §5.8 名称混淆单独报告，不进品牌提及总数。
func TestNameConfusionNotMention(t *testing.T) {
	all := []domain.RunAnswer{
		mention(ans("r1", "q1", domain.ModelChatGPT), true, false, false),
		func() domain.RunAnswer {
			a := mention(ans("r2", "q2", domain.ModelClaude), true, false, false)
			a.Mention.ConfusedWith = "同名竞品"
			return a
		}(),
	}
	m := Buyer(all)
	if m.Usable != 2 || m.Mentioned != 1 || m.ConfusionCount != 1 {
		t.Fatalf("usable=%d mentioned=%d confusion=%d, want 2/1/1", m.Usable, m.Mentioned, m.ConfusionCount)
	}
	c := Confusions(all)
	if len(c) != 1 || c[0].RunID != "r2" || c[0].ConfusedWith != "同名竞品" {
		t.Fatalf("confusions = %+v", c)
	}
}

// F-305 / DoD 6：逐模型输出，不提供单一合并总分。
func TestByModelHasNoCombinedScore(t *testing.T) {
	all := []domain.RunAnswer{
		mention(ans("r1", "q1", domain.ModelChatGPT), true, false, false),
		mention(ans("r2", "q1", domain.ModelClaude), false, false, false),
	}
	rows := ByModel(all, Filter{BrandCheck: BoolPtr(false)})
	if len(rows) != len(domain.AllModels()) {
		t.Fatalf("models = %d, want %d", len(rows), len(domain.AllModels()))
	}
	if rows[domain.ModelChatGPT].Metrics.MentionRate != 1 || rows[domain.ModelClaude].Metrics.MentionRate != 0 {
		t.Fatalf("逐模型指标错误: %+v", rows)
	}
}
