package domain

import "time"

// MethodologyVersion 是聚合口径版本。报告保存生成时的版本，避免历史报告被新算法静默改写（PRD §6）。
const MethodologyVersion = "1.0.0"

// ObservationNote 是复扫页面与报告必须出现的措辞：只显示变化，不做因果归因（PRD §5.5 / DoD 8）。
const ObservationNote = "仅显示变化，不证明修复导致变化"

// ReadinessCaveat 是站点就绪度页面必须出现的措辞（PRD F-404 / DoD 5）。
const ReadinessCaveat = "技术就绪 ≠ 被推荐"

// ---- 采集契约（F-1xx）----

// FailureType 是失败运行分类。失败运行 valid=false，绝不记为「未提及」。
type FailureType string

const (
	FailureLoading   FailureType = "loading_error"
	FailureEmpty     FailureType = "empty_answer"
	FailureRateLimit FailureType = "rate_limit"
	FailureAuth      FailureType = "auth_error"
	FailureParse     FailureType = "parse_error"
	FailureBlocked   FailureType = "blocked"
	FailureTimeout   FailureType = "timeout"
)

// SearchActivity 记录每题的搜索开关状态（F-109）。
type SearchActivity string

const (
	SearchOn      SearchActivity = "on"
	SearchOff     SearchActivity = "off"
	SearchUnknown SearchActivity = "unknown"
	SearchAuto    SearchActivity = "auto"
)

// SourceClass 是 citation 归一后的来源类型（F-108）。
type SourceClass string

const (
	SourceCompetitorSite     SourceClass = "competitor_site"
	SourceIndependentRoundup SourceClass = "independent_roundup"
	SourceDirectory          SourceClass = "directory"
	SourceCommunity          SourceClass = "community"
	SourceNewsResearch       SourceClass = "news_research"
	SourceBrandSite          SourceClass = "brand_site"
	SourceOther              SourceClass = "other"
)

// AllSourceClasses 返回来源类型的固定展示顺序。
func AllSourceClasses() []SourceClass {
	return []SourceClass{
		SourceCompetitorSite, SourceIndependentRoundup, SourceDirectory,
		SourceCommunity, SourceNewsResearch, SourceBrandSite, SourceOther,
	}
}

// Citation 是答案实际展示出来的链接。它不代表答案依据的全部，更不代表训练数据（PRD §5.7）。
// Δ-02 / EX-05：host 为精确 host（含子域名），registrable_domain 仅作展示分组，
// 二者独立存储，禁止由 registrable_domain 反推 host，禁止继承排序权重。
type Citation struct {
	URL               string      `json:"url"`
	Domain            string      `json:"domain"`
	PagePath          string      `json:"page_path"`
	SourceClass       SourceClass `json:"source_class"`
	DirectCitation    bool        `json:"direct_citation"`
	Host              string      `json:"host,omitempty"`
	RegistrableDomain string      `json:"registrable_domain,omitempty"`
}

// Mention 是三态分离的品牌识别结果（F-203 / §5.2）。三个布尔字段独立存储、独立计数、独立展示。
type Mention struct {
	BrandID                 string    `json:"brand_id"`
	Mentioned               bool      `json:"mentioned"`
	Recommended             bool      `json:"recommended"`
	Linked                  bool      `json:"linked"`
	ConfusedWith            string    `json:"confused_with,omitempty"`
	RankInList              int       `json:"rank_in_list"`
	InShortlist             bool      `json:"in_shortlist"`
	IsFinalPick             bool      `json:"is_final_pick"`
	Sentiment               Sentiment `json:"sentiment"`
	AssociatedNeeds         []string  `json:"associated_needs,omitempty"`
	AssociatedSellingPoints []string  `json:"associated_selling_points,omitempty"`
}

