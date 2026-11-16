package domain

// 本文件承载《CodeBuddy 执行规格书｜2026-11-16 AEO 增量批次（Δ-01~Δ-07）》
// 的共享契约。所有字段名、枚举值、事件名与规格书 §2.2 / §3 完全一致，
// 禁止同义改名（历史文档与 TC-16 测试用例按此拼写断言）。
//
// 规格书 §2 说明「SQL 为示意，按项目实际 ORM / 数据库调整」：本项目用
// Go 结构体 + JSON 文件持久化，新增列以「结构体追加字段 + json tag」落地，
// 迁移 / 回填 / 回滚由 internal/delta 的 Migration 实现，语义与 DDL 等价。

// ---- 批次元数据与 feature flag ----

const (
	// DeltaFeatureFlag 是所有新增行为的总开关；off 时 API 不返回任何新字段，
	// 行为与 v1.0 完全一致（§7 回滚第 1 步，秒级生效）。
	DeltaFeatureFlag = "aeo_delta_20261116"

	// MethodologyVersionDelta 是新生成报告的口径版本；历史报告保留生成时版本
	// （MethodologyVersion = 1.0.0），禁止用新算法静默重算（§2.3 / §7）。
	MethodologyVersionDelta = "1.1"
)

// DeltaProvenance 给新增数字组件标注新口径版本（沿用 A-02 的 source_type 要求）。
func DeltaProvenance(nValid int) Provenance {
	return Provenance{
		SourceType:         SourceObserved,
		MethodologyVersion: MethodologyVersionDelta,
		NValid:             nValid,
	}
}

// ---- 枚举契约 E1~E7（拼写固定）----

// E1 retrieval_locale_source
type RetrievalLocaleSource string

const (
	LocaleSourcePlatformHeader RetrievalLocaleSource = "platform_header"
	LocaleSourceCitationDomain RetrievalLocaleSource = "citation_domain"
	LocaleSourceAnswerText     RetrievalLocaleSource = "answer_text"
	LocaleSourceInferred       RetrievalLocaleSource = "inferred"
	LocaleSourceUnknown        RetrievalLocaleSource = "unknown"
)

// RetrievalLocaleSourceOrder 是 D-1 定稿的判定优先级，禁止调整顺序。
var RetrievalLocaleSourceOrder = []RetrievalLocaleSource{
	LocaleSourcePlatformHeader, LocaleSourceCitationDomain,
	LocaleSourceAnswerText, LocaleSourceInferred, LocaleSourceUnknown,
}

func ValidRetrievalLocaleSource(s RetrievalLocaleSource) bool {
	for _, v := range RetrievalLocaleSourceOrder {
		if v == s {
			return true
		}
	}
	return false
}

// RunAnswerLocaleFields 是 EX-02 判定器的输出，对应 run_answer 的五个语言列。
// 判定器只返回字段，由采集适配器一次性写入不可变的 RunAnswer（只追加）。
type RunAnswerLocaleFields struct {
	QueryLocale           string
	RetrievalLocale       string
	RetrievalLocaleSource RetrievalLocaleSource
	RetrievalConfidence   *float64
	LocaleMismatch        bool
}

// ApplyTo 把判定结果写入 RunAnswer（创建时调用，禁止对既有 RunAnswer UPDATE）。
func (f RunAnswerLocaleFields) ApplyTo(a *RunAnswer) {
	a.QueryLocale = f.QueryLocale
	a.RetrievalLocale = f.RetrievalLocale
	a.RetrievalLocaleSource = f.RetrievalLocaleSource
	a.RetrievalConfidence = f.RetrievalConfidence
	a.LocaleMismatch = f.LocaleMismatch
}

// E2 evidence_source
type EvidenceSource string

const (
	EvidenceAccessLog    EvidenceSource = "access_log"
	EvidenceActiveProbe  EvidenceSource = "active_probe"
	EvidenceUserProvided EvidenceSource = "user_provided"
	EvidencePlatformHint EvidenceSource = "platform_hint"
	EvidenceNone         EvidenceSource = "none"
)

