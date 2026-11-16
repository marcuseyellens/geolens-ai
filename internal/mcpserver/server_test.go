package mcpserver

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/geolens/geolens/domain"
)

func rpc(t *testing.T, s *Server, token, method string, params any) Response {
	t.Helper()
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		raw = b
	}
	return s.Handle(token, Request{
		JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: method, Params: raw,
	})
}

func newFixture(t *testing.T) (*Server, *MemorySource, domain.AgentToken) {
	t.Helper()
	reg := NewTokenRegistry()
	tok, err := reg.Issue("claude-code", domain.ScopeVisibilityRead, domain.ScopeEvidenceRead)
	if err != nil {
		t.Fatal(err)
	}
	src := NewMemorySource()
	src.PutVisibility("geolens", []ModelVisibility{
		{
			Model: "chatgpt", Region: "us", NValid: 25,
			Mentioned: 10, Recommended: 5, Linked: 3,
			MentionRate: 0.4, RecommendRate: 0.2, LinkRate: 0.12,
			Provenance: domain.ObservedProvenance(25),
		},
		{
			Model: "perplexity", Region: "us", NValid: 20,
			Mentioned: 8, Recommended: 4, Linked: 6,
			MentionRate: 0.4, RecommendRate: 0.2, LinkRate: 0.3,
			Provenance: domain.ObservedProvenance(20),
		},
	})
	src.PutEvidence(EvidenceResult{
		RunID: "run-abc", QuestionText: "推荐一个 AEO 平台",
		RawAnswerText: "原始答案全文：GeoLens 与竞品并列出现。",
		CitationURLs:  []string{"https://example.com/review"},
		ObservedOn:    "2026-11-14", Provenance: domain.ObservedProvenance(1),
	})
	return NewServer(reg, src, src), src, tok
}

// WP-5.1：非法 token 拒绝；token 只读。
func TestInvalidTokenRejectedAndReadOnly(t *testing.T) {
	s, _, tok := newFixture(t)
	if resp := rpc(t, s, "not-a-token", "tools/list", nil); resp.Error == nil {
		t.Fatal("非法 token 未被拒绝")
	}
	if resp := rpc(t, s, tok.Token, "tools/create", map[string]any{"name": "x"}); resp.Error == nil {
		t.Fatal("只读 token 的写操作未被拒绝")
	}
	// 撤销后拒绝
	s.Registry().Revoke(tok.Token)
	if resp := rpc(t, s, tok.Token, "tools/list", nil); resp.Error == nil {
		t.Error("已撤销 token 未被拒绝")
	}
	// 只读 scope 限制：只有 visibility:read 的 token 不能取证据
	limited, err := s.Registry().Issue("limited", domain.ScopeVisibilityRead)
	if err != nil {
		t.Fatal(err)
	}
	resp := rpc(t, s, limited.Token, "tools/call", map[string]any{
		"name": "get_evidence", "arguments": map[string]any{"run_id": "run-abc"},
	})
	if resp.Error == nil {
		t.Error("缺少 evidence:read scope 的 token 不应能取证据")
	}
}

