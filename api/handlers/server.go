package handlers

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/geolens/geolens/application"
	"github.com/geolens/geolens/domain"
	"github.com/geolens/geolens/internal/aimodel"
	"github.com/gin-gonic/gin"
)

// Server 持有所有 handler 依赖
type Server struct {
	detection *application.DetectionService
	engine    *aimodel.Engine
}

// NewServer 创建 API server
func NewServer(detection *application.DetectionService, engine *aimodel.Engine) *Server {
	return &Server{detection: detection, engine: engine}
}

// RegisterRoutes 注册所有 API 路由
func (s *Server) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/v1")
	{
		// 品牌 CRUD
		api.GET("/brands", s.listBrands)
		api.POST("/brands", s.createBrand)
		api.GET("/brands/:id", s.getBrand)
		api.PUT("/brands/:id", s.updateBrand)
		api.DELETE("/brands/:id", s.deleteBrand)

		// 查询管理
		api.GET("/brands/:id/queries", s.listQueries)
		api.POST("/brands/:id/queries", s.createQuery)
		api.DELETE("/brands/:id/queries/:qid", s.deleteQuery)

		// 检测执行
		api.POST("/brands/:id/queries/:qid/detect", s.runDetection)

		// 报告
		api.GET("/brands/:id/report", s.getReport)
		api.GET("/brands/:id/results", s.listResults)
		
		// 诊断报告
		api.GET("/brands/:id/diagnose", s.getDiagnosticReport)

		// 模型配置
		api.GET("/models", s.listModels)
		api.GET("/health", s.health)
	}
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
