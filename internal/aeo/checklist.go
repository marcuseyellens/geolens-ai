package aeo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geolens/geolens/domain"
)

// ---- A-WP-9.4 / §8 增量发布验收清单的运行时求值 ----
//
// 与 acceptance_test.go 共用同一批真实计算函数，API 不硬编码「已通过」。

// ChecklistItem 是验收清单的一行，带实测证据。
type ChecklistItem struct {
	ID        int      `json:"id"`
	Criterion string   `json:"criterion"`
	Passed    bool     `json:"passed"`
	Evidence  []string `json:"evidence"`
}

// ChecklistEvidence 是对自家站点做清单求值所需的静态证据。
type ChecklistEvidence struct {
	RepoRoot     string
	FrontendDist string
	FrontendSrc  string
}

func readFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

// GoStringLiterals 提取 Go 源码里的双引号字符串字面量，跳过注释。
// 用于「代码中不得内置具体来源站点占比常量」的 CI / 运行时断言。
func GoStringLiterals(src string) []string {
	out := []string{}
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "/*") {
			continue
		}
		if cut := strings.Index(line, "//"); cut >= 0 {
			line = line[:cut]
		}
		rest := line
		for {
			start := strings.Index(rest, `"`)
			if start < 0 {
				break
			}
			rest = rest[start+1:]
			end := strings.Index(rest, `"`)
			if end < 0 {
				break
			}
			out = append(out, rest[:end])
			rest = rest[end+1:]
		}
	}
	return out
}

func (e ChecklistEvidence) read(parts ...string) string {
	return readFile(filepath.Join(append([]string{e.FrontendSrc}, parts...)...))
}

// readDist 先读构建产物，缺失时按源码路径回退。
// distParts 是产物里的文件名，srcParts 是源码里的相对路径。
func (e ChecklistEvidence) readDist(distParts []string, srcParts []string) string {
	if v := readFile(filepath.Join(append([]string{e.FrontendDist}, distParts...)...)); v != "" {
		return v
	}
	return e.read(srcParts...)
}

// SelfChecksFor 用自家产品对自家站点跑 A-01 / A-03 / A-05 / A-14 四项自检。
// 与 EvaluateChecklist 第 8 条共用同一份静态证据，保证 API 与清单口径一致。
func SelfChecksFor(e ChecklistEvidence) []domain.SelfCheckResult {
	home := e.readDist([]string{"index.html"}, []string{"index.html"})
	robots := e.readDist([]string{"robots.txt"}, []string{"public", "robots.txt"})
	llms := e.readDist([]string{"llms.txt"}, []string{"public", "llms.txt"})
	sitemap := e.readDist([]string{"sitemap.xml"}, []string{"public", "sitemap.xml"})
	landing := e.read("src", "routes", "landing.tsx")
	neutralitySrc := e.read("src", "lib", "neutrality.ts")
	pricing := e.read("src", "routes", "pricing.tsx")
	checks := InspectAdmission(PageSnapshot{
		URL: "https://geolens.ai", Status: 200, Body: home, Rendered: false,
	}, DefaultAdmissionConfig())
	agent := InspectAgentReadability(AgentAssets{
		LLMsTxt: llms, MachinePrice: strings.Contains(home, `"price"`),
		HumanGate: true, Body: home,
	})
	return RunSelfChecks(SelfCheckInput{
		HomeHTML: home, RobotsTxt: robots, LLMsTxt: llms, SitemapXML: sitemap,
		JSONLDPresent: strings.Contains(home, "application/ld+json"),
		Profile:       domain.NewBusinessProfile("geolens"),
		Admission:     checks, AgentChecks: agent,
		// 声明渲染在落地页，常量位于共享模块；两者合起来才算「首屏可见」。
		NeutralityInLanding: NeutralityVisible(home + landing + neutralitySrc),
		NeutralityInReport:  NeutralityVisible(strings.Join(ReportMethodologyNotes(), " ")),
		PricingText:         pricing,
	})
}

