package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/geolens/geolens/internal/growth"
	"github.com/gin-gonic/gin"
)

// createShare / getShare —— WP-6.3：免登录可分享的站点检查结果。
func (s *Server) createShare(c *gin.Context) {
	if s.growth == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "share store unavailable"})
		return
	}
	var body struct {
		Score json.RawMessage `json:"score"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	token, err := s.growth.SaveShare(body.Score)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"token": token, "url": "/r/" + token})
}

func (s *Server) getShare(c *gin.Context) {
	if s.growth == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "share store unavailable"})
		return
	}
	score, ok := s.growth.GetShare(c.Param("token"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "share not found"})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", score)
}

// recordEvent / getFunnel —— WP-6.6：匿名转化漏斗。
func (s *Server) recordEvent(c *gin.Context) {
	if s.growth == nil {
		c.Status(http.StatusNoContent)
		return
	}
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.Status(http.StatusNoContent)
		return
	}
	event, _ := payload["event"].(string)
	path, _ := payload["path"].(string)
	var ts int64
	if v, ok := payload["ts"].(float64); ok {
		ts = int64(v)
	}
	_ = s.growth.RecordEvent(growth.FunnelEvent{Event: event, Path: path, TS: ts})
	c.Status(http.StatusNoContent)
}

func (s *Server) getFunnel(c *gin.Context) {
	if s.growth == nil {
		c.JSON(http.StatusOK, growth.FunnelStats{
			Steps: growth.Steps, Counts: map[string]int{}, Conversions: map[string]float64{},
		})
		return
	}
	c.JSON(http.StatusOK, s.growth.Funnel())
}
