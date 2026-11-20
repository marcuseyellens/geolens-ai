// Package mcpserver 实现基线 WBS WP-5「answerlens MCP server（最小两工具）」：
// JSON-RPC 2.0 / MCP tools 协议骨架 + Agent token 只读鉴权（5.1）、
// query_visibility（5.2）、get_evidence（5.3）。
//
// 数据源通过接口注入，本包不依赖具体仓储；真实 B2 指标层与 B1 run_answers
// 可在 handlers 层适配进来。协议刻意只暴露两个只读工具，不做导出/告警订阅（§7.2）。
package mcpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/answerlens/answerlens/domain"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid or revoked agent token")
	ErrScopeDenied  = errors.New("token scope denied")
	ErrWriteAttempt = errors.New("agent token is read-only: 写操作被拒绝")
	ErrUnknownTool  = errors.New("unknown tool")
	ErrBadParams    = errors.New("invalid tool arguments")
)

const ServerName = "answerlens"
const ServerVersion = "1.0.0"

// ---- 数据源端口 ----

// VisibilityQuery 是 query_visibility 的入参。
type VisibilityQuery struct {
	BrandID string `json:"brand"`
	Market  string `json:"market"`
	Model   string `json:"model"`
	Region  string `json:"region"`
}

// ModelVisibility 是逐模型一行；三态分列，刻意不提供综合分。
type ModelVisibility struct {
	Model         string            `json:"model"`
	Region        string            `json:"region"`
	NValid        int               `json:"n_valid"`
	Mentioned     int               `json:"mentioned"`
	Recommended   int               `json:"recommended"`
	Linked        int               `json:"linked"`
	MentionRate   float64           `json:"mention_rate"`
	RecommendRate float64           `json:"recommend_rate"`
	LinkRate      float64           `json:"link_rate"`
	SampleNote    string            `json:"sample_note,omitempty"`
	Provenance    domain.Provenance `json:"provenance"`
}

// VisibilityResult 是 agent 友好的结构化摘要：逐模型、三态分列、无综合分。
type VisibilityResult struct {
	BrandID string            `json:"brand_id"`
	Market  string            `json:"market"`
	Models  []ModelVisibility `json:"models"`
	Note    string            `json:"note"`
}

// EvidenceResult 是 get_evidence 的返回：原始答案文本与引用 URL。
type EvidenceResult struct {
	RunID         string            `json:"run_id"`
	QuestionText  string            `json:"question_text"`
	RawAnswerText string            `json:"raw_answer_text"`
	CitationURLs  []string          `json:"citation_urls"`
	ObservedOn    string            `json:"observed_on"`
	Provenance    domain.Provenance `json:"provenance"`
}

// VisibilitySource / EvidenceSource 是可插拔数据源端口。
type VisibilitySource interface {
	QueryVisibility(q VisibilityQuery) (VisibilityResult, error)
}

type EvidenceSource interface {
	GetEvidence(runID string) (EvidenceResult, error)
}

// ---- Agent token 鉴权（WP-5.1）----

type TokenRegistry struct {
	mu     sync.RWMutex
	tokens map[string]domain.AgentToken
}

func NewTokenRegistry() *TokenRegistry {
	return &TokenRegistry{tokens: map[string]domain.AgentToken{}}
}

// Issue 签发只读令牌（scopes 只能取读权限）。
func (r *TokenRegistry) Issue(owner string, scopes ...string) (domain.AgentToken, error) {
	for _, s := range scopes {
		if s != domain.ScopeVisibilityRead && s != domain.ScopeEvidenceRead {
			return domain.AgentToken{}, fmt.Errorf("%w: %s", ErrScopeDenied, s)
		}
	}
	t := domain.AgentToken{
		Token: "agt_" + uuid.NewString(), Owner: owner,
		Scopes: append([]string{}, scopes...), ReadOnly: true, CreatedAt: time.Now(),
	}
	r.mu.Lock()
	r.tokens[t.Token] = t
	r.mu.Unlock()
	return t, nil
}

func (r *TokenRegistry) Revoke(token string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tokens[token]; ok {
		t.Revoked = true
		r.tokens[token] = t
	}
}

func (r *TokenRegistry) Authorize(token, scope string) (domain.AgentToken, error) {
	r.mu.RLock()
	t, ok := r.tokens[token]
	r.mu.RUnlock()
	if !ok || t.Revoked || !t.ReadOnly {
		return domain.AgentToken{}, ErrInvalidToken
	}
	found := false
	for _, s := range t.Scopes {
		if s == scope {
			found = true
			break
		}
	}
	if !found {
		return domain.AgentToken{}, ErrScopeDenied
	}
	return t, nil
}

// ---- JSON-RPC / MCP 协议骨架 ----

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

// Server 是 MCP server 主体。
type Server struct {
	registry   *TokenRegistry
	visibility VisibilitySource
	evidence   EvidenceSource
}

func NewServer(reg *TokenRegistry, v VisibilitySource, e EvidenceSource) *Server {
	return &Server{registry: reg, visibility: v, evidence: e}
}

func (s *Server) Registry() *TokenRegistry { return s.registry }

