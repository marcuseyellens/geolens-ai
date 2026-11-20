package domain

import "time"

// 本文件承载《WBS_AnswerLens_MustHave_v1.0》基线 WP-3/5/7/8 的共享契约。
// 设计原则沿用增量 WBS：可插拔端口 + 本地可运行实现，外部系统（Stripe /
// Offstage / Omnigraph / 真实桌面端）通过接口替换，不在本仓库内硬编码凭据。

// ---- WP-3.1 成本分级路由 ----

// TaskKind 区分采集执行与判定任务，路由到不同成本通道。
type TaskKind string

const (
	TaskCollect        TaskKind = "collect"         // 采集执行：低成本通道
	TaskJudgeMention   TaskKind = "judge_mention"   // 三态判定：强模型通道
	TaskJudgeRecommend TaskKind = "judge_recommend" // 三态判定：强模型通道
	TaskJudgeLink      TaskKind = "judge_link"      // 三态判定：强模型通道
	TaskGenerateFix    TaskKind = "generate_fix"    // 修复件生成：低成本通道
)

// CostChannel 是路由表里的通道标识。
type CostChannel string

const (
	ChannelLowCost     CostChannel = "low_cost"     // 采集 / 生成：便宜、快
	ChannelStrongModel CostChannel = "strong_model" // 判定：质量优先，不得降级
)

// RouteRule 是一条可配置路由规则。EstimatedCost 是单次运行的预估成本（美元）。
type RouteRule struct {
	Task          TaskKind    `json:"task"`
	Channel       CostChannel `json:"channel"`
	Model         string      `json:"model"`
	EstimatedCost float64     `json:"estimated_cost"`
	// QualityCritical 为 true 时，即使预算紧张也不能切到低成本通道（同批次判定质量不降级）。
	QualityCritical bool `json:"quality_critical"`
}

// DefaultRouteTable 是默认分级路由表：采集走低成本、三态判定走强模型。
var DefaultRouteTable = []RouteRule{
	{Task: TaskCollect, Channel: ChannelLowCost, Model: "collector-batch-v1", EstimatedCost: 0.002},
	{Task: TaskJudgeMention, Channel: ChannelStrongModel, Model: "judge-strong-v1", EstimatedCost: 0.02, QualityCritical: true},
	{Task: TaskJudgeRecommend, Channel: ChannelStrongModel, Model: "judge-strong-v1", EstimatedCost: 0.02, QualityCritical: true},
	{Task: TaskJudgeLink, Channel: ChannelStrongModel, Model: "judge-strong-v1", EstimatedCost: 0.02, QualityCritical: true},
	{Task: TaskGenerateFix, Channel: ChannelLowCost, Model: "fix-small-v1", EstimatedCost: 0.003},
}

// ---- WP-3.2 运行五元组 ----

// RunPentad 是每次运行的五元组：发起者 / 任务 / 变更 / 成本 / 结局（WP-3.2）。
// 任一 run 可查五元组；审计页按 Operator 汇总。
type RunPentad struct {
	RunID    string      `json:"run_id"`
	Operator string      `json:"operator"` // 发起者：用户 ID / collector / scheduler
	Task     TaskKind    `json:"task"`     // 任务
	Change   string      `json:"change"`   // 变更：本次运行改了什么（配置/数据版本/无）
	Cost     float64     `json:"cost"`     // 成本（与 F-107 一致）
	Outcome  string      `json:"outcome"`  // 结局：ok / failed / budget_tripped / rate_limited
	Channel  CostChannel `json:"channel"`
	At       time.Time   `json:"at"`
	// RetryOf 非空表示这是一次重试，用于证明「重试不得绕过熔断」。
	RetryOf string `json:"retry_of,omitempty"`
}

// ---- WP-3.3 预算熔断 ----

// BudgetState 是预算状态快照。
type BudgetState struct {
	LimitUSD      float64   `json:"limit_usd"`
	SpentUSD      float64   `json:"spent_usd"`
	Tripped       bool      `json:"tripped"`
	TrippedAt     time.Time `json:"tripped_at,omitempty"`
	RejectedRuns  int       `json:"rejected_runs"`
	TrippedEvents int       `json:"tripped_events"`
}

// ---- WP-3.4 限额 wait-and-resume ----

// RateLimitSignal 表示撞到平台限速窗口（HTTP 429）。
type RateLimitSignal struct {
	Platform   string        `json:"platform"`
	RetryAfter time.Duration `json:"retry_after"`
	ResumeAt   time.Time     `json:"resume_at"`
}

// ---- WP-5.1 Agent token ----

// AgentToken 是 MCP 只读令牌；Scope 只允许读取可见性与证据。
type AgentToken struct {
	Token     string    `json:"token"`
	Owner     string    `json:"owner"`
	Scopes    []string  `json:"scopes"`
	ReadOnly  bool      `json:"read_only"`
	CreatedAt time.Time `json:"created_at"`
	Revoked   bool      `json:"revoked"`
}

const (
	ScopeVisibilityRead = "visibility:read"
	ScopeEvidenceRead   = "evidence:read"
)

// ---- WP-7.4 权益与限流 ----

type PlanID string

const (
	PlanFree    PlanID = "free"
	PlanOneTime PlanID = "one_time"
	PlanRescan  PlanID = "rescan"
	PlanStarter PlanID = "starter"
	PlanGrowth  PlanID = "growth"
	PlanScale   PlanID = "scale"
)

