// Package regression 实现基线 WBS WP-8.1「L0→L3 全链路脚本化回归」。
//
// 每一层的数字都必须能下钻到 run_id / 证据 ID；禁止只给一个汇总数字。
// 这里把站点检查（L0）、可见性指标（L1）、问题集与档案（L2）、修复件（L3）
// 串成一次可重复执行的脚本，并把运行治理、配额扣减、监控看板接进同一条链路。
package regression

import (
	"fmt"
	"time"

	"github.com/answerlens/answerlens/domain"
	"github.com/answerlens/answerlens/internal/aeo"
	"github.com/answerlens/answerlens/internal/billing"
	"github.com/answerlens/answerlens/internal/collector"
	"github.com/answerlens/answerlens/internal/governance"
	"github.com/answerlens/answerlens/internal/metrics"
	"github.com/answerlens/answerlens/internal/monitor"
	"github.com/answerlens/answerlens/internal/siteaudit"
)

// LayerReport 是一层的回归结果。EvidenceIDs 在 L1~L3 存 run_id，L0 存检查项 ID。
type LayerReport struct {
	Layer       string                  `json:"layer"`
	Name        string                  `json:"name"`
	Metrics     map[string]float64      `json:"metrics"`
	EvidenceIDs []string                `json:"evidence_ids"`
	Drilldowns  []*domain.DrilldownPath `json:"drilldowns"`
	Provenance  domain.Provenance       `json:"provenance"`
}

// ChainReport 是全链路回归报告。
type ChainReport struct {
	BrandID     string                     `json:"brand_id"`
	GeneratedAt time.Time                  `json:"generated_at"`
	Layers      []LayerReport              `json:"layers"`
	Governance  governance.OperatorSummary `json:"governance"`
	Usage       domain.QuotaUsage          `json:"usage"`
	Monitor     monitor.Snapshot           `json:"monitor"`
	Passed      bool                       `json:"passed"`
	Issues      []string                   `json:"issues"`
}

// ChainInput 是脚本化回归的输入；外部依赖全部可注入。
type ChainInput struct {
	Brand     domain.Brand
	SiteURL   string
	Fetcher   siteaudit.Fetcher
	Answers   []domain.RunAnswer
	Brief     domain.QuestionBrief
	Gov       *governance.Engine
	Bill      *billing.Engine
	Mon       *monitor.Engine
	AccountID string
	Clock     func() time.Time
}

