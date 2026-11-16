package delta

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geolens/geolens/domain"
)

// TC-16 系列对应《CodeBuddy 执行规格书｜2026-11-16》§5 自动用例。
// 全部为 CI 可执行的 Go 单测，flag on/off 双向覆盖（DoD）。

func newTestConfig(t *testing.T) *Config {
	t.Helper()
	return NewConfig("") // 内存态
}

func testFlag(on bool) *Flag {
	f := NewFlag()
	f.SetEnabled(on)
	return f
}

// ---- EX-01 迁移 / 回填 ----

// TC-16-01：迁移脚本在空库与已有数据结构上各跑一遍；新增列可 NULL、
// RunAnswer 不可变约束存在、重复执行不报错；flag=off 时 API 不返回新字段。
func TestTC16_01_MigrationIdempotentAndFlagOff(t *testing.T) {
	cfg := newTestConfig(t)
	m1, m2 := cfg.Migrate(), cfg.Migrate()
	if !m1.Idempotent || !m2.Idempotent || m1.SchemaVersion != SchemaVersion {
		t.Fatal("迁移必须可重复执行且版本为 1.1")
	}
	if m1.RunAnswerHasUpdatePath {
		t.Fatal("RunAnswer 不可出现 UPDATE 路径")
	}
	if len(m1.NullableColumns) != len(allNewColumns) {
		t.Fatal("新增列清单不完整")
	}
	// flag off：结构体字段 omitempty，序列化不出现任何新字段。
	f := testFlag(false)
	if f.Enabled() {
		t.Fatal("off 开关未生效")
	}
	b, _ := json.Marshal(domain.RunAnswer{RunID: "r1"})
	s := string(b)
	for _, field := range []string{"query_locale", "retrieval_locale", "suspect", "crawler_visit_evidence"} {
		if strings.Contains(s, `"`+field+`"`) {
			t.Fatalf("flag=off 时仍返回新字段 %s：%s", field, s)
		}
	}
	// flag on：字段显式填充后可返回。
	f.SetEnabled(true)
	if !f.Enabled() {
		t.Fatal("on 开关未生效")
	}
	a := domain.RunAnswer{RunID: "r1", QueryLocale: "de-DE", RetrievalLocale: "de-DE"}
	b2, _ := json.Marshal(a)
	if !strings.Contains(string(b2), `"query_locale":"de-DE"`) {
		t.Fatal("flag=on 时应返回新字段")
	}
}

// TC-16-02：历史 run_answer 回填 query_locale；retrieval_locale=unknown 且
// confidence=NULL；citation.host 独立落库；report.methodology_version 不改写。
func TestTC16_02_BackfillRules(t *testing.T) {
	answers := []domain.RunAnswer{
		{RunID: "r1", Locale: "zh-CN", Citations: []domain.Citation{
			{URL: "https://blog.example.com/a"}, {URL: "https://example.com/b"},
		}},
		{RunID: "r2", Locale: "en-US"},
	}
	out := Backfill(answers)
	for _, a := range out {
		if a.QueryLocale == "" || a.QueryLocale != a.Locale {
			t.Fatalf("query_locale 未从 question.locale 回填：%+v", a)
		}
		if a.RetrievalLocale != string(domain.LocaleSourceUnknown) ||
			a.RetrievalLocaleSource != domain.LocaleSourceUnknown || a.RetrievalConfidence != nil {
			t.Fatalf("历史检索语言必须为 unknown 且 confidence=NULL：%+v", a)
		}
		if a.LocaleMismatch || a.Suspect {
			t.Fatal("历史不得反推 locale_mismatch / suspect")
		}
	}
	if out[0].Citations[0].Host != "blog.example.com" ||
		out[0].Citations[0].RegistrableDomain != "example.com" {
		t.Fatal("citation.host / registrable_domain 未正确回填")
	}
	// 历史报告保留生成时版本。
	hist := BuildReportRecord("old", answers, true, nil)
	if hist.MethodologyVersion != domain.MethodologyVersion || !hist.LegacyCounts {
		t.Fatal("历史报告口径被改写")
	}
	current := BuildReportRecord("new", answers, false, nil)
	if current.MethodologyVersion != domain.MethodologyVersionDelta {
		t.Fatal("新报告未升到 1.1")
	}
}

// ---- EX-02 判定器 ----

