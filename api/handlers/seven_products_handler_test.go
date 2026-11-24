package handlers

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/answerlens/answerlens/internal/db"
	"github.com/answerlens/answerlens/internal/sevenproducts"
	"github.com/gin-gonic/gin"
)

func enableSevenForTest() {
	SetSevenProductsRuntime(nil, sevenproducts.NewFlag(true))
}

func setupSevenRoutes(t *testing.T, enabled bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	d, err := db.Open(filepath.Join(t.TempDir(), "seven.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.MigrateUpSeven(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	SetSevenProductsRuntime(d, sevenproducts.NewFlag(enabled))
	// 用最小 Server 注册路由（鉴权中间件在测试中通过 header 注入，这里直接测路由存在性）。
	s := &Server{}
	r := gin.New()
	s.registerSevenProductsRoutes(r)
	return r
}

// TC-D-20：flag off 时第六批端点不注册 → 404，行为回第五批。
func TestSevenFlagOffRoutesGone(t *testing.T) {
	r := setupSevenRoutes(t, false)
	for _, path := range []string{
		"/api/v1/attribution/citations",
		"/api/v1/competitors/1/gaps",
		"/api/v1/changes/proposals",
		"/api/v1/agents/runs/x/metrics",
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("flag off 时 %s 应 404，实得 %d", path, rec.Code)
		}
	}
}

// TC-D-07：未批准直接发布 → 409 approval_required。
func TestSevenPublishWithoutApproval(t *testing.T) {
	enableSevenForTest()
	s := &Server{}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/changes/proposals/1/publish", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	s.tcdProposalPublish(c)
	if rec.Code != http.StatusConflict {
		t.Fatalf("未批准发布应 409，实得 %d", rec.Code)
	}
}

// TC-D-08：非人批准 → 403。
func TestSevenNonHumanApprove(t *testing.T) {
	enableSevenForTest()
	s := &Server{}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/changes/proposals/1/approve", nil)
	c.Request.Header.Set("X-Approver", "agent:builder")
	s.tcdProposalApprove(c)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("非人批准应 403，实得 %d", rec.Code)
	}
}

// TC-D-11：send/delete → 403 forbidden（能力黑名单，不弹窗、不写 grant）。
func TestSevenForbiddenScope(t *testing.T) {
	enableSevenForTest()
	s := &Server{}
	for _, scope := range []string{"send", "delete"} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/agents/1/permission-request",
			strings.NewReader(`{"scope":"`+scope+`"}`))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Request.Header.Set("X-Human-Approved", "true")
		c.Params = gin.Params{{Key: "id", Value: "1"}}
		s.tcdPermissionRequest(c)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s 应 403，实得 %d", scope, rec.Code)
		}
	}
}

// TC-D-14：metrics 响应分列两字段，无合成字段。
func TestSevenMetricsSplit(t *testing.T) {
	enableSevenForTest()
	s := &Server{}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Params = gin.Params{{Key: "execId", Value: "exec-1"}}
	s.tcdMetrics(c)
	body := rec.Body.String()
	if !contains(body, "call_success_rate") || !contains(body, "task_completion_rate") {
		t.Fatal("两分母字段缺失")
	}
	for _, banned := range []string{"overall_success", "combined_", "success_rate_total"} {
		if contains(body, banned) {
			t.Errorf("metrics 响应含合成字段 %s", banned)
		}
	}
}
