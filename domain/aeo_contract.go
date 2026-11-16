package domain

import "time"

// 本文件承载《WBS_AEO_Feature_Extraction_v1.0》增量工作包的共享契约。
// 所有含数字的组件必须携带 SourceType / MethodologyVersion / NValid，
// 三类来源不得同表合并（§8 增量发布验收清单第 1、7 条）。

// ---- A-WP-1.4 来源类型全链路 ----

type SourceType string

const (
	// SourceFirstParty 是品牌自述或用户提供的第一方档案，未经平台观测。
	SourceFirstParty SourceType = "first_party"
	// SourceObserved 是平台实际采集到的第三方答案与引用。
	SourceObserved SourceType = "third_party_observed"
	// SourceReported 是外部报告/转述，未经平台本次采集复现。
	SourceReported SourceType = "third_party_reported"
	// SourceUnknown 用于尚不能判定的组件，不得与其他来源合并展示。
	SourceUnknown SourceType = "unknown"
)

func AllSourceTypes() []SourceType {
	return []SourceType{SourceFirstParty, SourceObserved, SourceReported}
}

// Provenance 是任何含数字组件的强制口径标注。
type Provenance struct {
	SourceType         SourceType `json:"source_type"`
	MethodologyVersion string     `json:"methodology_version"`
	NValid             int        `json:"n_valid"`
	// Window 标明采集窗口，避免把一次采样写成平台长期规律。
	Window string `json:"window,omitempty"`
}

func ObservedProvenance(nValid int) Provenance {
	return Provenance{
		SourceType: SourceObserved, MethodologyVersion: MethodologyVersion,
		NValid: nValid,
	}
}

// NeutralityStatement 是落地页与报告 PDF 都必须可见的中立性声明（A-WP-1.5）。
const NeutralityStatement = "本平台只报告可核验的公开事实，不代表任何 AI 平台的官方判定，不保证收录或被推荐。页面质量为概率性风险提示，阈值由本平台设定，非平台规则。"

// ---- A-WP-1.1 第一方档案状态模型 ----

type ProfileStatus string

const (
	ProfileUnknown   ProfileStatus = "unknown"   // 默认态，不扣分
	ProfileUnclaimed ProfileStatus = "unclaimed" // 公开可查但未认领
	ProfileClaimed   ProfileStatus = "claimed"   // 已认领，字段与官方一致
	ProfileStale     ProfileStatus = "stale"     // 已认领但信息过期
	ProfileConflict  ProfileStatus = "conflict"  // 三方字段冲突
)

// ProfileField 是第一方档案的 7 个可比字段（A-WP-1.2）。
const (
	FieldName        = "name"
	FieldWebsite     = "website"
	FieldCategory    = "category"
	FieldDescription = "description"
	FieldAddress     = "address"
	FieldPhone       = "phone"
	FieldHours       = "hours"
)

var ProfileFields = []string{
	FieldName, FieldWebsite, FieldCategory, FieldDescription,
	FieldAddress, FieldPhone, FieldHours,
}

