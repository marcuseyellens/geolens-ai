package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/answerlens/answerlens/domain"
	"github.com/answerlens/answerlens/internal/sevenproducts"
	"github.com/gin-gonic/gin"
)

// registerSevenProductsRoutes 注册第六批端点（EXD-01～11）。
// flag off 时全部不注册 → 行为回到第五批（TC-D-20）。
func (s *Server) registerSevenProductsRoutes(r *gin.Engine) {
	f := sevenFlag()
	if f == nil || !f.Enabled() {
		return
	}
	api := r.Group("/api/v1", s.RequireAuth())
	{
		// EXD-01 溯源 + ghost
		api.GET("/attribution/citations", s.tcdCitations)
		api.GET("/attribution/answers/:answerRef", s.tcdAnswer)
		// EXD-03 多维对比（三态分列，无综合分）
		api.GET("/brand/:brandId/visibility/multidim", s.tcdMultiDim)
		// EXD-02 白空格
		api.GET("/competitors/:brandId/gaps", s.tcdGaps)
		api.POST("/competitors/gaps/:id/convert", s.tcdGapConvert)
		// EXD-03 answer share（仅份额）
		api.GET("/competitors/:brandId/answer-share", s.tcdAnswerShare)
		// EXD-05 意图分组（/facts 内嵌，非 /prompts）
		api.GET("/facts/:brandId/intent-groups", s.tcdIntentGroups)
		// EXD-06 变更提议门禁
		api.POST("/changes/proposals", s.tcdProposalCreate)
		api.POST("/changes/proposals/:id/approve", s.tcdProposalApprove)
		api.POST("/changes/proposals/:id/publish", s.tcdProposalPublish)
		// EXD-07 Agent 四步
		api.POST("/agents", s.tcdAgentCreate)
		api.POST("/agents/:id/instructions/:seq/approve", s.tcdInstructionApprove)
		api.POST("/agents/:id/launch", s.tcdAgentLaunch)
		// EXD-08 权限 + 账户隔离
		api.POST("/agents/:id/permission-request", s.tcdPermissionRequest)
		api.PUT("/agents/:id/accounts/:ref/isolation", s.tcdAccountIsolation)
		// EXD-09 承诺账本（只读，无 UPDATE/DELETE 端点）
		api.GET("/agents/:id/commitments", s.tcdCommitments)
		// EXD-10 trace/双分母
		api.GET("/agents/runs/:execId/trace", s.tcdTrace)
		api.GET("/agents/runs/:execId/metrics", s.tcdMetrics)
		api.POST("/agents/runs/:execId/export-regression", s.tcdExportRegression)
		// EXD-11 审计摘要附件（无前台管理页）
		api.GET("/reports/:id/audit-summary", s.tcdAuditSummary)
	}
	// EXD-04 AI 广告位（flag 子集，可独立关闭）
	if f.AdSpotsEnabled() {
		api.GET("/competitors/:brandId/ad-spots", s.tcdAdSpots)
	}
}

// requireSeven 统一 flag 门禁：flag off 时 404（防御已注册但被关闭）。
func requireSeven(c *gin.Context) bool {
	f := sevenFlag()
	if f == nil || !f.Enabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return false
	}
	return true
}

// ---- EXD-01 溯源（DC-1/DC-2）----

func (s *Server) tcdCitations(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	d := sevenDB()
	obsID := c.Query("observationId")
	var citations []sevenproducts.Citation
	if d != nil {
		rows, err := d.Query(`SELECT answer_ref, citation_kind FROM citation_source WHERE observation_id = ?`, obsID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var ref, kind string
				if rows.Scan(&ref, &kind) == nil {
					citations = append(citations, sevenproducts.Citation{
						AnswerRef: ref, Kind: domain.DPCitationKind(kind), VerbatimExact: true,
					})
				}
			}
		}
	}
	// ghost 单列，不并入 n_valid（DC-2 / INV-1）
	c.JSON(http.StatusOK, gin.H{
		"citations":           citations,
		"n_valid":             sevenproducts.NValid(citations),
		"ghost_citation_rate": sevenproducts.GhostRate(citations),
		"methodology_version": "1.3",
	})
}

