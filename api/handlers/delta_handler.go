package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/geolens/geolens/domain"
	"github.com/geolens/geolens/internal/delta"
	"github.com/gin-gonic/gin"
)

// DeltaRuntime 是规格书 Δ 批次的运行时；由 main 注入，未注入时按需懒加载内存态，
// 保证旧部署（NewServer 不带该字段）也能通过编译与回归。
var serverDelta *delta.Runtime

func SetDeltaRuntime(rt *delta.Runtime) { serverDelta = rt }

func (s *Server) deltaRuntime() *delta.Runtime {
	if serverDelta == nil {
		serverDelta = delta.NewRuntime("")
	}
	return serverDelta
}

// guardDelta 是 §7 的 flag 总闸：off 时所有新增端点一律不返回新字段。
func guardDelta(c *gin.Context) (*delta.Runtime, bool) {
	rt := serverDelta
	if rt == nil {
		rt = delta.NewRuntime("")
		serverDelta = rt
	}
	if !rt.Enabled() {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "delta feature disabled",
			"flag":  domain.DeltaFeatureFlag,
			"on":    false,
		})
		return nil, false
	}
	return rt, true
}

// registerDeltaRoutes 注册 §3.1 六个 REST 端点 + §3.2 MCP manifest/tools/list。
// 规格路径为 /v1/*；同时挂到 /api/v1/delta/* 供前端鉴权体系复用。
func (s *Server) registerDeltaRoutes(protected *gin.RouterGroup, r *gin.Engine) {
	// 规格书契约路径（需要登录会话，与 /api/v1 一致）。
	v1 := r.Group("/v1", s.RequireAuth())
	{
		v1.GET("/runs/:run_id", s.deltaGetRun)
		v1.GET("/reports/:report_id", s.deltaGetReport)
		v1.GET("/audits/:audit_id/checks", s.deltaGetAuditChecks)
		v1.POST("/audits/:audit_id/crawler-evidence", s.deltaPostCrawlerEvidence)
		v1.GET("/market/:market_id/question-sets", s.deltaGetQuestionSets)
		v1.GET("/usage/retention", s.deltaGetRetention)
		v1.GET("/mcp/manifest", s.deltaGetManifest)
		v1.GET("/mcp/tools", s.deltaGetToolsList)
		v1.GET("/delta/status", s.deltaStatus)
	}
	// 前端别名（api.ts 的 BASE=/api/v1，统一带 RequireAuth）。
	d := protected.Group("/delta")
	{
		d.GET("/status", s.deltaStatus)
		d.GET("/runs/:run_id", s.deltaGetRun)
		d.GET("/reports/:report_id", s.deltaGetReport)
		d.GET("/audits/:audit_id/checks", s.deltaGetAuditChecks)
		d.POST("/audits/:audit_id/crawler-evidence", s.deltaPostCrawlerEvidence)
		d.GET("/market/:market_id/question-sets", s.deltaGetQuestionSets)
		d.GET("/usage/retention", s.deltaGetRetention)
		d.GET("/mcp/manifest", s.deltaGetManifest)
		d.GET("/mcp/tools", s.deltaGetToolsList)
		d.GET("/ci-assertions", s.deltaCIAssertions)
	}
}

// deltaStatus 暴露 flag 状态、口径版本、配置与迁移结果，供运维核对 §7。
func (s *Server) deltaStatus(c *gin.Context) {
	rt := s.deltaRuntime()
	c.JSON(http.StatusOK, gin.H{
		"flag":                domain.DeltaFeatureFlag,
		"on":                  rt.Enabled(),
		"methodology_version": domain.MethodologyVersionDelta,
		"legacy_version":      domain.MethodologyVersion,
		"schema_version":      delta.SchemaVersion,
		"migration":           rt.Config.Migrate(),
		"thresholds":          rt.Config.Thresholds(),
	})
}

