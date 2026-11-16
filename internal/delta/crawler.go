package delta

import (
	"strings"

	"github.com/geolens/geolens/domain"
)

// ---- EX-06 独立参考区（D-2 / 契约 10）----

// UserProvidedStore 严格隔离用户自述的传统 SEO 指标：
// 只进独立参考区，不进图表、不进 API 聚合、不参与排序、不参与任何均值。
type UserProvidedStore struct {
	metrics []domain.UserProvidedMetric
}

func NewUserProvidedStore() *UserProvidedStore {
	return &UserProvidedStore{metrics: []domain.UserProvidedMetric{}}
}

// Add 落库一条用户自述指标；disclaimer 固定不可为空，source_type 恒为 user_provided。
func (s *UserProvidedStore) Add(metricType string, value float64, projectID int64) domain.UserProvidedMetric {
	valid := false
	for _, t := range domain.ValidUserProvidedMetricTypes {
		if t == metricType {
			valid = true
			break
		}
	}
	if !valid {
		metricType = "user_provided_metric"
	}
	m := domain.UserProvidedMetric{
		MetricID:   int64(len(s.metrics) + 1),
		ProjectID:  projectID,
		MetricType: metricType,
		Value:      value,
		SourceType: string(domain.DeltaSourceUserProvided),
		Disclaimer: domain.UserProvidedDisclaimer,
		CreatedAt:  fixedTime(),
	}
	s.metrics = append(s.metrics, m)
	return m
}

// ReferenceOnly 返回独立参考区数据，仅用于单独表头渲染。
func (s *UserProvidedStore) ReferenceOnly() []domain.UserProvidedMetric {
	return append([]domain.UserProvidedMetric{}, s.metrics...)
}

// IsolatedFromAggregation 断言：user_provided 值不进入任何业务指标数值集合。
// 供 CI / TC-16-10 断言使用：聚合函数只能接收 answerlens_observation 数据。
func IsolatedFromAggregation(aggregatedSourceTypes []domain.DeltaSourceType) bool {
	for _, st := range aggregatedSourceTypes {
		if st == domain.DeltaSourceUserProvided {
			return false
		}
	}
	return true
}

// ---- EX-07 crawler_visit_evidence 检查项（Δ-03 / 合规红线）----

// CrawlerSignals 是第 9 项检查的可观测输入。
// 合规红线：不得模拟爬虫、不得伪造 UA 刷目标站日志；主动探测只针对用户自有
// 站点且需显式授权。触碰即整组下线（A-WP-2 §7）。
type CrawlerSignals struct {
	AccessLogHitAIUA    bool   // 访问日志命中 AI crawler UA（最高优先级证据）
	ActiveProbeObserved bool   // 用户自有站点、显式授权后的主动探测结果
	UserProvidedLog     string // 用户上传的访问日志片段（标注「用户自述，未经核验」）
	PlatformHint        string // 平台提示文案（如 sitemap "Couldn't fetch"），仅线索
	PlatformHintIsError bool
	ProbeAuthorized     bool // 主动探测是否获得显式授权
	IsOwnedSite         bool // 是否用户自有站点
	ComplianceViolation bool // 一旦为真，整组下线
}

// CrawlerVerdict 是第 9 项结论 + 分行展示所需的两类字段。
type CrawlerVerdict struct {
	Verdict                domain.CrawlerVisitEvidence `json:"verdict"`
	PlatformHint           string                      `json:"platform_hint"`
	ObservedEvidence       string                      `json:"observed_evidence"`
	EvidenceSource         domain.EvidenceSource       `json:"evidence_source"`
	Disabled               bool                        `json:"disabled"` // 合规触碰→整组下线
	UserProvidedUnverified bool                        `json:"user_provided_unverified"`
}

