package aeo

import (
	"sort"
	"strings"

	"github.com/geolens/geolens/domain"
)

// ---- A-WP-5.1 / 5.2 / 5.3 引用经济属性与来源基线 ----
//
// 硬约束：代码中不存在 Reddit 或任何具体域名的占比常量。
// source_economy 默认 unknown，不进任何聚合；每个占比都带 n_valid 与采集窗口。

// SourceEconomyFor 只按显式标注归类；无法判定时返回 unknown，绝不推断。
// 刻意不内置任何域名→经济属性映射表（§8 第 7 条）。
func SourceEconomyFor(annotation string) domain.SourceEconomy {
	switch domain.SourceEconomy(strings.ToLower(strings.TrimSpace(annotation))) {
	case domain.EconomyOwned:
		return domain.EconomyOwned
	case domain.EconomyEditorial:
		return domain.EconomyEditorial
	case domain.EconomyPaid:
		return domain.EconomyPaid
	case domain.EconomyAffiliate:
		return domain.EconomyAffiliate
	case domain.EconomyCommunity:
		return domain.EconomyCommunity
	default:
		return domain.EconomyUnknown
	}
}

// CitationEconomyRecord 是单次引用的一行输入。
type CitationEconomyRecord struct {
	Model   domain.AIModelID
	Region  string
	Intent  domain.QueryIntent
	Window  string
	Economy domain.SourceEconomy
	RunID   string
	Valid   bool
}

// SourceStructure 按 model × region × intent × 时间 切片，输出来源结构与样本量。
// 只统计 valid=true；unknown 与失效样本都不进分母（A-WP-5.1 / 6.1）。
func SourceStructure(records []CitationEconomyRecord) []domain.SourceSlice {
	type key struct {
		model  domain.AIModelID
		region string
		intent domain.QueryIntent
		window string
	}
	groups := map[key][]CitationEconomyRecord{}
	for _, r := range records {
		if !r.Valid || r.Economy == domain.EconomyUnknown {
			continue
		}
		k := key{r.Model, r.Region, r.Intent, r.Window}
		groups[k] = append(groups[k], r)
	}
	keys := make([]key, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.model != b.model {
			return a.model < b.model
		}
		if a.region != b.region {
			return a.region < b.region
		}
		if a.intent != b.intent {
			return a.intent < b.intent
		}
		return a.window < b.window
	})

	out := []domain.SourceSlice{}
	for _, k := range keys {
		rows := groups[k]
		counts := map[domain.SourceEconomy]int{}
		for _, r := range rows {
			counts[r.Economy]++
		}
		economies := make([]domain.SourceEconomy, 0, len(counts))
		for e := range counts {
			economies = append(economies, e)
		}
		sort.Slice(economies, func(i, j int) bool { return economies[i] < economies[j] })
		for _, e := range economies {
			out = append(out, domain.SourceSlice{
				Model: k.model, Region: k.region, Intent: k.intent, Window: k.window,
				Economy: e, NValid: len(rows),
				Share: float64(counts[e]) / float64(len(rows)),
				Provenance: domain.Provenance{
					SourceType: domain.SourceObserved, MethodologyVersion: domain.MethodologyVersion,
					NValid: len(rows), Window: k.window,
				},
			})
		}
	}
	return out
}

// ValidateSlices 是 CI 断言入口：任一占比必须带 n_valid、采集窗口与来源类型。
func ValidateSlices(slices []domain.SourceSlice) []string {
	errs := []string{}
	for _, s := range slices {
		if s.NValid <= 0 {
			errs = append(errs, "占比缺少 n_valid")
		}
		if s.Window == "" {
			errs = append(errs, "占比缺少采集窗口")
		}
		if err := ValidateNumericComponent("source_slice", s.Provenance); err != nil {
			errs = append(errs, err.Error())
		}
		if s.Provenance.NValid != s.NValid {
			errs = append(errs, "n_valid 与口径标注不一致")
		}
	}
	return errs
}

// SamplingOnlyNote 是报告必须出现的采样口径说明（A-WP-5.3）。
const SamplingOnlyNote = "来源结构仅代表本次采样，不代表平台整体规律；占比伴随 n_valid 与采集窗口。"
