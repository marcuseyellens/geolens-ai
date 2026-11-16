package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geolens/geolens/domain"
)

func testOrder() domain.Order {
	return domain.Order{
		ID: "ord_test_1", AccountID: "acct_1", Plan: domain.PlanGrowth,
		Kind: domain.PaySubscription, AmountUSD: 199, Status: domain.PayPending,
		CreatedAt: time.Now(), IdempotencyKey: "order-idem-1",
	}
}

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestProviderRegistryRequiresCredentials(t *testing.T) {
	r := NewProviderRegistryWith(&http.Client{}, envFrom(map[string]string{}))
	list := r.List()
	if len(list) != 1 || list[0].ID != ProviderSandbox || list[0].Live {
		t.Fatalf("无凭证时应只暴露演示网关，实际 %+v", list)
	}
	if _, ok := r.Get(ProviderAirwallex); ok {
		t.Fatal("无凭证不应注册 Airwallex")
	}
	session, err := r.CreateSession(context.Background(), ProviderSandbox, testOrder(), "", "")
	if err != nil || session.CheckoutURL != "" {
		t.Fatalf("演示网关不应产生外部收银台 URL: %+v %v", session, err)
	}
}

func TestAirwallexCreateSessionAndSignature(t *testing.T) {
	var mu sync.Mutex
	var loginSeen, linkSeen bool
	var linkBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/api/v1/authentication/login"):
			mu.Lock()
			loginSeen = r.Header.Get("x-client-id") == "cid" && r.Header.Get("x-api-key") == "key"
			mu.Unlock()
			_, _ = w.Write([]byte(`{"token":"tok_awx"}`))
		case strings.HasSuffix(r.URL.Path, "/api/v1/pa/payment_links/create"):
			mu.Lock()
			linkSeen = r.Header.Get("Authorization") == "Bearer tok_awx"
			_ = json.NewDecoder(r.Body).Decode(&linkBody)
			mu.Unlock()
			_, _ = w.Write([]byte(`{"id":"pl_1","url":"https://checkout.airwallex.com/pl_1"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	r := NewProviderRegistryWith(srv.Client(), envFrom(map[string]string{
		"AIRWALLEX_CLIENT_ID": "cid", "AIRWALLEX_API_KEY": "key",
		"AIRWALLEX_API_BASE": srv.URL, "AIRWALLEX_WEBHOOK_SECRET": "whsec",
	}))
	session, err := r.CreateSession(context.Background(), ProviderAirwallex, testOrder(),
		"https://app.test/return", "https://app.test/cancel")
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !loginSeen || !linkSeen {
		t.Fatal("未按预期完成鉴权与 payment link 创建")
	}
	if session.CheckoutURL != "https://checkout.airwallex.com/pl_1" || session.SessionID != "pl_1" {
		t.Fatalf("会话字段错误: %+v", session)
	}
	if linkBody["merchant_order_id"] != "ord_test_1" || linkBody["currency"] != "USD" ||
		linkBody["amount"] != float64(199) || linkBody["return_url"] != "https://app.test/return" {
		t.Fatalf("下单请求缺少订单引用/金额/回跳地址: %+v", linkBody)
	}

	// 验签：timestamp + body 的 HMAC-SHA256。
	body := []byte(`{"id":"evt_1","name":"payment_link.paid","data":{"object":{"status":"SUCCEEDED","merchant_order_id":"ord_test_1","metadata":{"idempotency_key":"order-idem-1"}}}}`)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte("whsec"))
	_, _ = mac.Write([]byte(ts))
	_, _ = mac.Write(body)
	h := http.Header{}
	h.Set("x-timestamp", ts)
	h.Set("x-signature", hex.EncodeToString(mac.Sum(nil)))
	ev, err := r.VerifyWebhook(context.Background(), ProviderAirwallex, h, body)
	if err != nil || ev.OrderID != "ord_test_1" || ev.Status != domain.PaySucceeded {
		t.Fatalf("合法回调应激活订单: %+v %v", ev, err)
	}
	// 篡改签名 / 过期时间戳必须拒绝。
	bad := h.Clone()
	bad.Set("x-signature", strings.Repeat("0", 64))
	if _, err := r.VerifyWebhook(context.Background(), ProviderAirwallex, bad, body); err != ErrInvalidSignature {
		t.Fatalf("篡改签名必须拒绝，实际 %v", err)
	}
	old := h.Clone()
	old.Set("x-timestamp", strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10))
	if _, err := r.VerifyWebhook(context.Background(), ProviderAirwallex, old, body); err != ErrInvalidSignature {
		t.Fatalf("过期回调必须拒绝，实际 %v", err)
	}
}

func TestPayPalCreateCaptureAndWebhook(t *testing.T) {
	var mu sync.Mutex
	var orderBody map[string]any
	var captureSeen, verifySeen bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth2/token"):
			user, pass, ok := r.BasicAuth()
			if !ok || user != "pid" || pass != "psec" {
				http.Error(w, "bad credentials", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"tok_pp"}`))
		case strings.HasSuffix(r.URL.Path, "/v2/checkout/orders") && r.Method == http.MethodPost:
			mu.Lock()
			_ = json.NewDecoder(r.Body).Decode(&orderBody)
			mu.Unlock()
			_, _ = w.Write([]byte(`{"id":"PP-1","links":[{"rel":"approve","href":"https://www.sandbox.paypal.com/checkoutnow?token=PP-1"}]}`))
		case strings.Contains(r.URL.Path, "/capture"):
			mu.Lock()
			captureSeen = true
			mu.Unlock()
			_, _ = w.Write([]byte(`{"status":"COMPLETED"}`))
		case strings.HasSuffix(r.URL.Path, "/v1/notifications/verify-webhook-signature"):
			mu.Lock()
			verifySeen = true
			mu.Unlock()
			_, _ = w.Write([]byte(`{"verification_status":"SUCCESS"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	r := NewProviderRegistryWith(srv.Client(), envFrom(map[string]string{
		"PAYPAL_CLIENT_ID": "pid", "PAYPAL_CLIENT_SECRET": "psec",
		"PAYPAL_API_BASE": srv.URL, "PAYPAL_WEBHOOK_ID": "WH-1",
	}))
	providers := r.List()
	if len(providers) != 2 || providers[0].ID != ProviderPayPal || !providers[0].SandboxMode {
		t.Fatalf("PayPal 应排在首位并标注沙箱: %+v", providers)
	}
	session, err := r.CreateSession(context.Background(), ProviderPayPal, testOrder(),
		"https://app.test/return", "https://app.test/cancel")
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	units, _ := orderBody["purchase_units"].([]any)
	first, _ := units[0].(map[string]any)
	amount, _ := first["amount"].(map[string]any)
	okBody := first["reference_id"] == "ord_test_1" && amount["value"] == "199.00" && amount["currency_code"] == "USD"
	mu.Unlock()
	if !okBody || !strings.Contains(session.CheckoutURL, "checkoutnow") {
		t.Fatalf("PayPal 下单/审批链接错误: body=%+v session=%+v", orderBody, session)
	}
	if err := r.Capture(context.Background(), ProviderPayPal, session.SessionID); err != nil {
		t.Fatalf("capture 失败: %v", err)
	}

	body := []byte(`{"id":"WH-EVT-1","event_type":"PAYMENT.CAPTURE.COMPLETED","resource":{"purchase_units":[{"reference_id":"ord_test_1"}]}}`)
	h := http.Header{}
	h.Set("PAYPAL-TRANSMISSION-ID", "trans-1")
	h.Set("PAYPAL-TRANSMISSION-TIME", time.Now().UTC().Format(time.RFC3339))
	h.Set("PAYPAL-CERT-URL", "https://api.sandbox.paypal.com/cert")
	h.Set("PAYPAL-AUTH-ALGO", "SHA256withRSA")
	h.Set("PAYPAL-TRANSMISSION-SIG", "sig")
	ev, err := r.VerifyWebhook(context.Background(), ProviderPayPal, h, body)
	if err != nil || ev.OrderID != "ord_test_1" || ev.Status != domain.PaySucceeded {
		t.Fatalf("合法回调应激活订单: %+v %v", ev, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !captureSeen || !verifySeen {
		t.Fatal("未调用 capture / 验签端点")
	}
	missing := h.Clone()
	missing.Del("PAYPAL-TRANSMISSION-SIG")
	if _, err := r.VerifyWebhook(context.Background(), ProviderPayPal, missing, body); err != ErrInvalidSignature {
		t.Fatalf("缺少签名必须拒绝，实际 %v", err)
	}
}

func TestHostedWebhookActivatesOnce(t *testing.T) {
	engine := NewEngine(NewLocalGateway(0), nil)
	order, err := engine.Checkout("acct_1", domain.PlanGrowth, domain.PaySubscription)
	if err != nil {
		t.Fatal(err)
	}
	ev := WebhookEvent{EventID: "evt_once", OrderID: order.ID, Status: domain.PaySucceeded}
	first, err := engine.ApplyWebhook(ev)
	if err != nil || first.Status != domain.PaySucceeded {
		t.Fatalf("首次回调未激活: %+v %v", first, err)
	}
	repeat, err := engine.ApplyWebhook(ev)
	if err != nil || repeat.Status != domain.PaySucceeded {
		t.Fatalf("重复回调不应失败: %+v %v", repeat, err)
	}
	plan, _ := engine.AccountPlan("acct_1")
	if plan != domain.PlanGrowth {
		t.Fatalf("权益未生效: %s", plan)
	}
	if len(engine.Panel("acct_1").Invoices) != 1 {
		t.Fatal("重复回调不得重复开票")
	}
}
