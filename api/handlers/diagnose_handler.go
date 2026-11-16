package handlers

import (
	"net/http"

	"github.com/geolens/geolens/domain"
	"github.com/geolens/geolens/internal/delta"
	"github.com/geolens/geolens/internal/siteaudit"
	"github.com/gin-gonic/gin"
)

// getDiagnosticReport 生成品牌的 AI 可见性诊断报告（F-501，固定 7 节）。
//
// 站点就绪度是报告固定章节，这里会真实抓取品牌网站；品牌未填网址时该节留空。
func (s *Server) getDiagnosticReport(c *gin.Context) {
	ctx := c.Request.Context()
	brandID := c.Param("id")

	var auditPtr *domain.SiteScore
	if brand, err := s.detection.GetBrand(ctx, brandID); err == nil && brand.Website != "" {
		// 与独立站点检查端点保持一致的抓取超时，保证结果稳定准确；失败仍返回带 caveat 的结果。
		audit, auditErr := siteaudit.Audit(brand.Website, siteaudit.HTTPFetcher{})
		if auditErr == nil {
			auditPtr = &audit
		}
	}

	report, err := s.detection.BuildDiagnostics(ctx, brandID, auditPtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Δ 批次：flag on 时给新生成报告追加四数 / 跨语言说明 / 独立参考区，
	// 并升 methodology_version 到 1.1。flag off 时 report 保持 v1.0 形状，
	// 前端 DeltaPanels 因 counts 缺失而整体隐藏。
	rt := s.deltaRuntime()
	if rt.Enabled() {
		counts := delta.FourCounts(report.Evidence)
		report.Counts = &counts
		report.MethodologyVersion = domain.MethodologyVersionDelta
		report.LanguageNotice = domain.LanguageNotice
		report.ReferenceOnly = rt.UserMetrics.ReferenceOnly()
		report.MethodologyNotes = append(report.MethodologyNotes, domain.Contract9Note)
		// Δ-03：把第 9 项分行结论下发给站点检查结果页共享结构。
		verdict := delta.JudgeCrawlerVisit(delta.CrawlerSignals{
			PlatformHint: "Couldn't fetch", PlatformHintIsError: true,
		})
		if auditPtr != nil {
			auditPtr.CrawlerVisit = &domain.CrawlerVisitRow{
				Verdict:          string(verdict.Verdict),
				PlatformHint:     verdict.PlatformHint,
				ObservedEvidence: verdict.ObservedEvidence,
				EvidenceSource:   string(verdict.EvidenceSource),
				FixAdvice:        delta.FixAdviceForCrawler(verdict),
			}
		}
	}

	c.JSON(http.StatusOK, report)
}
