package competitor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/answerlens/answerlens/domain"
	"github.com/answerlens/answerlens/internal/delta"
)

func repoRoot() string {
	return filepath.Join("..", "..")
}

func sampleAnswer(i int, channel domain.CaptureChannel) domain.AnswerCompetitorFields {
	return domain.AnswerCompetitorFields{
		AnswerID: fmt.Sprintf("a%d", i), RunID: "r1", QuestionID: "q1",
		SampleIndex: i, CaptureChannel: channel,
		RawAnswerText: fmt.Sprintf("原始回答 %d", i),
		Citations:     []domain.Citation{},
		SourceType:    string(domain.SourceObserved), CollectedAt: "2026-11-16T02:00:00Z",
	}
}

// TC-C-01 / C-2：20 轮采样产出 20 条独立答案，n_valid 定义不变。
func TestTCC01_SampleRoundsAndNValidOrthogonal(t *testing.T) {
	run := domain.CompetitorRun{
		RunID: "r1", SampleRounds: 20,
		SamplePolicyVersion: domain.SamplePolicyFixedNV1,
		MethodologyVersion:  domain.MethodologyVersionCompetitor,
	}
	for i := 0; i < 20; i++ {
		run.Answers = append(run.Answers, sampleAnswer(i, domain.CaptureRealUI))
	}
	if err := ValidateSampleIndices(run); err != nil {
		t.Fatalf("20 轮采样应合法: %v", err)
	}
	if got := ValidAnswerCount(run.Answers); got != 20 {
		t.Fatalf("n_valid 应为 20 条独立有效答案，实际 %d", got)
	}
	if NValidUnchangedBySampleRounds(20, run.Answers) != NValidUnchangedBySampleRounds(1, run.Answers) {
		t.Fatal("n_valid 定义不得随 sample_rounds 变化")
	}
}

// TC-C-02 / §2.3-2：同一 (run, question) 下重复 sample_index 必须报错。
func TestTCC02_DuplicateSampleIndexRejected(t *testing.T) {
	run := domain.CompetitorRun{
		RunID: "r1", SampleRounds: 4, SamplePolicyVersion: domain.SamplePolicyFixedNV1,
	}
	for _, i := range []int{0, 1, 3, 3} {
		run.Answers = append(run.Answers, sampleAnswer(i, domain.CaptureRealUI))
	}
	if err := ValidateSampleIndices(run); err == nil {
		t.Fatal("重复 sample_index=3 应被拒绝")
	}
	rt := NewRuntime("")
	if err := rt.AddRun(run); err == nil {
		t.Fatal("写入重复采样应报错，不产生重复 observation")
	}
}

// TC-C-03 / C-4：两跳下钻回原文，RunAnswer 无 UPDATE 路径。
func TestTCC03_EvidenceDrilldownAndImmutableAnswers(t *testing.T) {
	rt := NewRuntime("")
	run := domain.CompetitorRun{
		RunID: "r1", SampleRounds: 2, SamplePolicyVersion: domain.SamplePolicyFixedNV1,
		CaptureChannelDeclared: true, MethodologyVersion: domain.MethodologyVersionCompetitor,
		Answers: []domain.AnswerCompetitorFields{
			sampleAnswer(0, domain.CaptureRealUI), sampleAnswer(1, domain.CaptureAPISourced),
		},
	}
	if err := rt.AddRun(run); err != nil {
		t.Fatal(err)
	}
	ev, version, err := BuildEvidence(rt, "r1")
	if err != nil || len(ev) != 2 || version != domain.MethodologyVersionCompetitor {
		t.Fatalf("证据端点应可回溯两条采样: ev=%d version=%s err=%v", len(ev), version, err)
	}
	if ev[0].RawAnswerText != run.Answers[0].RawAnswerText {
		t.Fatal("下钻原文与库内记录不一致")
	}
	// 返回值是副本：调用方修改不能写回 run_answer。
	got, _ := rt.Run("r1")
	got.Answers[0].RawAnswerText = "TAMPERED"
	after, _ := rt.Run("r1")
	if after.Answers[0].RawAnswerText == "TAMPERED" {
		t.Fatal("RunAnswer 被写回，违反不可变约束")
	}
}