func ValidEvidenceSource(s EvidenceSource) bool {
	switch s {
	case EvidenceAccessLog, EvidenceActiveProbe, EvidenceUserProvided, EvidencePlatformHint, EvidenceNone:
		return true
	}
	return false
}

// E3 crawler_visit_evidence（Δ-03 / EX-07）
type CrawlerVisitEvidence string

const (
	CrawlerVisitPass    CrawlerVisitEvidence = "pass"
	CrawlerVisitPartial CrawlerVisitEvidence = "partial"
	CrawlerVisitFail    CrawlerVisitEvidence = "fail"
	CrawlerVisitUnknown CrawlerVisitEvidence = "unknown"
)

// E4 suspect_reason（Δ-05 / EX-11）
type SuspectReason string

const (
	SuspectPlatformIncident     SuspectReason = "platform_incident"
	SuspectErrorRateSpike       SuspectReason = "error_rate_spike"
	SuspectRateLimit            SuspectReason = "rate_limit"
	SuspectAdapterVersionChange SuspectReason = "adapter_version_change"
	SuspectTimeoutCluster       SuspectReason = "timeout_cluster"
	SuspectManual               SuspectReason = "manual"
)

func ValidSuspectReason(s SuspectReason) bool {
	switch s {
	case SuspectPlatformIncident, SuspectErrorRateSpike, SuspectRateLimit,
		SuspectAdapterVersionChange, SuspectTimeoutCluster, SuspectManual:
		return true
	}
	return false
}

// E5 question_set_type：在既有两套物理分离的问题集之外追加 brand_check。
const SetBrandCheck QuestionSetType = "brand_check"

func ValidQuestionSetType(s QuestionSetType) bool {
	switch s {
	case SetMarketBaseline, SetBrandTracking, SetBrandCheck:
		return true
	}
	return false
}

// E6 audit verdict：沿用 A-03 的 pass / partial / fail / unknown。
// CheckStatus 与 AdmissionStatus 已是同一组值，此处给出别名便于 Δ 契约引用。
type AuditVerdict = CheckStatus

// E7 source_type（沿用 A-02，但按规格书本节拼写独立定义，供新增 API schema 使用）
type DeltaSourceType string

const (
	DeltaSourcePlatformFirstParty DeltaSourceType = "platform_first_party"
	DeltaSourceObservation        DeltaSourceType = "answerlens_observation"
	DeltaSourceUserProvided       DeltaSourceType = "user_provided"
)

// ---- 固定文案（TC-16 / 快照断言按此拼写）----

const (
	// UserProvidedDisclaimer 是独立参考区固定免责文案，不可为空（D-2 / EX-06）。
	UserProvidedDisclaimer = "用户自述，未经核验；传统 SEO 指标，不代表 AI 答案可见性"

	// LanguageNotice 是跨语言报告的固定并列观察说明（EX-04 / TC-16-07）。
	LanguageNotice = "不同语言问题集不可直接比较，仅为并列观察"

	// RetrievalLocaleUndetermined 是 unknown 态的 UI 文案，禁止留白（EX-04）。
	RetrievalLocaleUndetermined = "检索语言未判定"

	// ToolsetIncomplete 是 tools/list 裁切时客户端必须给出的提示（EX-10）。
	ToolsetIncomplete = "能力集不完整"

	// CrawlerEvidenceFix 是 EX-08 修复清单在 unknown/fail 且平台报错时的优先建议。
	CrawlerEvidenceFix = "如何取得访问证据 / 如何提交抓取 / 如何验证爬虫到达"

	// Contract9Note 是报告方法论章节追加的契约 9 说明。
	Contract9Note = "检索语言只记录可观测信号，不声明模型内部检索机制；unknown 与 locale_mismatch 的运行不进默认分母。"
)

// ---- 阈值配置（D-1 / D-3 / D-4 默认值，写入 suspect_threshold_config）----

