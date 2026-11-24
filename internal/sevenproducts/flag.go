// Package sevenproducts 承载第六批（七款新品能力 EXD-01～14）的横切执行层：
// 独立 flag、能力黑名单、双分母、ghost 分母排除、never-made-up 校验、
// 状态机门禁与数字阻断。与第五批 customerfacing 正交，不修改其任何文件。
package sevenproducts

import (
	"sync"

	"github.com/answerlens/answerlens/domain"
)

// Flag 是本批总开关。默认 on；GEOLENS_FLAG_AEO_SEVEN_PRODUCTS_20261123=off
// 秒级关闭，行为回到第五批客户可见版，不影响前五 flag。
type Flag struct {
	mu      sync.RWMutex
	enabled bool
	// adSpotsEnabled 是 NF-43 AI 广告位 flag 子集，可独立关闭。
	adSpotsEnabled bool
}

func NewFlag(enabled bool) *Flag {
	return &Flag{enabled: enabled, adSpotsEnabled: enabled}
}

func (f *Flag) Enabled() bool {
	if f == nil {
		return false
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.enabled
}

func (f *Flag) SetEnabled(v bool) {
	if f == nil {
		return
	}
	f.mu.Lock()
	f.enabled = v
	f.adSpotsEnabled = v
	f.mu.Unlock()
}

// AdSpotsEnabled 返回 AI 广告位子集开关（flag 总关时必为 false）。
func (f *Flag) AdSpotsEnabled() bool {
	if f == nil || !f.Enabled() {
		return false
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.adSpotsEnabled
}

func (f *Flag) SetAdSpotsEnabled(v bool) {
	if f == nil {
		return
	}
	f.mu.Lock()
	f.adSpotsEnabled = v
	f.mu.Unlock()
}

func (f *Flag) Name() string { return domain.SevenProductsFeatureFlag }

// ---- DC-2 / INV-1：ghost citation 不计入有效分母 ----

// Citation 是单条引用来源（DC-1：answer_ref 必填）。
type Citation struct {
	AnswerRef     string
	Kind          domain.DPCitationKind
	VerbatimExact bool // 原文逐字一致（TC-D-03）
}

// Valid 引用可溯源：answer_ref 非空即通过（DC-1）；原文逐字由采集层保证。
func (c Citation) Valid() bool { return c.AnswerRef != "" }

// NValid 计算有效提及分母：只含 verified，ghost / unresolved 不计入（DC-2 / INV-1）。
func NValid(citations []Citation) int {
	n := 0
	for _, c := range citations {
		if c.Kind == domain.DPCitationVerified && c.Valid() {
			n++
		}
	}
	return n
}

// GhostRate 计算 ghost citation 率（独立指标，不并入可见度）：ghost / 总数。
func GhostRate(citations []Citation) float64 {
	if len(citations) == 0 {
		return 0
	}
	ghost := 0
	for _, c := range citations {
		if c.Kind == domain.DPCitationGhost {
			ghost++
		}
	}
	return float64(ghost) / float64(len(citations))
}

// ---- DC-5：变更提议门禁（proposed→published 无直接边）----

// ApprovalGate 内容变更提议状态机门禁。
type ApprovalGate struct{}

func (ApprovalGate) CanTransition(current, next domain.DPProposalStatus) bool {
	return domain.CanProposeTransition(current, next)
}

// ApproveAllowed 人批准：approver 非空且非 agent 标识即允许；否则 403。
func (ApprovalGate) ApproveAllowed(approver string) bool {
	return approver != "" && !isAgentActor(approver)
}

func isAgentActor(actor string) bool {
	return len(actor) >= 5 && actor[:5] == "agent"
}

// ---- DC-6：Agent 四步 + never made up ----

// AgentGate Agent 构建状态机门禁。
type AgentGate struct{}

func (AgentGate) CanStep(current, next domain.DPAgentStep) bool {
	return domain.CanAgentStep(current, next)
}

// LaunchAllowed 上线门禁：仅在 approved 步且所有指令已批准时允许（INV-3）。
func (AgentGate) LaunchAllowed(step domain.DPAgentStep, instructions []Instruction) bool {
	if step != domain.DPAgentApproved {
		return false
	}
	for _, ins := range instructions {
		if !ins.Approved {
			return false
		}
	}
	return true
}

// Instruction 是 Agent 一条指令（never made up 校验对象）。
type Instruction struct {
	Text          string
	SourceBound   bool // 引用的页面/文档是否在 source_binding 中存在
	Approved      bool
	EditedByHuman bool
}

// BuildAllowed never made up：所有指令的引用都必须存在 source_binding（TC-D-10）。
func (AgentGate) BuildAllowed(instructions []Instruction) bool {
	for _, ins := range instructions {
		if !ins.SourceBound {
			return false
		}
	}
	return true
}

// ---- DC-7：能力黑名单 + 账户隐藏 ----

// GrantDecision 权限决策：send/delete 恒 forbidden（INV-4）。
func GrantDecision(scope string, humanApproved bool) domain.DPGrantDecision {
	if domain.IsForbiddenScope(scope) {
		return domain.DPGrantForbidden
	}
	if humanApproved {
		return domain.DPGrantOnce
	}
	return domain.DPGrantDenied
}

// ---- DC-8：双分母 + 人工确认 + execution ID + 禁 mock ----

// DualDenominators 两个分母分列，禁止合成字段。
type DualDenominators struct {
	CallSuccessRate    float64 `json:"call_success_rate"`
	TaskCompletionRate float64 `json:"task_completion_rate"`
}

// ComputeDenominators 分别计算调用成功率与任务完成率（分母不同，禁止合并）。
func ComputeDenominators(spans []Span, traces []Trace) DualDenominators {
	var callTotal, callSuccess int
	for _, s := range spans {
		callTotal++
		if s.Status == domain.DPSpanSuccess {
			callSuccess++
		}
	}
	var taskTotal, taskDone int
	for _, t := range traces {
		taskTotal++
		if t.TaskSuccess {
			taskDone++
		}
	}
	var callRate, taskRate float64
	if callTotal > 0 {
		callRate = float64(callSuccess) / float64(callTotal)
	}
	if taskTotal > 0 {
		taskRate = float64(taskDone) / float64(taskTotal)
	}
	return DualDenominators{CallSuccessRate: callRate, TaskCompletionRate: taskRate}
}

// Span 是 trace span（evidence_ref 空 ⇒ 不可回放，INV-6）。
type Span struct {
	SpanType    string
	Status      domain.DPSpanStatus
	EvidenceRef string
	ExecutionID string
}

// Trace 是 agent trace（prompt_version / model_version 非空，INV-8）。
type Trace struct {
	ExecutionID   string
	PromptVersion string
	ModelVersion  string
	TaskSuccess   bool
}

// ReplayAllowed 回放门禁：任一 span 缺 evidence_ref 或 trace 缺版本即禁回放。
func ReplayAllowed(spans []Span, traces []Trace) bool {
	for _, t := range traces {
		if t.PromptVersion == "" || t.ModelVersion == "" {
			return false
		}
	}
	for _, s := range spans {
		if s.EvidenceRef == "" {
			return false
		}
	}
	return true
}
