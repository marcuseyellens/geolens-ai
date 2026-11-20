package aeo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/answerlens/answerlens/domain"
)

// 本文件把《WBS_AEO_Feature_Extraction_v1.0》§8 的 10 条增量发布验收清单
// 变成可执行的 CI 断言。每条测试都标注对应清单原文。

func repoRoot(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func readRepo(t *testing.T, parts ...string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(append([]string{repoRoot(t)}, parts...)...))
	if err != nil {
		t.Fatalf("读取 %v 失败：%v", parts, err)
	}
	return string(b)
}

// 清单 1：任一含数字的组件可查 source_type 与 methodology_version；三类来源不同表、不合并。
func TestAcceptance01_ProvenanceAndSourceSeparation(t *testing.T) {
	if err := ValidateNumericComponent("metric", domain.ObservedProvenance(12)); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]domain.Provenance{
		"no_source":  {MethodologyVersion: domain.MethodologyVersion, NValid: 1},
		"no_version": {SourceType: domain.SourceObserved, NValid: 1},
	} {
		if err := ValidateNumericComponent(name, p); err == nil {
			t.Errorf("%s 应被拒绝", name)
		}
	}
	mixed := []domain.SourceType{domain.SourceObserved, domain.SourceFirstParty}
	if err := ValidateSourceSeparation("report_table", mixed); err == nil {
		t.Error("三类来源同表必须报错")
	}
	if err := ValidateSourceSeparation("report_table", []domain.SourceType{domain.SourceObserved, domain.SourceObserved}); err != nil {
		t.Errorf("同类来源同表应允许：%v", err)
	}
	tables := SplitBySourceType([]domain.Provenance{
		domain.Provenance{SourceType: domain.SourceFirstParty},
		domain.Provenance{SourceType: domain.SourceObserved},
		domain.Provenance{SourceType: domain.SourceReported},
	})
	if len(tables) != 4 || len(tables[domain.SourceFirstParty]) != 1 || len(tables[domain.SourceUnknown]) != 0 {
		t.Errorf("三类来源必须分表：%+v", tables)
	}
	// 全量 API schema 不得含 overall_score
	for _, f := range []string{"domain/contract.go", "domain/models.go", "domain/aeo_contract.go", "frontend/src/types.ts"} {
		if strings.Contains(strings.ToLower(readRepo(t, f)), "overall_score") {
			t.Errorf("%s 出现 overall_score", f)
		}
	}
}

// 清单 2：unknown 档案状态不扣分；unknown 索引准入项不进分母。
func TestAcceptance02_UnknownNeverPenalized(t *testing.T) {
	base := 88.0
	for _, s := range []domain.ProfileStatus{domain.ProfileUnknown, domain.ProfileClaimed, domain.ProfileConflict} {
		if got := ScoreWithProfile(base, s); got != base {
			t.Errorf("档案状态 %s 改变了分数：%.1f", s, got)
		}
	}
	checks := []domain.AdmissionCheck{
		{Detector: domain.DetectThinContent, Status: domain.AdmissionPass},
		{Detector: domain.DetectCaptcha, Status: domain.AdmissionUnknown},
		{Detector: domain.DetectAntiBotWall, Status: domain.AdmissionUnknown},
	}
	pass, partial, fail, unknown := AdmissionDenominator(checks)
	if pass != 1 || partial != 0 || fail != 0 || unknown != 2 {
		t.Fatalf("unknown 未排除出分母：%d/%d/%d/%d", pass, partial, fail, unknown)
	}
	earned, scored := AdmissionGroupScore(checks, 10)
	if earned != 10 || scored != 10 {
		t.Errorf("unknown 项扣分了：earned=%.1f scored=%.1f", earned, scored)
	}
	allUnknown := []domain.AdmissionCheck{{Status: domain.AdmissionUnknown}}
	if earned, scored := AdmissionGroupScore(allUnknown, 10); earned != 0 || scored != 0 {
		t.Errorf("全 unknown 组应不参与评分：%.1f/%.1f", earned, scored)
	}
}