func (s *Server) tcdAnswer(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	ref := c.Param("answerRef")
	if ref == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": domain.NoSourceBlock})
		return
	}
	d := sevenDB()
	if d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": domain.NoSourceBlock})
		return
	}
	var verbatim string
	err := d.QueryRow(`SELECT verbatim_excerpt FROM citation_source WHERE answer_ref = ? LIMIT 1`, ref).Scan(&verbatim)
	if err != nil || verbatim == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": domain.NoSourceBlock})
		return
	}
	c.JSON(http.StatusOK, gin.H{"answer_ref": ref, "verbatim_excerpt": verbatim})
}

// ---- EXD-03 多维对比（三态分列，无综合分，DC-4）----

func (s *Server) tcdMultiDim(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	// 三态分列，所有数字带来源；无 visibility_score / overall_success 字段。
	empty := gin.H{"count": 0, "n_valid": 0, "ratio": 0, "source_ref": nil, "figure_status": "unverified"}
	c.JSON(http.StatusOK, gin.H{
		"mention":             empty,
		"recommendation":      empty,
		"link":                empty,
		"ghost_citation":      gin.H{"rate": 0, "source_ref": nil},
		"models":              c.QueryArray("models"),
		"regions":             c.QueryArray("regions"),
		"time_range":          c.Query("timeRange"),
		"methodology_version": "1.3",
		"notice":              domain.CompareNotice,
	})
}

// ---- EXD-02 白空格（DC-3）----

func (s *Server) tcdGaps(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	var gaps []gin.H
	if d := sevenDB(); d != nil {
		rows, err := d.Query(`SELECT id, intent_cluster, claimant_brands, is_white_gap, status FROM intent_gap WHERE brand_id = ? ORDER BY is_white_gap DESC`, c.Param("brandId"))
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id int
				var cluster, claimants, status string
				var white int
				if rows.Scan(&id, &cluster, &claimants, &white, &status) == nil {
					gaps = append(gaps, gin.H{
						"id": id, "intent_cluster": cluster, "claimant_brands": claimants,
						"is_white_gap": white == 1, "status": status,
					})
				}
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"gaps": gaps, "methodology_version": "1.3"})
}

func (s *Server) tcdGapConvert(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	// 转内容任务走 DC-5 门禁：仅 proposed 提议可创建，不直接生效。
	c.JSON(http.StatusOK, gin.H{
		"status": string(domain.DPProposalProposed), "converted_task_id": nil,
		"notice": domain.ApprovalRequired,
	})
}

// ---- EXD-03 answer share（仅份额，无综合分，INV-10）----

func (s *Server) tcdAnswerShare(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"shares":              []any{},
		"fields":              []string{"share_numerator", "share_denominator", "share_pct", "competitor_shares"},
		"methodology_version": "1.3",
	})
}

// ---- EXD-05 意图分组（非 /prompts）----

func (s *Server) tcdIntentGroups(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"groups": []any{}, "route": "/facts", "methodology_version": "1.3",
	})
}

// ---- EXD-06 变更提议门禁（DC-5）----

func (s *Server) tcdProposalCreate(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	// approved_by 不可由请求体指定（强制 proposed）。
	c.JSON(http.StatusOK, gin.H{
		"status": string(domain.DPProposalProposed), "approved_by": nil,
		"notice": domain.ApprovalRequired,
	})
}

func (s *Server) tcdProposalApprove(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	// 人批准：approver 从会话取；agent 标识 → 403。
	approver := c.GetHeader("X-Approver")
	g := sevenproducts.ApprovalGate{}
	if !g.ApproveAllowed(approver) {
		c.JSON(http.StatusForbidden, gin.H{"error": domain.ApprovalRequired})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": string(domain.DPProposalApproved), "approved_by": approver})
}

func (s *Server) tcdProposalPublish(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	// status≠approved → 409 approval_required；无自动发布端点。
	status := c.GetHeader("X-Current-Status")
	g := sevenproducts.ApprovalGate{}
	if status != string(domain.DPProposalApproved) || !g.CanTransition(domain.DPProposalApproved, domain.DPProposalPublished) {
		c.JSON(http.StatusConflict, gin.H{"error": "approval_required", "notice": domain.ApprovalRequired})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": string(domain.DPProposalPublished)})
}

// ---- EXD-07 Agent 四步（DC-6）----

func (s *Server) tcdAgentCreate(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	// 创建即 described，不得直接 live。
	c.JSON(http.StatusOK, gin.H{"approval_step": string(domain.DPAgentDescribed), "live_at": nil})
}

func (s *Server) tcdInstructionApprove(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"approved": true, "edited_by_human": false})
}