// JudgeCrawlerVisit 按 EX-07 优先级判定：
//
//	访问日志命中 AI crawler UA > 主动探测结果 > 平台提示文案
//
// 平台提示只记 platform_hint，不作 pass/fail 依据；无日志且无法探测 → unknown
// （不进分母、不扣分 /100）。
func JudgeCrawlerVisit(s CrawlerSignals) CrawlerVerdict {
	v := CrawlerVerdict{PlatformHint: s.PlatformHint}
	if s.ComplianceViolation {
		v.Disabled = true
		v.Verdict = domain.CrawlerVisitUnknown
		v.EvidenceSource = domain.EvidenceNone
		v.ObservedEvidence = "检查项因合规红线整组下线"
		return v
	}
	switch {
	case s.AccessLogHitAIUA:
		v.Verdict = domain.CrawlerVisitPass
		v.EvidenceSource = domain.EvidenceAccessLog
		v.ObservedEvidence = "访问日志命中 AI crawler UA"
	case s.ActiveProbeObserved && s.ProbeAuthorized && s.IsOwnedSite:
		v.Verdict = domain.CrawlerVisitPass
		v.EvidenceSource = domain.EvidenceActiveProbe
		v.ObservedEvidence = "用户自有站点、显式授权下的主动探测观察到抓取"
	case strings.TrimSpace(s.UserProvidedLog) != "":
		// 用户上传日志片段只能标「用户自述，未经核验」，不判 pass。
		v.Verdict = domain.CrawlerVisitUnknown
		v.EvidenceSource = domain.EvidenceUserProvided
		v.ObservedEvidence = "用户自述，未经核验：" + s.UserProvidedLog
		v.UserProvidedUnverified = true
	case s.PlatformHint != "":
		// 平台提示只作线索：存在平台提示但无访问证据 → 不判 pass。
		v.Verdict = domain.CrawlerVisitUnknown
		v.EvidenceSource = domain.EvidencePlatformHint
		v.ObservedEvidence = "仅有平台提示文案，无访问日志或主动探测证据"
	default:
		v.Verdict = domain.CrawlerVisitUnknown
		v.EvidenceSource = domain.EvidenceNone
		v.ObservedEvidence = "无日志且无法探测"
	}
	// 平台提示报错且结论为 unknown/fail 时，修复建议优先给「取得访问证据」。
	return v
}

// ---- EX-08 结果页分行 + 修复清单改写（FE+PM）----

// ResultRows 把「平台提示」与「实际观察」拆成两行独立字段，互不填充，
// 供 DOM 快照断言（TC-16-12 / §6「结果页快照」）。
type ResultRows struct {
	PlatformHintRow     string `json:"platform_hint_row"`
	ObservedEvidenceRow string `json:"observed_evidence_row"`
}

func BuildResultRows(v CrawlerVerdict) ResultRows {
	return ResultRows{
		PlatformHintRow:     v.PlatformHint,
		ObservedEvidenceRow: v.ObservedEvidence,
	}
}

// RowsIndependent 断言两行互不填充：platform_hint 不写进 observed_evidence，反之亦然。
func RowsIndependent(v CrawlerVerdict) bool {
	if v.PlatformHint == "" || v.ObservedEvidence == "" {
		return true
	}
	return !strings.Contains(v.ObservedEvidence, v.PlatformHint) &&
		!strings.Contains(v.PlatformHint, v.ObservedEvidence)
}

var deterministicIndexingClaims = []string{
	"平台会拒绝收录", "会拒绝收录", "AI 平台因此改变收录", "Perplexity 会拒绝收录",
	"一定会被索引", "必然被索引", "保证被索引",
}

// FixAdviceForCrawler 是 F-502 修复清单改写：当 crawler_visit_evidence ∈
// {unknown, fail} 且 platform_hint 报错时，优先建议「如何取得访问证据」，
// 而不是直接改 sitemap 文件；全部结论带「风险提示」+ 来源链接。
func FixAdviceForCrawler(v CrawlerVerdict) string {
	if (v.Verdict == domain.CrawlerVisitUnknown || v.Verdict == domain.CrawlerVisitFail) && v.PlatformHint != "" {
		return "风险提示：" + domain.CrawlerEvidenceFix + "；平台提示仅为线索，不作为收录结论。来源：站点检查报告 / F-502。"
	}
	return "风险提示：按实际观察证据复核站点可抓取性；结论仅为概率性判断。来源：站点检查报告 / F-502。"
}

// NoDeterministicClaim 检查修复文案不含确定性收录表述（禁词表 §6）。
func NoDeterministicClaim(text string) bool {
	for _, p := range deterministicIndexingClaims {
		if strings.Contains(text, p) {
			return false
		}
	}
	return true
}
