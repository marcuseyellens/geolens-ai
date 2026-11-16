package domain

// 本文件承载《CodeBuddy 工程施工规格｜竞品功能采集批次（CF-01~CF-08）》
// 第三批次的共享契约。与 Δ 批次（domain/delta_contract.go）完全正交：
// 独立 feature flag、独立枚举编号段 CE-1~CE-4、独立口径版本 1.2。
//
// 规格 §2.1 说明「SQL 为示意，按项目实际 ORM / 数据库调整；表名、字段名、
// 枚举值拼写不可改」。本项目沿用 Δ 批次约定：Go 结构体 + JSON 文件持久化，
// 新增列以「结构体追加字段 + json tag」落地，迁移 / 回填 / 回滚由
// internal/competitor 实现，语义与 DDL 等价。
//
// 红线：不修改 D-1~D-6 / EX-01~EX-14 / TC-16-01~19 / E1~E7 任何既有定稿；
// 不占用 PRD §5 契约 1~11 与 §9 DoD 1~21 编号。

// ---- 批次元数据与 feature flag ----

const (
	// CompetitorFeatureFlag 是第三批次所有新增行为的总开关；off 时 API 不返回
	// 任何本批次字段，行为与「11/16 增量批次完成、竞品批次未上线」完全一致。
	CompetitorFeatureFlag = "aeo_competitor_20261116"

	// MethodologyVersionCompetitor 仅对新生成报告生效。历史 1.0.0 / 1.1
	// 报告保留生成时版本，禁止用 1.2 静默重算（§2.3-10 / §2.4）。
	MethodologyVersionCompetitor = "1.2"

	// CompetitorSchemaVersion 是 competitor.json 的迁移版本号。
	CompetitorSchemaVersion = "1.2"

	// NeutralityNotice 是 §3.2 逐字固定文案（CF-05 / EC-05）。
	// 渲染层常量，不落库、不可被租户配置 / 模板变量 / API 参数覆盖。
	NeutralityNotice = "本报告的观测数据独立采集；修复件与优化建议不改变任何历史运行记录。两次运行仅为独立 observation，不构成因果结论。"

	// ChannelUnlabelledNotice 是 capture_channel=unknown 时的 UI 文案。
	ChannelUnlabelledNotice = "采集通道未标注"
)

// ---- 枚举契约 CE-1~CE-4（拼写固定，独立编号段）----

// CE-1 capture_channel ∈ {real_ui, api_sourced, hybrid, unknown}
// 与 E1 retrieval_locale_source 是两个独立字段，不得互相填充、共用取值或反推。
type CaptureChannel string

const (
	CaptureRealUI     CaptureChannel = "real_ui"
	CaptureAPISourced CaptureChannel = "api_sourced"
	CaptureHybrid     CaptureChannel = "hybrid"
	CaptureUnknown    CaptureChannel = "unknown"
)

var AllCaptureChannels = []CaptureChannel{
	CaptureRealUI, CaptureAPISourced, CaptureHybrid, CaptureUnknown,
}

func ValidCaptureChannel(c CaptureChannel) bool {
	for _, v := range AllCaptureChannels {
		if v == c {
			return true
		}
	}
	return false
}

// CE-2 location_scope ∈ {country, region, state, city, none}
type LocationScope string

const (
	ScopeCountry LocationScope = "country"
	ScopeRegion  LocationScope = "region"
	ScopeState   LocationScope = "state"
	ScopeCity    LocationScope = "city"
	ScopeNone    LocationScope = "none"
)

var AllLocationScopes = []LocationScope{
	ScopeCountry, ScopeRegion, ScopeState, ScopeCity, ScopeNone,
}

func ValidLocationScope(s LocationScope) bool {
	for _, v := range AllLocationScopes {
		if v == s {
			return true
		}
	}
	return false
}

// CE-3 gap_type：竞品出现 / 被引用 / 被推荐而本品牌缺席
type GapType string

const (
	GapCompetitorPresentBrandAbsent  GapType = "competitor_present_brand_absent"
	GapCompetitorCitedBrandNot       GapType = "competitor_cited_brand_not"
	GapCompetitorRecommendedBrandNot GapType = "competitor_recommended_brand_not"
)

var AllGapTypes = []GapType{
	GapCompetitorPresentBrandAbsent, GapCompetitorCitedBrandNot, GapCompetitorRecommendedBrandNot,
}

func ValidGapType(g GapType) bool {
	for _, v := range AllGapTypes {
		if v == g {
			return true
		}
	}
	return false
}