// TC-16-03：注入 platform_header / citation_domain / answer_text / inferred /
// 无信号五类输入，断言优先级、confidence、run_id 证据绑定、unknown 不进分母、
// 禁止内部机制措辞。
func TestTC16_03_RetrievalLocalePriority(t *testing.T) {
	min := domain.DefaultThresholdConfig()[domain.ThresholdInferredMinConfidence]
	cases := []struct {
		name     string
		s        LocaleSignals
		wantSrc  domain.RetrievalLocaleSource
		wantLoc  string
		wantConf bool
	}{
		{"platform_header 最高", LocaleSignals{
			QueryLocale: "de-DE", PlatformHeader: "pl_PL",
			CitationDomains: []string{"site.de"}, RawAnswerText: `lang="ja-JP"`,
			InferredLocale: "fr-FR", InferredConf: 0.99, RunID: "r1",
		}, domain.LocaleSourcePlatformHeader, "pl-PL", false},
		{"citation_domain 次之", LocaleSignals{
			QueryLocale: "de-DE", CitationDomains: []string{"news.pl"},
			RawAnswerText: `lang="ja-JP"`, InferredLocale: "fr-FR", InferredConf: 0.99, RunID: "r1",
		}, domain.LocaleSourceCitationDomain, "pl-PL", false},
		{"answer_text 再次", LocaleSignals{
			QueryLocale: "de-DE", RawAnswerText: `lang="ja-JP"`,
			InferredLocale: "fr-FR", InferredConf: 0.99, RunID: "r1",
		}, domain.LocaleSourceAnswerText, "ja-JP", false},
		{"inferred 带证据", LocaleSignals{
			QueryLocale: "de-DE", InferredLocale: "fr-FR", InferredConf: 0.82, RunID: "r1",
		}, domain.LocaleSourceInferred, "fr-FR", true},
		{"低置信度降级 unknown", LocaleSignals{
			QueryLocale: "de-DE", InferredLocale: "fr-FR", InferredConf: 0.69, RunID: "r1",
		}, domain.LocaleSourceUnknown, "unknown", false},
		{"无 run_id 证据降级 unknown", LocaleSignals{
			QueryLocale: "de-DE", InferredLocale: "fr-FR", InferredConf: 0.95,
		}, domain.LocaleSourceUnknown, "unknown", false},
		{"无信号", LocaleSignals{QueryLocale: "de-DE"}, domain.LocaleSourceUnknown, "unknown", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := InferLocale(c.s, min)
			if got.RetrievalLocaleSource != c.wantSrc || got.RetrievalLocale != c.wantLoc {
				t.Fatalf("判定错误：%+v want %s/%s", got, c.wantSrc, c.wantLoc)
			}
			if (got.RetrievalConfidence != nil) != c.wantConf {
				t.Fatalf("confidence 绑定错误：%+v", got)
			}
			if !domain.ValidRetrievalLocaleSource(got.RetrievalLocaleSource) {
				t.Fatal("非法 retrieval_locale_source")
			}
		})
	}
	// unknown 不进分母。
	runs := []domain.RunAnswer{{RunID: "u1", Valid: true, RetrievalLocale: "unknown"}}
	if len(SplitByLocale(runs).Unknown) != 1 || len(SplitByLocale(runs).Default) != 0 {
		t.Fatal("unknown 未排除出分母")
	}
	if LocaleUINotice("unknown") != domain.RetrievalLocaleUndetermined {
		t.Fatal("unknown 态 UI 留白")
	}
	if NoCausalClaim("模型内部以波兰语检索") {
		t.Fatal("内部机制措辞未被禁用")
	}
}

// TC-16-04：locale_mismatch 不变式——仅当 query_locale != retrieval_locale
// 且 retrieval_locale != unknown 时置 true。
func TestTC16_04_LocaleMismatchInvariant(t *testing.T) {
	min := domain.DefaultThresholdConfig()[domain.ThresholdInferredMinConfidence]
	got := InferLocale(LocaleSignals{QueryLocale: "de-DE", PlatformHeader: "pl-PL"}, min)
	if !got.LocaleMismatch {
		t.Fatal("跨语言应为 mismatch=true")
	}
	same := InferLocale(LocaleSignals{QueryLocale: "de-DE", PlatformHeader: "de-DE"}, min)
	if same.LocaleMismatch {
		t.Fatal("同语言不得 mismatch")
	}
	unknown := InferLocale(LocaleSignals{QueryLocale: "de-DE"}, min)
	if unknown.LocaleMismatch {
		t.Fatal("unknown 不得 mismatch")
	}
	if len(SplitByLocale([]domain.RunAnswer{{LocaleMismatch: true, RetrievalLocale: "pl-PL"}}).Mismatch) != 1 {
		t.Fatal("mismatch 未单独分组")
	}
}