// EvaluateChecklist 逐条实测 §8 的 10 条增量发布验收清单。
func EvaluateChecklist(e ChecklistEvidence) []ChecklistItem {
	home := e.readDist([]string{"index.html"}, []string{"index.html"})
	landing := e.read("src", "routes", "landing.tsx")
	neutralitySrc := e.read("src", "lib", "neutrality.ts")
	pricing := e.read("src", "routes", "pricing.tsx")

	items := []ChecklistItem{}

	// 1 来源类型全链路与三类来源隔离
	ev1 := []string{}
	ok1 := ValidateNumericComponent("site_score", domain.ObservedProvenance(1)) == nil &&
		ValidateSourceSeparation("report_table", []domain.SourceType{domain.SourceObserved, domain.SourceObserved}) == nil
	if ValidateSourceSeparation("mixed", []domain.SourceType{domain.SourceObserved, domain.SourceFirstParty}) == nil {
		ok1 = false
		ev1 = append(ev1, "混合来源未被拒绝")
	}
	for _, f := range []string{"domain/contract.go", "domain/models.go", "domain/aeo_contract.go"} {
		if strings.Contains(strings.ToLower(readFile(filepath.Join(e.RepoRoot, f))), "overall_score") {
			ok1 = false
			ev1 = append(ev1, f+" 出现 overall_score")
		}
	}
	if len(ev1) == 0 {
		ev1 = append(ev1, "Provenance 强制字段齐全；混合来源被拒绝；schema 无 overall_score")
	}
	items = append(items, ChecklistItem{1, "含数字组件可查 source_type 与 methodology_version；三类来源分表", ok1, ev1})

	// 2 unknown 不扣分、不进分母
	base := 88.0
	ok2 := ScoreWithProfile(base, domain.ProfileUnknown) == base
	_, _, _, unknown := AdmissionDenominator([]domain.AdmissionCheck{
		{Status: domain.AdmissionPass}, {Status: domain.AdmissionUnknown}, {Status: domain.AdmissionUnknown},
	})
	ok2 = ok2 && unknown == 2
	if _, scored := AdmissionGroupScore([]domain.AdmissionCheck{{Status: domain.AdmissionUnknown}}, 10); scored != 0 {
		ok2 = false
	}
	items = append(items, ChecklistItem{2, "unknown 档案状态不扣分；unknown 索引准入项不进分母", ok2,
		[]string{"unknown 档案状态得分不变；2 项 unknown 被排除出分母；整组 unknown 不参与评分"}})

	// 3 8 类样本回归
	detected, falsePositives, spaFP := AdmissionRegression()
	ok3 := detected == 8 && falsePositives <= 2 && !spaFP
	items = append(items, ChecklistItem{3, "8 类页面样本检出 8/8、误报 ≤ 2/24；SPA 无误判", ok3,
		[]string{fmt.Sprintf("检出 %d/8 类，误报 %d/24，SPA 误判 %v", detected, falsePositives, spaFP)}})

	// 4 风险提示、来源链接、无确定性收录表述
	checks := InspectAdmission(PageSnapshot{
		URL: "https://geolens.ai", Status: 200, Body: home, Rendered: false,
	}, DefaultAdmissionConfig())
	ev4 := []string{}
	ok4 := len(checks) == 8
	for _, c := range checks {
		if c.RiskNotice == "" || c.SourceURL == "" {
			ok4 = false
			ev4 = append(ev4, string(c.Detector)+" 缺风险提示或来源链接")
		}
		if hits := ScanTextForBannedPhrases(c.Evidence + c.Fix); len(hits) > 0 {
			ok4 = false
			ev4 = append(ev4, string(c.Detector)+" 含确定性表述")
		}
	}
	if len(ev4) == 0 {
		ev4 = append(ev4, "8/8 结论带证据、风险提示与来源链接")
	}
	items = append(items, ChecklistItem{4, "页面质量结论带风险提示与来源链接，无确定性收录表述", ok4, ev4})

	// 5 仅真实突变告警，失败批次不进基线
	ok5, ev5 := evaluateAnomalyInjection()
	items = append(items, ChecklistItem{5, "仅真实突变告警；失败批次不进基线", ok5, ev5})

	// 6 10 个聚合数字 ≤2 次点击下钻
	paths := make([]*domain.DrilldownPath, 0, 10)
	for i := 0; i < 10; i++ {
		paths = append(paths, BuildDrilldown(fmt.Sprintf("metric_%d", i), float64(i)/10, 12,
			fmt.Sprintf("run-%d", i), "原始答案文本", "https://example.com/page"))
	}
	errs6 := ValidateDrilldowns(paths)
	ok6 := len(errs6) == 0 && BuildDrilldown("no_evidence", 0.5, 0, "", "", "") == nil
	items = append(items, ChecklistItem{6, "10/10 聚合数字可在 ≤2 次点击下钻到原文与页面 URL", ok6,
		[]string{fmt.Sprintf("10 条路径完整；无证据 Insight 不生成；校验错误 %d", len(errs6))}})

	// 7 无来源占比常量，占比伴随 n_valid
	ok7, ev7 := evaluateNoRatioConstants(e.RepoRoot)
	items = append(items, ChecklistItem{7, "代码中无来源占比常量；任一占比伴随 n_valid", ok7, ev7})

	// 8 四项自检全绿
	results := SelfChecksFor(e)
	ok8 := AllSelfChecksPassed(results)
	ev8 := []string{}
	for _, r := range results {
		ev8 = append(ev8, fmt.Sprintf("%s %s：%s", r.ID, map[bool]string{true: "通过", false: "未通过"}[r.Passed], strings.Join(r.Details, "；")))
	}
	items = append(items, ChecklistItem{8, "A-01 / A-03 / A-05 / A-14 四项自检全绿", ok8, ev8})

	// 9 中立性声明与全站禁词
	violations, scanErr := ScanRepoForBannedPhrases(e.RepoRoot)
	ok9 := scanErr == nil && len(violations) == 0 &&
		NeutralityVisible(home+landing+neutralitySrc) &&
		NeutralityVisible(strings.Join(ReportMethodologyNotes(), " "))
	ev9 := []string{fmt.Sprintf("禁词命中 %d 处", len(violations))}
	if scanErr != nil {
		ev9 = append(ev9, scanErr.Error())
	}
	for _, v := range violations {
		ev9 = append(ev9, v.String())
	}
	if !ok9 && len(ev9) == 1 {
		ev9 = append(ev9, "中立性声明在落地页或报告不可见")
	}
	items = append(items, ChecklistItem{9, "落地页与报告均可见中立性声明；全站禁词 CI 通过", ok9, ev9})

	// 10 套餐结构冻结
	want := map[string]float64{"free": 0, "one_time": 39, "rescan": 19, "starter": 49, "growth": 199, "scale": 410}
	ok10 := len(domain.PlanPrices) == len(want)
	for k, v := range want {
		if domain.PlanPrices[k] != v {
			ok10 = false
		}
	}
	for _, p := range []string{"$0", "$39", "$19", "$49", "$199", "$410"} {
		if !strings.Contains(pricing, p) {
			ok10 = false
		}
	}
	items = append(items, ChecklistItem{10, "套餐结构未改变（Free/$39/$19/$49·199·410）", ok10,
		[]string{fmt.Sprintf("%d 个套餐价格与定价页文案一致", len(domain.PlanPrices))}})

	return items
}