// CE-4 bot_verification ∈ {ip_verified, reverse_dns_verified, ua_only,
// suspected_spoof, unverified}
type BotVerification string

const (
	BotIPVerified         BotVerification = "ip_verified"
	BotReverseDNSVerified BotVerification = "reverse_dns_verified"
	BotUAOnly             BotVerification = "ua_only"
	BotSuspectedSpoof     BotVerification = "suspected_spoof"
	BotUnverified         BotVerification = "unverified"
)

var AllBotVerifications = []BotVerification{
	BotIPVerified, BotReverseDNSVerified, BotUAOnly, BotSuspectedSpoof, BotUnverified,
}

func ValidBotVerification(v BotVerification) bool {
	for _, b := range AllBotVerifications {
		if b == v {
			return true
		}
	}
	return false
}

// 补充取值约定（非枚举，但拼写固定）——禁止同义改名。
const (
	SamplePolicySingleV1   = "single_v1"
	SamplePolicyFixedNV1   = "fixed_n_v1"
	SamplePolicyAdaptiveV1 = "adaptive_v1"

	GapStatusOpen      = "open"
	GapStatusDismissed = "dismissed"
	GapStatusActed     = "acted"

	CrawlerOperatorGPTBot        = "GPTBot"
	CrawlerOperatorClaudeBot     = "ClaudeBot"
	CrawlerOperatorPerplexityBot = "PerplexityBot"
	CrawlerOperatorUnknown       = "unknown"
)

func ValidSamplePolicyVersion(v string) bool {
	return v == SamplePolicySingleV1 || v == SamplePolicyFixedNV1 || v == SamplePolicyAdaptiveV1
}

func ValidGapStatus(v string) bool {
	return v == GapStatusOpen || v == GapStatusDismissed || v == GapStatusActed
}

func ValidCrawlerOperator(v string) bool {
	switch v {
	case CrawlerOperatorGPTBot, CrawlerOperatorClaudeBot,
		CrawlerOperatorPerplexityBot, CrawlerOperatorUnknown:
		return true
	}
	return false
}

// ---- 批次内数据契约 C-1~C-4（不占 PRD 编号）----

const (
	ContractC1 = "采集通道必须显式标注；unknown 不进任何分母或基线，CE-1 与 E1 互相独立，禁止比较通道准确度或推断平台行为。"
	ContractC2 = "采样与分母正交：多轮采样解决随机性，失败排除分母解决偏差；sample_rounds 不改变 n_valid 定义。"
	ContractC3 = "爬虫访问证据以 IP 校验后的实际访问记录为准；禁止模拟爬虫、伪造 UA 或虚构 crawl→citation 路径。"
	ContractC4 = "观测与行动中立：修复件、优化建议、Action Board 信号均不改变任何历史 run / run_answer。"
)

// ---- §2.1 新增列与表（Go 结构体等价 DDL）----

// RunCompetitorFields 对应 run 表三列追加字段。
// 历史回填：sample_rounds=1 / sample_policy_version=single_v1 /
// capture_channel_declared=false，禁止反推。
type RunCompetitorFields struct {
	SampleRounds           int    `json:"sample_rounds"`
	SamplePolicyVersion    string `json:"sample_policy_version"`
	CaptureChannelDeclared bool   `json:"capture_channel_declared"`
}

// AnswerCompetitorFields 对应 run_answer 两列追加字段 + 原始证据回溯所需字段。
// RunAnswer 保持不可变：这些字段只在创建时一次性写入，不提供 UPDATE 路径。
type AnswerCompetitorFields struct {
	AnswerID       string         `json:"answer_id"`
	RunID          string         `json:"run_id"`
	QuestionID     string         `json:"question_id"`
	SampleIndex    int            `json:"sample_index"`
	CaptureChannel CaptureChannel `json:"capture_channel"`
	RawAnswerText  string         `json:"raw_answer_text"`
	Citations      []Citation     `json:"citations"`
	SourceType     string         `json:"source_type"`
	CollectedAt    string         `json:"collected_at"`
}

// CompetitorRun 是 run 表在本批次内的完整记录（含逐条答案）。
// Answers 按 sample_index 升序，同一 (run, question) 下连续且唯一。
type CompetitorRun struct {
	RunID                  string                   `json:"run_id"`
	QuestionSetID          string                   `json:"question_set_id"`
	ProjectID              string                   `json:"project_id"`
	SampleRounds           int                      `json:"sample_rounds"`
	SamplePolicyVersion    string                   `json:"sample_policy_version"`
	CaptureChannelDeclared bool                     `json:"capture_channel_declared"`
	MethodologyVersion     string                   `json:"methodology_version"`
	Answers                []AnswerCompetitorFields `json:"answers"`
}

