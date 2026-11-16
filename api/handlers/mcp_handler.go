package handlers

import (
	"net/http"
	"strings"

	"github.com/geolens/geolens/domain"
	"github.com/geolens/geolens/internal/mcpserver"
	"github.com/gin-gonic/gin"
)

// mcpEndpoint 是 WP-5 的 streamable HTTP / JSON-RPC 端点。
// 鉴权在 mcpserver.Handle 内部强制：Bearer token 只读，非法/撤销 token 拒绝。
func (s *Server) mcpEndpoint(c *gin.Context) {
	if s.mcp == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "mcp server not configured"})
		return
	}
	auth := c.GetHeader("Authorization")
	token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	if auth != "" && !strings.HasPrefix(auth, "Bearer ") {
		token = ""
	}
	var req mcpserver.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"jsonrpc": "2.0",
			"error":   gin.H{"code": -32700, "message": "parse error"},
		})
		return
	}
	resp := s.mcp.Handle(token, req)
	status := http.StatusOK
	if resp.Error != nil {
		switch resp.Error.Code {
		case -32001, -32003:
			status = http.StatusUnauthorized
		case -32601, -32002:
			status = http.StatusBadRequest
		}
	}
	c.JSON(status, resp)
}

// mcpIssueToken 供管理员签发只读 Agent token（WP-5.1）。
// 挂在 admin 路由组上：登录后的 admin 角色或 X-Admin-Token 均可通过。
func (s *Server) mcpIssueToken(c *gin.Context) {
	if s.mcp == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "mcp server not configured"})
		return
	}
	if !s.requireAdmin(c) {
		return
	}
	var body struct {
		Owner  string   `json:"owner"`
		Scopes []string `json:"scopes"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式不正确"})
		return
	}
	if len(body.Scopes) == 0 {
		body.Scopes = []string{domain.ScopeVisibilityRead, domain.ScopeEvidenceRead}
	}
	tok, err := s.mcp.Registry().Issue(body.Owner, body.Scopes...)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token": tok, "read_only": true,
		"tools":      []string{"query_visibility", "get_evidence"},
		"provenance": domain.ObservedProvenance(2),
	})
}

// mcpInfo 返回 MCP 接入说明，供 Claude Code 配置与 A-WP-4.3 自查文档对齐。
func (s *Server) mcpInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"server":      mcpserver.ServerName,
		"version":     mcpserver.ServerVersion,
		"endpoint":    "/mcp",
		"auth":        "Authorization: Bearer <agent_token>",
		"tools":       []string{"query_visibility", "get_evidence"},
		"read_only":   true,
		"issue_token": "POST /api/v1/mcp/tokens (X-Admin-Token)",
		"methodology": domain.MethodologyVersion,
		"note":        "逐模型三态分列，无综合分；每个比例字段伴随 n_valid。",
	})
}