// GET /v1/runs/{run_id}：追加语言观测与 suspect 字段。
func (s *Server) deltaGetRun(c *gin.Context) {
	rt, ok := guardDelta(c)
	if !ok {
		return
	}
	run, found := rt.Run(c.Param("run_id"))
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return
	}
	// 第三批次字段追加：仅在 aeo_competitor_20261116 = on 时返回，
	// off 时响应与本批次前逐字节一致（§3.2 / §8）。
	if crt := s.competitorRuntime(); crt.Enabled() {
		out := gin.H{}
		if b, err := json.Marshal(run); err == nil {
			_ = json.Unmarshal(b, &out)
		}
		sampleRounds, policy, declared := 1, domain.SamplePolicySingleV1, false
		if cr, ok := crt.Run(c.Param("run_id")); ok {
			sampleRounds, policy, declared = cr.SampleRounds, cr.SamplePolicyVersion, cr.CaptureChannelDeclared
		}
		out["sample_rounds"] = sampleRounds
		out["sample_policy_version"] = policy
		out["capture_channel_declared"] = declared
		c.JSON(http.StatusOK, out)
		return
	}
	c.JSON(http.StatusOK, run)
}

// GET /v1/reports/{report_id}：四数 counts + 并列观察 + 独立参考区 +
// source_type / methodology_version（含数字组件强制标注）。
func (s *Server) deltaGetReport(c *gin.Context) {
	rt, ok := guardDelta(c)
	if !ok {
		return
	}
	rec, found := rt.Report(c.Param("report_id"))
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
		return
	}
	provenance := domain.DeltaProvenance(rec.Counts.Usable)
	if rec.LegacyCounts {
		provenance = domain.ObservedProvenance(rec.Counts.Usable)
	}
	resp := gin.H{
		"report_id":           rec.ReportID,
		"methodology_version": rec.MethodologyVersion,
		"generated_at":        rec.GeneratedAt,
		"counts": gin.H{
			"planned": rec.Counts.Planned, "usable": rec.Counts.Usable,
			"failed": rec.Counts.Failed, "suspect": rec.Counts.Suspect,
		},
		"source_type":           provenance.SourceType,
		"provenance":            provenance,
		"language_notice":       rec.LanguageNotice,
		"reference_only":        nonNilMetrics(rec.ReferenceOnly),
		"reference_only_notice": domain.UserProvidedDisclaimer,
		"contract9_note":        domain.Contract9Note,
	}
	if rec.LegacyCounts {
		resp["counts_notice"] = "旧口径（1.0）：三数，缺 suspect 按 0 展示，禁止用 1.1 算法重算。"
	}
	// EC-05：固定中立性声明逐字注入报告 API；历史报告仍保留生成时版本，
	// 渲染层按版本分支决定是否展示，不把旧口径伪装成 1.2。
	if s.competitorRuntime().Enabled() {
		resp["neutrality_notice"] = domain.NeutralityNotice
		resp["competitor_contracts"] = []string{
			domain.ContractC1, domain.ContractC2, domain.ContractC3, domain.ContractC4,
		}
	}
	c.JSON(http.StatusOK, resp)
}

// reference_only 与业务指标必须是不同数组；即使为空也返回独立空数组而非 null。
func nonNilMetrics(m []domain.UserProvidedMetric) []domain.UserProvidedMetric {
	if m == nil {
		return []domain.UserProvidedMetric{}
	}
	return m
}

// GET /v1/audits/{audit_id}/checks：分行字段 + verdict + crawler_visit_evidence。
func (s *Server) deltaGetAuditChecks(c *gin.Context) {
	rt, ok := guardDelta(c)
	if !ok {
		return
	}
	checks, found := rt.AuditChecks(c.Param("audit_id"))
	if !found {
		c.JSON(http.StatusOK, []delta.AuditCheckView{})
		return
	}
	c.JSON(http.StatusOK, checks)
}

