package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// getDiagnosticReport 生成品牌的 AI 可见性诊断报告
func (s *Server) getDiagnosticReport(c *gin.Context) {
	brandID := c.Param("id")
	
	// 调用应用层生成诊断报告
	report, err := s.detection.BuildDiagnostics(c.Request.Context(), brandID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, report)
}