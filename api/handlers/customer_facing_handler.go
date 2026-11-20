package handlers

import (
	"net/http"

	"github.com/answerlens/answerlens/domain"
	"github.com/answerlens/answerlens/internal/customerfacing"
	"github.com/answerlens/answerlens/internal/db"
	"github.com/gin-gonic/gin"
)

// registerCustomerFacingRoutes 注册第五批客户可见端点（ECF-01～11）。
// flag=off 时 /nav 返回批次前导航，其余端点 404（TC-F-02）。
func (s *Server) registerCustomerFacingRoutes(r *gin.Engine) {
	if s.customerFlag == nil || !s.customerFlag.Enabled() {
		return
	}
	api := r.Group("/api/v1", s.RequireAuth())
	{
		api.GET("/nav", s.getNav)
		api.GET("/market/baseline", s.getMarketBaseline)
		api.GET("/finance/overview", s.getFinanceOverview)
		api.GET("/approval-inbox", s.getApprovalInbox)
		api.GET("/agents", s.getAgents)
		api.GET("/attribution/attribution-methods", s.getAttributionMethods)
		api.GET("/ad/activity-feed", s.getAdActivityFeed)
	}
}

// getNav 从 nav_item 读取（按角色过滤，排除 retired）。flag off 由上层的批次前导航兜底。
func (s *Server) getNav(c *gin.Context) {
	role := 7
	if v, ok := c.GetQuery("role"); ok {
		switch v {
		case "management":
			role = 1
		case "market":
			role = 2
		case "analyst":
			role = 4
		}
	}
	if s.customerDB == nil {
		c.JSON(http.StatusOK, []db.NavItem{})
		return
	}
	nav, err := s.customerDB.VisibleNav(role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nav)
}

// getMarketBaseline 返回市场基线只读投影。所有数字带来源与口径。
func (s *Server) getMarketBaseline(c *gin.Context) {
	stamp := gin.H{
		"source_ref":          "market://baseline",
		"figure_status":       domain.FigureUnverified,
		"methodology_version": domain.MethodologyVersionCustomerFacing,
	}
	c.JSON(http.StatusOK, gin.H{
		"top_brands":            []any{},
		"question_distribution": []any{},
		"citation_sources":      []any{},
		"coverage":              gin.H{"questions": 0, "models": 0, "regions": 0},
		"activity_feed":         []any{},
		"_provenance":           stamp,
		"compare_notice":        domain.CompareNotice,
	})
}

// getFinanceOverview 财务健康分属财务域、分区展示，命名 finance_health_score。
func (s *Server) getFinanceOverview(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"finance_health_score": gin.H{
			"value": 0, "figure_status": domain.FigureUnverified,
			"source_ref": "", "methodology_version": domain.MethodologyVersionCustomerFacing,
		},
		"five_questions":    []any{},
		"domain":            "finance",
		"write_gate_notice": domain.WriteGateNotice,
	})
}

// getApprovalInbox 审批收件箱空启动；写操作须人批准。
func (s *Server) getApprovalInbox(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"items": []any{}, "write_gate_notice": domain.WriteGateNotice,
	})
}

// getAgents Agent 清单（空启动，回放只用真实 evidence_ref）。
func (s *Server) getAgents(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"agents": []any{}})
}

// getAttributionMethods 六口径并列，只并列不合并、无第七种。
func (s *Server) getAttributionMethods(c *gin.Context) {
	methods := make([]string, 0, len(domain.AllSourceProfileMethods))
	for _, m := range domain.AllSourceProfileMethods {
		methods = append(methods, string(m))
	}
	c.JSON(http.StatusOK, gin.H{
		"methods": methods, "count": len(methods),
		"methodology_version": domain.MethodologyVersionCustomerFacing,
	})
}

// getAdActivityFeed 活动动态流；金额须带 source_ref，无来源不填。
func (s *Server) getAdActivityFeed(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"events": []any{}})
}

var _ = customerfacing.IsReadOnlySQL
