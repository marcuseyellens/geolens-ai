package delta

import (
	"fmt"
	"strings"
	"time"

	"github.com/geolens/geolens/domain"
)

// ---- EX-11 suspect 批次标记器（D-3 定稿）----

// BatchSignals 是一个批次（platform × model × 1 小时窗口）的可观测输入。
type BatchSignals struct {
	Platform           string
	Model              string
	Window             time.Time // 采集窗口起点（按小时对齐）
	TotalRuns          int
	FailedRuns         int
	RateLimitRuns      int
	AuthErrorRuns      int
	TimeoutRuns        int
	AdapterVersion     string
	PreviousAdapterVer string
	PlatformIncident   bool // 平台故障登记
	ManualMark         bool // 人工标记（必须写 F-903 审计日志）
	RunIDs             []string
}

// BatchResult 是标记器输出。
type BatchResult struct {
	BatchID      string
	Suspect      bool
	Reasons      []domain.SuspectReason
	ThresholdHit []string // 命中的规则说明
}

// MarkSuspect 按 D-3 五条规则判定批次：命中任一即标 suspect。
//
//	① 批次失败率 >= failure_rate(0.20)
//	② rate_limit / auth_error / timeout 占比 >= error_ratio(0.30)
//	③ 适配器版本变更
//	④ 平台故障登记
//	⑤ 人工标记
//
// 关键：单次失败不得升级为 suspect（需要构成一个批次的统计信号）。
// suspect 与 valid 正交，不互相覆盖（§2.2）。
func MarkSuspect(s BatchSignals, cfg *Config) BatchResult {
	failureRate := cfg.Get(domain.ThresholdFailureRate)
	errorRatio := cfg.Get(domain.ThresholdErrorRatio)
	windowMin := int(cfg.Get(domain.ThresholdWindowMinutes))

	r := BatchResult{
		BatchID: fmt.Sprintf("%s|%s|%s", s.Platform, s.Model,
			s.Window.UTC().Truncate(time.Duration(windowMin)*time.Minute).Format(time.RFC3339)),
	}
	add := func(reason domain.SuspectReason, hit string) {
		r.Suspect = true
		if !containsReason(r.Reasons, reason) {
			r.Reasons = append(r.Reasons, reason)
		}
		r.ThresholdHit = append(r.ThresholdHit, hit)
	}

	if s.TotalRuns > 1 { // 单次失败不升级为 suspect 批次
		fr := float64(s.FailedRuns) / float64(s.TotalRuns)
		if fr >= failureRate {
			add(domain.SuspectErrorRateSpike, fmt.Sprintf("failure_rate=%.3f >= %.3f", fr, failureRate))
		}
		er := float64(s.RateLimitRuns+s.AuthErrorRuns+s.TimeoutRuns) / float64(s.TotalRuns)
		if er >= errorRatio {
			add(domain.SuspectRateLimit, fmt.Sprintf("error_ratio=%.3f >= %.3f", er, errorRatio))
		}
		if s.TimeoutRuns > 0 && float64(s.TimeoutRuns)/float64(s.TotalRuns) >= errorRatio {
			add(domain.SuspectTimeoutCluster, "timeout_cluster")
		}
	}
	if s.AdapterVersion != "" && s.PreviousAdapterVer != "" && s.AdapterVersion != s.PreviousAdapterVer {
		add(domain.SuspectAdapterVersionChange, "adapter_version_change")
	}
	if s.PlatformIncident {
		add(domain.SuspectPlatformIncident, "platform_incident")
	}
	if s.ManualMark {
		add(domain.SuspectManual, "manual")
		cfg.Audit("manual", "suspect_manual_mark", r.BatchID)
	}
	return r
}

func containsReason(list []domain.SuspectReason, r domain.SuspectReason) bool {
	for _, v := range list {
		if v == r {
			return true
		}
	}
	return false
}

// ApplySuspectToRuns 把批次标记写回运行（创建/回填时调用）。
// suspect 与 valid 正交：绝不修改 Valid 字段。
func ApplySuspectToRuns(runs []domain.RunAnswer, res BatchResult) []domain.RunAnswer {
	out := make([]domain.RunAnswer, len(runs))
	copy(out, runs)
	reason := domain.SuspectReason("")
	if len(res.Reasons) > 0 {
		reason = res.Reasons[0]
	}
	for i := range out {
		out[i].Suspect = res.Suspect
		if res.Suspect {
			out[i].SuspectReason = reason
		}
	}
	return out
}