// Entitlement 是某套餐的硬上限；按日免费查询、按月订阅运行配额。
type Entitlement struct {
	Plan             PlanID  `json:"plan"`
	DailyQueries     int     `json:"daily_queries"` // 0 表示不适用
	MonthlyRuns      int     `json:"monthly_runs"`  // 0 表示不适用
	PromptQuota      int     `json:"prompt_quota"`  // CR-v1.1 决策 4：50/300/700 prompt 档位
	Projects         int     `json:"projects"`
	OneTimeQuestions int     `json:"one_time_questions"` // $39 报告题数
	MCPEnabled       bool    `json:"mcp_enabled"`
	APIEnabled       bool    `json:"api_enabled"`
	AlertsEnabled    bool    `json:"alerts_enabled"`
	CustomExport     bool    `json:"custom_export"`
	PriceUSD         float64 `json:"price_usd"`
}

// DefaultEntitlements 对应 WBS §3.7 套餐矩阵；价格沿用现价（CR-v1.1 决策 4：不改价）。
// MonthlyRuns 保留作为内部执行配额，PromptQuota 是客户可见的 prompt 额度档位。
var DefaultEntitlements = map[PlanID]Entitlement{
	PlanFree:    {Plan: PlanFree, DailyQueries: 5, PriceUSD: 0},
	PlanOneTime: {Plan: PlanOneTime, OneTimeQuestions: 30, PriceUSD: 39},
	PlanRescan:  {Plan: PlanRescan, PriceUSD: 19},
	PlanStarter: {Plan: PlanStarter, MonthlyRuns: 1500, PromptQuota: 50, Projects: 1, PriceUSD: 49},
	PlanGrowth:  {Plan: PlanGrowth, MonthlyRuns: 9000, PromptQuota: 300, Projects: 2, MCPEnabled: true, APIEnabled: true, AlertsEnabled: true, CustomExport: true, PriceUSD: 199},
	PlanScale:   {Plan: PlanScale, MonthlyRuns: 21000, PromptQuota: 700, Projects: 5, MCPEnabled: true, APIEnabled: true, AlertsEnabled: true, CustomExport: true, PriceUSD: 410},
}

// QuotaUsage 是配额余量；失败运行口径与五元组一致（失败运行不计）。
type QuotaUsage struct {
	Plan            PlanID `json:"plan"`
	DailyUsed       int    `json:"daily_used"`
	DailyLimit      int    `json:"daily_limit"`
	MonthlyUsed     int    `json:"monthly_used"`
	MonthlyLimit    int    `json:"monthly_limit"`
	FailedNotBilled int    `json:"failed_not_billed"`
	Period          string `json:"period"`
}

// QuotaDecision 是超限时的明确指引，绝不静默超采。
type QuotaDecision struct {
	Allowed   bool   `json:"allowed"`
	Reason    string `json:"reason,omitempty"`
	UpgradeTo PlanID `json:"upgrade_to,omitempty"`
	Message   string `json:"message"`
}

const QuotaUpgradeMessage = "已达到当前套餐的用量上限。运行已停止，请升级套餐后继续；本平台不会静默超采。"

// ---- WP-7.2/7.3 支付与账单 ----

type PaymentKind string

const (
	PayOneTime      PaymentKind = "one_time"
	PaySubscription PaymentKind = "subscription"
	PayAddOn        PaymentKind = "add_on"
)

type PaymentStatus string

const (
	PayPending   PaymentStatus = "pending"
	PaySucceeded PaymentStatus = "succeeded"
	PayFailed    PaymentStatus = "failed"
	PayRefunded  PaymentStatus = "refunded"
)

type Order struct {
	ID        string        `json:"id"`
	AccountID string        `json:"account_id"`
	Plan      PlanID        `json:"plan"`
	Kind      PaymentKind   `json:"kind"`
	AmountUSD float64       `json:"amount_usd"`
	Status    PaymentStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	SettledAt *time.Time    `json:"settled_at,omitempty"`
	// IdempotencyKey 让 Webhook 对账可重放：同一事件重复投递不改变权益。
	IdempotencyKey string `json:"idempotency_key"`
}

// Invoice 是账单历史的一行；FailedRunsNotBilled 对应 PRD §10 默认「不计」。
type Invoice struct {
	ID                  string    `json:"id"`
	AccountID           string    `json:"account_id"`
	Period              string    `json:"period"`
	AmountUSD           float64   `json:"amount_usd"`
	RunsBilled          int       `json:"runs_billed"`
	FailedRunsNotBilled int       `json:"failed_runs_not_billed"`
	CreatedAt           time.Time `json:"created_at"`
}

// RedeemCode 是冷启动一次性兑换码：无支付、无账号即可兑换。
type RedeemCode struct {
	Code       string     `json:"code"`
	Plan       PlanID     `json:"plan"`
	Batch      string     `json:"batch"`
	Redeemed   bool       `json:"redeemed"`
	RedeemedBy string     `json:"redeemed_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	RedeemedAt *time.Time `json:"redeemed_at,omitempty"`
}

// ---- WP-8.2 监控五块看板 ----

// MonitorSnapshot 是五块看板的一次快照 + 阈值告警结果。
type MonitorSnapshot struct {
	CollectionSuccessRate float64        `json:"collection_success_rate"`
	DailyCostUSD          float64        `json:"daily_cost_usd"`
	BudgetTrips           int            `json:"budget_trips"`
	PaymentFailureRate    float64        `json:"payment_failure_rate"`
	FunnelConversion      float64        `json:"funnel_conversion"`
	Alerts                []MonitorAlert `json:"alerts"`
	Provenance            Provenance     `json:"provenance"`
}

// MonitorAlert 是阈值告警；阈值由本平台设定，非平台规则。
type MonitorAlert struct {
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Message   string  `json:"message"`
}

const (
	// WP-8.2 验收：采集有效率 < 95% 告警；成本超日预算告警。
	ThresholdCollectionRate = 0.95
	ThresholdDailyCostUSD   = 50.0
)