// Tools 返回两个最小工具定义。
func (s *Server) Tools() []ToolDef {
	return []ToolDef{
		{
			Name:        "query_visibility",
			Description: "按 brand / market / model / region 返回结构化可见性摘要。逐模型、三态分列，无综合分；每个比例伴随 n_valid。",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"brand":  map[string]any{"type": "string"},
					"market": map[string]any{"type": "string"},
					"model":  map[string]any{"type": "string"},
					"region": map[string]any{"type": "string"},
				},
				"required": []string{"brand"},
			},
		},
		{
			Name:        "get_evidence",
			Description: "输入 run_id，返回该次运行的原始答案文本与引用 URL，用于 2 步下钻取证。",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"run_id": map[string]any{"type": "string"}},
				"required":   []string{"run_id"},
			},
		},
	}
}

// Handle 是唯一协议入口：所有请求（含 tools/call）都先做 token 鉴权。
// 非法 token 一律拒绝；token 只读，任何写方法直接拒绝。
func (s *Server) Handle(token string, req Request) Response {
	resp := Response{JSONRPC: "2.0", ID: req.ID}
	if req.JSONRPC != "" && req.JSONRPC != "2.0" {
		resp.Error = &RPCError{Code: -32600, Message: "invalid request"}
		return resp
	}
	if strings.HasPrefix(req.Method, "tools/") || strings.HasPrefix(req.Method, "resources/") || req.Method == "initialize" {
		if _, err := s.registry.Authorize(token, domain.ScopeVisibilityRead); err != nil {
			if _, err2 := s.registry.Authorize(token, domain.ScopeEvidenceRead); err2 != nil {
				resp.Error = &RPCError{Code: -32001, Message: err.Error()}
				return resp
			}
		}
	}
	// 只读约束：任何非读方法（含通知型写入）直接拒绝。
	if isWriteMethod(req.Method) {
		resp.Error = &RPCError{Code: -32003, Message: ErrWriteAttempt.Error()}
		return resp
	}
	switch req.Method {
	case "initialize":
		resp.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]any{"name": ServerName, "version": ServerVersion},
			"capabilities":    map[string]any{"tools": map[string]any{}},
		}
	case "tools/list":
		resp.Result = map[string]any{"tools": s.Tools()}
	case "tools/call":
		result, err := s.callTool(token, req.Params)
		if err != nil {
			resp.Error = &RPCError{Code: -32002, Message: err.Error()}
			return resp
		}
		resp.Result = result
	default:
		resp.Error = &RPCError{Code: -32601, Message: "method not found: " + req.Method}
	}
	return resp
}

func isWriteMethod(m string) bool {
	for _, prefix := range []string{"tools/create", "tools/update", "tools/delete", "resources/write", "prompts/set"} {
		if strings.HasPrefix(m, prefix) {
			return true
		}
	}
	return false
}

type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) callTool(token string, raw json.RawMessage) (any, error) {
	var p callParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, ErrBadParams
	}
	switch p.Name {
	case "query_visibility":
		if _, err := s.registry.Authorize(token, domain.ScopeVisibilityRead); err != nil {
			return nil, err
		}
		var q VisibilityQuery
		if err := json.Unmarshal(p.Arguments, &q); err != nil || strings.TrimSpace(q.BrandID) == "" {
			return nil, ErrBadParams
		}
		if s.visibility == nil {
			return nil, errors.New("visibility source unavailable")
		}
		return s.visibility.QueryVisibility(q)
	case "get_evidence":
		if _, err := s.registry.Authorize(token, domain.ScopeEvidenceRead); err != nil {
			return nil, err
		}
		var a struct {
			RunID string `json:"run_id"`
		}
		if err := json.Unmarshal(p.Arguments, &a); err != nil || strings.TrimSpace(a.RunID) == "" {
			return nil, ErrBadParams
		}
		if s.evidence == nil {
			return nil, errors.New("evidence source unavailable")
		}
		return s.evidence.GetEvidence(a.RunID)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownTool, p.Name)
	}
}

// ---- 本地可运行数据源（接既有指标口径与 run_answers）----

// MemorySource 用注入的逐模型指标与 run 记录提供两个工具的数据。
type MemorySource struct {
	rows map[string][]ModelVisibility
	runs map[string]EvidenceResult
}

func NewMemorySource() *MemorySource {
	return &MemorySource{rows: map[string][]ModelVisibility{}, runs: map[string]EvidenceResult{}}
}

func (m *MemorySource) PutVisibility(brandID string, rows []ModelVisibility) {
	cp := append([]ModelVisibility{}, rows...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Model < cp[j].Model })
	m.rows[brandID] = cp
}

func (m *MemorySource) PutEvidence(e EvidenceResult) { m.runs[e.RunID] = e }

func (m *MemorySource) QueryVisibility(q VisibilityQuery) (VisibilityResult, error) {
	rows, ok := m.rows[q.BrandID]
	if !ok {
		return VisibilityResult{}, fmt.Errorf("brand %s 暂无可见性数据", q.BrandID)
	}
	out := VisibilityResult{
		BrandID: q.BrandID, Market: q.Market, Models: []ModelVisibility{},
		Note: "逐模型三态分列，刻意不提供综合 AI 分数；每个比例字段伴随 n_valid。仅代表本次采样。",
	}
	for _, r := range rows {
		if q.Model != "" && r.Model != q.Model {
			continue
		}
		if q.Region != "" && r.Region != q.Region {
			continue
		}
		out.Models = append(out.Models, r)
	}
	return out, nil
}

func (m *MemorySource) GetEvidence(runID string) (EvidenceResult, error) {
	e, ok := m.runs[runID]
	if !ok {
		return EvidenceResult{}, fmt.Errorf("run %s 不存在", runID)
	}
	return e, nil
}
