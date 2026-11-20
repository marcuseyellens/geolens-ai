package competitor

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/answerlens/answerlens/domain"
	"github.com/answerlens/answerlens/internal/delta"
)

// ---- §7 CI 追加项：作为既有 A-WP-1.5 / 执行规格 §6 通道的规则输入追加 ----
// 不新建第二套检查器：复用 delta.CICheck / delta.RunCIAssertions /
// delta.ValidateIncrementDoc，只在结果里追加本批次检查项。

// ExtraBannedPhrases 是 §7 追加到既有禁词表表尾的条目。
var ExtraBannedPhrases = []string{
	"visibility_score", "ai_attention_score", "composite_score", "ai_visibility_index",
	"real_ui 比 api_sourced 更准", "因此平台收录不同", "采集通道决定可见性",
	"综合可见度分", "综合 AI 分数", "综合指数",
}

// CompositeScoreTerms 是 DoD-C-1 禁止出现在五处输出面的字段名。
var CompositeScoreTerms = []string{
	"overall_score", "visibility_score", "ai_attention_score",
	"composite_score", "ai_visibility_index",
}

// OutputSurfaces 是五个输出面在项目里的实际落点：
// 报告模板 / 导出序列化器 / PDF 渲染模板 / 告警模板 / Action Board 返回值
// 集中在 internal/competitor/output.go；前端报告页与 Action Board 页各一处。
var OutputSurfaces = []string{
	"internal/competitor/output.go",
	"frontend/src/routes/diagnostic-report.tsx",
	"frontend/src/routes/monitor-page.tsx",
	"api/handlers/competitor_handler.go",
}

