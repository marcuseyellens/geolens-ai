package handlers

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/answerlens/answerlens/application"
	"github.com/answerlens/answerlens/domain"
	"github.com/answerlens/answerlens/internal/aimodel"
	"github.com/answerlens/answerlens/internal/auth"
	"github.com/answerlens/answerlens/internal/billing"
	"github.com/answerlens/answerlens/internal/collector"
	"github.com/answerlens/answerlens/internal/customerfacing"
	"github.com/answerlens/answerlens/internal/db"
	"github.com/answerlens/answerlens/internal/governance"
	"github.com/answerlens/answerlens/internal/growth"
	"github.com/answerlens/answerlens/internal/mcpserver"
	"github.com/answerlens/answerlens/internal/monitor"
	"github.com/answerlens/answerlens/internal/omnigraph"
	"github.com/gin-gonic/gin"
)

// Platform 聚合基线 WBS WP-1/2/3/5/7/8 的运行时实例。
type Platform struct {
	Governance *governance.Engine
	Graph      *omnigraph.Store
	Collector  *collector.Framework
	MCP        *mcpserver.Server
	Billing    *billing.Engine
	Monitor    *monitor.Engine
	Auth       *auth.Engine
	AdminToken string
	// Payments 可选注入；为空时 NewServer 从环境变量装配（凭证缺失只有演示网关）。
	Payments *billing.ProviderRegistry
	// CustomerDB / CustomerFlag 为第五批客户可见层依赖（CR-v1.1 决策 2/3）。
	CustomerDB   *db.DB
	CustomerFlag *customerfacing.Flag
}

// Server 持有所有 handler 依赖
type Server struct {
	detection    *application.DetectionService
	engine       *aimodel.Engine
	growth       *growth.Store
	platform     *Platform
	mcp          *mcpserver.Server
	auth         *auth.Engine
	adminToken   string
	billing      *billing.Engine
	monitor      *monitor.Engine
	governance   *governance.Engine
	graph        *omnigraph.Store
	collector    *collector.Framework
	payments     *billing.ProviderRegistry
	customerDB   *db.DB
	customerFlag *customerfacing.Flag
}

// NewServer 创建 API server。platform 为 nil 时保持旧行为（仅 WP-6 + 增量 WBS 端点）。
func NewServer(detection *application.DetectionService, engine *aimodel.Engine, growthStore *growth.Store, platform ...*Platform) *Server {
	s := &Server{detection: detection, engine: engine, growth: growthStore,
		payments: billing.NewProviderRegistry()}
	if len(platform) > 0 && platform[0] != nil {
		p := platform[0]
		s.platform = p
		s.mcp = p.MCP
		s.adminToken = p.AdminToken
		s.auth = p.Auth
		s.billing = p.Billing
		s.monitor = p.Monitor
		s.governance = p.Governance
		s.graph = p.Graph
		s.collector = p.Collector
		if p.Payments != nil {
			s.payments = p.Payments
		}
		s.customerDB = p.CustomerDB
		s.customerFlag = p.CustomerFlag
	}
	return s
}

// RegisterRoutes 注册所有 API 路由。
//
// 鉴权边界（fail-closed）：除显式列出的匿名端点外，所有 /api/v1 接口都要求
// 有效会话；管理端点额外要求 admin 角色或 X-Admin-Token。account_id 一律取自
// 会话，客户端传参不再被信任。/mcp 使用独立的只读 Agent token，不走用户会话。
func (s *Server) RegisterRoutes(r *gin.Engine) {
	// 直接构造 Server 的测试没有走 NewServer，这里补齐支付供应商注册表。
	if s.payments == nil {
		s.payments = billing.NewProviderRegistry()
	}
	api := r.Group("/api/v1")

	// ---- 匿名可用：健康检查、登录注册、免费检查、分享、埋点、公开自检 ----
	api.GET("/health", s.health)
	api.GET("/models", s.listModels)
	api.POST("/auth/login", s.authLogin)
	api.POST("/auth/register", s.authRegister)
	api.POST("/site-audits", s.auditSite)
	api.POST("/shares", s.createShare)
	api.GET("/shares/:token", s.getShare)
	api.POST("/events", s.recordEvent)
	api.GET("/self-checks", s.selfChecks)
	api.GET("/acceptance-checklist", s.acceptanceChecklist)
	api.GET("/mcp/info", s.mcpInfo)
	api.POST("/redeem", s.redeem)

	// ---- 需要登录 ----
	protected := api.Group("", s.RequireAuth())
	{
		protected.GET("/auth/me", s.authMe)
		protected.POST("/auth/logout", s.authLogout)

		// 品牌 CRUD
		protected.GET("/brands", s.listBrands)
		protected.POST("/brands", s.createBrand)
		protected.GET("/brands/:id", s.getBrand)
		protected.PUT("/brands/:id", s.updateBrand)
		protected.DELETE("/brands/:id", s.deleteBrand)

		// 查询管理
		protected.GET("/brands/:id/queries", s.listQueries)
		protected.POST("/brands/:id/queries", s.createQuery)
		protected.DELETE("/brands/:id/queries/:qid", s.deleteQuery)

		// 检测执行
		protected.POST("/brands/:id/queries/:qid/detect", s.runDetection)

		// 报告
		protected.GET("/brands/:id/report", s.getReport)
		protected.GET("/brands/:id/results", s.listResults)
		protected.GET("/brands/:id/diagnose", s.getDiagnosticReport)
		protected.GET("/brands/:id/visibility", s.getVisibility)

		// 漏斗与增量 WBS
		protected.GET("/funnel", s.getFunnel)
		protected.POST("/profile-compare", s.compareProfile)
		protected.POST("/question-sets", s.generateQuestionSets)
	}

	// ---- 仅管理员 ----
	admin := api.Group("", s.RequireAdmin())

	// 基线 WBS WP-1/2/3/5/7/8 运行时端点（含根路径 /mcp）
	s.registerPlatformRoutes(api, protected, admin, r)

	// 规格书 Δ-01~Δ-07 / §3.1 端点（受 feature flag 总闸控制）。
	s.registerDeltaRoutes(protected, r)

	// 第三批次 CF-01~CF-08 / §3.1 端点（独立 flag，与 Δ 批次正交）。
	s.registerCompetitorRoutes(protected, admin, r)

	// 第五批客户可见层端点（独立 flag，与前三 flag 正交；off 时不注册）。
	s.registerCustomerFacingRoutes(r)
}