// PromptTargeting 对应 prompt_targeting 表（CF-04 / EC-04）。
// 定位配置只影响请求位置，不得改写 question.query_locale（§2.3-8）。
// 未设定项写 none / 空字符串，不留 NULL。
type PromptTargeting struct {
	TargetID      string        `json:"target_id"`
	QuestionSetID string        `json:"question_set_id"`
	LocationScope LocationScope `json:"location_scope"`
	LocationValue string        `json:"location_value"`
	Persona       string        `json:"persona"`
	Timezone      string        `json:"timezone"`
	ConfigVersion string        `json:"config_version"`
	QueryLocale   string        `json:"query_locale"` // 只由问题集语言决定，定位配置不得改写
}

// CrawlerVisitLog 对应 crawler_visit_log 表（CF-02 / EC-02）。
// 记录「已经发生的访问」的核验结果，不制造访问、不模拟爬虫。
type CrawlerVisitLog struct {
	VisitID         string          `json:"visit_id"`
	ProjectID       string          `json:"project_id"`
	PageURL         string          `json:"page_url"`
	CrawlerOperator string          `json:"crawler_operator"`
	UAString        string          `json:"ua_string"`
	SourceIP        string          `json:"source_ip"`
	BotVerification BotVerification `json:"bot_verification"`
	SuspectedSpoof  bool            `json:"suspected_spoof"`
	ObservedAt      string          `json:"observed_at"`
	EvidenceSource  EvidenceSource  `json:"evidence_source"`
	IngestedAt      string          `json:"ingested_at"`
}

// CrawlCitationPath 对应 crawl_citation_path 表。
// citation_id 必须指向真实存在的 citation，禁止虚构记录（§2.3-6）。
type CrawlCitationPath struct {
	PathID     string `json:"path_id"`
	ProjectID  string `json:"project_id"`
	PageURL    string `json:"page_url"`
	CitationID string `json:"citation_id"`
	RunID      string `json:"run_id"`
	CrawledAt  string `json:"crawled_at"`
	CitedAt    string `json:"cited_at"`
	VisitID    string `json:"visit_id,omitempty"`
}

// CompetitorGapSignal 对应 competitor_gap_signal 表（CF-06 / EC-06）。
// priority_score 仅用于 Action Board 内部排序，不进入任何指标聚合。
type CompetitorGapSignal struct {
	SignalID          string  `json:"signal_id"`
	ProjectID         string  `json:"project_id"`
	QuestionID        string  `json:"question_id"`
	RunID             string  `json:"run_id"`
	CompetitorBrandID string  `json:"competitor_brand_id"`
	GapType           GapType `json:"gap_type"`
	PriorityScore     float64 `json:"priority_score"`
	Status            string  `json:"status"`
	DetectedAt        string  `json:"detected_at"`
}

// AuditCheckCompetitor 对应 audit_check 两列追加字段（CF-02）。
type AuditCheckCompetitor struct {
	AuditID            string `json:"audit_id"`
	BotFilterApplied   bool   `json:"bot_filter_applied"`
	VerifiedVisitCount int    `json:"verified_visit_count"`
}

// CompetitorReport 是报告记录。neutrality_notice 为渲染层常量，不落库。
// 1.0 / 1.1 历史报告保留生成时版本，渲染时标注「旧口径」。
type CompetitorReport struct {
	ReportID           string         `json:"report_id"`
	RunID              string         `json:"run_id"`
	MethodologyVersion string         `json:"methodology_version"`
	GeneratedAt        string         `json:"generated_at"`
	Counts             Counts         `json:"counts"`
	LegacyCounts       bool           `json:"legacy_counts,omitempty"`
	CaptureChannel     CaptureChannel `json:"capture_channel,omitempty"`
}

// EvidenceRecord 是 GET /v1/evidence/{run_id} 的单条证据（CF-07 / EC-07）。
type EvidenceRecord struct {
	RunID              string         `json:"run_id"`
	MethodologyVersion string         `json:"methodology_version"`
	RawAnswerText      string         `json:"raw_answer_text"`
	SampleIndex        int            `json:"sample_index"`
	CaptureChannel     CaptureChannel `json:"capture_channel"`
	Citations          []Citation     `json:"citations"`
	SourceType         string         `json:"source_type"`
	CollectedAt        string         `json:"collected_at"`
}
