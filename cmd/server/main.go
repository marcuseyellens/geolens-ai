package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/geolens/geolens/api/handlers"
	"github.com/geolens/geolens/application"
	"github.com/geolens/geolens/domain"
	"github.com/geolens/geolens/internal/aimodel"
	"github.com/geolens/geolens/internal/storage"
	"github.com/gin-gonic/gin"
)

// corsMiddleware 手动实现 CORS 中间件（替代 gin-contrib/cors）
func corsMiddleware() gin.HandlerFunc {
	allowedOrigins := map[string]bool{
		"http://localhost:5173":    true,
		"http://localhost:3000":    true,
		"http://127.0.0.1:5173":    true,
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowedOrigins[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
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
		if _, err := os.Stat(filepath.Join(frontendDist, "favicon.ico")); err == nil {
			r.StaticFile("/favicon.ico", filepath.Join(frontendDist, "favicon.ico"))
		}
	}

	server := handlers.NewServer(detectionSvc, engine)
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