// BusinessProfile 只接受用户自述或公开可查信息；代码无模拟登录态路径。
type BusinessProfile struct {
	BrandID    string            `json:"brand_id"`
	Status     ProfileStatus     `json:"status"`
	Fields     map[string]string `json:"fields"`
	SourceType SourceType        `json:"source_type"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

func NewBusinessProfile(brandID string) BusinessProfile {
	return BusinessProfile{
		BrandID: brandID, Status: ProfileUnknown,
		Fields: map[string]string{}, SourceType: SourceUnknown,
	}
}

type DiffKind string

const (
	DiffMissing  DiffKind = "missing"
	DiffConflict DiffKind = "conflict"
	DiffStale    DiffKind = "stale"
)

// ProfileDiff 是单字段差异，必须附证据片段与官方入口链接（A-WP-1.2/1.3）。
type ProfileDiff struct {
	Field       string   `json:"field"`
	Kind        DiffKind `json:"kind"`
	FirstParty  string   `json:"first_party"`
	ThirdParty  string   `json:"third_party"`
	Excerpt     string   `json:"excerpt"`
	OfficialURL string   `json:"official_url"`
}

// ---- A-WP-2 索引准入检查组 ----

const GroupAdmission AuditGroup = "index_admission"

type AdmissionDetector string

const (
	DetectAntiBotWall    AdmissionDetector = "anti_bot_wall"
	DetectCaptcha        AdmissionDetector = "captcha"
	DetectErrorPage      AdmissionDetector = "error_page"
	DetectThinContent    AdmissionDetector = "thin_content"
	DetectSEOFiller      AdmissionDetector = "seo_filler"
	DetectNavigationOnly AdmissionDetector = "navigation_only"
	DetectUsefulInfo     AdmissionDetector = "useful_information"
	DetectPIIExposure    AdmissionDetector = "pii_exposure"
)

func AllAdmissionDetectors() []AdmissionDetector {
	return []AdmissionDetector{
		DetectAntiBotWall, DetectCaptcha, DetectErrorPage, DetectThinContent,
		DetectSEOFiller, DetectNavigationOnly, DetectUsefulInfo, DetectPIIExposure,
	}
}

// AdmissionStatus 是 4 态；unknown 不进分母、不扣分（§8 第 2 条）。
type AdmissionStatus string

const (
	AdmissionPass    AdmissionStatus = "pass"
	AdmissionPartial AdmissionStatus = "partial"
	AdmissionFail    AdmissionStatus = "fail"
	AdmissionUnknown AdmissionStatus = "unknown"
)

// AdmissionCheck 每条结论都带证据片段、修复说明、风险提示与来源链接。
type AdmissionCheck struct {
	Detector   AdmissionDetector `json:"detector"`
	Status     AdmissionStatus   `json:"status"`
	Evidence   string            `json:"evidence"`
	Fix        string            `json:"fix"`
	RiskNotice string            `json:"risk_notice"`
	SourceURL  string            `json:"source_url"`
	Provenance Provenance        `json:"provenance"`
}

// AdmissionRiskNotice 是页面质量结论的固定风险标识（A-WP-2 专属验收）。
const AdmissionRiskNotice = "风险提示：这是概率性判断，阈值由本平台设定，不代表任何 AI 平台的收录规则。"

// AdmissionWeights 可配置，便于平台口径变化时单配置下线整组（A-WP-2.4）。
var AdmissionWeights = struct {
	Enabled     bool
	GroupWeight float64
	// 每组 detector 可在配置里关掉，关闭后返回 unknown 而非 fail。
	Disabled map[AdmissionDetector]bool
}{
	Enabled:     true,
	GroupWeight: 10,
	Disabled:    map[AdmissionDetector]bool{},
}

// ---- A-WP-5 引用经济属性 ----

type SourceEconomy string

const (
	EconomyUnknown   SourceEconomy = "unknown" // 默认，不进任何聚合
	EconomyOwned     SourceEconomy = "owned"
	EconomyEditorial SourceEconomy = "editorial"
	EconomyPaid      SourceEconomy = "paid"
	EconomyAffiliate SourceEconomy = "affiliate"
	EconomyCommunity SourceEconomy = "community"
)

func AllSourceEconomies() []SourceEconomy {
	return []SourceEconomy{
		EconomyOwned, EconomyEditorial, EconomyPaid, EconomyAffiliate, EconomyCommunity,
	}
}

// SourceSlice 是分模型来源结构的一行，每个占比都必须附 n_valid 与采集窗口。
type SourceSlice struct {
	Model      AIModelID     `json:"model"`
	Region     string        `json:"region"`
	Intent     QueryIntent   `json:"intent"`
	Window     string        `json:"window"`
	Economy    SourceEconomy `json:"source_economy"`
	NValid     int           `json:"n_valid"`
	Share      float64       `json:"share"`
	Provenance Provenance    `json:"provenance"`
}

// ---- A-WP-6 基线学习与突变告警 ----

type BaselinePoint struct {
	BatchID    string   `json:"batch_id"`
	ObservedOn string   `json:"observed_on"`
	Value      float64  `json:"value"`
	Valid      bool     `json:"valid"`
	RunIDs     []string `json:"run_ids"`
}

type AnomalyKind string

const (
	AnomalyRealShift    AnomalyKind = "real_shift"
	AnomalySeasonal     AnomalyKind = "seasonal"
	AnomalyCollectFault AnomalyKind = "collection_fault"
)

type AnomalyAlert struct {
	ID         string      `json:"id"`
	BrandID    string      `json:"brand_id"`
	Model      AIModelID   `json:"model"`
	Region     string      `json:"region"`
	Metric     string      `json:"metric"`
	Kind       AnomalyKind `json:"kind"`
	Deviation  float64     `json:"deviation"`
	RunIDs     []string    `json:"run_ids"`
	NonCausal  string      `json:"non_causal"`
	Folded     bool        `json:"folded"`
	ObservedOn string      `json:"observed_on"`
	Provenance Provenance  `json:"provenance"`
}

const AnomalyNonCausal = "仅显示变化，不证明任何改动导致了变化"

// ---- A-WP-7 问题集与证据链 ----

type QuestionSetType string

const (
	SetMarketBaseline QuestionSetType = "market_baseline"
	SetBrandTracking  QuestionSetType = "brand_tracking"
)

// QuestionBrief 是生成问题集的六要素；缺任一要素拒绝生成。
type QuestionBrief struct {
	Category   string `json:"category"`
	Audience   string `json:"audience"`
	Need       string `json:"need"`
	Region     string `json:"region"`
	Locale     string `json:"locale"`
	Constraint string `json:"constraint"`
}

// IntentQuota 让四类意图占比可配置（A-WP-7.1）。
type IntentQuota struct {
	Recommend   float64 `json:"recommend"`
	Compare     float64 `json:"compare"`
	HowTo       float64 `json:"howto"`
	Alternative float64 `json:"alternative"`
}

func DefaultIntentQuota() IntentQuota {
	return IntentQuota{Recommend: 0.35, Compare: 0.3, HowTo: 0.2, Alternative: 0.15}
}

type GeneratedQuestion struct {
	ID              string          `json:"id"`
	Text            string          `json:"text"`
	Intent          QueryIntent     `json:"intent"`
	QuestionSetType QuestionSetType `json:"question_set_type"`
	IsBrandCheck    bool            `json:"is_brand_check"`
	// Δ-01 / EX-03：BCP-47 问题语言与所属问题集语言，分别落库。
	QueryLocale string `json:"query_locale,omitempty"`
	Locale      string `json:"locale,omitempty"`
}

// DrilldownHop 是三跳下钻的一跳，每跳 ≤ 2 次点击（A-WP-7.3）。
type DrilldownHop struct {
	Label  string `json:"label"`
	Target string `json:"target"`
	Clicks int    `json:"clicks"`
}

// DrilldownPath 是「聚合数字 → 关系/竞品 → 原文 + 页面 URL」的完整路径。
type DrilldownPath struct {
	Metric    string         `json:"metric"`
	Value     float64        `json:"value"`
	NValid    int            `json:"n_valid"`
	Hops      []DrilldownHop `json:"hops"`
	RunID     string         `json:"run_id"`
	RawAnswer string         `json:"raw_answer"`
	PageURL   string         `json:"page_url"`
	Complete  bool           `json:"complete"`
}

// ---- A-WP-3 判定阈值（按任务独立，不用统一 cutoff）----

type JudgementTask string

const (
	TaskMention       JudgementTask = "mention"
	TaskRecommend     JudgementTask = "recommend"
	TaskLink          JudgementTask = "link"
	TaskSentiment     JudgementTask = "sentiment"
	TaskNameConfusion JudgementTask = "name_confusion"
)

// SuspectScore 是 A-WP-3.3 的 suspect 静默打分结果（0-100，重复打分方差需 ≤5）。
type SuspectScore struct {
	Score      float64       `json:"score"`
	Suspect    bool          `json:"suspect"`
	Confidence float64       `json:"confidence"`
	Task       JudgementTask `json:"task"`
}

type TaskThreshold struct {
	Task   JudgementTask `json:"task"`
	Cutoff float64       `json:"cutoff"`
	DevSet string        `json:"dev_set"`
}

// ---- A-WP-8 按请求付费成本核算 ----

type CostKind string

const (
	CostPerRequest CostKind = "per_request"
	CostPerSecond  CostKind = "per_second"
)

// CostBreakdown 把单次 cost 拆成三项，与 Offstage 按秒计费并列写入 F-107。
type CostBreakdown struct {
	Kind           CostKind `json:"kind"`
	ModelCallCost  float64  `json:"model_call_cost"`
	SandboxSeconds float64  `json:"sandbox_seconds"`
	SandboxCost    float64  `json:"sandbox_cost"`
	RetrievalCount int      `json:"retrieval_count"`
	RetrievalCost  float64  `json:"retrieval_cost"`
	Total          float64  `json:"total"`
}

// ---- A-WP-4 agent_readability ----

type AgentReadabilityCheck struct {
	ID         string          `json:"id"`
	Status     AdmissionStatus `json:"status"`
	Evidence   string          `json:"evidence"`
	Fix        string          `json:"fix"`
	Notice     string          `json:"notice"`
	Provenance Provenance      `json:"provenance"`
}

const AgentReadabilityNotice = "模式借鉴，非平台要求；本检查只做提示，不代建资产。"

// SelfCheckID 是 A-WP-9.4 四项自检的标识。
type SelfCheckID string

const (
	SelfCheckA01 SelfCheckID = "A-01" // 第一方档案
	SelfCheckA03 SelfCheckID = "A-03" // 索引准入
	SelfCheckA05 SelfCheckID = "A-05" // Agent 可调用性
	SelfCheckA14 SelfCheckID = "A-14" // 中立性与自举
)

type SelfCheckResult struct {
	ID      SelfCheckID `json:"id"`
	Name    string      `json:"name"`
	Passed  bool        `json:"passed"`
	Details []string    `json:"details"`
}

// ---- A-WP-9.3 套餐结构冻结（不得因本次提取改变）----

var PlanPrices = map[string]float64{
	"free":     0,
	"one_time": 39,
	"rescan":   19,
	"starter":  49,
	"growth":   199,
	"scale":    410,
}