// ExtraChecks 返回本批次追加的 CI 断言。root 为仓库根目录，只读。
func ExtraChecks(root string) []delta.CICheck {
	checks := []delta.CICheck{}

	// 1. 禁词表覆盖追加条目（原有条目全部保留，只追加到表尾）。
	banned, _ := os.ReadFile(filepath.Join(root, delta.BannedPhrasesFile))
	missing := []string{}
	for _, p := range ExtraBannedPhrases {
		if !strings.Contains(string(banned), p) {
			missing = append(missing, p)
		}
	}
	checks = append(checks, delta.CICheck{
		Name: "禁词表覆盖竞品批次追加项", Pass: len(missing) == 0,
		Detail: strings.Join(missing, ", "),
	})

	// 2. 综合分禁令扩展到五处输出面（TC-C-14 / DoD-C-1）。
	// 只扫白名单文件，避免命中检查器自身持有的禁词字符串。
	hits := []string{}
	for _, f := range OutputSurfaces {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			hits = append(hits, f+" (missing)")
			continue
		}
		low := strings.ToLower(string(b))
		for _, term := range CompositeScoreTerms {
			if strings.Contains(low, term) {
				hits = append(hits, f+":"+term)
			}
		}
	}
	checks = append(checks, delta.CICheck{
		Name: "五处输出面均无综合分字段", Pass: len(hits) == 0,
		Detail: strings.Join(hits, ", "),
	})

	// 3. 采样 / 分母正交：ValidAnswerCount 函数体不得读取 sample_rounds。
	judge := readFile(root, "internal/competitor/judge.go")
	fn := stripComments(between(judge, "func ValidAnswerCount(", "func NValidUnchangedBySampleRounds("))
	orthogonal := fn != "" &&
		!strings.Contains(fn, "sample_rounds") &&
		!strings.Contains(fn, "SampleRounds") &&
		strings.Contains(fn, "n++")
	checks = append(checks, delta.CICheck{
		Name: "n_valid 定义不随 sample_rounds 变化", Pass: orthogonal,
	})

	// 4. 中立性文案逐字固定，且为渲染层常量（不落库、不可配置覆盖）。
	contract := readFile(root, "domain/competitor_contract.go")
	neutralityOK := strings.Contains(contract, domain.NeutralityNotice) &&
		strings.Contains(contract, "不落库")
	checks = append(checks, delta.CICheck{
		Name: "neutrality_notice 逐字固定且不可覆盖", Pass: neutralityOK,
	})

	// 5. 中立性不变式：修复件允许写入的集合里没有 run / run_answer，
	// 且本包源码不含对 run_answer 的 UPDATE / DELETE 写路径。
	writable := strings.Join(FixArtifactWritableTargets, ",")
	noWritePath := !strings.Contains(writable, "run_answer") &&
		!strings.Contains(writable, ",run,") &&
		!strings.Contains(judge, "UPDATE run_answer") &&
		!strings.Contains(readFile(root, "internal/competitor/runtime.go"), "UPDATE run_answer")
	checks = append(checks, delta.CICheck{
		Name: "fix_artifact 与 run_answer 无写路径", Pass: noWritePath,
	})

	// 6. 采集通道必填 + unknown 不进分母 + CE-1 与 E1 不互相反推。
	channelOK := strings.Contains(judge, "func ChannelCountsTowardsBaseline(") &&
		strings.Contains(judge, "CaptureUnknown") &&
		!strings.Contains(judge, "RetrievalLocaleSource =") &&
		!strings.Contains(judge, "retrieval_locale_source =")
	checks = append(checks, delta.CICheck{
		Name: "采集通道必填且与检索语言信号独立", Pass: channelOK,
	})

	// 7. 爬虫证据校验：spoof / ua_only / unverified 不得作为 pass 依据。
	botOK := strings.Contains(judge, "func CanUseAsPassEvidence(") &&
		strings.Contains(judge, "if v.SuspectedSpoof") &&
		strings.Contains(judge, "case domain.BotIPVerified, domain.BotReverseDNSVerified:")
	checks = append(checks, delta.CICheck{
		Name: "爬虫证据校验：spoof 与 ua_only 不参与 pass", Pass: botOK,
	})

	// 8. 路径真实性：citation_id 必须指向真实 citation。
	pathOK := strings.Contains(judge, "func ValidatePath(") &&
		strings.Contains(judge, "fabricated") &&
		strings.Contains(readFile(root, "internal/competitor/runtime.go"), "citation does not exist")
	checks = append(checks, delta.CICheck{
		Name: "crawl→citation 路径 citation_id 真实性", Pass: pathOK,
	})

	// 9. 导出完整性：raw_answer_text 非空、citations 完整。
	exportOK := strings.Contains(readFile(root, "internal/competitor/output.go"), "empty raw_answer_text") &&
		strings.Contains(readFile(root, "internal/competitor/output.go"), "Citations: r.Citations")
	checks = append(checks, delta.CICheck{
		Name: "evidence 导出含原文与完整 citations", Pass: exportOK,
	})

	// 10. 定位配置不改写 query_locale。
	runtimeSrc := readFile(root, "internal/competitor/runtime.go")
	setFn := between(runtimeSrc, "func (r *Runtime) SetTargeting(", "func (r *Runtime) AddVisitLog(")
	targetingOK := setFn != "" && !strings.Contains(setFn, ".QueryLocale =") &&
		!strings.Contains(setFn, "QueryLocale:")
	checks = append(checks, delta.CICheck{
		Name: "prompt_targeting 不改写 query_locale", Pass: targetingOK,
	})

	// 11. 三级标记模板：本批次文档通过 EX-14 模板校验（沿用既有检查器）。
	docBytes, _ := os.ReadFile(filepath.Join(root, "docs", "competitor-features-20261116.md"))
	docOK := len(docBytes) > 0 && delta.ValidateIncrementDoc(string(docBytes)).Valid
	checks = append(checks, delta.CICheck{
		Name: "本批次文档通过三级标记与排除项清单校验", Pass: docOK,
	})

	return checks
}

// RunAllAssertions 合并既有 §6 断言与本批次追加项，供 CI 与运维端点使用。
func RunAllAssertions(root string) delta.CIReport {
	base := delta.RunCIAssertions(root)
	checks := append([]delta.CICheck{}, base.Checks...)
	checks = append(checks, ExtraChecks(root)...)
	all := true
	for _, c := range checks {
		if !c.Pass {
			all = false
		}
	}
	return delta.CIReport{Checks: checks, AllPass: all}
}

func readFile(root, rel string) string {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return ""
	}
	return string(b)
}

// stripComments 去掉整行注释，避免把「不读取某字段」的说明文字
// 误判为实际代码引用（函数体级静态断言只看可执行语句）。
func stripComments(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "//") {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// between 截取 [start, end) 之间的源码片段，用于函数体级静态断言。
func between(src, start, end string) string {
	i := strings.Index(src, start)
	if i < 0 {
		return ""
	}
	rest := src[i:]
	j := strings.Index(rest, end)
	if j < 0 {
		return rest
	}
	return rest[:j]
}