// WP-5.2 / A-WP-4.3：query_visibility 逐模型、三态分列、无综合分、比例带 n_valid。
func TestQueryVisibilityPerModelNoCombinedScore(t *testing.T) {
	s, _, tok := newFixture(t)
	resp := rpc(t, s, tok.Token, "tools/call", map[string]any{
		"name":      "query_visibility",
		"arguments": map[string]any{"brand": "geolens", "market": "AEO 平台", "region": "us"},
	})
	if resp.Error != nil {
		t.Fatalf("query_visibility 失败: %v", resp.Error)
	}
	b, _ := json.Marshal(resp.Result)
	var out VisibilityResult
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Models) != 2 {
		t.Fatalf("应返回 2 个模型，实际 %d", len(out.Models))
	}
	for _, m := range out.Models {
		if m.NValid <= 0 || m.Provenance.NValid != m.NValid {
			t.Errorf("模型 %s 的比例未伴随 n_valid: %+v", m.Model, m)
		}
		if m.Provenance.SourceType != domain.SourceObserved {
			t.Errorf("模型 %s 缺少来源类型", m.Model)
		}
	}
	low := strings.ToLower(string(b))
	for _, banned := range []string{"overall_score", "combined_score", "综合分"} {
		if strings.Contains(low, banned) {
			t.Errorf("query_visibility 输出不应含综合分字段 %q", banned)
		}
	}
	// 按 model 过滤生效
	filtered := rpc(t, s, tok.Token, "tools/call", map[string]any{
		"name":      "query_visibility",
		"arguments": map[string]any{"brand": "geolens", "model": "chatgpt"},
	})
	fb, _ := json.Marshal(filtered.Result)
	var fout VisibilityResult
	_ = json.Unmarshal(fb, &fout)
	if len(fout.Models) != 1 || fout.Models[0].Model != "chatgpt" {
		t.Errorf("model 过滤未生效: %+v", fout.Models)
	}
}

// WP-5.3 / 5.4：两次调用拿到证据原文；Claude Code 内 ≤ 2 步自查。
func TestGetEvidenceTwoStepSelfCheck(t *testing.T) {
	s, _, tok := newFixture(t)
	// Step 1：查自家可见性
	step1 := rpc(t, s, tok.Token, "tools/call", map[string]any{
		"name": "query_visibility", "arguments": map[string]any{"brand": "geolens"},
	})
	if step1.Error != nil {
		t.Fatalf("第 1 步失败: %v", step1.Error)
	}
	// Step 2：按 run_id 取证据原文与引用 URL
	step2 := rpc(t, s, tok.Token, "tools/call", map[string]any{
		"name": "get_evidence", "arguments": map[string]any{"run_id": "run-abc"},
	})
	if step2.Error != nil {
		t.Fatalf("第 2 步失败: %v", step2.Error)
	}
	b, _ := json.Marshal(step2.Result)
	var ev EvidenceResult
	if err := json.Unmarshal(b, &ev); err != nil {
		t.Fatal(err)
	}
	if ev.RawAnswerText == "" || len(ev.CitationURLs) == 0 {
		t.Error("证据原文或引用 URL 缺失")
	}
	if ev.Provenance.NValid != 1 {
		t.Error("证据缺少口径标注")
	}
	// 不存在的 run_id 明确报错
	if resp := rpc(t, s, tok.Token, "tools/call", map[string]any{
		"name": "get_evidence", "arguments": map[string]any{"run_id": "missing"},
	}); resp.Error == nil {
		t.Error("不存在的 run_id 应明确报错")
	}
	// tools/list 只暴露两个工具
	list := rpc(t, s, tok.Token, "tools/list", nil)
	if list.Error != nil {
		t.Fatal(list.Error)
	}
	lb, _ := json.Marshal(list.Result)
	var def struct {
		Tools []ToolDef `json:"tools"`
	}
	_ = json.Unmarshal(lb, &def)
	if len(def.Tools) != 2 {
		t.Errorf("MCP 应严格限定两工具，实际 %d", len(def.Tools))
	}
}

// 未知工具与坏参数显式拒绝，不静默返回空。
func TestUnknownToolAndBadParams(t *testing.T) {
	s, _, tok := newFixture(t)
	if resp := rpc(t, s, tok.Token, "tools/call", map[string]any{
		"name": "export_all", "arguments": map[string]any{},
	}); resp.Error == nil || !strings.Contains(resp.Error.Message, "unknown tool") {
		t.Errorf("未知工具未拒绝: %+v", resp)
	}
	if resp := rpc(t, s, tok.Token, "tools/call", map[string]any{
		"name": "query_visibility", "arguments": map[string]any{},
	}); resp.Error == nil || !strings.Contains(resp.Error.Message, ErrBadParams.Error()) {
		t.Errorf("缺 brand 应拒绝: %+v", resp)
	}
}
