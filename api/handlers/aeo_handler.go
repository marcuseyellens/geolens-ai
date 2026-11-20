package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/answerlens/answerlens/domain"
	"github.com/answerlens/answerlens/internal/aeo"
	"github.com/gin-gonic/gin"
)

// ---- A-WP-9.4 四项自检 API ----

// selfChecks 用自家产品对自家站点跑 A-01 / A-03 / A-05 / A-14 四项自检。
// 只读前端静态文件，不发起网络请求。
func (s *Server) selfChecks(c *gin.Context) {
	dist := s.frontendRoot()
	results := aeo.SelfChecksFor(aeo.ChecklistEvidence{
		RepoRoot: repoRootOf(dist), FrontendDist: dist, FrontendSrc: s.frontendSource(),
	})
	c.JSON(http.StatusOK, gin.H{
		"results":     results,
		"all_passed":  aeo.AllSelfChecksPassed(results),
		"provenance":  domain.ObservedProvenance(4),
		"methodology": aeo.ReportMethodologyNotes(),
	})
}

// frontendRoot 优先使用已构建的 dist（服务端实际 serve 的内容），否则回退到源码。
func (s *Server) frontendRoot() string {
	if dist := os.Getenv("GEOLENS_FRONTEND_DIST"); dist != "" {
		if _, err := os.Stat(filepath.Join(dist, "index.html")); err == nil {
			return dist
		}
	}
	for _, candidate := range []string{"frontend/dist", "../../frontend/dist", "frontend", "../../frontend"} {
		if _, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil {
			return candidate
		}
	}
	return "frontend/dist"
}

// frontendSource 返回前端源码目录（landing.tsx / pricing.tsx 只在源码里）。
func (s *Server) frontendSource() string {
	for _, candidate := range []string{"frontend", "../../frontend", "../frontend", "../../../frontend"} {
		if _, err := os.Stat(filepath.Join(candidate, "src", "routes", "landing.tsx")); err == nil {
			return candidate
		}
	}
	return "frontend"
}

// ---- A-WP-7 问题集生成 API ----

type questionSetRequest struct {
	Brief   domain.QuestionBrief `json:"brief"`
	SetType string               `json:"set_type"`
	Quota   *domain.IntentQuota  `json:"quota"`
	Count   int                  `json:"count"`
}

// generateQuestionSets 生成两套物理分离的问题集。缺任一六要素直接 400。
func (s *Server) generateQuestionSets(c *gin.Context) {
	var req questionSetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式不正确"})
		return
	}
	if err := aeo.ValidateBrief(req.Brief); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	quota := domain.DefaultIntentQuota()
	if req.Quota != nil {
		quota = *req.Quota
	}
	setType := domain.QuestionSetType(req.SetType)
	if setType == "" {
		setType = domain.SetMarketBaseline
	}
	questions, err := aeo.GenerateQuestionSet(req.Brief, setType, quota, req.Count)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"question_set_type": setType,
		"questions":         questions,
		"count":             len(questions),
		"provenance":        domain.ObservedProvenance(len(questions)),
		"note":              "market_baseline 与 brand_tracking 分表返回，绝不合并计算。",
	})
}

// ---- A-WP-1 第一方档案比对 API ----

type profileCompareRequest struct {
	Profile  domain.BusinessProfile `json:"profile"`
	Snapshot aeo.ThirdPartySnapshot `json:"snapshot"`
	Provider string                 `json:"provider"`
}

// compareProfile 返回 7 字段差异与认领引导修复件；不代认领、不代填。
func (s *Server) compareProfile(c *gin.Context) {
	var req profileCompareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式不正确"})
		return
	}
	if req.Profile.Status == "" {
		req.Profile.Status = domain.ProfileUnknown
	}
	diffs := aeo.CompareProfiles(req.Profile, req.Snapshot, time.Now(), req.Provider)
	c.JSON(http.StatusOK, gin.H{
		"status":     aeo.ProfileStatusFor(req.Profile, diffs),
		"diffs":      diffs,
		"checklist":  aeo.ProfileSyncChecklist(req.Profile.BrandID, diffs),
		"provenance": domain.ObservedProvenance(len(diffs)),
		"note":       domain.NeutralityStatement,
	})
}

// ---- A-WP-9.4 发布验收清单自检 ----

// acceptanceChecklist 逐条实测增量 WBS §8 十条清单，不硬编码通过状态。
func (s *Server) acceptanceChecklist(c *gin.Context) {
	dist := s.frontendRoot()
	items := aeo.EvaluateChecklist(aeo.ChecklistEvidence{
		RepoRoot: repoRootOf(dist), FrontendDist: dist, FrontendSrc: s.frontendSource(),
	})
	c.JSON(http.StatusOK, gin.H{
		"items": items, "prices": domain.PlanPrices,
		"provenance":          domain.ObservedProvenance(len(items)),
		"methodology_version": domain.MethodologyVersion,
	})
}

func repoRootOf(frontend string) string {
	// frontend 或 frontend/dist 都向上两级到仓库根
	p := frontend
	for i := 0; i < 3; i++ {
		if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil {
			return p
		}
		p = filepath.Dir(p)
	}
	return ".."
}