// ---- Brand handlers ----

func (s *Server) listBrands(c *gin.Context) {
	brands, err := s.detection.ListBrands(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, brands)
}

func (s *Server) createBrand(c *gin.Context) {
	var brand domain.Brand
	if err := c.ShouldBindJSON(&brand); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := s.detection.CreateBrand(c.Request.Context(), &brand); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, brand)
}

func (s *Server) getBrand(c *gin.Context) {
	brand, err := s.detection.GetBrand(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(404, gin.H{"error": "brand not found"})
		return
	}
	c.JSON(200, brand)
}

func (s *Server) updateBrand(c *gin.Context) {
	var brand domain.Brand
	if err := c.ShouldBindJSON(&brand); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	brand.ID = c.Param("id")
	if err := s.detection.UpdateBrand(c.Request.Context(), &brand); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, brand)
}

func (s *Server) deleteBrand(c *gin.Context) {
	if err := s.detection.DeleteBrand(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(204, nil)
}

// ---- Query handlers ----

func (s *Server) listQueries(c *gin.Context) {
	queries, err := s.detection.ListQueries(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if queries == nil {
		queries = []*domain.DetectionQuery{}
	}
	c.JSON(200, queries)
}

func (s *Server) createQuery(c *gin.Context) {
	var query domain.DetectionQuery
	if err := c.ShouldBindJSON(&query); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	query.BrandID = c.Param("id")
	if err := s.detection.CreateQuery(c.Request.Context(), &query); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, query)
}

func (s *Server) deleteQuery(c *gin.Context) {
	if err := s.detection.DeleteQuery(c.Request.Context(), c.Param("qid")); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(204, nil)
}

// ---- Detection handlers ----

func (s *Server) runDetection(c *gin.Context) {
	brandID := c.Param("id")
	queryID := c.Param("qid")
	results, err := s.detection.RunDetection(c.Request.Context(), brandID, queryID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"results": results})
}

// ---- Report handlers ----

func (s *Server) getReport(c *gin.Context) {
	report, err := s.detection.GetReport(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, report)
}

func (s *Server) listResults(c *gin.Context) {
	results, err := s.detection.ListResults(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if results == nil {
		results = []*domain.DetectionResult{}
	}
	c.JSON(200, results)
}

// ---- Model handlers ----

func (s *Server) listModels(c *gin.Context) {
	models := domain.AllModels()
	var result []gin.H
	for _, m := range models {
		key := ""
		switch m {
		case domain.ModelChatGPT:
			key = os.Getenv("OPENAI_API_KEY")
		case domain.ModelClaude:
			key = os.Getenv("ANTHROPIC_API_KEY")
		case domain.ModelGemini:
			key = os.Getenv("GEMINI_API_KEY")
		case domain.ModelPerplexity:
			key = os.Getenv("PERPLEXITY_API_KEY")
		}
		result = append(result, gin.H{
			"id":      m,
			"name":    modelName(m),
			"enabled": key != "" || s.engine.IsDemo(),
			"demo":    s.engine.IsDemo(),
		})
	}
	c.JSON(200, result)
}

func (s *Server) health(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok", "demo": s.engine.IsDemo()})
}

func modelName(m domain.AIModelID) string {
	switch m {
	case domain.ModelChatGPT:
		return "ChatGPT (GPT-4o)"
	case domain.ModelClaude:
		return "Claude (Sonnet 4)"
	case domain.ModelGemini:
		return "Gemini (1.5 Flash)"
	case domain.ModelPerplexity:
		return "Perplexity (Sonar)"
	default:
		return string(m)
	}
}

// parseCtx 辅助函数（保留以备扩展）
func parseCtx(c *gin.Context) context.Context {
	return c.Request.Context()
}

// safeSplit 工具函数
func safeSplit(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ",")
}

// suppress lint error for unused
var _ = http.StatusOK
