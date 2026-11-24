package sevenproducts

import (
	"testing"

	"github.com/answerlens/answerlens/domain"
)

// TC-D-01/02/03：引用溯源 + ghost 分母
func TestTCD01_CitationAnswerRef(t *testing.T) {
	if !(Citation{AnswerRef: "ans://123"}).Valid() {
		t.Fatal("有 answer_ref 的引用应有效")
	}
	if (Citation{}).Valid() {
		t.Fatal("缺 answer_ref 的引用应无效（NO_SOURCE_BLOCK）")
	}
}

func TestTCD04_GhostRateSeparate(t *testing.T) {
	cits := []Citation{
		{AnswerRef: "a", Kind: domain.DPCitationVerified},
		{AnswerRef: "b", Kind: domain.DPCitationGhost},
		{AnswerRef: "c", Kind: domain.DPCitationVerified},
		{AnswerRef: "d", Kind: domain.DPCitationGhost},
	}
	if got := GhostRate(cits); got != 0.5 {
		t.Fatalf("ghost 率应 0.5，实得 %v", got)
	}
}

func TestTCD05_NValidExcludesGhost(t *testing.T) {
	cits := []Citation{
		{AnswerRef: "a", Kind: domain.DPCitationVerified},
		{AnswerRef: "b", Kind: domain.DPCitationGhost},
		{AnswerRef: "c", Kind: domain.DPCitationUnresolved},
		{AnswerRef: "", Kind: domain.DPCitationVerified}, // 无来源也不计
	}
	if got := NValid(cits); got != 1 {
		t.Fatalf("n_valid 只含 verified 且有来源，应为 1，实得 %d", got)
	}
	// ghost 数为 0 时不报错
	if NValid([]Citation{{AnswerRef: "a", Kind: domain.DPCitationVerified}}) != 1 {
		t.Fatal("ghost=0 时 n_valid 应正常")
	}
}

// TC-D-07/08：变更提议门禁
func TestTCD07_NoDirectPublish(t *testing.T) {
	g := ApprovalGate{}
	if g.CanTransition(domain.DPProposalProposed, domain.DPProposalPublished) {
		t.Fatal("proposed→published 无直接边（409 approval_required）")
	}
	if !g.CanTransition(domain.DPProposalProposed, domain.DPProposalApproved) {
		t.Fatal("proposed 应可到 approved")
	}
	if g.CanTransition(domain.DPProposalRejected, domain.DPProposalPublished) {
		t.Fatal("rejected 不得发布")
	}
}

func TestTCD08_NonHumanApprove(t *testing.T) {
	g := ApprovalGate{}
	if !g.ApproveAllowed("user:alice") {
		t.Fatal("人批准应允许")
	}
	if g.ApproveAllowed("agent:builder") {
		t.Fatal("非人批准应 403")
	}
	if g.ApproveAllowed("") {
		t.Fatal("空 approver 应 403")
	}
}

// TC-D-09/10：Agent 四步 + never made up
func TestTCD09_LaunchNeedsAllApproved(t *testing.T) {
	g := AgentGate{}
	ins := []Instruction{{Text: "a", SourceBound: true, Approved: true}, {Text: "b", SourceBound: true, Approved: false}}
	if g.LaunchAllowed(domain.DPAgentApproved, ins) {
		t.Fatal("存在未批准指令禁上线（409 instructions_not_approved）")
	}
	if g.CanStep(domain.DPAgentBuilt, domain.DPAgentLive) {
		t.Fatal("built→live 无直接边")
	}
	if !g.CanStep(domain.DPAgentReviewing, domain.DPAgentBuilt) {
		t.Fatal("逐条驳回应可回 built")
	}
}

func TestTCD10_NeverMadeUp(t *testing.T) {
	g := AgentGate{}
	if g.BuildAllowed([]Instruction{{Text: "a", SourceBound: true}, {Text: "b", SourceBound: false}}) {
		t.Fatal("引用不存在 source_binding 应构建失败")
	}
	if !g.BuildAllowed([]Instruction{{Text: "a", SourceBound: true}}) {
		t.Fatal("全部有来源应构建成功")
	}
}

// TC-D-11/12：黑名单 + 账户隐藏
func TestTCD11_ForbiddenScopes(t *testing.T) {
	if GrantDecision("send", true) != domain.DPGrantForbidden {
		t.Fatal("send 恒 forbidden")
	}
	if GrantDecision("delete", true) != domain.DPGrantForbidden {
		t.Fatal("delete 恒 forbidden")
	}
	if GrantDecision("search", true) != domain.DPGrantOnce {
		t.Fatal("search 人批准应 granted_once")
	}
	if GrantDecision("search", false) != domain.DPGrantDenied {
		t.Fatal("search 未批准应 denied")
	}
}

// TC-D-14~17：双分母 + 人工确认 + execution ID + 版本完整
func TestTCD14_DualDenominators(t *testing.T) {
	spans := []Span{
		{Status: domain.DPSpanSuccess, EvidenceRef: "e", ExecutionID: "x"},
		{Status: domain.DPSpanFailure, EvidenceRef: "e", ExecutionID: "x"},
	}
	traces := []Trace{{ExecutionID: "x", PromptVersion: "p1", ModelVersion: "m1", TaskSuccess: true}}
	d := ComputeDenominators(spans, traces)
	if d.CallSuccessRate != 0.5 || d.TaskCompletionRate != 1.0 {
		t.Fatalf("两分母应分列 0.5/1.0，实得 %v", d)
	}
}

func TestTCD16_ExecutionIDSpan(t *testing.T) {
	spans := []Span{{SpanType: "model", Status: domain.DPSpanSuccess, EvidenceRef: "e", ExecutionID: "exec-1"}}
	traces := []Trace{{ExecutionID: "exec-1", PromptVersion: "p", ModelVersion: "m"}}
	if !ReplayAllowed(spans, traces) {
		t.Fatal("同 execution ID 且证据完整应可回放")
	}
}

func TestTCD17_MetadataRequired(t *testing.T) {
	spans := []Span{{EvidenceRef: "e"}}
	if ReplayAllowed(spans, []Trace{{PromptVersion: "", ModelVersion: "m"}}) {
		t.Fatal("prompt_version 缺失应禁回放（INV-8）")
	}
}

// TC-D-18：evidence_ref 空禁回放
func TestTCD18_NoMockReplay(t *testing.T) {
	spans := []Span{{EvidenceRef: ""}}
	if ReplayAllowed(spans, []Trace{{PromptVersion: "p", ModelVersion: "m"}}) {
		t.Fatal("evidence_ref 空应禁回放（禁 mock）")
	}
}

// TC-D-19：flag 隔离
func TestTCD19_FlagIsolation(t *testing.T) {
	f := NewFlag(false)
	if f.Enabled() || f.AdSpotsEnabled() {
		t.Fatal("flag off 应全关")
	}
	f.SetEnabled(true)
	if !f.Enabled() || !f.AdSpotsEnabled() {
		t.Fatal("flag on 应全开")
	}
	f.SetAdSpotsEnabled(false)
	if !f.Enabled() || f.AdSpotsEnabled() {
		t.Fatal("广告位子集应可独立关，不影响总开关")
	}
	if f.Name() != domain.SevenProductsFeatureFlag {
		t.Fatal("flag 名不符")
	}
}