// POST /v1/audits/{audit_id}/crawler-evidence：用户上传访问日志片段，
// 标注「用户自述，未经核验」，不判 pass。
func (s *Server) deltaPostCrawlerEvidence(c *gin.Context) {
	rt, ok := guardDelta(c)
	if !ok {
		return
	}
	var body struct {
		EvidenceSource string `json:"evidence_source"`
		Content        string `json:"content"`
		Note           string `json:"note"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	src := domain.EvidenceSource(body.EvidenceSource)
	if !domain.ValidEvidenceSource(src) || (src != domain.EvidenceAccessLog && src != domain.EvidenceUserProvided) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "evidence_source 必须是 access_log 或 user_provided"})
		return
	}
	checkID, verdict := rt.SetCrawlerEvidence(c.Param("audit_id"), src, body.Content, body.Note)
	c.JSON(http.StatusCreated, gin.H{
		"check_id":            checkID,
		"verdict":             verdict,
		"notice":              "用户自述，未经核验；access_log 需可核验的到达记录。",
		"source_type":         string(domain.DeltaSourceUserProvided),
		"methodology_version": domain.MethodologyVersionDelta,
	})
}

// GET /v1/market/{market_id}/question-sets：分语言问题集与独立配额（D-6）。
func (s *Server) deltaGetQuestionSets(c *gin.Context) {
	rt, ok := guardDelta(c)
	if !ok {
		return
	}
	list := rt.QuestionSets.List(c.Param("market_id"))
	c.JSON(http.StatusOK, gin.H{
		"market_id":           c.Param("market_id"),
		"question_sets":       list,
		"total_quota":         rt.QuestionSets.TotalQuota(c.Param("market_id")),
		"bundle_notice":       "同一品类多语言按不同采集任务分别计费；每套问题集独立计入 prompt quota。",
		"source_type":         string(domain.DeltaSourceObservation),
		"methodology_version": domain.MethodologyVersionDelta,
	})
}

// GET /v1/usage/retention：采纳与 30/60 天留存（EX-13）。
func (s *Server) deltaGetRetention(c *gin.Context) {
	rt, ok := guardDelta(c)
	if !ok {
		return
	}
	_ = c.Query("window") // 30|60，统计同时返回两个窗口
	stats := rt.Retention.Stats()
	c.JSON(http.StatusOK, gin.H{
		"viewed": stats.Viewed, "downloaded": stats.Downloaded,
		"applied": stats.Applied, "retest_requested": stats.RetestRequested,
		"retained_30d": stats.Retained30d, "retained_60d": stats.Retained60d,
		"by_fix_item":         rt.Retention.ByFixItem(),
		"source_type":         string(domain.DeltaSourceObservation),
		"methodology_version": domain.MethodologyVersionDelta,
	})
}

// GET /v1/mcp/manifest：当前工具清单版本。
func (s *Server) deltaGetManifest(c *gin.Context) {
	rt, ok := guardDelta(c)
	if !ok {
		return
	}
	versions := rt.Manifests.Versions()
	current := ""
	if len(versions) > 0 {
		current = versions[len(versions)-1]
	}
	c.JSON(http.StatusOK, gin.H{
		"manifest_version":   current,
		"published_at":       delta.SchemaVersion,
		"is_breaking_change": false,
		"retained_versions":  versions,
	})
}

// GET /v1/mcp/tools：tools/list 固定包装；limit 可由查询参数控制，默认不裁切。
func (s *Server) deltaGetToolsList(c *gin.Context) {
	rt, ok := guardDelta(c)
	if !ok {
		return
	}
	limit := 0
	if raw := c.Query("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	tools := []delta.ToolDescriptor{{Name: "query_visibility"}, {Name: "evidence_by_run"}}
	for _, n := range delta.SelfCheckToolNames {
		tools = append(tools, delta.ToolDescriptor{Name: n})
	}
	versions := rt.Manifests.Versions()
	version := "1.1"
	if len(versions) > 0 {
		version = versions[len(versions)-1]
	}
	resp := delta.BuildToolsList(version, tools, limit)
	out := gin.H{
		"manifest_version": resp.ManifestVersion,
		"tools":            resp.Tools,
		"returned_count":   resp.ReturnedCount,
		"total_count":      resp.TotalCount,
		"truncated":        resp.Truncated,
	}
	if resp.Truncated {
		out["client_notice"] = delta.ClientNotice(resp)
	}
	c.JSON(http.StatusOK, out)
}

// GET /api/v1/delta/ci-assertions：§6 全部静态断言结果，供 CI 拉取核对。
func (s *Server) deltaCIAssertions(c *gin.Context) {
	report := delta.RunCIAssertions(repoRootOf(s.frontendSource()))
	c.JSON(http.StatusOK, report)
}