// RunFullChain 执行 L0→L3 全链路脚本化回归。
func RunFullChain(in ChainInput) ChainReport {
	now := in.Clock
	if now == nil {
		now = time.Now
	}
	rep := ChainReport{
		BrandID: in.Brand.ID, GeneratedAt: now(),
		Layers: []LayerReport{}, Issues: []string{},
	}
	addIssue := func(format string, args ...any) {
		rep.Issues = append(rep.Issues, fmt.Sprintf(format, args...))
	}

	// ---- L0 站点就绪度 ----
	audit, err := siteaudit.Audit(in.SiteURL, in.Fetcher)
	if err != nil {
		addIssue("L0 站点检查失败: %v", err)
	}
	l0 := LayerReport{
		Layer: "L0", Name: "站点就绪度",
		Metrics:     map[string]float64{"readiness_score": audit.Score},
		EvidenceIDs: []string{},
		Provenance:  audit.Provenance,
	}
	for _, c := range audit.Checks {
		l0.EvidenceIDs = append(l0.EvidenceIDs, c.ID)
	}
	if len(l0.EvidenceIDs) == 0 {
		addIssue("L0 没有可下钻的检查项证据")
	}
	rep.Layers = append(rep.Layers, l0)

	// ---- L1 可见性聚合（10 个指标逐项下钻 run_id）----
	visible := metrics.Compute(in.Answers, metrics.Filter{})
	if visible.Usable == 0 {
		addIssue("L1 有效样本为 0，聚合指标不可解释")
	}
	anchor := firstEvidence(in.Answers)
	l1 := LayerReport{
		Layer: "L1", Name: "AI 可见性",
		Metrics: map[string]float64{
			"planned": float64(visible.Planned), "usable": float64(visible.Usable),
			"failed": float64(visible.Failed), "forced_excluded": float64(visible.ForcedExcluded),
			"brand_check_excluded": float64(visible.BrandCheckExcluded),
			"mentioned":            float64(visible.Mentioned), "recommended": float64(visible.Recommended),
			"linked":       float64(visible.Linked),
			"mention_rate": visible.MentionRate, "citation_rate": visible.CitationRate,
		},
		EvidenceIDs: []string{}, Drilldowns: []*domain.DrilldownPath{},
		Provenance: domain.ObservedProvenance(visible.Usable),
	}
	for _, a := range in.Answers {
		if a.Valid {
			l1.EvidenceIDs = append(l1.EvidenceIDs, a.RunID)
		}
	}
	for name, value := range l1.Metrics {
		p := aeo.BuildDrilldown(name, value, visible.Usable, anchor.runID, anchor.rawAnswer, anchor.pageURL)
		if p == nil {
			addIssue("L1 指标 %s 无法下钻到 run_id", name)
			continue
		}
		l1.Drilldowns = append(l1.Drilldowns, p)
	}
	if len(l1.Drilldowns) != 10 {
		addIssue("L1 应有 10 个聚合指标可下钻，实际 %d", len(l1.Drilldowns))
	}
	if errs := aeo.ValidateDrilldowns(l1.Drilldowns); len(errs) > 0 {
		rep.Issues = append(rep.Issues, errs...)
	}
	rep.Layers = append(rep.Layers, l1)

	// ---- L2 问题集与档案 ----
	quota := domain.DefaultIntentQuota()
	market, err := aeo.GenerateQuestionSet(in.Brief, domain.SetMarketBaseline, quota, 40)
	if err != nil {
		addIssue("L2 市场基线问题集生成失败: %v", err)
	}
	tracking, err := aeo.GenerateQuestionSet(in.Brief, domain.SetBrandTracking, quota, 40)
	if err != nil {
		addIssue("L2 品牌追踪问题集生成失败: %v", err)
	}
	if err := aeo.SeparatedSets(market, tracking); err != nil {
		addIssue("L2 两类问题集未隔离: %v", err)
	}
	if banned := aeo.ValidateNoInducement(append(market, tracking...)); len(banned) > 0 {
		addIssue("L2 问题集含诱导性表述: %v", banned)
	}
	l2 := LayerReport{
		Layer: "L2", Name: "问题集与档案",
		Metrics: map[string]float64{
			"market_questions":   float64(len(market)),
			"tracking_questions": float64(len(tracking)),
		},
		EvidenceIDs: []string{}, Drilldowns: []*domain.DrilldownPath{},
		Provenance: domain.ObservedProvenance(len(market) + len(tracking)),
	}
	for _, q := range append(append([]domain.GeneratedQuestion{}, market...), tracking...) {
		l2.EvidenceIDs = append(l2.EvidenceIDs, q.ID)
	}
	if len(l2.EvidenceIDs) == 0 {
		addIssue("L2 没有可下钻的问题 ID")
	}
	rep.Layers = append(rep.Layers, l2)

	// ---- L3 修复件与优先级清单 ----
	artifacts := siteaudit.GenerateArtifacts(in.Brand)
	fixes := siteaudit.FixList(&audit, visible)
	l3 := LayerReport{
		Layer: "L3", Name: "修复件与优先级清单",
		Metrics: map[string]float64{
			"artifacts": float64(len(artifacts)), "fix_items": float64(len(fixes)),
		},
		EvidenceIDs: []string{}, Drilldowns: []*domain.DrilldownPath{},
		Provenance: domain.ObservedProvenance(len(artifacts) + len(fixes)),
	}
	for _, a := range artifacts {
		l3.EvidenceIDs = append(l3.EvidenceIDs, a.TargetPath)
		if a.TargetPath == "" || a.Validation == "" || a.Content == "" {
			addIssue("L3 修复件 %s 缺少放置路径/验证方式/内容", a.Filename)
		}
	}
	for _, f := range fixes {
		l3.EvidenceIDs = append(l3.EvidenceIDs, f.TargetMetric)
	}
	if len(artifacts) == 0 {
		addIssue("L3 未生成任何修复件")
	}
	rep.Layers = append(rep.Layers, l3)

	// ---- 治理 / 配额 / 监控接进同一链路 ----
	if in.Gov != nil {
		if _, err := in.Gov.Dispatch(governance.RunRequest{
			Operator: "regression", Task: domain.TaskCollect, Change: "full-chain-L0-L3",
		}); err != nil {
			addIssue("治理路由拒绝运行: %v", err)
		}
		for _, s := range in.Gov.ByOperator() {
			if s.Operator == "regression" {
				rep.Governance = s
			}
		}
	}
	if in.Bill != nil {
		// 用增量而非绝对值断言，保证脚本可重复执行（可重放）。
		before := in.Bill.Usage(in.AccountID).MonthlyUsed
		if err := in.Bill.Consume(in.AccountID, len(l1.EvidenceIDs), false); err != nil {
			addIssue("配额扣减失败: %v", err)
		}
		_ = in.Bill.Consume(in.AccountID, visible.Failed, true) // 失败运行不计
		rep.Usage = in.Bill.Usage(in.AccountID)
		if after := rep.Usage.MonthlyUsed - before; after != len(l1.EvidenceIDs) {
			addIssue("配额增量 %d 与 L1 有效 run 数 %d 不一致", after, len(l1.EvidenceIDs))
		}
	}
	if in.Mon != nil {
		in.Mon.Update(monitor.Sources{
			CollectionValid: visible.Usable, CollectionTotal: visible.Planned,
			DailyCostUSD: totalCost(in.Answers),
			BudgetTrips:  budgetTrips(in.Gov),
		})
		rep.Monitor = in.Mon.Snapshot()
	}

	rep.Passed = len(rep.Issues) == 0
	return rep
}

type evidence struct{ runID, rawAnswer, pageURL string }

func firstEvidence(answers []domain.RunAnswer) evidence {
	for _, a := range answers {
		if !a.Valid || a.RawAnswerText == "" {
			continue
		}
		url := "https://example.com/public-review"
		if len(a.Citations) > 0 && a.Citations[0].URL != "" {
			url = a.Citations[0].URL
		}
		return evidence{runID: a.RunID, rawAnswer: a.RawAnswerText, pageURL: url}
	}
	return evidence{}
}

func totalCost(answers []domain.RunAnswer) float64 {
	var sum float64
	for _, a := range answers {
		sum += a.Cost
	}
	return sum
}

func budgetTrips(g *governance.Engine) int {
	if g == nil {
		return 0
	}
	return g.Budget().TrippedEvents
}

// CollectorAnswers 把采集框架落库记录转成指标层输入，供集成脚本复用。
func CollectorAnswers(f *collector.Framework) []domain.RunAnswer {
	out := []domain.RunAnswer{}
	for _, r := range f.Runs() {
		out = append(out, r.Answer)
	}
	return out
}