// 清单 3：8 类页面样本检出 8/8 类、误报 ≤ 2/24；SPA 无误判。
func TestAcceptance03_EightDetectorsWith24Samples(t *testing.T) {
	samples := AdmissionSamples()
	if len(samples) != 24 {
		t.Fatalf("样本数应为 24，实际 %d", len(samples))
	}
	detected, falsePositives, spaFalsePositive := AdmissionRegression()
	if detected != 8 {
		t.Errorf("仅检出 %d/8 类", detected)
	}
	if falsePositives > 2 {
		t.Errorf("误报 %d > 2/24", falsePositives)
	}
	if spaFalsePositive {
		t.Error("SPA 被误判为 thin_content")
	}
	perDetector := map[domain.AdmissionDetector]int{}
	for _, s := range samples {
		perDetector[s.Expect]++
	}
	for d, n := range perDetector {
		if n != 3 {
			t.Errorf("%s 样本数 %d ≠ 3", d, n)
		}
	}
}

// 清单 3 附加：SPA 的 thin_content 必须返回 unknown 而非 fail。
func TestAcceptance03_SPANotThinContent(t *testing.T) {
	spa := PageSnapshot{
		URL: "https://example.com", Status: 200, Rendered: false,
		Body: `<html><head></head><body><div id="root"></div><script type="module" src="/app.js"></script></body></html>`,
	}
	for _, c := range InspectAdmission(spa, DefaultAdmissionConfig()) {
		if c.Detector == domain.DetectThinContent {
			if c.Status == domain.AdmissionFail {
				t.Fatal("SPA 被误判为 thin_content")
			}
			if c.Status != domain.AdmissionUnknown {
				t.Errorf("SPA 应返回 unknown，实际 %s", c.Status)
			}
		}
	}
}

const goodPageHTML = GoodPageHTML

// 清单 4：全部页面质量结论带「风险提示」+ 来源链接，无确定性收录表述。
func TestAcceptance04_RiskNoticeAndNoDeterministicClaims(t *testing.T) {
	checks := InspectAdmission(PageSnapshot{URL: "https://example.com", Status: 200, Rendered: true, Body: goodPageHTML}, DefaultAdmissionConfig())
	for _, c := range checks {
		if c.RiskNotice == "" {
			t.Errorf("%s 缺风险提示", c.Detector)
		}
		if c.SourceURL == "" {
			t.Errorf("%s 缺来源链接", c.Detector)
		}
		if hits := ScanTextForBannedPhrases(c.Evidence + c.Fix); len(hits) > 0 {
			t.Errorf("%s 含确定性表述：%v", c.Detector, hits)
		}
	}
	config := DefaultAdmissionConfig()
	config.Disabled[domain.DetectThinContent] = true
	for _, c := range InspectAdmission(PageSnapshot{URL: "https://example.com", Status: 200, Body: goodPageHTML}, config) {
		if c.Detector == domain.DetectThinContent && c.Status != domain.AdmissionUnknown {
			t.Error("单配置下线整组失败")
		}
	}
	config.Enabled = false
	for _, c := range InspectAdmission(PageSnapshot{URL: "https://example.com", Status: 200, Body: goodPageHTML}, config) {
		if c.Status != domain.AdmissionUnknown {
			t.Errorf("整组下线后 %s 仍给出结论", c.Detector)
		}
	}
}

