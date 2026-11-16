package regression

import (
	"strings"
	"testing"
	"time"

	"github.com/geolens/geolens/domain"
	"github.com/geolens/geolens/internal/billing"
	"github.com/geolens/geolens/internal/governance"
	"github.com/geolens/geolens/internal/monitor"
	"github.com/geolens/geolens/internal/siteaudit"
)

// offlineFetcher 让 L0 站点检查完全离线可跑。
type offlineFetcher struct{}

func (offlineFetcher) Fetch(rawURL string) siteaudit.FetchResult {
	switch {
	case strings.HasSuffix(rawURL, "/robots.txt"):
		return siteaudit.FetchResult{Status: 200, Body: "User-agent: *\nAllow: /", ElapsedMS: 10}
	case strings.HasSuffix(rawURL, "/llms.txt"):
		return siteaudit.FetchResult{Status: 200, Body: "# GeoLens AI\n> AEO 平台\n", ElapsedMS: 10}
	case strings.HasSuffix(rawURL, "/sitemap.xml"):
		return siteaudit.FetchResult{Status: 200, Body: `<?xml version="1.0"?><urlset></urlset>`, ElapsedMS: 10}
	default:
		return siteaudit.FetchResult{
			Status: 200, ElapsedMS: 20,
			Body: `<!doctype html><html lang="zh-CN"><head><title>GeoLens AI - AEO 可见性平台</title>` +
				`<meta name="description" content="看懂跨平台 AI 可见性差异，拿到能改的东西。">` +
				`</head><body><h1>GeoLens AI</h1><p>监测品牌在 AI 回答中的提及、推荐与引用。</p>` +
				`<a href="/pricing">定价</a><a href="/privacy">隐私</a></body></html>`,
		}
	}
}

func regressionAnswers(n, failed int) []domain.RunAnswer {
	out := []domain.RunAnswer{}
	for i := 0; i < n+failed; i++ {
		a := domain.RunAnswer{
			RunID:      "run-" + string(rune('A'+i%26)) + "-" + string(rune('0'+i/26)),
			QuestionID: "q-1", QuestionText: "推荐一个 AEO 平台",
			Intent: domain.IntentRecommend, Platform: domain.ModelChatGPT,
			Region: "us", Locale: "zh-CN", ObservedOn: "2026-11-14",
			SearchActivity: domain.SearchOn, Valid: true,
			RawAnswerText: "公开形态下采集到的答案原文：GeoLens 与竞品并列出现。",
			Citations: []domain.Citation{{
				URL: "https://example.com/public-review", Domain: "example.com",
				SourceClass: domain.SourceIndependentRoundup,
			}},
			Mention: domain.Mention{
				BrandID: "brand:geolens", Mentioned: i%2 == 0,
				Recommended: i%3 == 0, Linked: i%5 == 0,
			},
			TotalBrandMentions: 1, Cost: 0.012,
		}
		if i >= n {
			a.Valid = false
			a.FailureType = domain.FailureTimeout
			a.RawAnswerText = ""
		}
		out = append(out, a)
	}
	return out
}

func chainInput(answers []domain.RunAnswer) ChainInput {
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	bill := billing.NewEngine(billing.NewLocalGateway(0), clock)
	// 开通订阅档，避免免费层按日 5 次上限影响回归
	o, _ := bill.Checkout("regression-acct", domain.PlanStarter, domain.PaySubscription)
	_, _ = bill.Pay(o.ID)
	return ChainInput{
		Brand:   domain.Brand{ID: "brand:geolens", Name: "GeoLens", Website: "https://example.com"},
		SiteURL: "https://example.com", Fetcher: offlineFetcher{},
		Answers: answers,
		Brief: domain.QuestionBrief{
			Category: "AEO 平台", Audience: "品牌市场负责人", Need: "了解 AI 回答中的品牌可见性",
			Region: "us", Locale: "zh-CN", Constraint: "无",
		},
		Gov: governance.NewEngine(100, clock), Bill: bill,
		Mon: monitor.NewEngine(clock), AccountID: "regression-acct", Clock: clock,
	}
}

// WP-8.1：L0→L3 全链路脚本化回归，每一层数字可下钻到 run_id / 证据 ID。
func TestFullChainL0ToL3Passes(t *testing.T) {
	answers := regressionAnswers(30, 2)
	rep := RunFullChain(chainInput(answers))
	if !rep.Passed {
		t.Fatalf("全链路回归应通过，实际失败: %v", rep.Issues)
	}
	if len(rep.Layers) != 4 {
		t.Fatalf("应有 L0~L3 四层，实际 %d", len(rep.Layers))
	}
	want := []string{"L0", "L1", "L2", "L3"}
	for i, l := range rep.Layers {
		if l.Layer != want[i] {
			t.Errorf("第 %d 层应为 %s，实际 %s", i, want[i], l.Layer)
		}
		if len(l.EvidenceIDs) == 0 {
			t.Errorf("%s 缺少可下钻的证据 ID", l.Layer)
		}
		if l.Provenance.SourceType != domain.SourceObserved {
			t.Errorf("%s 缺少口径标注", l.Layer)
		}
	}
	l1 := rep.Layers[1]
	if len(l1.Drilldowns) != 10 {
		t.Fatalf("L1 应有 10 个聚合指标可下钻，实际 %d", len(l1.Drilldowns))
	}
	for _, d := range l1.Drilldowns {
		if d.RunID == "" || !d.Complete || len(d.Hops) != 3 {
			t.Errorf("下钻路径不完整: %+v", d)
		}
	}
}

// WP-8.1：治理、配额、监控与 L1 数字一致；失败运行不计配额。
func TestFullChainGovernanceQuotaMonitorConsistency(t *testing.T) {
	answers := regressionAnswers(30, 2)
	rep := RunFullChain(chainInput(answers))
	if rep.Governance.Operator != "regression" || rep.Governance.Runs != 1 {
		t.Errorf("治理五元组未记录本次回归: %+v", rep.Governance)
	}
	if rep.Usage.MonthlyUsed != 30 {
		t.Errorf("配额用量应等于 30 条有效 run，实际 %d", rep.Usage.MonthlyUsed)
	}
	if rep.Usage.FailedNotBilled != 2 {
		t.Errorf("失败运行应标记不计，实际 %d", rep.Usage.FailedNotBilled)
	}
	var wantCost float64
	for _, a := range answers {
		wantCost += a.Cost
	}
	if rep.Monitor.DailyCostUSD != wantCost {
		t.Errorf("监控成本与 run 成本不一致: %.4f vs %.4f", rep.Monitor.DailyCostUSD, wantCost)
	}
	if rep.Monitor.CollectionSuccessRate < 0.9 || rep.Monitor.CollectionSuccessRate > 1 {
		t.Errorf("采集有效率异常: %.4f", rep.Monitor.CollectionSuccessRate)
	}
}

// 脚本必须能变红：无有效样本时不得静默通过。
func TestFullChainFailsWithoutValidSamples(t *testing.T) {
	in := chainInput(regressionAnswers(0, 2))
	rep := RunFullChain(in)
	if rep.Passed {
		t.Fatal("无有效样本时全链路回归不应通过")
	}
	found := false
	for _, issue := range rep.Issues {
		if strings.Contains(issue, "L1") {
			found = true
		}
	}
	if !found {
		t.Errorf("失败原因应指出 L1 问题，实际 %v", rep.Issues)
	}
}
