package handlers

import (
	"net/http"

	"github.com/geolens/geolens/domain"
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

	c.JSON(http.StatusOK, report)
}