// 清单 5：注入真实突变 / 季节性 / 采集故障，仅真实突变告警；失败批次不进基线。
func TestAcceptance05_OnlyRealShiftAlerts(t *testing.T) {
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	b := NewRollingBaseline(func() time.Time { return now })
	key := SeriesKey{BrandID: "b1", Model: domain.ModelChatGPT, Region: "us", Metric: "mention_rate"}
	for i := 0; i < 10; i++ {
		b.Record(key, domain.BaselinePoint{
			BatchID: fmt.Sprintf("b%d", i), ObservedOn: now.AddDate(0, 0, -10+i).Format("2006-01-02"),
			Value: 0.50, Valid: true, RunIDs: []string{"r1"},
		})
	}
	// 失败批次必须被排除
	if b.Record(key, domain.BaselinePoint{Valid: false, Value: 999, ObservedOn: now.Format("2006-01-02")}) {
		t.Error("失败批次进入了基线")
	}
	cfg := DefaultAnomalyConfig()
	pending := map[string]int{}
	// 季节性不告警
	if kind, alert := ClassifyShift(b, AnomalyInput{Key: key, Point: domain.BaselinePoint{Valid: true, Value: 0.9}, Seasonal: true}, cfg, pending); kind != domain.AnomalySeasonal || alert {
		t.Errorf("季节性应识别且不告警：%s/%v", kind, alert)
	}
	// 采集故障不告警
	if kind, alert := ClassifyShift(b, AnomalyInput{Key: key, Point: domain.BaselinePoint{Valid: true, Value: 0.9}, FaultRate: 0.4}, cfg, pending); kind != domain.AnomalyCollectFault || alert {
		t.Errorf("采集故障应识别且不告警：%s/%v", kind, alert)
	}
	// 真实突变需连续 2 批确认
	if _, alert := ClassifyShift(b, AnomalyInput{Key: key, Point: domain.BaselinePoint{Valid: true, Value: 0.95}}, cfg, pending); alert {
		t.Error("第 1 批不应告警")
	}
	if kind, alert := ClassifyShift(b, AnomalyInput{Key: key, Point: domain.BaselinePoint{Valid: true, Value: 0.95}}, cfg, pending); kind != domain.AnomalyRealShift || !alert {
		t.Errorf("第 2 批应确认为真实突变并告警：%s/%v", kind, alert)
	}
	// 样本 <6 不出告警
	sparse := NewRollingBaseline(func() time.Time { return now })
	for i := 0; i < 5; i++ {
		sparse.Record(key, domain.BaselinePoint{Valid: true, Value: 0.5, ObservedOn: now.AddDate(0, 0, -i).Format("2006-01-02")})
	}
	if _, _, ready := sparse.Stats(key); ready {
		t.Error("样本 <6 不应 ready")
	}

	store := NewAlertStore()
	for i := 0; i < 4; i++ {
		store.Emit(key, domain.AnomalyRealShift, 0.45, []string{"run-1"}, "2026-11-14", 10)
	}
	alerts := store.All()
	if !alerts[3].Folded || alerts[2].Folded {
		t.Error("第 4 条告警应自动折叠")
	}
	if errs := ValidateAlerts(alerts); len(errs) > 0 {
		t.Errorf("告警证据校验失败：%v", errs)
	}
	if !strings.Contains(store.ExportCSV(), "run-1") {
		t.Error("CSV 未包含 run_id")
	}
	w := NewWebhookDelivery()
	if delivered, dup := w.Deliver("evt-1"); !delivered || dup {
		t.Error("首次投递失败")
	}
	if delivered, dup := w.Deliver("evt-1"); delivered || !dup {
		t.Error("Webhook 重放应幂等")
	}
}

// 清单 6：抽 10 个聚合数字 10/10 可在 ≤2 次点击内下钻到原文与页面 URL。
func TestAcceptance06_TenAggregatesDrilldown(t *testing.T) {
	paths := make([]*domain.DrilldownPath, 0, 10)
	for i := 0; i < 10; i++ {
		paths = append(paths, BuildDrilldown(fmt.Sprintf("metric_%d", i), float64(i)/10, 12,
			fmt.Sprintf("run-%d", i), "原始答案文本", "https://example.com/page"))
	}
	if errs := ValidateDrilldowns(paths); len(errs) > 0 {
		t.Errorf("下钻校验失败：%v", errs)
	}
	// 无证据的 Insight 必须不生成
	if p := BuildDrilldown("no_evidence", 0.5, 0, "", "", ""); p != nil {
		t.Error("无证据 Insight 不应生成")
	}
	if p := BuildDrilldown("no_url", 0.5, 3, "run-1", "答案", ""); p != nil {
		t.Error("缺页面 URL 的下钻路径不应生成")
	}
}