// ---- EX-12 四数隔离（DE 主责 / 契约 11）----

// ReportRecord 是落库的报告快照（等价 report 表；不改历史报告口径）。
type ReportRecord struct {
	ReportID           string                      `json:"report_id"`
	MethodologyVersion string                      `json:"methodology_version"`
	Counts             domain.Counts               `json:"counts"`
	LanguageNotice     string                      `json:"language_notice"`
	ReferenceOnly      []domain.UserProvidedMetric `json:"reference_only,omitempty"`
	LegacyCounts       bool                        `json:"legacy_counts,omitempty"`
	GeneratedAt        string                      `json:"generated_at"`
}

// FourCounts 计算四数：planned / usable / failed / suspect 互不折算，
// suspect 不并入 failed 或 usable（§3.1 强制响应约束 / TC-16-17）。
func FourCounts(runs []domain.RunAnswer) domain.Counts {
	var c domain.Counts
	c.Planned = len(runs)
	for _, r := range runs {
		switch {
		case r.Suspect:
			c.Suspect++
		case !r.Valid:
			c.Failed++
		default:
			c.Usable++
		}
	}
	return c
}

// IsolatedForMetrics 返回可进入四类指标分子分母 / 八周滚动基线 / 突变告警的运行：
// 排除 failed、suspect、locale unknown、locale_mismatch（EX-04 + EX-12）。
func IsolatedForMetrics(runs []domain.RunAnswer) []domain.RunAnswer {
	out := []domain.RunAnswer{}
	for _, r := range runs {
		if r.Suspect || !r.Valid {
			continue
		}
		if r.RetrievalLocale == "" || r.RetrievalLocale == string(domain.LocaleSourceUnknown) {
			continue
		}
		if r.LocaleMismatch {
			continue
		}
		out = append(out, r)
	}
	return out
}

// HealthAlert 只由 suspect 批次触发「采集健康」告警；品牌表现告警熔断
// （契约 11：禁止新增「品牌表现变化」类事件由 suspect 批次触发）。
type AlertKind string

const (
	AlertCollectionHealth AlertKind = "collection_health"
	AlertBrandPerformance AlertKind = "brand_performance"
)

// ClassifyAlert 判定告警类别。suspect 批次只能产生采集健康告警，
// 且文案禁「因平台故障导致品牌下降」类因果表述。
func ClassifyAlert(runs []domain.RunAnswer) (AlertKind, string) {
	suspectCount := 0
	for _, r := range runs {
		if r.Suspect {
			suspectCount++
		}
	}
	if suspectCount > 0 {
		return AlertCollectionHealth, "采集健康：存在 suspect 批次，已排除出业务指标；仅描述观测数值，不作因果推断。"
	}
	return AlertBrandPerformance, ""
}

// ForbiddenCausalPhrases 是 EX-12 禁止的因果表述（禁词表 §6 同源）。
var ForbiddenCausalPhrases = []string{
	"因平台故障导致品牌下降", "故障导致可见性下降", "因为改了", "因修复",
	"模型内部以",
}

// NoCausalClaim 检查告警文案不含禁用的因果 / 机制声明。
func NoCausalClaim(text string) bool {
	for _, p := range ForbiddenCausalPhrases {
		if strings.Contains(text, p) {
			return false
		}
	}
	return true
}

// BuildReportRecord 为新生成报告装配 1.1 口径；历史 1.0 报告按三数 + 旧口径标注。
func BuildReportRecord(reportID string, runs []domain.RunAnswer, legacy bool, reference []domain.UserProvidedMetric) ReportRecord {
	rec := ReportRecord{
		ReportID:       reportID,
		LanguageNotice: domain.LanguageNotice,
		ReferenceOnly:  reference,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if legacy {
		rec.MethodologyVersion = domain.MethodologyVersion // 1.0.0，历史不改写
		rec.LegacyCounts = true
		var c domain.Counts
		c.Planned = len(runs)
		for _, r := range runs {
			if !r.Valid {
				c.Failed++
			} else {
				c.Usable++
			}
		}
		rec.Counts = c // 三数，suspect 按 0 展示并标注「旧口径」
	} else {
		rec.MethodologyVersion = domain.MethodologyVersionDelta // 1.1
		rec.Counts = FourCounts(runs)
	}
	return rec
}
