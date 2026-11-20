package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/answerlens/answerlens/internal/delta"
	"github.com/gin-gonic/gin"
)

// 规格书 §3.1 端点集成测试：验证响应字段拼写、四数、独立参考区、
// 跨语言说明、MCP 固定包装，以及 flag=off 时不返回新字段。

func newDeltaRouter(t *testing.T) (*gin.Engine, *delta.Runtime) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rt := delta.NewRuntime("")
	SetDeltaRuntime(rt)
	s := &Server{}
	r := gin.New()
	// 直接注册 delta 端点（无需 auth 引擎，测试只关心响应契约）。
	r.GET("/v1/runs/:run_id", s.deltaGetRun)
	r.GET("/v1/reports/:report_id", s.deltaGetReport)
	r.GET("/v1/audits/:audit_id/checks", s.deltaGetAuditChecks)
	r.POST("/v1/audits/:audit_id/crawler-evidence", s.deltaPostCrawlerEvidence)
	r.GET("/v1/market/:market_id/question-sets", s.deltaGetQuestionSets)
	r.GET("/v1/usage/retention", s.deltaGetRetention)
	r.GET("/v1/mcp/manifest", s.deltaGetManifest)
	r.GET("/v1/mcp/tools", s.deltaGetToolsList)
	r.GET("/delta/status", s.deltaStatus)
	return r, rt
}

func getJSON(t *testing.T, r *gin.Engine, path string) (int, map[string]interface{}) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w.Code, body
}

func TestDeltaEndpoints_ReportContract(t *testing.T) {
	r, _ := newDeltaRouter(t)
	code, body := getJSON(t, r, "/v1/reports/report_demo_1")
	if code != http.StatusOK {
		t.Fatalf("报告端点返回 %d", code)
	}
	if body["methodology_version"] != "1.1" {
		t.Fatal("新报告口径未升 1.1")
	}
	counts, ok := body["counts"].(map[string]interface{})
	if !ok {
		t.Fatal("counts 缺失")
	}
	for _, k := range []string{"planned", "usable", "failed", "suspect"} {
		if _, ok := counts[k]; !ok {
			t.Fatalf("counts 缺键 %s", k)
		}
	}
	if _, ok := counts["overall_score"]; ok {
		t.Fatal("schema 混入 overall_score")
	}
	if body["language_notice"] == "" || body["reference_only_notice"] == "" {
		t.Fatal("跨语言说明 / 独立参考区免责文案缺失")
	}
	ref, ok := body["reference_only"].([]interface{})
	if !ok {
		t.Fatal("reference_only 必须是独立数组")
	}
	if len(ref) == 0 {
		t.Fatal("独立参考区应含 user_provided DR 指标")
	}
}

func TestDeltaEndpoints_LegacyReport(t *testing.T) {
	r, _ := newDeltaRouter(t)
	code, body := getJSON(t, r, "/v1/reports/report_legacy_1")
	if code != http.StatusOK {
		t.Fatalf("历史报告端点返回 %d", code)
	}
	if body["methodology_version"] != "1.0.0" {
		t.Fatal("历史报告口径被改写")
	}
	if body["counts_notice"] == nil {
		t.Fatal("历史报告缺旧口径标注")
	}
}

func TestDeltaEndpoints_RunAddsLanguageFields(t *testing.T) {
	r, _ := newDeltaRouter(t)
	code, body := getJSON(t, r, "/v1/runs/run_demo_1")
	if code != http.StatusOK {
		t.Fatalf("运行端点返回 %d", code)
	}
	for _, k := range []string{"query_locale", "retrieval_locale", "retrieval_locale_source", "locale_mismatch"} {
		if _, ok := body[k]; !ok {
			t.Fatalf("运行响应缺新字段 %s", k)
		}
	}
	if body["retrieval_locale_source"] != "platform_header" {
		t.Fatal("判定优先级未按 D-1")
	}
	if body["locale_mismatch"] != true {
		t.Fatal("跨语言未标 mismatch")
	}
}

func TestDeltaEndpoints_AuditChecksRows(t *testing.T) {
	r, _ := newDeltaRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/audits/audit_demo_1/checks", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("checks 端点返回 %d", w.Code)
	}
	var checks []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &checks); err != nil || len(checks) == 0 {
		t.Fatal("checks 解析失败或为空")
	}
	c := checks[0]
	for _, k := range []string{"platform_hint", "observed_evidence", "evidence_source", "verdict", "crawler_visit_evidence"} {
		if _, ok := c[k]; !ok {
			t.Fatalf("check 缺字段 %s", k)
		}
	}
	if c["platform_hint"] == c["observed_evidence"] {
		t.Fatal("平台提示与实际观察发生互相填充")
	}
}

func TestDeltaEndpoints_ToolsListWrapper(t *testing.T) {
	r, _ := newDeltaRouter(t)
	code, body := getJSON(t, r, "/v1/mcp/tools?limit=2")
	if code != http.StatusOK {
		t.Fatalf("tools/list 返回 %d", code)
	}
	for _, k := range []string{"manifest_version", "returned_count", "total_count", "truncated"} {
		if _, ok := body[k]; !ok {
			t.Fatalf("tools/list 固定包装缺键 %s", k)
		}
	}
	if body["truncated"] != true {
		t.Fatal("limit=2 应 truncated")
	}
	if body["client_notice"] != "能力集不完整" {
		t.Fatal("裁切缺少「能力集不完整」提示")
	}
}

func TestDeltaEndpoints_FlagOffHidesNewFields(t *testing.T) {
	r, rt := newDeltaRouter(t)
	rt.Flag.SetEnabled(false)
	defer rt.Flag.SetEnabled(true)
	code, body := getJSON(t, r, "/v1/reports/report_demo_1")
	if code != http.StatusNotFound {
		t.Fatalf("flag=off 时端点应停用，实际 %d", code)
	}
	if body["on"] != false || body["flag"] != "aeo_delta_20261116" {
		t.Fatal("flag=off 响应未标明开关状态")
	}
	// status 端点始终可读，用于运维核对。
	_, status := getJSON(t, r, "/delta/status")
	if status["on"] != false {
		t.Fatal("status 未反映 off")
	}
}

func TestDeltaEndpoints_CrawlerEvidencePost(t *testing.T) {
	r, _ := newDeltaRouter(t)
	w := httptest.NewRecorder()
	body := `{"evidence_source":"user_provided","content":"some log","note":"自述"}`
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost,
		"/v1/audits/audit_demo_1/crawler-evidence", strings.NewReader(body)))
	if w.Code != http.StatusCreated {
		t.Fatalf("上传证据返回 %d", w.Code)
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["verdict"] != "unknown" {
		t.Fatal("用户自述日志不得判 pass")
	}
	if resp["check_id"] != "crawler_visit_9" {
		t.Fatal("check_id 拼写不符")
	}
}
