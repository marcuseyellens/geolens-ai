package handlers

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/geolens/geolens/internal/auth"
	"github.com/geolens/geolens/internal/billing"
	"github.com/gin-gonic/gin"
)

// 海外收款端点的 HTTP 级回归：供应商目录、兼容旧流程、匿名回调强制验签、订单归属。
func newPaymentRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	authEngine, err := auth.NewEngine(filepath.Join(t.TempDir(), "auth.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authEngine.Seed("demo@example.com", "password123", auth.RoleAdmin, "demo-account"); err != nil {
		t.Fatal(err)
	}
	if _, err := authEngine.Register("other@example.com", "password123"); err != nil {
		t.Fatal(err)
	}
	bill := billing.NewEngine(billing.NewLocalGateway(0), nil)
	registry := billing.NewProviderRegistryWith(&http.Client{}, func(string) string { return "" })
	s := &Server{
		auth: authEngine, billing: bill, payments: registry,
		platform: &Platform{Billing: bill}, adminToken: "adm_test_token",
	}
	r := gin.New()
	api := r.Group("/api/v1")
	api.POST("/auth/login", s.authLogin)
	protected := api.Group("", s.RequireAuth())
	protected.GET("/billing/providers", s.billingProviders)
	protected.GET("/billing/orders/:id", s.billingOrder)
	protected.POST("/billing/checkout", s.billingCheckout)
	protected.POST("/billing/pay", s.billingPay)
	api.POST("/billing/webhook/:provider", s.billingProviderWebhook)
	return r
}

func authHeaders(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func TestPaymentProvidersRequireSessionAndDegradeToSandbox(t *testing.T) {
	r := newPaymentRouter(t)
	if w := jsonRequest(r, http.MethodGet, "/api/v1/billing/providers", nil, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应被拒绝，实际 %d", w.Code)
	}
	token := loginToken(t, r, "demo@example.com", "password123")
	w := jsonRequest(r, http.MethodGet, "/api/v1/billing/providers", nil, authHeaders(token))
	if w.Code != http.StatusOK {
		t.Fatalf("供应商目录不可用: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Providers []billing.PaymentProviderInfo `json:"providers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Providers) != 1 || out.Providers[0].ID != billing.ProviderSandbox || out.Providers[0].Live {
		t.Fatalf("无凭证时只应有演示网关: %+v", out.Providers)
	}
}

func TestCheckoutKeepsLegacyFlowAndRejectsUnavailableProvider(t *testing.T) {
	r := newPaymentRouter(t)
	token := loginToken(t, r, "demo@example.com", "password123")

	// 不传 provider 时保持旧行为：创建待支付订单，前端继续调 /billing/pay。
	w := jsonRequest(r, http.MethodPost, "/api/v1/billing/checkout",
		map[string]string{"plan": "growth", "kind": "subscription"}, authHeaders(token))
	if w.Code != http.StatusOK {
		t.Fatalf("旧流程下单失败: %d %s", w.Code, w.Body.String())
	}
	var checkout struct {
		Order struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"order"`
		Next        string `json:"next"`
		CheckoutURL string `json:"checkout_url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &checkout); err != nil {
		t.Fatal(err)
	}
	if checkout.Next != "pay" || checkout.CheckoutURL != "" || checkout.Order.Status != "pending" {
		t.Fatalf("演示网关应返回 next=pay 且无外部收银台: %+v", checkout)
	}
	paid := jsonRequest(r, http.MethodPost, "/api/v1/billing/pay",
		map[string]string{"order_id": checkout.Order.ID}, authHeaders(token))
	if paid.Code != http.StatusOK {
		t.Fatalf("演示支付失败: %d %s", paid.Code, paid.Body.String())
	}

	bad := jsonRequest(r, http.MethodPost, "/api/v1/billing/checkout",
		map[string]string{"plan": "growth", "provider": "paypal"}, authHeaders(token))
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("未配置供应商应被拒绝，实际 %d %s", bad.Code, bad.Body.String())
	}
}

func TestProviderWebhookRequiresSignatureAndOrderOwnership(t *testing.T) {
	r := newPaymentRouter(t)
	token := loginToken(t, r, "demo@example.com", "password123")
	other := loginToken(t, r, "other@example.com", "password123")

	created := jsonRequest(r, http.MethodPost, "/api/v1/billing/checkout",
		map[string]string{"plan": "starter"}, authHeaders(token))
	var out struct {
		Order struct {
			ID string `json:"id"`
		} `json:"order"`
	}
	_ = json.Unmarshal(created.Body.Bytes(), &out)

	// 匿名回调不能凭 body 直接改订单：验签失败一律 401。
	unsigned := jsonRequest(r, http.MethodPost, "/api/v1/billing/webhook/sandbox",
		map[string]any{"event_id": "fake", "order_id": out.Order.ID, "status": "succeeded"}, nil)
	if unsigned.Code != http.StatusUnauthorized {
		t.Fatalf("未验签回调必须拒绝，实际 %d %s", unsigned.Code, unsigned.Body.String())
	}
	if unknown := jsonRequest(r, http.MethodPost, "/api/v1/billing/webhook/paypal",
		map[string]any{}, nil); unknown.Code != http.StatusNotFound {
		t.Fatalf("未注册供应商应返回 404，实际 %d", unknown.Code)
	}
	// 订单状态仍为 pending，说明匿名回调没有越权开通权益。
	status := jsonRequest(r, http.MethodGet, "/api/v1/billing/orders/"+out.Order.ID, nil, authHeaders(token))
	var state struct {
		Order struct {
			Status string `json:"status"`
		} `json:"order"`
	}
	_ = json.Unmarshal(status.Body.Bytes(), &state)
	if state.Order.Status != "pending" {
		t.Fatalf("未验签回调不应改变订单状态: %s", state.Order.Status)
	}
	if cross := jsonRequest(r, http.MethodGet, "/api/v1/billing/orders/"+out.Order.ID, nil,
		authHeaders(other)); cross.Code != http.StatusForbidden {
		t.Fatalf("跨账户查询订单必须 403，实际 %d", cross.Code)
	}
}