// TC-C-04 / C-3：UA 声称 GPTBot 但 IP 不在已核验段 → suspected_spoof，不参与 pass。
func TestTCC04_SpoofedBotRejectedAsPassEvidence(t *testing.T) {
	verification, spoof := VerifyBotVisit(
		domain.CrawlerOperatorGPTBot, "GPTBot/1.0", "198.51.100.7",
		[]string{"203.0.113.0/24"}, false,
	)
	if verification != domain.BotSuspectedSpoof || !spoof {
		t.Fatalf("期望 suspected_spoof，实际 %s / spoof=%v", verification, spoof)
	}
	log := domain.CrawlerVisitLog{
		BotVerification: verification, SuspectedSpoof: spoof,
		EvidenceSource: domain.EvidenceAccessLog,
	}
	if CanUseAsPassEvidence(log) {
		t.Fatal("suspected_spoof 记录不得作为 pass 依据")
	}
	// 没有已核验 IP 段时绝不臆造，降级为 ua_only，同样不能判 pass。
	v2, spoof2 := VerifyBotVisit(domain.CrawlerOperatorGPTBot, "GPTBot/1.0", "198.51.100.7", nil, false)
	if v2 != domain.BotUAOnly || spoof2 {
		t.Fatalf("缺 IP 段应降级 ua_only，实际 %s", v2)
	}
	if CanUseAsPassEvidence(domain.CrawlerVisitLog{BotVerification: v2, EvidenceSource: domain.EvidenceAccessLog}) {
		t.Fatal("ua_only 不得单独判 pass")
	}
}

// TC-C-05 / C-3：逐页路径可查且 citation 真实；虚构 citation 写入被拒。
func TestTCC05_CitationPathMustReferenceRealCitation(t *testing.T) {
	rt := NewRuntime("")
	rt.AddCitation(CitationRecord{
		CitationID: "c1", RunID: "r1",
		Citation: domain.Citation{URL: "https://example.com/guide", Domain: "example.com", PagePath: "/guide"},
	})
	p := domain.CrawlCitationPath{
		PathID: "p1", ProjectID: "p", PageURL: "https://example.com/guide",
		CitationID: "c1", RunID: "r1", CitedAt: "2026-11-16T02:00:00Z",
	}
	if err := ValidatePath(p, func(id string) bool { _, ok := rt.Citation(id); return ok }); err != nil {
		t.Fatal(err)
	}
	if err := rt.AddPath(p); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range rt.Paths("") {
		if p.PathID == "p1" && p.CitationID == "c1" {
			found = true
		}
	}
	if !found {
		t.Fatal("逐页路径应可查")
	}
	fake := p
	fake.PathID, fake.CitationID = "p2", "fabricated_id"
	if err := rt.AddPath(fake); err == nil {
		t.Fatal("虚构 citation 必须被拒绝")
	}
}

// TC-C-06 / Δ-03：平台报错不影响已核验访问日志的 pass 判定，
// platform_hint 与 observed_evidence 仍为独立字段。
func TestTCC06_PlatformHintNeverDecidesPass(t *testing.T) {
	verified := domain.CrawlerVisitLog{
		BotVerification: domain.BotIPVerified, SuspectedSpoof: false,
		EvidenceSource: domain.EvidenceAccessLog,
	}
	if !CanUseAsPassEvidence(verified) {
		t.Fatal("IP 校验访问日志应判 pass")
	}
	hint := verified
	hint.EvidenceSource = domain.EvidencePlatformHint
	if CanUseAsPassEvidence(hint) {
		t.Fatal("platform_hint 不得作为 pass 依据")
	}
	reverse := verified
	reverse.BotVerification = domain.BotReverseDNSVerified
	if !CanUseAsPassEvidence(reverse) {
		t.Fatal("反向 DNS 校验通过应判 pass")
	}
}

