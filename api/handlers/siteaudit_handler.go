package handlers

import (
	"net/http"

	"github.com/geolens/geolens/internal/siteaudit"
	"github.com/gin-gonic/gin"
)

// auditSiteRequest 是免注册站点检查的请求体。
type auditSiteRequest struct {
	URL string `json:"url" binding:"required"`
}

// auditSite 执行免注册站点就绪度检查（F-401~F-404）。
// 不要求登录，只输入 URL 即出 /100 分、逐项证据与首要修复建议。
func (s *Server) auditSite(c *gin.Context) {
	var req auditSiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写网址"})
		return
	}
	score, err := siteaudit.Audit(req.URL, siteaudit.HTTPFetcher{})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, score)
}

// getVisibility 返回品牌可见性（F-305）：Buyer Question 汇总 + 逐模型指标。
func (s *Server) getVisibility(c *gin.Context) {
	v, err := s.detection.GetVisibility(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "brand not found"})
		return
	}
	c.JSON(http.StatusOK, v)
}