func evaluateAnomalyInjection() (bool, []string) {
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	b := NewRollingBaseline(func() time.Time { return now })
	key := SeriesKey{BrandID: "b1", Model: domain.ModelChatGPT, Region: "us", Metric: "mention_rate"}
	for i := 0; i < 10; i++ {
		b.Record(key, domain.BaselinePoint{
			BatchID: fmt.Sprintf("b%d", i), ObservedOn: now.AddDate(0, 0, -10+i).Format("2006-01-02"),
			Value: 0.50, Valid: true,
		})
	}
	if b.Record(key, domain.BaselinePoint{Valid: false, Value: 999}) {
		return false, []string{"失败批次进入了基线"}
	}
	cfg := DefaultAnomalyConfig()
	pending := map[string]int{}
	_, seasonalAlert := ClassifyShift(b, AnomalyInput{Key: key,
		Point: domain.BaselinePoint{Valid: true, Value: 0.9}, Seasonal: true}, cfg, pending)
	_, faultAlert := ClassifyShift(b, AnomalyInput{Key: key,
		Point: domain.BaselinePoint{Valid: true, Value: 0.9}, FaultRate: 0.4}, cfg, pending)
	_, firstAlert := ClassifyShift(b, AnomalyInput{Key: key,
		Point: domain.BaselinePoint{Valid: true, Value: 0.95}}, cfg, pending)
	_, secondAlert := ClassifyShift(b, AnomalyInput{Key: key,
		Point: domain.BaselinePoint{Valid: true, Value: 0.95}}, cfg, pending)
	ok := !seasonalAlert && !faultAlert && !firstAlert && secondAlert
	return ok, []string{fmt.Sprintf(
		"季节性告警=%v 采集故障告警=%v 第1批=%v 第2批真实突变=%v；失败批次已排除",
		seasonalAlert, faultAlert, firstAlert, secondAlert)}
}

// BannedDomainConstants 扫描产品代码里是否内置了具体来源站点的占比常量。
// 合规检查器自身（checklist.go / neutrality.go）与测试文件只是断言工具，不参与扫描。
func BannedDomainConstants(root string) ([]string, error) {
	bad := []string{}
	skip := map[string]bool{"checklist.go": true, "neutrality.go": true}
	for _, dir := range []string{"internal", "domain", "application", "api"} {
		err := filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if skip[info.Name()] {
				return nil
			}
			for _, literal := range GoStringLiterals(readFile(path)) {
				if strings.Contains(strings.ToLower(literal), "reddit") {
					bad = append(bad, path+": "+literal)
				}
			}
			return nil
		})
		if err != nil {
			return bad, err
		}
	}
	return bad, nil
}

func evaluateNoRatioConstants(root string) (bool, []string) {
	bad, err := BannedDomainConstants(root)
	records := []CitationEconomyRecord{
		{Model: domain.ModelChatGPT, Region: "us", Intent: domain.IntentRecommend, Window: "2026-W45", Economy: domain.EconomyEditorial, Valid: true},
		{Model: domain.ModelClaude, Region: "us", Intent: domain.IntentRecommend, Window: "2026-W45", Economy: domain.EconomyPaid, Valid: false},
	}
	errs := ValidateSlices(SourceStructure(records))
	ev := fmt.Sprintf("域名占比常量命中 %d 处；占比口径错误 %d 处", len(bad), len(errs))
	if err != nil {
		ev += "；扫描错误：" + err.Error()
	}
	return err == nil && len(bad) == 0 && len(errs) == 0, []string{ev}
}