// TC-C-07 / C-1：unknown 不进分母，UI 有标注，CE-1 与 E1 独立。
func TestTCC07_CaptureChannelUnknownExcluded(t *testing.T) {
	answers := []domain.AnswerCompetitorFields{
		sampleAnswer(0, domain.CaptureRealUI),
		sampleAnswer(1, domain.CaptureAPISourced),
		sampleAnswer(2, domain.CaptureUnknown),
	}
	if got := ValidAnswerCount(answers); got != 2 {
		t.Fatalf("unknown 不进分母，期望 2 实际 %d", got)
	}
	if ChannelCountsTowardsBaseline(domain.CaptureUnknown) {
		t.Fatal("unknown 不得进基线")
	}
	if domain.ChannelUnlabelledNotice != "采集通道未标注" {
		t.Fatal("UI 未标注通道文案")
	}
	if domain.ValidCaptureChannel(domain.CaptureChannel("platform_header")) {
		t.Fatal("CE-1 与 E1 取值不得共用")
	}
	if by := CountsByChannel(answers); by[domain.CaptureUnknown] != 1 {
		t.Fatal("通道分组应保留 unknown 计数但不进分母")
	}
}

// TC-C-08 / C-1：全站文案无通道准确度比较类表述。
func TestTCC08_NoChannelAccuracyClaims(t *testing.T) {
	for _, phrase := range []string{
		"real_ui 比 api_sourced 更准", "因此平台收录不同", "采集通道决定可见性",
	} {
		if hits := delta.ScanBannedPhrases(domain.ContractC1+domain.ContractC2+domain.ContractC3+domain.ContractC4,
			[]string{phrase}); len(hits) > 0 {
			t.Fatalf("契约文案命中禁词 %s", phrase)
		}
	}
	report := RunAllAssertions(repoRoot())
	assertCheckPass(t, report, "禁词表覆盖竞品批次追加项")
}

// TC-C-09 / §2.4：新报告 1.2，历史 1.0 / 1.1 不改写，三口径均可渲染。
func TestTCC09_ThreeMethodologyVersionsRender(t *testing.T) {
	rt := NewRuntime("")
	reports := rt.Reports()
	if len(reports) < 3 {
		t.Fatal("应存在 1.0 / 1.1 / 1.2 三口径演示报告")
	}
	byVersion := map[string]domain.CompetitorReport{}
	for _, r := range reports {
		byVersion[r.MethodologyVersion] = r
	}
	for _, v := range []string{domain.MethodologyVersion, domain.MethodologyVersionDelta, domain.MethodologyVersionCompetitor} {
		if _, ok := byVersion[v]; !ok {
			t.Fatalf("缺少 %s 口径报告", v)
		}
	}
	t12 := BuildReportTemplate(byVersion[domain.MethodologyVersionCompetitor])
	if t12.NeutralityNotice != domain.NeutralityNotice {
		t.Fatal("1.2 报告必须含逐字中立性声明")
	}
	for _, v := range []string{domain.MethodologyVersion, domain.MethodologyVersionDelta} {
		legacy := BuildReportTemplate(byVersion[v])
		if legacy.MethodologyVersion != v {
			t.Fatal("历史报告版本被改写")
		}
		if legacy.NeutralityNotice != "" || !strings.Contains(legacy.CountNotice, "旧口径") {
			t.Fatal("历史口径不得伪装成 1.2，必须标注旧口径")
		}
	}
	// 回填可重复执行且不改写历史报告版本。
	rt.Backfill()
	rt.Backfill()
	again, _ := rt.Report("demo_report_110")
	if again.MethodologyVersion != domain.MethodologyVersionDelta {
		t.Fatal("回填不得改写历史报告版本")
	}
}

// TC-C-10 / §2.3-8：定位配置落库，query_locale 不被位置改写。
func TestTCC10_TargetingDoesNotRewriteQueryLocale(t *testing.T) {
	rt := NewRuntime("")
	want := domain.PromptTargeting{
		TargetID: "t1", QuestionSetID: "qs1",
		LocationScope: domain.ScopeCity, LocationValue: "Seattle",
		Persona: "采购负责人", Timezone: "America/Los_Angeles",
		QueryLocale: "en-US",
	}
	if err := rt.SetTargeting(want); err != nil {
		t.Fatal(err)
	}
	got, ok := rt.Targeting("qs1")
	if !ok || got.LocationScope != domain.ScopeCity || got.QueryLocale != "en-US" {
		t.Fatalf("定位配置未正确落库或 query_locale 被改写: %+v", got)
	}
	blank := domain.PromptTargeting{TargetID: "t2", QuestionSetID: "qs2", LocationScope: domain.ScopeNone}
	if err := rt.SetTargeting(blank); err != nil {
		t.Fatal(err)
	}
	got2, _ := rt.Targeting("qs2")
	if got2.LocationScope != domain.ScopeNone || got2.LocationValue != "" {
		t.Fatal("未设定项应为 none 与空串，不留 NULL")
	}
	if err := rt.SetTargeting(domain.PromptTargeting{QuestionSetID: "qs3", LocationScope: "planet"}); err == nil {
		t.Fatal("非法 CE-2 取值必须被拒绝")
	}
}