// RunAnswer 是一次采集的不可变原始证据（F-107）。所有聚合指标由它派生，可重算。
// Δ-01：追加语言观测字段。RunAnswer 保持只追加、不 UPDATE（§2.2 不变式 / CI 断言）。
type RunAnswer struct {
	RunID            string         `json:"run_id"`
	QuestionID       string         `json:"question_id"`
	QuestionText     string         `json:"question_text"`
	IsBrandCheck     bool           `json:"is_brand_check"`
	Intent           QueryIntent    `json:"intent"`
	QuestionCategory string         `json:"question_category"`
	Platform         AIModelID      `json:"platform"`
	ModelVersion     string         `json:"model_version"`
	Region           string         `json:"region"`
	Locale           string         `json:"locale"`
	SearchActivity   SearchActivity `json:"search_activity"`
	ForcedSearch     bool           `json:"forced_search"`
	RawAnswerText    string         `json:"raw_answer_text"`
	Citations        []Citation     `json:"citations"`
	Valid            bool           `json:"valid"`
	FailureType      FailureType    `json:"failure_type,omitempty"`
	Mention          Mention        `json:"mention"`
	// CompetitorMentions 是同一答案里各竞品被提及的次数，用于 Share of Voice 分母。
	CompetitorMentions map[string]int `json:"competitor_mentions,omitempty"`
	TotalBrandMentions int            `json:"total_brand_mentions"`
	ObservedOn         string         `json:"observed_on"`
	CollectedAt        time.Time      `json:"collected_at"`
	LatencyMS          int            `json:"latency_ms"`
	Cost               float64        `json:"cost"`

	// ---- Δ-01 / EX-01/EX-02 新增列（flag off 时一律不填充，故 omitempty）----
	QueryLocale           string                `json:"query_locale,omitempty"`
	RetrievalLocale       string                `json:"retrieval_locale,omitempty"`
	RetrievalLocaleSource RetrievalLocaleSource `json:"retrieval_locale_source,omitempty"`
	RetrievalConfidence   *float64              `json:"retrieval_confidence,omitempty"`
	LocaleMismatch        bool                  `json:"locale_mismatch,omitempty"`
	CrawlerVisitEvidence  CrawlerVisitEvidence  `json:"crawler_visit_evidence,omitempty"`
	// Suspect / SuspectReason 是 Δ-05 批次标记（EX-11），与 valid 正交。
	Suspect       bool          `json:"suspect,omitempty"`
	SuspectReason SuspectReason `json:"suspect_reason,omitempty"`
}

// ---- 指标契约（F-2xx）----

// MetricSet 是一组答案的聚合指标。分母恒为 usable（有效答案数），不是 planned。
// Δ-05：Suspect 为第四数，与 valid 正交，不并入 usable / failed（EX-12）。
type MetricSet struct {
	Planned            int     `json:"planned"`
	Usable             int     `json:"usable"`
	Failed             int     `json:"failed"`
	Suspect            int     `json:"suspect,omitempty"`
	ForcedExcluded     int     `json:"forced_excluded"`
	BrandCheckExcluded int     `json:"brand_check_excluded"`
	Mentioned          int     `json:"mentioned"`
	Recommended        int     `json:"recommended"`
	Linked             int     `json:"linked"`
	MentionRate        float64 `json:"mention_rate"`
	RecommendRate      float64 `json:"recommend_rate"`
	LinkRate           float64 `json:"link_rate"`
	CitationRate       float64 `json:"citation_rate"`
	ShareOfVoice       float64 `json:"share_of_voice"`
	ConfusionCount     int     `json:"confusion_count"`
	// SampleNote 在分母为 0 时给出「样本不足」，避免把无数据显示成 0%（PRD DoD 3）。
	SampleNote string `json:"sample_note,omitempty"`
}

// ModelMetricSet 是逐模型指标。不合并成综合 AI 分数（F-305）。
type ModelMetricSet struct {
	Platform      AIModelID `json:"platform"`
	Metrics       MetricSet `json:"metrics"`
	Positive      int       `json:"positive"`
	Neutral       int       `json:"neutral"`
	Negative      int       `json:"negative"`
	SearchOn      int       `json:"search_on"`
	SearchOff     int       `json:"search_off"`
	SearchUnknown int       `json:"search_unknown"`
	SearchAuto    int       `json:"search_auto"`
}

// SOVRow 是竞品 Share of Voice 的一行，附 run_id 证据。
type SOVRow struct {
	Brand    string   `json:"brand"`
	Mentions int      `json:"mentions"`
	Share    float64  `json:"share"`
	RunIDs   []string `json:"run_ids"`
}

// CitationGroup 是 Citation Source Map 的一组，附可下钻的 run_id 证据（F-304）。
type CitationGroup struct {
	SourceClass SourceClass      `json:"source_class"`
	Count       int              `json:"count"`
	Domains     []string         `json:"domains"`
	Evidence    []CitationSample `json:"evidence"`
}

// CitationSample 是一条可点击下钻的引用证据。
type CitationSample struct {
	RunID    string `json:"run_id"`
	URL      string `json:"url"`
	Intent   string `json:"intent"`
	IsDirect bool   `json:"is_direct"`
}

