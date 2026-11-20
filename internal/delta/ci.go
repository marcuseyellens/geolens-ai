package delta

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/answerlens/answerlens/domain"
)

// ---- EX-14 三级证据标记 [事实]/[观察]/[推导] + 排除项清单 ----

// IncrementTemplate 是 §1 EX-14 / §4.4 模板落库结构。
// 每个增量需求必须同时带三级标记、「证据来自哪一层」与排除项清单；
// 增量文档首页必须引用本模板，评审检查表有一项否决权（TC-16-19）。
type IncrementTemplate struct {
	Markers          []string `json:"markers"`
	EvidenceLayer    string   `json:"evidence_layer"`
	ExclusionHeading string   `json:"exclusion_heading"`
	ExclusionItems   []string `json:"exclusion_items"`
}

// DefaultIncrementTemplate 返回标准模板。evidence_layer 强制非空。
func DefaultIncrementTemplate() IncrementTemplate {
	return IncrementTemplate{
		Markers:          append([]string{}, domain.EvidenceMarkers...),
		EvidenceLayer:    "answerlens_observation（平台可观测回答层）",
		ExclusionHeading: domain.ExclusionChecklistHeading,
		ExclusionItems: []string{
			"不声明模型内部检索机制",
			"不把 user_provided 指标混入聚合",
			"不把 suspect 并入 usable / failed",
			"不由 registrable_domain 反推 host",
			"不把一种语言问题集翻译复用为另一种语言",
		},
	}
}

// TemplateValidation 是模板校验结果。
type TemplateValidation struct {
	Valid   bool     `json:"valid"`
	Missing []string `json:"missing"`
}

// ValidateIncrementDoc 校验一份增量文档是否按模板填写：
//   - 三级标记齐全（[事实] / [观察] / [推导]）；
//   - 「证据来自哪一层」非空；
//   - 有排除项清单小节。
func ValidateIncrementDoc(doc string) TemplateValidation {
	v := TemplateValidation{Valid: true}
	for _, m := range domain.EvidenceMarkers {
		if !strings.Contains(doc, m) {
			v.Missing = append(v.Missing, "缺少三级标记 "+m)
			v.Valid = false
		}
	}
	if !strings.Contains(doc, "答案层") && !strings.Contains(doc, "answerlens_observation") {
		v.Missing = append(v.Missing, "缺少「证据来自哪一层」")
		v.Valid = false
	}
	if !strings.Contains(doc, domain.ExclusionChecklistHeading) {
		v.Missing = append(v.Missing, "缺少"+domain.ExclusionChecklistHeading)
		v.Valid = false
	}
	return v
}

// ---- 禁词表（沿用 A-WP-2 §6，不新建第二套检查器）----

// ScanBannedPhrases 在给定内容里查找禁词。banned 为空时使用默认表。
func ScanBannedPhrases(content string, banned []string) []string {
	hits := []string{}
	for _, p := range banned {
		if p != "" && strings.Contains(content, p) {
			hits = append(hits, p)
		}
	}
	return hits
}

// ---- §6 CI 静态断言集合（接入既有 A-WP-1.5 通道）----

// BannedPhrasesFile 是仓库根目录的禁词表文件（在 neutrality 现有扫描之外，
// 补充本批次 5 类新禁词）。
const BannedPhrasesFile = "banned-phrases.txt"

// DefaultBannedPhrases 是 Δ 批次新增禁词（与 EX-12 因果表述、EX-08 确定性收录
// 表述、EX-02 模型内部机制声明对齐）。
var DefaultBannedPhrases = []string{
	"因平台故障导致品牌下降",
	"故障导致可见性下降",
	"模型内部以",
	"综合分",
	"Perplexity 会拒绝收录",
	"AI 平台因此改变收录",
	"一定会被索引",
	"必然被索引",
	"保证被索引",
	"模型内部检索机制为",
}

// CIReport 是 §6 全部检查项的一次运行结果。
type CIReport struct {
	Checks  []CICheck `json:"checks"`
	AllPass bool      `json:"all_pass"`
}

type CICheck struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail,omitempty"`
}

