package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/answerlens/answerlens/internal/customerfacing"
	"github.com/answerlens/answerlens/internal/db"
	"github.com/gin-gonic/gin"
)

func newCFTestServer(t *testing.T, enabled bool) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	d, err := db.Open(t.TempDir() + "/cf.db")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.MigrateUp(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return &Server{customerDB: d, customerFlag: customerfacing.NewFlag(enabled)}
}

func TestCustomerFacingNav(t *testing.T) {
	s := newCFTestServer(t, true)
	r := gin.New()
	s.registerCustomerFacingRoutes(r)
	// 直接调 getNav（跳过鉴权中间件，鉴权由 E2E 基线覆盖）
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	s.getNav(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("nav 状态码 %d", rec.Code)
	}
	var nav []db.NavItem
	if err := json.Unmarshal(rec.Body.Bytes(), &nav); err != nil {
		t.Fatal(err)
	}
	if len(nav) != 9 {
		t.Fatalf("客户导航应 9 项，实得 %d", len(nav))
	}
	for _, n := range nav {
		if n.NavKey == "evidence_drawer" || n.RoutePath == "/reports" {
			t.Fatal("retired 项泄漏")
		}
	}
}

func TestCustomerFacingFlagOff(t *testing.T) {
	s := newCFTestServer(t, false)
	r := gin.New()
	s.registerCustomerFacingRoutes(r)
	// flag off 时 /nav 未注册 → 404
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nav", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("flag off 时 /nav 应 404，实得 %d", rec.Code)
	}
}

func TestAttributionMethodsSix(t *testing.T) {
	s := newCFTestServer(t, true)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	s.getAttributionMethods(c)
	var body struct {
		Methods []string `json:"methods"`
		Count   int      `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Count != 6 || len(body.Methods) != 6 {
		t.Fatalf("应六口径并列，实得 %d", body.Count)
	}
}

func TestFixedNoticesInResponses(t *testing.T) {
	s := newCFTestServer(t, true)
	checks := []struct {
		name   string
		fn     gin.HandlerFunc
		notice string
	}{
		{"market", s.getMarketBaseline, "两次独立观察，不证明因果关系。"},
		{"finance", s.getFinanceOverview, "本操作生成草案，须经人工批准后生效；广告以暂停态上线，财务不自动过账。"},
	}
	for _, ch := range checks {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		ch.fn(c)
		if !contains(rec.Body.String(), ch.notice) {
			t.Errorf("%s 响应缺固定文案 %q", ch.name, ch.notice)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