// ExecutiveSummary 是报告第一节的执行摘要（F-501.1），固定三段：现状、差距、下一步。
type ExecutiveSummary struct {
	Current string `json:"current"`
	Gap     string `json:"gap"`
	Next    string `json:"next"`
}

// Visibility 是品牌可见性接口（F-305）的返回：Buyer Question 汇总 + 逐模型指标。
// 刻意不合并成单一 AI 分数（DoD 6）。
type Visibility struct {
	BrandID string                       `json:"brand_id"`
	Buyer   MetricSet                    `json:"buyer"`
	Models  map[AIModelID]ModelMetricSet `json:"models"`
}

// Comparison 是 matched comparison：只比较两次都 valid 的问题子集（F-208 / §5.4）。
type Comparison struct {
	MatchedQuestions int     `json:"matched_questions"`
	BeforeUsable     int     `json:"before_usable"`
	AfterUsable      int     `json:"after_usable"`
	BeforeMentioned  int     `json:"before_mentioned"`
	AfterMentioned   int     `json:"after_mentioned"`
	BeforeRate       float64 `json:"before_rate"`
	AfterRate        float64 `json:"after_rate"`
	Note             string  `json:"note"`
}

// NameConfusion 是名称混淆的单独一节（F-202 / §5.8）。
type NameConfusion struct {
	RunID        string `json:"run_id"`
	ConfusedWith string `json:"confused_with"`
	Excerpt      string `json:"excerpt"`
}

// ---- 站点就绪度契约（F-4xx）----

type CheckStatus string

const (
	CheckPass    CheckStatus = "pass"
	CheckPartial CheckStatus = "partial"
	CheckFail    CheckStatus = "fail"
	// CheckUnknown 用于无法判定的检查项：不进分母、不扣分（A-WP-2.2）。
	CheckUnknown CheckStatus = "unknown"
)

type AuditGroup string

const (
	GroupCrawlability AuditGroup = "crawlability"
	GroupStructured   AuditGroup = "structured_data"
	GroupContent      AuditGroup = "content_signals"
	GroupHealth       AuditGroup = "basic_health"
)

// AuditGroupWeights 是 /100 分组权重。
// A-WP-2.3 权重评审结论：内容信号 25 拆为「内容 15 + 索引准入 10」，总和仍为 100。
// 该表是唯一配置入口：把 AdmissionWeights.Enabled 设为 false 即可整组下线。
var AuditGroupWeights = map[AuditGroup]float64{
	GroupCrawlability: 30,
	GroupStructured:   30,
	GroupContent:      15,
	GroupHealth:       15,
	GroupAdmission:    10,
}

// AuditCheck 是单个检查项，必须带证据与修复说明。
// Δ-03：platform_hint 只记平台提示文案（仅线索，不作 pass/fail 依据），
// observed_evidence 是实际观察证据；两者独立字段、互不填充（EX-08 / TC-16-12）。
type AuditCheck struct {
	ID       string      `json:"id"`
	Group    AuditGroup  `json:"group"`
	Label    string      `json:"label"`
	Status   CheckStatus `json:"status"`
	Evidence string      `json:"evidence"`
	Fix      string      `json:"fix"`

	PlatformHint         string               `json:"platform_hint,omitempty"`
	ObservedEvidence     string               `json:"observed_evidence,omitempty"`
	EvidenceSource       EvidenceSource       `json:"evidence_source,omitempty"`
	CrawlerVisitEvidence CrawlerVisitEvidence `json:"crawler_visit_evidence,omitempty"`
}

// GroupScore 是分组得分。
type GroupScore struct {
	Group  AuditGroup `json:"group"`
	Label  string     `json:"label"`
	Weight float64    `json:"weight"`
	Earned float64    `json:"earned"`
	Score  float64    `json:"score"`
}

