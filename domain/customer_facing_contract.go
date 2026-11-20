package domain

// 本文件承载《CodeBuddy 工程施工规格｜客户可见版（第五批次）》的共享契约：
// 独立 feature flag、独立枚举段 FE-1～FE-7、批次内契约 FC-1～FC-8、
// 三个逐字固定文案常量。不占用 PRD §5 契约 1～11 / §9 DoD 1～21 编号，
// 不修改 D-1～D-6 / EX / EC / ES 任何既有定稿；口径版本保持 1.3。

const (
	// CustomerFacingFeatureFlag 是本批所有新增行为的总开关。
	// 与 aeo_delta / aeo_competitor / aeo_social_signals 三者正交。
	CustomerFacingFeatureFlag = "aeo_customer_facing_20261120"

	// MethodologyVersionCustomerFacing 固定为 1.3（第五批不改口径）。
	MethodologyVersionCustomerFacing = MethodologyVersionSocial
)

// ---- 三个逐字固定文案常量（§4.3，渲染层常量，不可被配置覆盖）----

const (
	CompareNotice   = "两次独立观察，不证明因果关系。"
	WriteGateNotice = "本操作生成草案，须经人工批准后生效；广告以暂停态上线，财务不自动过账。"
	PeekNotice      = "引用与提名仅为观测，引用不等于答案依据。"
)

// ---- FE-1～FE-7 枚举（拼写固定，独立段）----

type AdCampaignState string

const (
	AdDraft           AdCampaignState = "draft"
	AdPendingApproval AdCampaignState = "pending_approval"
	AdApproved        AdCampaignState = "approved"
	AdPausedLive      AdCampaignState = "paused_live"
	AdLive            AdCampaignState = "live"
	AdRejected        AdCampaignState = "rejected"
)

type ApprovalState string

const (
	ApprovalPending  ApprovalState = "pending"
	ApprovalApproved ApprovalState = "approved"
	ApprovalRejected ApprovalState = "rejected"
	ApprovalEdited   ApprovalState = "edited"
)

type FigureStatus string

const (
	FigureActual     FigureStatus = "actual"
	FigureUnverified FigureStatus = "unverified"
	FigureBudget     FigureStatus = "budget"
	FigureForecast   FigureStatus = "forecast"
	FigureAIEstimate FigureStatus = "ai_estimate"
)

type EvidenceStatus string

const (
	EvidenceMeasured EvidenceStatus = "measured"
	EvidenceClaimed  EvidenceStatus = "claimed"
	EvidenceObserved EvidenceStatus = "observed"
	EvidenceDerived  EvidenceStatus = "derived"
)

type AgentStepKind string

const (
	StepContext AgentStepKind = "context"
	StepCollect AgentStepKind = "collect"
	StepParse   AgentStepKind = "parse"
	StepVerify  AgentStepKind = "verify"
	StepReport  AgentStepKind = "report"
)

type NavKind string

const (
	NavCustomerPage NavKind = "customer_page"
	NavModule       NavKind = "module"
	NavAccount      NavKind = "account"
)

// IntegrationScope 恒为 read_only（FE-7）。
type IntegrationScope string

const IntegrationReadOnly IntegrationScope = "read_only"

// ---- 三态指标（FC-2：分列、不出聚合分）----

// TriState 是品牌可见性三态快照。三卡分列，无任何聚合字段。
type TriState struct {
	Mention        StateMetric `json:"mention"`
	Recommendation StateMetric `json:"recommendation"` // 默认 [未验证]，除非可点回证据
	Link           StateMetric `json:"link"`
}

// StateMetric 是单态指标。强制携带来源与口径（FC-3）。
type StateMetric struct {
	Count              int          `json:"count"`
	NValid             int          `json:"n_valid"`
	Ratio              float64      `json:"ratio"`
	SourceRef          string       `json:"source_ref"`
	FigureStatus       FigureStatus `json:"figure_status"`
	MethodologyVersion string       `json:"methodology_version"`
	Verified           bool         `json:"verified"`
}

// ---- 数字徽章（FC-3）----

// Figure 是任何客户可见数字的载体。缺 source_ref / figure_status 即不渲染。
type Figure struct {
	Label              string       `json:"label"`
	Value              float64      `json:"value"`
	FigureStatus       FigureStatus `json:"figure_status"`
	SourceRef          string       `json:"source_ref"`
	FigureCheckFlag    bool         `json:"figure_check_flag"` // true=无来源，高亮+阻断
	MethodologyVersion string       `json:"methodology_version"`
}

// Valid 判断数字是否满足出数条件（FC-3）。无来源数字不得出报告。
func (f Figure) Valid() bool {
	return f.SourceRef != "" && !f.FigureCheckFlag &&
		f.FigureStatus != "" && f.MethodologyVersion != ""
}

// ---- 写操作状态机（FC-4）----

// CanAdvance 实现 §2.1 统一状态机：draft→pending→approved→终态，
// 任何写操作不得跳过审批直接终态；广告 approved 后只到 paused_live。
func CanAdvance(current, next AdCampaignState) bool {
	switch current {
	case AdDraft:
		return next == AdPendingApproval || next == AdRejected
	case AdPendingApproval:
		return next == AdApproved || next == AdRejected || next == AdDraft
	case AdApproved:
		return next == AdPausedLive // 禁止直接 AdLive
	case AdPausedLive:
		return next == AdLive // 人工复开
	default:
		return false
	}
}
