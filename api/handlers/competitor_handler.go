package handlers

import (
	"net/http"

	"github.com/geolens/geolens/domain"
	"github.com/geolens/geolens/internal/competitor"
	"github.com/gin-gonic/gin"
)

// 第三批次（CF-01~CF-08）运行时。由 main 注入；未注入时懒加载内存态，
// 保证旧部署 / 单测仍能编译与回归。
var serverCompetitor *competitor.Runtime

func SetCompetitorRuntime(rt *competitor.Runtime) { serverCompetitor = rt }

func (s *Server) competitorRuntime() *competitor.Runtime {
	if serverCompetitor == nil {
		serverCompetitor = competitor.NewRuntime("")
	}
	return serverCompetitor
}

// guardCompetitor 是本批次独立总闸：off 时所有新增端点返回 404，
// 行为与批次前完全一致；不影响 aeo_delta_20261116。
func guardCompetitor(c *gin.Context) (*competitor.Runtime, bool) {
	if serverCompetitor == nil {
		serverCompetitor = competitor.NewRuntime("")
	}
	if !serverCompetitor.Enabled() {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "competitor feature disabled",
			"flag":  domain.CompetitorFeatureFlag,
			"on":    false,
		})
		return nil, false
	}
	return serverCompetitor, true
}

// registerCompetitorRoutes 注册 §3.1 规格路径 + 前端别名。
// 既有 /v1/runs/:run_id 与 /v1/reports/:report_id 由 delta_handler.go
// 在本批次 flag on 时追加字段（不改既有字段，off 时不返回）。
func (s *Server) registerCompetitorRoutes(protected, admin *gin.RouterGroup, r *gin.Engine) {
	v1 := r.Group("/v1", s.RequireAuth())
	{
		v1.GET("/runs/:run_id/answers", s.competitorGetAnswers)
		v1.GET("/evidence/:run_id", s.competitorGetEvidence)
		v1.GET("/evidence/:run_id/export", s.competitorExportEvidence)
		v1.GET("/audits/:audit_id/crawler-visits", s.competitorGetCrawlerVisits)
		v1.GET("/audits/:audit_id/crawl-citation-paths", s.competitorGetPaths)
		v1.GET("/projects/:project_id/competitor-gaps", s.competitorGetGaps)
		v1.GET("/question-sets/:question_set_id/targeting", s.competitorGetTargeting)
	}

	c := protected.Group("/competitor")
	{
		c.GET("/status", s.competitorStatus)
		c.GET("/runs/:run_id", s.competitorGetRun)
		c.GET("/runs/:run_id/answers", s.competitorGetAnswers)
		c.GET("/reports", s.competitorListReports)
		c.GET("/reports/:report_id", s.competitorGetReport)
		c.GET("/evidence/:run_id", s.competitorGetEvidence)
		c.GET("/evidence/:run_id/export", s.competitorExportEvidence)
		c.GET("/audits/:audit_id/crawler-visits", s.competitorGetCrawlerVisits)
		c.GET("/audits/:audit_id/crawl-citation-paths", s.competitorGetPaths)
		c.GET("/projects/:project_id/competitor-gaps", s.competitorGetGaps)
		c.GET("/projects/:project_id/action-board", s.competitorGetActionBoard)
		c.GET("/question-sets/:question_set_id/targeting", s.competitorGetTargeting)
		c.GET("/ci-assertions", s.competitorCIAssertions)
	}
	admin.POST("/competitor/flag", s.competitorSetFlag)
}

// competitorStatus 始终可访问：运维需要核对 flag 与两个批次的正交状态。
func (s *Server) competitorStatus(c *gin.Context) {
	rt := s.competitorRuntime()
	c.JSON(http.StatusOK, gin.H{
		"flag":                domain.CompetitorFeatureFlag,
		"on":                  rt.Enabled(),
		"delta_flag":          domain.DeltaFeatureFlag,
		"delta_on":            s.deltaRuntime().Enabled(),
		"methodology_version": domain.MethodologyVersionCompetitor,
		"legacy_versions":     []string{domain.MethodologyVersion, domain.MethodologyVersionDelta},
		"schema_version":      domain.CompetitorSchemaVersion,
		"migration":           rt.Migrate(),
		"contracts":           []string{domain.ContractC1, domain.ContractC2, domain.ContractC3, domain.ContractC4},
		"neutrality_notice":   domain.NeutralityNotice,
	})
}