// SiteScore 是站点就绪度 /100。它与 AI 可见性分始终分栏并列，不合成综合分（F-404）。
type SiteScore struct {
	URL       string       `json:"url"`
	Score     float64      `json:"score"`
	Checks    []AuditCheck `json:"checks"`
	Groups    []GroupScore `json:"groups"`
	TopFix    string       `json:"top_fix"`
	Caveat    string       `json:"caveat"`
	Source    string       `json:"source"`
	AuditedAt time.Time    `json:"audited_at"`

	// A-WP-1.4：任何含数字的组件都必须可查来源类型与方法论版本。
	Provenance Provenance `json:"provenance"`
	// A-WP-1.1：免费检查不要求登录，档案状态默认 unknown，不扣分。
	ProfileStatus ProfileStatus `json:"profile_status"`
	// A-WP-2 / 9.2：索引准入 8 类结论，每条带证据、修复说明、风险提示与来源链接。
	Admission []AdmissionCheck `json:"admission,omitempty"`
	// Δ-03：第 9 项爬虫访问证据分行结论（flag off 时整体缺失）。
	CrawlerVisit *CrawlerVisitRow `json:"crawler_visit,omitempty"`
}

// CrawlerVisitRow 是 Δ-03 / EX-07/EX-08 的第 9 项结论。
// PlatformHint 与 ObservedEvidence 是两条独立字段，互不填充。
type CrawlerVisitRow struct {
	Verdict          string `json:"verdict"`
	PlatformHint     string `json:"platform_hint"`
	ObservedEvidence string `json:"observed_evidence"`
	EvidenceSource   string `json:"evidence_source"`
	Disabled         bool   `json:"disabled,omitempty"`
	FixAdvice        string `json:"fix_advice,omitempty"`
}

// ---- 报告与交付件契约（F-5xx）----

type ArtifactType string

const (
	ArtifactLLMsTxt    ArtifactType = "llms_txt"
	ArtifactJSONLD     ArtifactType = "json_ld"
	ArtifactComparison ArtifactType = "comparison_page"
	ArtifactFAQ        ArtifactType = "faq_page"
	ArtifactRobots     ArtifactType = "robots_txt"
)

// FixArtifact 是可直接粘贴/下载的成品文件，必须带放置路径与验证方式（F-503 / DoD 7）。
type FixArtifact struct {
	Type       ArtifactType `json:"type"`
	Filename   string       `json:"filename"`
	TargetPath string       `json:"target_path"`
	Validation string       `json:"validation"`
	Content    string       `json:"content"`
}

// FixItem 是优先级修复清单的一条（F-502）。
type FixItem struct {
	Priority      string         `json:"priority"`
	Problem       string         `json:"problem"`
	Evidence      []string       `json:"evidence"`
	Change        string         `json:"change"`
	TargetMetric  string         `json:"target_metric"`
	Acceptance    string         `json:"acceptance"`
	ArtifactTypes []ArtifactType `json:"artifact_types"`
}

// DiagnosticReport 是报告固定 7 节（F-501）。站点就绪度与 AI 可见性是两个独立字段。
// Δ 批次追加：counts 四数、language_notice 跨语言并列观察、reference_only 独立参考区。
// 渲染层需同时支持 1.0（三数，缺 suspect 按 0 展示并标注「旧口径」）与 1.1（§7）。
type DiagnosticReport struct {
	MethodologyVersion string                       `json:"methodology_version"`
	BrandID            string                       `json:"brand_id"`
	BrandName          string                       `json:"brand_name"`
	GeneratedAt        time.Time                    `json:"generated_at"`
	ExecutiveSummary   ExecutiveSummary             `json:"executive_summary"`
	SiteReadiness      *SiteScore                   `json:"site_readiness"`
	AIVisibility       MetricSet                    `json:"ai_visibility"`
	BuyerQuestions     map[QueryIntent]MetricSet    `json:"buyer_questions"`
	BrandCheck         MetricSet                    `json:"brand_check"`
	ModelBreakdown     map[AIModelID]ModelMetricSet `json:"model_breakdown"`
	CompetitorSOV      []SOVRow                     `json:"competitor_sov"`
	CitationMap        []CitationGroup              `json:"citation_map"`
	NameConfusion      []NameConfusion              `json:"name_confusion"`
	FixList            []FixItem                    `json:"fix_list"`
	Artifacts          []FixArtifact                `json:"artifacts"`
	Evidence           []RunAnswer                  `json:"evidence"`
	Comparison         *Comparison                  `json:"comparison,omitempty"`
	MethodologyNotes   []string                     `json:"methodology_notes"`

	Counts         *Counts              `json:"counts,omitempty"`
	LanguageNotice string               `json:"language_notice,omitempty"`
	ReferenceOnly  []UserProvidedMetric `json:"reference_only,omitempty"`
	// LegacyCounts 标注 1.0 口径的历史报告（缺 suspect），禁止静默重算。
	LegacyCounts bool `json:"legacy_counts,omitempty"`
}