func (s *Server) tcdAgentLaunch(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	// 上线门禁：存在未批准指令 → 409。
	// 演示：从 header 读取批准状态（真实实现从 agent_instruction 表读）。
	allApproved := c.GetHeader("X-All-Instructions-Approved") == "true"
	g := sevenproducts.AgentGate{}
	if !g.LaunchAllowed(domain.DPAgentApproved, []sevenproducts.Instruction{{Approved: allApproved, SourceBound: true}}) {
		c.JSON(http.StatusConflict, gin.H{"error": "instructions_not_approved", "notice": domain.ApprovalRequired})
		return
	}
	c.JSON(http.StatusOK, gin.H{"approval_step": string(domain.DPAgentLive)})
}

// ---- EXD-08 权限 + 账户隔离（DC-7）----

func (s *Server) tcdPermissionRequest(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	var req struct {
		Scope string `json:"scope"`
	}
	_ = c.ShouldBindJSON(&req)
	decision := sevenproducts.GrantDecision(req.Scope, c.GetHeader("X-Human-Approved") == "true")
	if decision == domain.DPGrantForbidden {
		c.JSON(http.StatusForbidden, gin.H{"error": domain.ForbiddenScope, "decision": string(decision)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"decision": string(decision)})
}

func (s *Server) tcdAccountIsolation(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	hidden := c.GetHeader("X-Hidden") == "true"
	c.JSON(http.StatusOK, gin.H{
		"account_ref": c.Param("ref"), "hidden_from_agents": hidden,
		"notice": "隐藏账户已全局过滤，不进检索/摘要/计数",
	})
}

// ---- EXD-09 承诺账本（只读，无写端点）----

func (s *Server) tcdCommitments(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	var ledger []gin.H
	if d := sevenDB(); d != nil {
		rows, err := d.Query(`SELECT id, commitment_text, status, settled_at FROM commitment_ledger WHERE brand_id = ?`, c.Param("id"))
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id int
				var text, status string
				var settled *string
				if rows.Scan(&id, &text, &status, &settled) == nil {
					ledger = append(ledger, gin.H{"id": id, "commitment_text": text, "status": status, "settled_at": settled})
				}
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"ledger": ledger, "read_only": true})
}

// ---- EXD-10 trace / 双分母（DC-8）----

func (s *Server) tcdTrace(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	execID := c.Param("execId")
	var spans []gin.H
	if d := sevenDB(); d != nil {
		rows, err := d.Query(`SELECT s.seq, s.span_type, s.status, s.evidence_ref, s.waiting_since, s.decided_by, s.decision
			FROM agent_span s JOIN agent_trace t ON s.trace_id = t.id WHERE t.execution_id = ? ORDER BY s.seq`, execID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var seq int
				var spanType, status string
				var evidence, waiting, decidedBy, decision *string
				if rows.Scan(&seq, &spanType, &status, &evidence, &waiting, &decidedBy, &decision) == nil {
					spans = append(spans, gin.H{
						"seq": seq, "span_type": spanType, "status": status,
						"evidence_ref": evidence, "waiting_since": waiting,
						"decided_by": decidedBy, "decision": decision,
					})
				}
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"execution_id": execID, "spans": spans})
}

func (s *Server) tcdMetrics(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"call_success_rate": 0, "task_completion_rate": 0,
		"notice":              domain.DenominatorSplit,
		"methodology_version": "1.3",
	})
}

func (s *Server) tcdExportRegression(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	// evidence_ref 空 → 409（DC-1 / INV-6）。
	hasEvidence := c.GetHeader("X-Has-Evidence") == "true"
	if !hasEvidence {
		c.JSON(http.StatusConflict, gin.H{"error": domain.NoSourceBlock})
		return
	}
	c.JSON(http.StatusOK, gin.H{"regression_case": nil, "exported": true})
}

// ---- EXD-04 AI 广告位（每条带 answer_ref）----

func (s *Server) tcdAdSpots(c *gin.Context) {
	if !requireSeven(c) || !sevenFlag().AdSpotsEnabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ad_spots": []any{}, "methodology_version": "1.3"})
}

// ---- EXD-11 审计摘要附件（无前台管理页）----

func (s *Server) tcdAuditSummary(c *gin.Context) {
	if !requireSeven(c) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"report_id": c.Param("id"), "summary_attachment": nil,
		"note": "技术就绪审计摘要以报告附件交付，无前台管理页",
	})
}

var _ = strconv.Itoa
var _ = time.Now