// competitorSetFlag 是 §8 第 1 步「秒级关 flag」的运维入口（仅管理员）。
func (s *Server) competitorSetFlag(c *gin.Context) {
	var body struct {
		On *bool `json:"on"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.On == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "on (bool) required"})
		return
	}
	rt := s.competitorRuntime()
	rt.Flag.SetEnabled(*body.On)
	c.JSON(http.StatusOK, gin.H{
		"flag": domain.CompetitorFeatureFlag, "on": rt.Enabled(),
		"delta_flag": domain.DeltaFeatureFlag, "delta_on": s.deltaRuntime().Enabled(),
	})
}

// GET /api/v1/competitor/runs/{id}：run 元数据（sample_rounds 等）。
func (s *Server) competitorGetRun(c *gin.Context) {
	rt, ok := guardCompetitor(c)
	if !ok {
		return
	}
	run, found := rt.Run(c.Param("run_id"))
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return
	}
	c.JSON(http.StatusOK, run)
}

// GET /v1/runs/{id}/answers：按 sample_index 升序（EC-01）。
func (s *Server) competitorGetAnswers(c *gin.Context) {
	rt, ok := guardCompetitor(c)
	if !ok {
		return
	}
	runID := c.Param("run_id")
	if _, found := rt.Run(runID); !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"run_id": runID, "answers": rt.Answers(runID)})
}

// GET /v1/reports/{id}：三口径渲染 + 固定中立性声明（EC-03 / EC-05）。
func (s *Server) competitorGetReport(c *gin.Context) {
	rt, ok := guardCompetitor(c)
	if !ok {
		return
	}
	rec, found := rt.Report(c.Param("report_id"))
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
		return
	}
	c.JSON(http.StatusOK, competitor.BuildReportTemplate(rec))
}

// GET /api/v1/competitor/reports：三口径报告列表（供渲染层分支回归）。
func (s *Server) competitorListReports(c *gin.Context) {
	rt, ok := guardCompetitor(c)
	if !ok {
		return
	}
	items := []competitor.ReportTemplate{}
	for _, rec := range rt.Reports() {
		items = append(items, competitor.BuildReportTemplate(rec))
	}
	c.JSON(http.StatusOK, gin.H{"reports": items})
}

// GET /v1/evidence/{run_id}：只读证据端点，调用不产生任何写操作（EC-07）。
func (s *Server) competitorGetEvidence(c *gin.Context) {
	rt, ok := guardCompetitor(c)
	if !ok {
		return
	}
	runID := c.Param("run_id")
	evidence, version, err := competitor.BuildEvidence(rt, runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	citations := 0
	for _, e := range evidence {
		citations += len(e.Citations)
	}
	c.JSON(http.StatusOK, gin.H{
		"run_id": runID, "methodology_version": version,
		"evidence": evidence, "citation_count": citations,
		"read_only": true,
	})
}

// GET /v1/evidence/{run_id}/export?format=csv|json：含原文与完整 citations。
func (s *Server) competitorExportEvidence(c *gin.Context) {
	rt, ok := guardCompetitor(c)
	if !ok {
		return
	}
	runID := c.Param("run_id")
	records, _, err := competitor.BuildExportRecords(rt, runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if c.Query("format") == "csv" {
		b, err := competitor.ExportCSV(records)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Disposition", "attachment; filename=evidence-"+runID+".csv")
		c.Data(http.StatusOK, "text/csv; charset=utf-8", b)
		return
	}
	b, err := competitor.ExportJSON(records)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Disposition", "attachment; filename=evidence-"+runID+".json")
	c.Data(http.StatusOK, "application/json; charset=utf-8", b)
}

// GET /v1/audits/{id}/crawler-visits：IP 核验后的访问记录（EC-02）。
func (s *Server) competitorGetCrawlerVisits(c *gin.Context) {
	rt, ok := guardCompetitor(c)
	if !ok {
		return
	}
	logs := rt.VisitLogs("")
	c.JSON(http.StatusOK, gin.H{
		"audit_id":    c.Param("audit_id"),
		"visits":      logs,
		"audit_check": rt.AuditCheck(c.Param("audit_id")),
	})
}

// GET /v1/audits/{id}/crawl-citation-paths：逐页真实路径（EC-02）。
func (s *Server) competitorGetPaths(c *gin.Context) {
	rt, ok := guardCompetitor(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"audit_id": c.Param("audit_id"), "paths": rt.Paths("")})
}

// GET /v1/projects/{id}/competitor-gaps?status=open
// priority_score 仅在此端点出现，且只用于 Action Board 内部排序。
func (s *Server) competitorGetGaps(c *gin.Context) {
	rt, ok := guardCompetitor(c)
	if !ok {
		return
	}
	status := c.Query("status")
	if status != "" && !domain.ValidGapStatus(status) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"project_id": c.Param("project_id"),
		"status":     status,
		"gaps":       rt.Gaps(c.Param("project_id"), status),
	})
}

// GET /api/v1/competitor/projects/{id}/action-board：M-8「今天值得做的 3 件事」。
func (s *Server) competitorGetActionBoard(c *gin.Context) {
	rt, ok := guardCompetitor(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"project_id": c.Param("project_id"),
		"items":      competitor.BuildActionBoard(rt, c.Param("project_id"), 3),
	})
}

// GET /v1/question-sets/{id}/targeting：定位配置（EC-04）。
func (s *Server) competitorGetTargeting(c *gin.Context) {
	rt, ok := guardCompetitor(c)
	if !ok {
		return
	}
	t, found := rt.Targeting(c.Param("question_set_id"))
	if !found {
		c.JSON(http.StatusOK, gin.H{
			"question_set_id": c.Param("question_set_id"),
			"location_scope":  domain.ScopeNone,
			"location_value":  "",
			"persona":         "",
			"timezone":        "",
			"config_version":  domain.CompetitorSchemaVersion,
			"query_locale":    "",
		})
		return
	}
	c.JSON(http.StatusOK, t)
}

// GET /api/v1/competitor/ci-assertions：既有 §6 断言 + 本批次追加项。
func (s *Server) competitorCIAssertions(c *gin.Context) {
	c.JSON(http.StatusOK, competitor.RunAllAssertions(repoRootOf(s.frontendSource())))
}
