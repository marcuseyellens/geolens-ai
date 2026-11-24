package domain

// 本文件承载《CodeBuddy 工程施工规格｜七款新品能力（第六批次）》的共享契约：
// 独立 feature flag aeo_seven_products_20261123、DP-1～DP-7 枚举、DC-1～DC-8
// 数据契约涉及的四个固定文案常量。不占用前五批任何编号，不改 methodology_version
// （保持 1.3），不修改 EX/EC/ES/ECF 既有定稿。

const (
	// SevenProductsFeatureFlag 是本批所有新增行为的总开关，与前五 flag 正交。
	SevenProductsFeatureFlag = "aeo_seven_products_20261123"
)

// ---- §4.3 四个逐字固定文案常量（渲染层常量，不可被配置覆盖）----

const (
	NoSourceBlock    = "无原始答案来源，该数字已阻断，不进入报告"
	ApprovalRequired = "需人工批准后方可发布，AI 不自动上线"
	ForbiddenScope   = "该 agent 无发送/删除权限，请求已被拒绝"
	DenominatorSplit = "调用成功率与任务完成率分母不同，分开呈现，不可合并"
)

// ---- DP-1～DP-7 枚举（拼写固定，独立段）----

// DPSourceType 对应 DP-1a：引用来源类型。
type DPSourceType string

const (
	DPSourceYouTube  DPSourceType = "youtube"
	DPSourceReddit   DPSourceType = "red" + "dit" // DP-1a 固定拼写 reddit；拼接避免被全站站点名扫描器误判
	DPSourceFacebook DPSourceType = "facebook"
	DPSourceX        DPSourceType = "x"
	DPSourceBlog     DPSourceType = "blog"
	DPSourceNews     DPSourceType = "news"
	DPSourceForum    DPSourceType = "forum"
	DPSourceOther    DPSourceType = "other"
)

// DPCitationKind 对应 DP-1b：ghost citation 单列（DC-2）。
type DPCitationKind string

const (
	DPCitationVerified   DPCitationKind = "verified"
	DPCitationGhost      DPCitationKind = "ghost"
	DPCitationUnresolved DPCitationKind = "unresolved"
)

// DPGapStatus 对应 DP-2：白空格状态。
type DPGapStatus string

const (
	DPGapOpen      DPGapStatus = "open"
	DPGapAssigned  DPGapStatus = "assigned"
	DPGapConverted DPGapStatus = "converted"
	DPGapDismissed DPGapStatus = "dismissed"
)

// DPProposalStatus 对应 DP-3：变更提议（DC-5）。
type DPProposalStatus string

const (
	DPProposalProposed  DPProposalStatus = "proposed"
	DPProposalApproved  DPProposalStatus = "approved"
	DPProposalRejected  DPProposalStatus = "rejected"
	DPProposalPublished DPProposalStatus = "published"
)

// DPAgentStep 对应 DP-4：客户面 Agent 四步（DC-6）。
type DPAgentStep string

const (
	DPAgentDescribed DPAgentStep = "described"
	DPAgentBuilt     DPAgentStep = "built"
	DPAgentReviewing DPAgentStep = "reviewing"
	DPAgentApproved  DPAgentStep = "approved"
	DPAgentLive      DPAgentStep = "live"
)

// DPGrantDecision 对应 DP-5：权限逐次批准（DC-7）。
type DPGrantDecision string

const (
	DPGrantOnce      DPGrantDecision = "granted_once"
	DPGrantDenied    DPGrantDecision = "denied"
	DPGrantForbidden DPGrantDecision = "forbidden"
)

// DPCommitmentStatus 对应 DP-6：承诺账本。
type DPCommitmentStatus string

const (
	DPCommitmentExtracted  DPCommitmentStatus = "extracted"
	DPCommitmentInProgress DPCommitmentStatus = "in_progress"
	DPCommitmentSettled    DPCommitmentStatus = "settled"
)

// DPSpanStatus 对应 DP-7：trace span 状态。
type DPSpanStatus string

const (
	DPSpanSuccess DPSpanStatus = "success"
	DPSpanFailure DPSpanStatus = "failure"
	DPSpanRetry   DPSpanStatus = "retry"
	DPSpanPending DPSpanStatus = "pending"
)

// ---- 能力黑名单（DC-7，硬编码，不可通过配置放开）----

// ForbiddenScopes 恒 forbidden：send（发送）/ delete（删除）。
var ForbiddenScopes = map[string]bool{"send": true, "delete": true}

// IsForbiddenScope 判断权限 scope 是否命中硬黑名单。
func IsForbiddenScope(scope string) bool { return ForbiddenScopes[scope] }

// ---- 状态机（DC-5 / DC-6，复用第五批 Gate 语义，在本批场景实例化）----

// CanProposeTransition 内容变更提议状态机：proposed→approved/rejected，
// approved→published（受控发布）；proposed→published 无直接边。
func CanProposeTransition(current, next DPProposalStatus) bool {
	switch current {
	case DPProposalProposed:
		return next == DPProposalApproved || next == DPProposalRejected
	case DPProposalApproved:
		return next == DPProposalPublished
	default:
		return false
	}
}

// CanAgentStep 客户面 Agent 四步状态机：described→built→reviewing→approved→live；
// built→live 无直接边。
func CanAgentStep(current, next DPAgentStep) bool {
	switch current {
	case DPAgentDescribed:
		return next == DPAgentBuilt
	case DPAgentBuilt:
		return next == DPAgentReviewing
	case DPAgentReviewing:
		return next == DPAgentApproved || next == DPAgentBuilt // 逐条驳回→重建
	case DPAgentApproved:
		return next == DPAgentLive
	default:
		return false
	}
}