// 清单 7：代码中无 Reddit 或任何来源占比常量；任一占比伴随 n_valid。
func TestAcceptance07_NoRatioConstantsAndNValidAlways(t *testing.T) {
	root := repoRoot(t)
	// 与运行时验收清单共用同一扫描器：只检查产品代码的字符串字面量，
	// 注释、测试文件与合规检查器自身都不算「内置站点占比常量」。
	bad, err := BannedDomainConstants(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range bad {
		t.Errorf("字符串常量出现具体站点：%s", v)
	}
	records := []CitationEconomyRecord{
		{Model: domain.ModelChatGPT, Region: "us", Intent: domain.IntentRecommend, Window: "2026-W45", Economy: domain.EconomyEditorial, RunID: "r1", Valid: true},
		{Model: domain.ModelChatGPT, Region: "us", Intent: domain.IntentRecommend, Window: "2026-W45", Economy: domain.EconomyOwned, RunID: "r2", Valid: true},
		{Model: domain.ModelClaude, Region: "us", Intent: domain.IntentRecommend, Window: "2026-W45", Economy: domain.EconomyUnknown, RunID: "r3", Valid: true},
		{Model: domain.ModelClaude, Region: "us", Intent: domain.IntentRecommend, Window: "2026-W45", Economy: domain.EconomyPaid, RunID: "r4", Valid: false},
	}
	slices := SourceStructure(records)
	if errs := ValidateSlices(slices); len(errs) > 0 {
		t.Errorf("占比口径校验失败：%v", errs)
	}
	for _, s := range slices {
		if s.Economy == domain.EconomyUnknown {
			t.Error("unknown 不得进入聚合占比")
		}
		if s.NValid != 2 {
			t.Errorf("失效/未知样本误入分母：n_valid=%d", s.NValid)
		}
	}
	// 切换模型图应随之变化
	byModel := map[domain.AIModelID]bool{}
	for _, s := range slices {
		byModel[s.Model] = true
	}
	if len(byModel) != 1 || !byModel[domain.ModelChatGPT] {
		t.Error("分模型切片未按模型区分")
	}
	if SourceEconomyFor("unknown-annotation") != domain.EconomyUnknown {
		t.Error("无法判定应返回 unknown")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// 清单 8：用自家产品对自家站点跑 A-01 / A-03 / A-05 / A-14 四项自检全绿。
func TestAcceptance08_FourSelfChecksGreen(t *testing.T) {
	home := readRepo(t, "frontend", "index.html")
	robots := readRepo(t, "frontend", "public", "robots.txt")
	llms := readRepo(t, "frontend", "public", "llms.txt")
	sitemap := readRepo(t, "frontend", "public", "sitemap.xml")
	pricing := readRepo(t, "frontend", "src", "routes", "pricing.tsx")
	admission := InspectAdmission(PageSnapshot{
		URL: "https://geolens.ai", Status: 200, Rendered: true, Body: home,
	}, DefaultAdmissionConfig())
	agent := InspectAgentReadability(AgentAssets{
		LLMsTxt: llms, MachinePrice: strings.Contains(home, `"price"`),
		HumanGate: true, Body: home,
	})
	results := RunSelfChecks(SelfCheckInput{
		HomeHTML: home, RobotsTxt: robots, LLMsTxt: llms, SitemapXML: sitemap,
		JSONLDPresent: strings.Contains(home, "application/ld+json"),
		Profile:       domain.NewBusinessProfile("geolens"),
		Admission:     admission, AgentChecks: agent,
		NeutralityInLanding: NeutralityVisible(
			readRepo(t, "frontend", "src", "routes", "landing.tsx") +
				readRepo(t, "frontend", "src", "lib", "neutrality.ts")),
		NeutralityInReport: NeutralityVisible(strings.Join(ReportMethodologyNotes(), " ")),
		PricingText:        pricing,
	})
	for _, r := range results {
		if !r.Passed {
			t.Errorf("自检 %s 未通过：%v", r.ID, r.Details)
		}
	}
	if !AllSelfChecksPassed(results) {
		t.Error("四项自检未全绿")
	}
}

// 清单 9：落地页与报告 PDF 均可见中立性声明；全站禁词 CI 通过。
func TestAcceptance09_NeutralityAndBannedWords(t *testing.T) {
	root := repoRoot(t)
	violations, err := ScanRepoForBannedPhrases(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range violations {
		t.Errorf("禁词命中：%s", v)
	}
	// 声明抽到共享模块后，落地页通过 import 使用；两者合起来才算「首屏可见」。
	landing := readRepo(t, "frontend", "src", "routes", "landing.tsx") +
		readRepo(t, "frontend", "src", "lib", "neutrality.ts")
	if !NeutralityVisible(landing) {
		t.Error("落地页缺少中立性声明")
	}
	if !strings.Contains(readRepo(t, "frontend", "src", "routes", "landing.tsx"), "NEUTRALITY") {
		t.Error("落地页未渲染中立性声明")
	}
	notes := strings.Join(ReportMethodologyNotes(), " ")
	if !NeutralityVisible(notes) {
		t.Error("报告口径说明缺少中立性声明")
	}
	if !strings.Contains(notes, "仅代表本次采样") {
		t.Error("报告缺少「仅代表本次采样」")
	}
}

// 清单 10：套餐结构未因本次提取改变（Free / $39 / $19 / $49·199·410）。
func TestAcceptance10_PricingStructureUnchanged(t *testing.T) {
	want := map[string]float64{
		"free": 0, "one_time": 39, "rescan": 19,
		"starter": 49, "growth": 199, "scale": 410,
	}
	if len(domain.PlanPrices) != len(want) {
		t.Errorf("套餐数量变化：%d", len(domain.PlanPrices))
	}
	for k, v := range want {
		if domain.PlanPrices[k] != v {
			t.Errorf("%s 价格变化：%.0f → %.0f", k, v, domain.PlanPrices[k])
		}
	}
	pricing := readRepo(t, "frontend", "src", "routes", "pricing.tsx")
	for _, p := range []string{"$0", "$39", "$19", "$49", "$199", "$410"} {
		if !strings.Contains(pricing, p) {
			t.Errorf("定价页缺少 %s", p)
		}
	}
}

// 清单运行时求值必须与 CI 断言一致：API 不能硬编码「已通过」。
func TestEvaluateChecklistAllGreen(t *testing.T) {
	root := repoRoot(t)
	items := EvaluateChecklist(ChecklistEvidence{
		RepoRoot:     root,
		FrontendDist: filepath.Join(root, "frontend", "dist"),
		FrontendSrc:  filepath.Join(root, "frontend"),
	})
	if len(items) != 10 {
		t.Fatalf("验收清单应有 10 条，实际 %d", len(items))
	}
	for _, it := range items {
		if !it.Passed {
			t.Errorf("清单 %d 未通过：%v", it.ID, it.Evidence)
		}
	}
}

// ---- 其余工作包的口径回归 ----

func TestAWp1_ProfileDiffTypes(t *testing.T) {
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	profile := domain.BusinessProfile{Status: domain.ProfileClaimed, Fields: map[string]string{
		domain.FieldName: "GeoLens", domain.FieldWebsite: "https://geolens.ai",
		domain.FieldPhone: "+1 555 0100", domain.FieldCategory: "AEO",
	}}
	snap := ThirdPartySnapshot{
		Fields: map[string]string{
			domain.FieldName: "GeoLens AI", domain.FieldWebsite: "https://geolens.ai/",
			domain.FieldPhone: "15550100", domain.FieldAddress: "Public St",
		},
		Excerpts:   map[string]string{domain.FieldName: "页面标题显示 GeoLens AI"},
		ObservedAt: now.AddDate(0, -6, 0), StaleAfter: 30 * 24 * time.Hour,
	}
	diffs := CompareProfiles(profile, snap, now, "google")
	kinds := map[domain.DiffKind]int{}
	for _, d := range diffs {
		kinds[d.Kind]++
		if d.OfficialURL == "" {
			t.Errorf("%s 缺官方链接", d.Field)
		}
	}
	for _, k := range []domain.DiffKind{domain.DiffMissing, domain.DiffConflict, domain.DiffStale} {
		if kinds[k] == 0 {
			t.Errorf("差异类型 %s 未检出", k)
		}
	}
	artifact := ProfileSyncChecklist("GeoLens", diffs)
	if !strings.Contains(artifact.Content, "不代认领") || !strings.Contains(artifact.Content, "business.google.com") {
		t.Error("认领修复件缺官方入口或免责说明")
	}
	// 10 组人工样本检出 ≥9 组
	hit := 0
	for i := 0; i < 10; i++ {
		s := snap
		s.Fields = map[string]string{domain.FieldName: fmt.Sprintf("变体 %d", i)}
		if len(CompareProfiles(profile, s, now, "google")) > 0 {
			hit++
		}
	}
	if hit < 9 {
		t.Errorf("10 组样本仅检出 %d 组", hit)
	}
	if got := ProfileStatusFor(profile, diffs); got != domain.ProfileConflict {
		t.Errorf("冲突状态推导错误：%s", got)
	}
}

func TestAWp3_DistinctThresholdsAndStableSuspect(t *testing.T) {
	table := DefaultThresholdTable()
	if errs := table.ValidateDistinctDevSets(); len(errs) > 0 {
		t.Errorf("分任务开发集不独立：%v", errs)
	}
	all := table.All()
	if len(all) != 5 {
		t.Fatalf("应有 5 类判定阈值，实际 %d", len(all))
	}
	for _, a := range all {
		for _, b := range all {
			if a.Task != b.Task && a.Cutoff == b.Cutoff {
				t.Errorf("%s 与 %s 共用 cutoff", a.Task, b.Task)
			}
		}
	}
	first := ScoreSuspect(domain.TaskMention, 0.6)
	for i := 0; i < 20; i++ {
		if got := ScoreSuspect(domain.TaskMention, 0.6); got.Score != first.Score {
			t.Fatal("重复打分方差不为 0")
		}
	}
	if !first.Suspect {
		t.Error("低置信度应标 suspect")
	}
}

func TestAWp5To8_QuestionsCostAndJSONSchema(t *testing.T) {
	brief := domain.QuestionBrief{
		Category: "AEO 平台", Audience: "增长团队", Need: "评估 AI 可见性",
		Region: "us", Locale: "en-US", Constraint: "预算有限",
	}
	if err := ValidateBrief(domain.QuestionBrief{}); err == nil {
		t.Error("缺要素应拒绝生成")
	}
	quota := domain.DefaultIntentQuota()
	market, err := GenerateQuestionSet(brief, domain.SetMarketBaseline, quota, 40)
	if err != nil {
		t.Fatal(err)
	}
	tracking, err := GenerateQuestionSet(brief, domain.SetBrandTracking, quota, 40)
	if err != nil {
		t.Fatal(err)
	}
	if len(market) < 40 || len(tracking) < 40 {
		t.Errorf("题量不足：%d / %d", len(market), len(tracking))
	}
	if err := SeparatedSets(market, tracking); err != nil {
		t.Errorf("两套问题集未分离：%v", err)
	}
	intents := map[domain.QueryIntent]bool{}
	for _, q := range market {
		intents[q.Intent] = true
	}
	if len(intents) != 4 {
		t.Errorf("四类意图未覆盖：%v", intents)
	}
	if errs := ValidateNoInducement(market[:20]); len(errs) > 0 {
		t.Errorf("含诱导式点名：%v", errs)
	}
	cost := ComputeCost(domain.CostPerRequest, 2, 0, 3, DefaultCostRates())
	if delta, ok := ReconcileCost(cost, cost.Total, 1); !ok || delta != 0 {
		t.Error("成本对账有差额")
	}
	if cost.ModelCallCost == 0 || cost.RetrievalCost == 0 || cost.Total <= 0 {
		t.Error("cost 未拆成三项")
	}
	perSecond := ComputeCost(domain.CostPerSecond, 1, 12.5, 0, DefaultCostRates())
	if perSecond.SandboxSeconds != 12.5 || perSecond.SandboxCost <= 0 {
		t.Error("按秒计费未归集")
	}
	// JSON schema 强制字段存在
	b, _ := json.Marshal(domain.ObservedProvenance(3))
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	for _, k := range []string{"source_type", "methodology_version", "n_valid"} {
		if _, ok := m[k]; !ok {
			t.Errorf("Provenance schema 缺 %s", k)
		}
	}
}