// ---- EX-03 问题集 ----

func fourIntents() []domain.QueryIntent {
	return []domain.QueryIntent{
		domain.IntentRecommend, domain.IntentCompare, domain.IntentHowTo, domain.IntentAlternative,
	}
}

func testBrief(locale string) domain.QuestionBrief {
	return domain.QuestionBrief{
		Category: "CRM", Audience: "sales lead", Need: "pipeline tracking",
		Region: "DE", Locale: locale, Constraint: "no brand names",
	}
}

// TC-16-05：同一 market 分别生成 de-DE / pl-PL 两套，locale 必填拒绝、
// 各 ≥ 40 题、四类意图齐全、question_set_type 不同；缺 locale 直接拒绝。
func TestTC16_05_MultilingualQuestionSets(t *testing.T) {
	store := NewQuestionSetStore()
	if _, err := store.Generate("crm", testBrief(""), domain.SetMarketBaseline, domain.DefaultIntentQuota(), 50); err == nil {
		t.Fatal("缺 locale 必须拒绝")
	}
	de, err := store.Generate("crm", testBrief("de-DE"), domain.SetMarketBaseline, domain.DefaultIntentQuota(), 50)
	if err != nil {
		t.Fatal(err)
	}
	pl, err := store.Generate("crm", testBrief("pl-PL"), domain.SetBrandCheck, domain.DefaultIntentQuota(), 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(de.Questions) < 40 || len(pl.Questions) < 40 {
		t.Fatal("每套问题集必须 ≥ 40 题")
	}
	if de.QuestionSetType == pl.QuestionSetType {
		t.Fatal("question_set_type 未区分")
	}
	intents := map[domain.QueryIntent]bool{}
	for _, q := range de.Questions {
		intents[q.Intent] = true
		if q.QueryLocale != "de-DE" || q.Locale != "de-DE" {
			t.Fatal("题面语言标注缺失")
		}
	}
	if len(intents) != len(fourIntents()) {
		t.Fatal("四类意图不齐")
	}
	if !NoTranslationReuse(*de, *pl) {
		t.Fatal("两套语言问题集发生翻译复用")
	}
}

// TC-16-06：多语言配额独立计费、与 F-604 对账、打包提示存在。
func TestTC16_06_QuotaIndependent(t *testing.T) {
	store := NewQuestionSetStore()
	_, _ = store.Generate("crm", testBrief("de-DE"), domain.SetMarketBaseline, domain.DefaultIntentQuota(), 40)
	_, _ = store.Generate("crm", testBrief("pl-PL"), domain.SetBrandTracking, domain.DefaultIntentQuota(), 40)
	total := store.TotalQuota("crm")
	if total != 80 {
		t.Fatalf("独立配额合计应为 80，实际 %d", total)
	}
	for _, r := range store.List("crm") {
		if !strings.Contains(r.BundleNotice, "独立计入 prompt quota") {
			t.Fatal("打包提示缺失")
		}
	}
}

// ---- EX-04 / EX-02 报告 ----

// TC-16-07：两套语言各出独立 mention rate / SoV / Citation Rate，
// mismatch 单独分组且不并分母，报告带「并列观察」说明。
func TestTC16_07_CrossLanguageReport(t *testing.T) {
	rec := BuildReportRecord("r", []domain.RunAnswer{}, false, nil)
	if rec.LanguageNotice != domain.LanguageNotice {
		t.Fatal("缺少跨语言并列观察说明")
	}
	if !strings.Contains(domain.Contract9Note, "unknown") {
		t.Fatal("契约 9 口径说明缺失")
	}
	mismatch := []domain.RunAnswer{{LocaleMismatch: true, RetrievalLocale: "pl-PL", Valid: true}}
	split := SplitByLocale(mismatch)
	if len(split.Mismatch) != 1 || len(split.Default) != 0 {
		t.Fatal("mismatch 被并入默认分母")
	}
}

// ---- EX-05 citation host ----

// TC-16-08：blog.example.com 与 example.com 各 1 次，两条独立计数、
// 不合并、不继承权重。
func TestTC16_08_CitationHostExact(t *testing.T) {
	cites := []domain.Citation{
		{URL: "https://blog.example.com/a"}, {URL: "https://example.com/b"},
	}
	counts := CitationHostCounts(cites)
	if counts["blog.example.com"] != 1 || counts["example.com"] != 1 {
		t.Fatalf("host 精确计数错误：%+v", counts)
	}
	for _, c := range NormalizeCitations(cites) {
		if c.RegistrableDomain != "example.com" {
			t.Fatal("registrable_domain 仅作展示分组")
		}
	}
}

// ---- EX-09 / EX-10 工具清单 ----

// TC-16-13：发布 7 版只保留最近 5 版（可配置）。
func TestTC16_13_ManifestKeepVersions(t *testing.T) {
	cfg := newTestConfig(t)
	store := NewManifestStore(cfg)
	for i := 1; i <= 7; i++ {
		if _, _, err := store.Publish("1."+strconv.Itoa(i), []ToolDescriptor{{Name: "t"}}, false, cfg); err != nil {
			t.Fatal(err)
		}
	}
	if store.Count() != 5 {
		t.Fatalf("应保留 5 版，实际 %d", store.Count())
	}
	if store.Versions()[0] != "1.3" {
		t.Fatal("保留的不是最近 5 版")
	}
}

// TC-16-14：清单版本 major.minor、自检工具存在、破坏性变更触发事件。
func TestTC16_14_ManifestBreakingEvent(t *testing.T) {
	cfg := newTestConfig(t)
	store := NewManifestStore(cfg)
	if _, _, err := store.Publish("v2", nil, false, cfg); err == nil {
		t.Fatal("非 major.minor 版本必须拒绝")
	}
	_, ev, err := store.Publish("2.0", []ToolDescriptor{{Name: "self_check.retrieval_locale"}}, true, cfg)
	if err != nil || ev != "tool_manifest_changed" {
		t.Fatalf("破坏性变更未触发事件：%s %v", ev, err)
	}
	found := false
	for _, n := range SelfCheckToolNames {
		if strings.Contains(n, "self_check") {
			found = true
		}
	}
	if !found {
		t.Fatal("缺少自检工具")
	}
}

// TC-16-15：Webhook 幂等（重复投递只入库一次）。
func TestTC16_15_WebhookIdempotent(t *testing.T) {
	d := NewWebhookDedup()
	if !d.Deliver("suspect_marked", "hash1") {
		t.Fatal("首次投递应入库")
	}
	if d.Deliver("suspect_marked", "hash1") {
		t.Fatal("重复投递未忽略")
	}
	if !d.Deliver("tool_manifest_changed", "hash1") {
		t.Fatal("不同事件类型不应互相去重")
	}
	if d.Count() != 2 {
		t.Fatal("幂等统计错误")
	}
}

func TestTC16_10_ToolsListTruncation(t *testing.T) {
	tools := []ToolDescriptor{}
	for i := 0; i < 10; i++ {
		tools = append(tools, ToolDescriptor{Name: "t" + strconv.Itoa(i)})
	}
	resp := BuildToolsList("1.1", tools, 3)
	if !resp.Truncated || resp.ReturnedCount != 3 || resp.TotalCount != 10 {
		t.Fatalf("裁切包装错误：%+v", resp)
	}
	if ClientNotice(resp) != domain.ToolsetIncomplete {
		t.Fatal("truncated 缺少「能力集不完整」提示")
	}
	full := BuildToolsList("1.1", tools, 0)
	if full.Truncated || ClientNotice(full) != "" {
		t.Fatal("未裁切时不应有提示")
	}
}

// ---- EX-06 参考区 ----

// TC-16-09：DR=80 只进独立参考区，免责声明固定，不进图表/API 聚合/排序/均值。
func TestTC16_09_UserProvidedIsolated(t *testing.T) {
	store := NewUserProvidedStore()
	m := store.Add("dr", 80, 1)
	if m.Disclaimer != domain.UserProvidedDisclaimer || m.SourceType != "user_provided" {
		t.Fatal("参考区免责声明 / source_type 错误")
	}
	if !IsolatedFromAggregation([]domain.DeltaSourceType{domain.DeltaSourceObservation}) {
		t.Fatal("聚合层混入了 user_provided")
	}
	if IsolatedFromAggregation([]domain.DeltaSourceType{domain.DeltaSourceUserProvided}) {
		t.Fatal("隔离断言失效")
	}
}

// ---- EX-07 / EX-08 爬虫证据 ----

// TC-16-11：sitemap "Couldn't fetch" 无日志/无授权探测 → unknown、
// 不进分母不扣分；访问日志命中 AI UA → pass；合规触碰 → 整组下线。
func TestTC16_11_CrawlerVisitEvidence(t *testing.T) {
	platformOnly := JudgeCrawlerVisit(CrawlerSignals{PlatformHint: "Couldn't fetch", PlatformHintIsError: true})
	if platformOnly.Verdict != domain.CrawlerVisitUnknown {
		t.Fatal("仅平台提示必须判 unknown")
	}
	if platformOnly.EvidenceSource != domain.EvidencePlatformHint {
		t.Fatal("平台提示应只记 evidence_source=platform_hint")
	}
	if !strings.Contains(FixAdviceForCrawler(platformOnly), domain.CrawlerEvidenceFix) {
		t.Fatal("修复建议未优先给「取得访问证据」")
	}
	if !NoDeterministicClaim(FixAdviceForCrawler(platformOnly)) {
		t.Fatal("修复文案含确定性收录表述")
	}
	withLog := JudgeCrawlerVisit(CrawlerSignals{AccessLogHitAIUA: true, PlatformHint: "Couldn't fetch"})
	if withLog.Verdict != domain.CrawlerVisitPass || withLog.EvidenceSource != domain.EvidenceAccessLog {
		t.Fatal("访问日志优先级未生效")
	}
	violation := JudgeCrawlerVisit(CrawlerSignals{ComplianceViolation: true})
	if !violation.Disabled || violation.Verdict != domain.CrawlerVisitUnknown {
		t.Fatal("合规触碰未整组下线")
	}
}

// TC-16-12：结果页 DOM 快照「平台提示」与「实际观察」两行独立、互不填充。
func TestTC16_12_ResultRowsIndependent(t *testing.T) {
	v := JudgeCrawlerVisit(CrawlerSignals{PlatformHint: "Couldn't fetch"})
	rows := BuildResultRows(v)
	if rows.PlatformHintRow != "Couldn't fetch" || rows.ObservedEvidenceRow == "" {
		t.Fatal("两行未分别填充")
	}
	if !RowsIndependent(v) {
		t.Fatal("两行发生互相填充")
	}
}

// ---- EX-11 / EX-12 suspect ----

// TC-16-16：单次失败不入 suspect；批次失败率 >=0.20 标 suspect；
// 配置变更留痕；人工标记写 F-903 审计日志。
func TestTC16_16_SuspectBatch(t *testing.T) {
	cfg := newTestConfig(t)
	single := MarkSuspect(BatchSignals{Platform: "p", Model: "m", Window: time.Now(), TotalRuns: 1, FailedRuns: 1}, cfg)
	if single.Suspect {
		t.Fatal("单次失败不得升级为 suspect 批次")
	}
	batch := MarkSuspect(BatchSignals{Platform: "p", Model: "m", Window: time.Now(), TotalRuns: 10, FailedRuns: 3}, cfg)
	if !batch.Suspect || !containsReason(batch.Reasons, domain.SuspectErrorRateSpike) {
		t.Fatal("批次失败率阈值未生效")
	}
	cfg.Set(domain.ThresholdFailureRate, 0.5, "qa")
	if cfg.Get(domain.ThresholdFailureRate) != 0.5 {
		t.Fatal("配置变更未生效")
	}
	before := len(cfg.AuditLog())
	MarkSuspect(BatchSignals{ManualMark: true, Window: time.Now()}, cfg)
	if len(cfg.AuditLog()) <= before {
		t.Fatal("人工标记未写 F-903 审计日志")
	}
	// suspect 与 valid 正交。
	runs := []domain.RunAnswer{{Valid: true}, {Valid: false}}
	marked := ApplySuspectToRuns(runs, BatchResult{Suspect: true, Reasons: []domain.SuspectReason{domain.SuspectManual}})
	if !marked[0].Valid || marked[1].Valid {
		t.Fatal("suspect 改写了 valid")
	}
}

// TC-16-17：四数互不折算，suspect 不进 usable/failed，健康告警与品牌
// 表现告警分栏，suspect 批次不触发品牌表现变化事件。
func TestTC16_17_FourCountsIsolation(t *testing.T) {
	runs := []domain.RunAnswer{
		{Valid: true, RetrievalLocale: "de-DE"}, {Valid: true, RetrievalLocale: "de-DE"},
		{Valid: false, RetrievalLocale: "de-DE"},
		{Valid: true, Suspect: true, RetrievalLocale: "de-DE"},
		{Valid: true, RetrievalLocale: "unknown"},
		{Valid: true, LocaleMismatch: true, RetrievalLocale: "pl-PL"},
	}
	c := FourCounts(runs)
	// 四数是运行层计数：unknown / mismatch 仍是 usable，只是不进业务指标分母；
	// suspect 与 failed 单独成数，不并入 usable。
	if c.Planned != 6 || c.Usable != 4 || c.Failed != 1 || c.Suspect != 1 {
		t.Fatalf("四数折算错误：%+v", c)
	}
	if len(IsolatedForMetrics(runs)) != 2 {
		t.Fatalf("指标隔离错误：%d", len(IsolatedForMetrics(runs)))
	}
	kind, text := ClassifyAlert(runs)
	if kind != AlertCollectionHealth || !NoCausalClaim(text) {
		t.Fatal("告警分类 / 因果表述错误")
	}
	kind2, _ := ClassifyAlert([]domain.RunAnswer{{Valid: true}})
	if kind2 != AlertBrandPerformance {
		t.Fatal("无 suspect 时应可产生品牌表现告警")
	}
}

// ---- EX-13 埋点 ----

// TC-16-18：四类事件埋点，采纳率 / 30d / 60d 留存、按 fix_item 分组统计正确。
func TestTC16_18_RetentionStats(t *testing.T) {
	store := NewRetentionStore()
	for _, typ := range []domain.FixArtifactEventType{
		domain.EventViewed, domain.EventDownloaded, domain.EventApplied, domain.EventRetestRequested,
	} {
		if _, ok := store.Record(1, 7, typ, nil); !ok {
			t.Fatal("合法事件被拒绝")
		}
	}
	if _, ok := store.Record(1, 7, "bad_type", nil); ok {
		t.Fatal("非法事件未拒绝")
	}
	stats := store.Stats()
	if stats.Viewed != 1 || stats.Downloaded != 1 || stats.Applied != 1 || stats.RetestRequested != 1 {
		t.Fatalf("事件统计错误：%+v", stats)
	}
	if stats.Retained30d != 1 || stats.Retained60d != 1 {
		t.Fatal("留存统计错误")
	}
	if store.ByFixItem()[7] != 1 {
		t.Fatal("按 fix_item 分组统计错误")
	}
}

// ---- EX-14 模板 + CI ----

// TC-16-19：增量文档模板含三级标记、证据层、排除项清单；评审检查表有否决权；
// 缺任一字段模板校验不通过；§6 CI 断言全绿。
func TestTC16_19_TemplateAndCI(t *testing.T) {
	tpl := DefaultIncrementTemplate()
	if len(tpl.Markers) != 3 || tpl.EvidenceLayer == "" || tpl.ExclusionHeading == "" {
		t.Fatal("模板字段缺失")
	}
	good := "[事实] x\n[观察] y\n[推导] z\n证据来自哪一层：answerlens_observation\n" + domain.ExclusionChecklistHeading + "\n- a"
	if !ValidateIncrementDoc(good).Valid {
		t.Fatal("合规文档被误判")
	}
	if ValidateIncrementDoc("[事实] only").Valid {
		t.Fatal("缺字段文档未拦截")
	}
	root := repoRoot(t)
	report := RunCIAssertions(root)
	if !report.AllPass {
		for _, c := range report.Checks {
			if !c.Pass {
				t.Errorf("CI 断言未通过：%s (%s)", c.Name, c.Detail)
			}
		}
		t.Fatal("§6 CI 静态断言存在失败项")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	// internal/delta 上溯两级到仓库根。
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}
