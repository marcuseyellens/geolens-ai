package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/geolens/geolens/api/handlers"
	"github.com/geolens/geolens/application"
	"github.com/geolens/geolens/domain"
	"github.com/geolens/geolens/internal/aimodel"
	"github.com/geolens/geolens/internal/auth"
	"github.com/geolens/geolens/internal/billing"
	"github.com/geolens/geolens/internal/collector"
	"github.com/geolens/geolens/internal/competitor"
	"github.com/geolens/geolens/internal/delta"
	"github.com/geolens/geolens/internal/governance"
	"github.com/geolens/geolens/internal/growth"
	"github.com/geolens/geolens/internal/monitor"
	"github.com/geolens/geolens/internal/omnigraph"
	"github.com/geolens/geolens/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// corsMiddleware 手动实现 CORS 中间件（替代 gin-contrib/cors）
func corsMiddleware() gin.HandlerFunc {
	allowedOrigins := map[string]bool{
		"http://localhost:5173": true,
		"http://localhost:3000": true,
		"http://127.0.0.1:5173": true,
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowedOrigins[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Admin-Token")
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func main() {
	// 数据库初始化
	dbPath := os.Getenv("GEOLENS_DB")
	if dbPath == "" {
		dbPath = filepath.Join(os.Getenv("HOME"), ".geolens", "geolens.json")
	}
	os.MkdirAll(filepath.Dir(dbPath), 0755)

	store, err := storage.NewStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	// WP-6.3 分享结果与 WP-6.6 漏斗事件，使用独立文件便于单独清理
	growthStore, err := growth.NewStore(filepath.Join(filepath.Dir(dbPath), "growth.json"))
	if err != nil {
		log.Fatalf("Failed to init growth store: %v", err)
	}

	// AI 模型 API Keys
	apiKeys := map[domain.AIModelID]string{
		domain.ModelChatGPT:    os.Getenv("OPENAI_API_KEY"),
		domain.ModelClaude:     os.Getenv("ANTHROPIC_API_KEY"),
		domain.ModelGemini:     os.Getenv("GEMINI_API_KEY"),
		domain.ModelPerplexity: os.Getenv("PERPLEXITY_API_KEY"),
	}
	engine := aimodel.NewEngine(apiKeys)

	// 组装应用层（适配器桥接 store -> repository 接口）
	detectionSvc := application.NewDetectionService(
		&brandRepoAdapter{store: store},
		&queryRepoAdapter{store: store},
		&resultRepoAdapter{store: store},
		engine,
	)

	// 基线 WBS 运行时：治理 / 图谱 / 采集 / 计费 / 监控 / MCP。
	// 外部系统（Offstage Cloud、Stripe、Omnigraph 服务）通过各自端口替换本地实现。
	budgetLimit := domain.ThresholdDailyCostUSD
	if raw := os.Getenv("GEOLENS_BUDGET_USD"); raw != "" {
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v > 0 {
			budgetLimit = v
		}
	}
	adminToken := os.Getenv("GEOLENS_ADMIN_TOKEN")
	if adminToken == "" {
		adminToken = "adm_" + uuid.NewString()
		fmt.Printf("GEOLENS_ADMIN_TOKEN 未设置，本次启动生成临时管理员令牌: %s\n", adminToken)
	}
	// 用户与会话：密码用 bcrypt 哈希持久化到 auth.json，会话仅存内存。
	authEngine, err := auth.NewEngine(filepath.Join(filepath.Dir(dbPath), "auth.json"), nil)
	if err != nil {
		log.Fatalf("Failed to init auth store: %v", err)
	}
	demoEmail := os.Getenv("GEOLENS_DEMO_EMAIL")
	if demoEmail == "" {
		demoEmail = "demo@geolens.ai"
	}
	demoPassword := os.Getenv("GEOLENS_DEMO_PASSWORD")
	if demoPassword == "" {
		demoPassword = "demo1234"
	}
	if _, err := authEngine.Seed(demoEmail, demoPassword, auth.RoleAdmin, "demo-account"); err != nil {
		log.Fatalf("Failed to seed demo account: %v", err)
	}

	mcpServer, agentToken := handlers.NewMCPRuntime(detectionSvc)
	platform := &handlers.Platform{
		Governance: governance.NewEngine(budgetLimit, nil),
		Graph:      omnigraph.NewStore(nil),
		Collector:  collector.NewFramework(collector.NewLocalSandbox(), nil),
		MCP:        mcpServer,
		Billing:    billing.NewEngine(billing.NewLocalGateway(0), nil),
		Monitor:    monitor.NewEngine(nil),
		Auth:       authEngine,
		AdminToken: adminToken,
	}
	// Δ-01~Δ-07 增量批次运行时：阈值配置 / 采纳埋点 / 工具清单等落 delta.json。
	deltaRuntime := delta.NewRuntime(filepath.Join(filepath.Dir(dbPath), "delta.json"))
	handlers.SetDeltaRuntime(deltaRuntime)
	_ = deltaRuntime.Persist()
	fmt.Printf("Delta feature %s: on=%v (GEOLENS_FLAG_AEO_DELTA_20261116=off 可秒级回滚)\n",
		domain.DeltaFeatureFlag, deltaRuntime.Enabled())

	// 第三批次 CF-01~CF-08 运行时：独立 flag、独立持久化文件，
	// 与 Δ 批次正交，关闭后行为回到批次前状态。
	competitorRuntime := competitor.NewRuntime(filepath.Join(filepath.Dir(dbPath), "competitor.json"))
	handlers.SetCompetitorRuntime(competitorRuntime)
	_ = competitorRuntime.Persist()
	fmt.Printf("Competitor feature %s: on=%v (GEOLENS_FLAG_AEO_COMPETITOR_20261116=off 可秒级回滚)\n",
		domain.CompetitorFeatureFlag, competitorRuntime.Enabled())

	fmt.Printf("MCP read-only agent token: %s\n", agentToken.Token)
	fmt.Printf("Demo login: %s / %s (admin)\n", demoEmail, demoPassword)

	// HTTP 服务
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(corsMiddleware())

	// 前端静态文件（构建后嵌入）
	frontendDist := os.Getenv("GEOLENS_FRONTEND_DIST")
	if frontendDist == "" {
		frontendDist = "./frontend/dist"
	}
	if _, err := os.Stat(frontendDist); err == nil {
		r.Static("/assets", filepath.Join(frontendDist, "assets"))
		// WP-6.4 AEO 自举：根目录可抓取文件必须真实返回，不能被 SPA fallback 吞成 HTML
		for _, name := range []string{"llms.txt", "robots.txt", "sitemap.xml", "favicon.ico"} {
			p := filepath.Join(frontendDist, name)
			if _, err := os.Stat(p); err == nil {
				r.StaticFile("/"+name, p)
			}
		}
	}

	server := handlers.NewServer(detectionSvc, engine, growthStore, platform)
	server.RegisterRoutes(r)

	// SPA fallback：API 路径保持 JSON 404，避免接口错误被伪装成前端页面
	if _, err := os.Stat(frontendDist); err == nil {
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(404, gin.H{"error": "api not found"})
				return
			}
			c.File(filepath.Join(frontendDist, "index.html"))
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("GeoLens AI server starting on :%s\n", port)
	fmt.Printf("Database: %s\n", dbPath)
	fmt.Printf("Frontend dist: %s\n", frontendDist)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