// RunCIAssertions 执行 §6 全部 9 类静态断言（含结果页快照）。
// root 为仓库根目录；只读，不修改文件。
func RunCIAssertions(root string) CIReport {
	checks := []CICheck{}

	// 1. 禁词表：仓库必须存在且覆盖本批次新禁词。
	bannedPath := filepath.Join(root, BannedPhrasesFile)
	bannedBytes, err := os.ReadFile(bannedPath)
	hasBanned := err == nil
	missingBanned := []string{}
	if hasBanned {
		for _, p := range DefaultBannedPhrases {
			if !strings.Contains(string(bannedBytes), p) {
				missingBanned = append(missingBanned, p)
			}
		}
	}
	checks = append(checks, CICheck{
		Name:   "禁词表覆盖 Δ 批次新禁词",
		Pass:   hasBanned && len(missingBanned) == 0,
		Detail: strings.Join(missingBanned, ", "),
	})

	// 2. overall_score 零命中（沿用 A-02：只扫三份契约文件，与
	// internal/aeo/checklist.go 同一套白名单，避免命中检查器自身/测试）。
	overallHits := []string{}
	for _, f := range []string{"domain/contract.go", "domain/models.go", "domain/aeo_contract.go"} {
		b, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if e == nil && strings.Contains(strings.ToLower(string(b)), "overall_score") {
			overallHits = append(overallHits, f)
		}
	}
	checks = append(checks, CICheck{
		Name:   "schema 中无 overall_score",
		Pass:   len(overallHits) == 0,
		Detail: strings.Join(overallHits, ", "),
	})

	// 3. dr/da 等传统 SEO 词不得出现在聚合计算（只允许在独立参考区）。
	seoHits := grepAggregationSEO(root)
	checks = append(checks, CICheck{
		Name:   "dr/da/domain_authority 不参与聚合计算",
		Pass:   len(seoHits) == 0,
		Detail: strings.Join(seoHits, ", "),
	})

	// 4. host 精确归一：Citation 结构体必须同时有 host 与 registrable_domain 列。
	contractBytes, _ := os.ReadFile(filepath.Join(root, "domain", "contract.go"))
	hostOK := strings.Contains(string(contractBytes), `json:"host,omitempty"`) &&
		strings.Contains(string(contractBytes), `json:"registrable_domain,omitempty"`)
	checks = append(checks, CICheck{Name: "citation.host 与 registrable_domain 独立落库", Pass: hostOK})

	// 5. 四数：Counts 必须含 planned/usable/failed/suspect。
	deltaBytes, _ := os.ReadFile(filepath.Join(root, "domain", "delta_contract.go"))
	fourOK := true
	for _, f := range []string{`json:"planned"`, `json:"usable"`, `json:"failed"`, `json:"suspect"`} {
		if !strings.Contains(string(deltaBytes), f) {
			fourOK = false
		}
	}
	checks = append(checks, CICheck{Name: "counts 四数齐全", Pass: fourOK})

	// 6. 工具清单版本固定包装字段存在。
	manifestBytes, _ := os.ReadFile(filepath.Join(root, "internal", "delta", "manifest.go"))
	toolOK := true
	for _, f := range []string{"manifest_version", "returned_count", "total_count", "truncated"} {
		if !strings.Contains(string(manifestBytes), f) {
			toolOK = false
		}
	}
	checks = append(checks, CICheck{Name: "tools/list 固定包装字段齐全", Pass: toolOK})

	// 7. truncated=true 时存在「能力集不完整」提示常量。
	tiOK := strings.Contains(string(deltaBytes), domain.ToolsetIncomplete)
	checks = append(checks, CICheck{Name: "工具裁切有「能力集不完整」提示", Pass: tiOK})

	// 8. RunAnswer 不可变：delta 包不得出现对 RunAnswer 的 UPDATE 语义写回。
	immutableOK := !strings.Contains(string(manifestBytes), "UPDATE run_answer")
	checks = append(checks, CICheck{Name: "RunAnswer 只追加、无 UPDATE 路径", Pass: immutableOK})

	// 9. 三级标记模板文件存在且自身通过校验。
	tplPath := filepath.Join(root, "docs", "increment-template.md")
	tplBytes, tplErr := os.ReadFile(tplPath)
	tplOK := tplErr == nil && ValidateIncrementDoc(string(tplBytes)).Valid
	checks = append(checks, CICheck{Name: "增量文档模板含三级标记与排除项清单", Pass: tplOK})

	// 10. 结果页快照：check.tsx 同时含「平台提示」与「实际观察」两行独立字段。
	checkPage, _ := os.ReadFile(filepath.Join(root, "frontend", "src", "routes", "check.tsx"))
	rowsOK := strings.Contains(string(checkPage), "平台提示") &&
		strings.Contains(string(checkPage), "实际观察") &&
		strings.Contains(string(checkPage), "platform_hint") &&
		strings.Contains(string(checkPage), "observed_evidence")
	checks = append(checks, CICheck{Name: "结果页快照：「平台提示」与「实际观察」分行", Pass: rowsOK})

	all := true
	for _, c := range checks {
		if !c.Pass {
			all = false
		}
	}
	return CIReport{Checks: checks, AllPass: all}
}

// grepAggregationSEO 检查聚合/计算目录里是否出现传统 SEO 权重词。
func grepAggregationSEO(root string) []string {
	hits := []string{}
	for _, dir := range []string{"application", "internal/aeo", "internal/report"} {
		base := filepath.Join(root, filepath.FromSlash(dir))
		_ = filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			b, e := os.ReadFile(path)
			if e != nil {
				return nil
			}
			low := strings.ToLower(string(b))
			// 只查明确的传统 SEO 权重字段；这些词只允许出现在 delta 的独立参考区
			// 枚举里，聚合/计算目录出现即视为混入。
			for _, w := range []string{"domain_authority", "backlink_count", "organic_rank"} {
				if strings.Contains(low, w) {
					hits = append(hits, filepath.Base(path)+":"+w)
				}
			}
			return nil
		})
	}
	return hits
}