const (
	ThresholdFailureRate           = "failure_rate"
	ThresholdErrorRatio            = "error_ratio"
	ThresholdWindowMinutes         = "window_minutes"
	ThresholdManifestKeepVersions  = "manifest_keep_versions"
	ThresholdInferredMinConfidence = "inferred_min_confidence"
)

// DefaultThresholdConfig 是 §2.1 初始行，首版直接实现，不等历史回放（D-3）。
func DefaultThresholdConfig() map[string]float64 {
	return map[string]float64{
		ThresholdFailureRate:           0.20,
		ThresholdErrorRatio:            0.30,
		ThresholdWindowMinutes:         60,
		ThresholdManifestKeepVersions:  5,
		ThresholdInferredMinConfidence: 0.700,
	}
}

// ---- 四数 counts（Δ-05 / EX-12 / §3.1 强制响应约束）----

// Counts 四数互不折算，suspect 不得并入 failed 或 usable。
// 任何含数字的组件必须带 source_type 与 methodology_version（沿用 A-02）。
type Counts struct {
	Planned int `json:"planned"`
	Usable  int `json:"usable"`
	Failed  int `json:"failed"`
	Suspect int `json:"suspect"`
}

// ---- Δ-02 user_provided_metric（D-2 独立参考区，严格隔离）----

type UserProvidedMetric struct {
	MetricID   int64   `json:"metric_id"`
	ProjectID  int64   `json:"project_id"`
	MetricType string  `json:"metric_type"` // dr | da | domain_authority | organic_rank | backlink_count
	Value      float64 `json:"value"`
	SourceType string  `json:"source_type"` // 恒为 user_provided
	Disclaimer string  `json:"disclaimer"`  // 固定文案，不可为空
	CreatedAt  string  `json:"created_at"`
}

var ValidUserProvidedMetricTypes = []string{
	"dr", "da", "domain_authority", "organic_rank", "backlink_count",
}

// ---- Δ-06 fix_artifact_event（EX-13 采纳埋点）----

type FixArtifactEventType string

const (
	EventViewed          FixArtifactEventType = "viewed"
	EventDownloaded      FixArtifactEventType = "downloaded"
	EventApplied         FixArtifactEventType = "applied"
	EventRetestRequested FixArtifactEventType = "retest_requested"
)

type FixArtifactEvent struct {
	EventID    int64                `json:"event_id"`
	ReportID   int64                `json:"report_id"`
	FixItemID  int64                `json:"fix_item_id"`
	EventType  FixArtifactEventType `json:"event_type"`
	OccurredAt string               `json:"occurred_at"`
	ActorID    *int64               `json:"actor_id"`
}

// RetentionStats 是 GET /v1/usage/retention 的返回。
type RetentionStats struct {
	Viewed          int `json:"viewed"`
	Downloaded      int `json:"downloaded"`
	Applied         int `json:"applied"`
	RetestRequested int `json:"retest_requested"`
	Retained30d     int `json:"retained_30d"`
	Retained60d     int `json:"retained_60d"`
}

// ---- Δ-05 suspect_threshold_config 落库行 ----

type SuspectThresholdRow struct {
	ConfigKey   string  `json:"config_key"`
	ConfigValue float64 `json:"config_value"`
	UpdatedAt   string  `json:"updated_at"`
	UpdatedBy   string  `json:"updated_by"`
}

// ---- Δ-04 tool_manifest（EX-09，M6 后小版本）----

type ToolManifest struct {
	ManifestID       int64  `json:"manifest_id"`
	ManifestVersion  string `json:"manifest_version"` // major.minor
	PublishedAt      string `json:"published_at"`
	ToolsJSON        string `json:"tools_json"`
	IsBreakingChange bool   `json:"is_breaking_change"`
	PreviousVersion  string `json:"previous_version,omitempty"`
}

// ---- 三级证据标记（Δ-07 / EX-14）----

var EvidenceMarkers = []string{"[事实]", "[观察]", "[推导]"}

const ExclusionChecklistHeading = "排除项清单"