// TC-C-11 / C-4：中立性声明逐字固定，修复件不改变历史 run。
func TestTCC11_NeutralityNoticeAndNoWritePath(t *testing.T) {
	const want = "本报告的观测数据独立采集；修复件与优化建议不改变任何历史运行记录。两次运行仅为独立 observation，不构成因果结论。"
	if domain.NeutralityNotice != want {
		t.Fatal("neutrality_notice 文案不逐字一致")
	}
	rt := NewRuntime("")
	before, _, err := BuildEvidence(rt, "demo_run_1")
	if err != nil {
		t.Fatal(err)
	}
	// 模拟「交付修复件」只写 fix_artifact_event 等价集合，不触碰 run / run_answer。
	rt.MarkBotFilter("demo_audit_1", 1)
	after, _, _ := BuildEvidence(rt, "demo_run_1")
	b1, _ := json.Marshal(before)
	b2, _ := json.Marshal(after)
	if string(b1) != string(b2) {
		t.Fatal("交付修复件后历史 run / run_answer 被改写")
	}
	report := RunAllAssertions(repoRoot())
	assertCheckPass(t, report, "fix_artifact 与 run_answer 无写路径")
	assertCheckPass(t, report, "neutrality_notice 逐字固定且不可覆盖")
}

// TC-C-12 / §2.3-9：gap 信号生成、可下钻、priority_score 不进指标端点。
func TestTCC12_GapSignalInternalPriorityOnly(t *testing.T) {
	rt := NewRuntime("")
	gaps := DetectGaps(GapInput{
		ProjectID: "p", QuestionID: "q2", RunID: "demo_run_1",
		CompetitorBrandID: "competitor_a", CompetitorPresent: true, BrandPresent: false,
		IntentWeight: 1.2, CompetitorFrequency: 3, TimeDecay: 0.8,
		DetectedAt: "2026-11-16T02:00:00Z",
	})
	if len(gaps) != 1 || gaps[0].GapType != domain.GapCompetitorPresentBrandAbsent {
		t.Fatalf("应生成 competitor_present_brand_absent: %+v", gaps)
	}
	for _, g := range gaps {
		if err := rt.AddGap(g); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := BuildEvidence(rt, gaps[0].RunID); err != nil {
		t.Fatal("gap 信号必须可沿 run_id 两跳下钻: ", err)
	}
	board := BuildActionBoard(rt, "p", 3)
	b, _ := json.Marshal(board)
	if strings.Contains(strings.ToLower(string(b)), "priority_score") {
		t.Fatal("priority_score 不得出现在 Action Board 返回值")
	}
	if strings.Contains(strings.ToLower(string(b)), "visibility_score") {
		t.Fatal("Action Board 不得含可见度分数")
	}
	// gap 不触发品牌表现类告警：告警模板只描述观测数值与采集健康。
	alert := BuildAlertTemplate("a1", "采集健康", "待复核运行 1 条", "2026-11-16T02:00:00Z")
	ab, _ := json.Marshal(alert)
	if strings.Contains(string(ab), "gap") || strings.Contains(string(ab), "品牌表现") {
		t.Fatal("gap 信号不得触发品牌表现告警")
	}
}

// TC-C-13 / C-4：evidence 端点完整、只读、无综合分字段。
func TestTCC13_EvidenceExportCompleteAndReadOnly(t *testing.T) {
	rt := NewRuntime("")
	before, _ := rt.Run("demo_run_1")
	ev, version, err := BuildEvidence(rt, "demo_run_1")
	if err != nil || version != domain.MethodologyVersionCompetitor {
		t.Fatalf("evidence 端点失败: %v", err)
	}
	if len(ev) == 0 {
		t.Fatal("evidence 不得为空")
	}
	citations := 0
	for _, e := range ev {
		if e.RawAnswerText == "" {
			t.Fatal("raw_answer_text 不得为空")
		}
		if e.CaptureChannel == "" {
			t.Fatal("导出必须含 capture_channel")
		}
		citations += len(e.Citations)
	}
	after, _ := rt.Run("demo_run_1")
	b1, _ := json.Marshal(before)
	b2, _ := json.Marshal(after)
	if string(b1) != string(b2) {
		t.Fatal("只读端点产生了写操作")
	}
	records, _, _ := BuildExportRecords(rt, "demo_run_1")
	jsonBytes, _ := ExportJSON(records)
	csvBytes, _ := ExportCSV(records)
	for _, payload := range []string{string(jsonBytes), string(csvBytes)} {
		low := strings.ToLower(payload)
		for _, term := range CompositeScoreTerms {
			if strings.Contains(low, term) {
				t.Fatalf("导出含禁用的综合分字段 %s", term)
			}
		}
		if !strings.Contains(payload, "raw_answer_text") && !strings.Contains(payload, "原始回答") {
			t.Fatal("导出未含原文")
		}
	}
	if citations == 0 {
		t.Fatal("citations[] 条数应与 run 内记录一致且非空")
	}
}

// TC-C-14 / DoD-C-1：既有 §6 断言 + 本批次追加项全部通过。
func TestTCC14_AllCIAssertionsPass(t *testing.T) {
	report := RunAllAssertions(repoRoot())
	for _, c := range report.Checks {
		if !c.Pass {
			t.Errorf("CI 断言未通过: %s (%s)", c.Name, c.Detail)
		}
	}
	if !report.AllPass {
		t.Fatal("all_pass 必须为 true")
	}
	if len(report.Checks) < 20 {
		t.Fatalf("应含既有 10 项 + 本批次追加项，实际 %d", len(report.Checks))
	}
}

// DoD-C-5：两个 flag 正交，off 时回到批次前状态且不影响 delta flag。
func TestTCCFlagOrthogonal(t *testing.T) {
	t.Setenv("GEOLENS_FLAG_AEO_COMPETITOR_20261116", "off")
	t.Setenv("GEOLENS_FLAG_AEO_DELTA_20261116", "")
	rt := NewRuntime("")
	if rt.Enabled() {
		t.Fatal("competitor flag 应为 off")
	}
	if !delta.NewFlag().Enabled() {
		t.Fatal("关闭 competitor flag 不得影响 delta flag")
	}
	steps := rt.Rollback()
	if len(steps) == 0 || !strings.Contains(strings.Join(steps, ""), domain.DeltaFeatureFlag) {
		t.Fatal("回滚步骤必须声明 delta flag 保持不变")
	}
	rt.Flag.SetEnabled(true)
	if !rt.Enabled() {
		t.Fatal("on 方向应可恢复")
	}
	if !delta.NewFlag().Enabled() {
		t.Fatal("开启 competitor flag 不得影响 delta flag")
	}
	// data_path 幂等：重复 Persist / Migrate 不改写历史报告版本。
	m1 := rt.Migrate()
	m2 := rt.Migrate()
	if m1["schema_version"] != m2["schema_version"] {
		t.Fatal("迁移不可幂等")
	}
	if err := rt.Persist(); err != nil {
		t.Fatal(err)
	}
}

// 三级标记模板：本批次文档含 [事实]/[观察]/[推导] 与排除项清单。
func TestTCCIncrementTemplate(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot(), "docs", "competitor-features-20261116.md"))
	if err != nil {
		t.Fatal(err)
	}
	v := delta.ValidateIncrementDoc(string(b))
	if !v.Valid {
		t.Fatalf("批次文档未通过三级标记模板: %v", v.Missing)
	}
}

func assertCheckPass(t *testing.T, report delta.CIReport, name string) {
	t.Helper()
	for _, c := range report.Checks {
		if c.Name == name {
			if !c.Pass {
				t.Fatalf("CI 断言 %q 未通过: %s", name, c.Detail)
			}
			return
		}
	}
	t.Fatalf("未找到 CI 断言 %q", name)
}
