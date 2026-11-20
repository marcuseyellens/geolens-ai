package billing

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/answerlens/answerlens/domain"
)

// 海外收款：Airwallex（空中云汇）与 PayPal 走宿主收银台，供应商回调验签后
// 由既有 ApplyWebhook 幂等开通权益。凭证缺失时只暴露 sandbox 演示网关，
// 仓库内不含任何真实凭据，测试用 httptest 注入，不发起外部支付请求。

type Provider string

const (
	ProviderSandbox   Provider = "sandbox"
	ProviderAirwallex Provider = "airwallex"
	ProviderPayPal    Provider = "paypal"
)

var (
	ErrProviderUnavailable = errors.New("payment provider not configured")
	ErrProviderUnknown     = errors.New("unknown payment provider")
	ErrInvalidSignature    = errors.New("invalid webhook signature")
	ErrCheckoutURLMissing  = errors.New("payment provider returned no checkout url")
)

// PaymentProviderInfo 是给前端选择支付方式用的展示信息。
type PaymentProviderInfo struct {
	ID          Provider `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Methods     []string `json:"methods"`
	Currencies  []string `json:"currencies"`
	Live        bool     `json:"live"`
	SandboxMode bool     `json:"sandbox_mode"`
}

// CheckoutSession 记录订单与供应商会话的对应关系。
type CheckoutSession struct {
	OrderID     string    `json:"order_id"`
	Provider    Provider  `json:"provider"`
	SessionID   string    `json:"session_id"`
	CheckoutURL string    `json:"checkout_url"`
	CreatedAt   time.Time `json:"created_at"`
}

// HostedGateway 是宿主收银台供应商端口。
type HostedGateway interface {
	Info() PaymentProviderInfo
	CreateSession(ctx context.Context, order domain.Order, returnURL, cancelURL string) (*CheckoutSession, error)
	// VerifyWebhook 必须先验签，再解析事件；验签失败一律返回 ErrInvalidSignature。
	VerifyWebhook(ctx context.Context, headers http.Header, body []byte) (*WebhookEvent, error)
	// Capture 用于 PayPal 在买家批准后完成扣款；其他供应商为空实现。
	Capture(ctx context.Context, sessionID string) error
}

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type providerHTTP struct {
	client  httpDoer
	baseURL string
}

func newJSONRequest(ctx context.Context, method, rawURL string, body any, headers map[string]string) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req, nil
}

func doJSON(ctx context.Context, client httpDoer, method, rawURL string, body any, headers map[string]string, out any) error {
	req, err := newJSONRequest(ctx, method, rawURL, body, headers)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("payment provider %s returned %d: %s", method, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out != nil && len(bytes.TrimSpace(raw)) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func amountString(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

// ---------------- Airwallex（空中云汇） ----------------

type AirwallexGateway struct {
	clientID      string
	apiKey        string
	webhookSecret string
	baseURL       string
	http          providerHTTP
}

func NewAirwallexGateway(client httpDoer, baseURL, clientID, apiKey, webhookSecret string) *AirwallexGateway {
	if baseURL == "" {
		baseURL = "https://api-demo.airwallex.com"
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &AirwallexGateway{
		clientID: clientID, apiKey: apiKey, webhookSecret: webhookSecret,
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    providerHTTP{client: client, baseURL: strings.TrimRight(baseURL, "/")},
	}
}

func (g *AirwallexGateway) Info() PaymentProviderInfo {
	return PaymentProviderInfo{
		ID: ProviderAirwallex, Name: "Airwallex",
		Description: "信用卡 / 借记卡、Apple Pay、Google Pay",
		Methods:     []string{"card", "apple_pay", "google_pay"},
		Currencies:  []string{"USD", "EUR", "GBP", "SGD", "AUD", "HKD"},
		// 只有官方生产域名才算 live；demo 域名与本地 httptest 一律标注沙箱。
		Live: true, SandboxMode: !strings.Contains(g.baseURL, "api.airwallex.com") || strings.Contains(g.baseURL, "demo"),
	}
}

func (g *AirwallexGateway) token(ctx context.Context) (string, error) {
	var out struct {
		Token string `json:"token"`
	}
	err := doJSON(ctx, g.http.client, http.MethodPost, g.baseURL+"/api/v1/authentication/login",
		map[string]any{}, map[string]string{
			"x-client-id": g.clientID, "x-api-key": g.apiKey,
		}, &out)
	if err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", errors.New("airwallex authentication returned empty token")
	}
	return out.Token, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (g *AirwallexGateway) CreateSession(ctx context.Context, order domain.Order, returnURL, cancelURL string) (*CheckoutSession, error) {
	token, err := g.token(ctx)
	if err != nil {
		return nil, err
	}
	// Payment Link 让买家在 Airwallex 托管页面完成付款；卡号不经过本服务。
	payload := map[string]any{
		"amount":            order.AmountUSD,
		"currency":          "USD",
		"merchant_order_id": order.ID,
		"description":       "GeoLens AI " + string(order.Plan),
		"return_url":        returnURL,
		"cancel_url":        cancelURL,
		"reusable":          false,
		"metadata":          map[string]string{"order_id": order.ID, "idempotency_key": order.IdempotencyKey},
	}
	var out struct {
		ID             string `json:"id"`
		URL            string `json:"url"`
		PaymentLinkURL string `json:"payment_link_url"`
		Link           string `json:"link"`
		Href           string `json:"href"`
	}
	err = doJSON(ctx, g.http.client, http.MethodPost, g.baseURL+"/api/v1/pa/payment_links/create",
		payload, map[string]string{
			"Authorization": "Bearer " + token,
			"x-client-id":   g.clientID,
		}, &out)
	if err != nil {
		return nil, err
	}
	link := firstNonEmpty(out.URL, out.PaymentLinkURL, out.Link, out.Href)
	if link == "" {
		return nil, ErrCheckoutURLMissing
	}
	return &CheckoutSession{
		OrderID: order.ID, Provider: ProviderAirwallex,
		SessionID: out.ID, CheckoutURL: link, CreatedAt: time.Now(),
	}, nil
}

// VerifyWebhook 校验 x-timestamp + x-signature（HMAC-SHA256），并拒绝 5 分钟外的重放。
func (g *AirwallexGateway) VerifyWebhook(_ context.Context, headers http.Header, body []byte) (*WebhookEvent, error) {
	if g.webhookSecret == "" {
		return nil, ErrInvalidSignature
	}
	ts := headers.Get("x-timestamp")
	sig := strings.TrimPrefix(headers.Get("x-signature"), "sha256=")
	if ts == "" || sig == "" {
		return nil, ErrInvalidSignature
	}
	if sec, err := strconv.ParseInt(ts, 10, 64); err == nil {
		stamp := time.Unix(sec, 0)
		if sec > 1e12 { // 毫秒时间戳
			stamp = time.UnixMilli(sec)
		}
		if time.Since(stamp) > 5*time.Minute || time.Since(stamp) < -5*time.Minute {
			return nil, ErrInvalidSignature
		}
	}
	mac := hmac.New(sha256.New, []byte(g.webhookSecret))
	_, _ = mac.Write([]byte(ts))
	_, _ = mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	got, err := hex.DecodeString(sig)
	if err != nil || !hmac.Equal(got, mustHex(expected)) {
		return nil, ErrInvalidSignature
	}
	var raw struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Data struct {
			Object struct {
				ID              string            `json:"id"`
				MerchantOrderID string            `json:"merchant_order_id"`
				Status          string            `json:"status"`
				Metadata        map[string]string `json:"metadata"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	orderID := firstNonEmpty(raw.Data.Object.MerchantOrderID, raw.Data.Object.Metadata["order_id"])
	if orderID == "" {
		return nil, errors.New("airwallex webhook missing order reference")
	}
	status := domain.PayFailed
	if strings.Contains(strings.ToLower(raw.Name), "succeeded") ||
		strings.Contains(strings.ToLower(raw.Data.Object.Status), "succeeded") ||
		strings.EqualFold(raw.Data.Object.Status, "success") {
		status = domain.PaySucceeded
	}
	return &WebhookEvent{
		EventID: "awx_" + raw.ID, OrderID: orderID, Status: status,
		IdempotencyKey: raw.Data.Object.Metadata["idempotency_key"],
	}, nil
}

func mustHex(s string) []byte {
	b, _ := hex.DecodeString(s)
	return b
}

func (g *AirwallexGateway) Capture(context.Context, string) error { return nil }

// ---------------- PayPal ----------------

type PayPalGateway struct {
	clientID    string
	secret      string
	webhookID   string
	baseURL     string
	http        providerHTTP
	returnBrand string
}

func NewPayPalGateway(client httpDoer, baseURL, clientID, secret, webhookID string) *PayPalGateway {
	if baseURL == "" {
		baseURL = "https://api-m.sandbox.paypal.com"
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &PayPalGateway{
		clientID: clientID, secret: secret, webhookID: webhookID,
		baseURL: strings.TrimRight(baseURL, "/"), returnBrand: "GeoLens AI",
		http: providerHTTP{client: client, baseURL: strings.TrimRight(baseURL, "/")},
	}
}

func (g *PayPalGateway) Info() PaymentProviderInfo {
	return PaymentProviderInfo{
		ID: ProviderPayPal, Name: "PayPal",
		Description: "PayPal 余额、绑定的银行卡与 Pay Later",
		Methods:     []string{"paypal_wallet", "card", "pay_later"},
		Currencies:  []string{"USD", "EUR", "GBP", "AUD", "CAD", "JPY"},
		Live:        true, SandboxMode: !strings.Contains(g.baseURL, "api-m.paypal.com"),
	}
}

func (g *PayPalGateway) token(ctx context.Context) (string, error) {
	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.baseURL+"/v1/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(g.clientID, g.secret)
	resp, err := g.http.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("paypal oauth returned %d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.AccessToken == "" {
		return "", errors.New("paypal oauth returned empty access token")
	}
	return out.AccessToken, nil
}

func (g *PayPalGateway) CreateSession(ctx context.Context, order domain.Order, returnURL, cancelURL string) (*CheckoutSession, error) {
	token, err := g.token(ctx)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"intent": "CAPTURE",
		"purchase_units": []map[string]any{{
			"reference_id": order.ID,
			"custom_id":    order.IdempotencyKey,
			"description":  "GeoLens AI " + string(order.Plan),
			"amount": map[string]any{
				"currency_code": "USD", "value": amountString(order.AmountUSD),
			},
		}},
		"application_context": map[string]any{
			"brand_name":          g.returnBrand,
			"return_url":          returnURL,
			"cancel_url":          cancelURL,
			"user_action":         "PAY_NOW",
			"shipping_preference": "NO_SHIPPING",
		},
	}
	var out struct {
		ID    string `json:"id"`
		Links []struct {
			Rel  string `json:"rel"`
			Href string `json:"href"`
		} `json:"links"`
	}
	err = doJSON(ctx, g.http.client, http.MethodPost, g.baseURL+"/v2/checkout/orders", payload,
		map[string]string{
			"Authorization":     "Bearer " + token,
			"PayPal-Request-Id": order.IdempotencyKey,
			"Prefer":            "return=representation",
		}, &out)
	if err != nil {
		return nil, err
	}
	approve := ""
	for _, l := range out.Links {
		if strings.EqualFold(l.Rel, "approve") || strings.EqualFold(l.Rel, "payer-action") {
			approve = l.Href
			break
		}
	}
	if approve == "" {
		return nil, ErrCheckoutURLMissing
	}
	return &CheckoutSession{
		OrderID: order.ID, Provider: ProviderPayPal,
		SessionID: out.ID, CheckoutURL: approve, CreatedAt: time.Now(),
	}, nil
}

// Capture 在买家批准回到 return_url 后完成扣款。
func (g *PayPalGateway) Capture(ctx context.Context, sessionID string) error {
	token, err := g.token(ctx)
	if err != nil {
		return err
	}
	var out struct {
		Status string `json:"status"`
	}
	err = doJSON(ctx, g.http.client, http.MethodPost,
		g.baseURL+"/v2/checkout/orders/"+url.PathEscape(sessionID)+"/capture",
		map[string]any{}, map[string]string{"Authorization": "Bearer " + token}, &out)
	if err != nil {
		return err
	}
	if !strings.EqualFold(out.Status, "COMPLETED") {
		return fmt.Errorf("paypal capture status %s", out.Status)
	}
	return nil
}

// VerifyWebhook 调 PayPal 的验签端点，绝不直接信任回调 body。
func (g *PayPalGateway) VerifyWebhook(ctx context.Context, headers http.Header, body []byte) (*WebhookEvent, error) {
	if g.webhookID == "" {
		return nil, ErrInvalidSignature
	}
	token, err := g.token(ctx)
	if err != nil {
		return nil, err
	}
	var event map[string]any
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, err
	}
	verify := map[string]any{
		"transmission_id":   headers.Get("PAYPAL-TRANSMISSION-ID"),
		"transmission_time": headers.Get("PAYPAL-TRANSMISSION-TIME"),
		"cert_url":          headers.Get("PAYPAL-CERT-URL"),
		"auth_algo":         headers.Get("PAYPAL-AUTH-ALGO"),
		"transmission_sig":  headers.Get("PAYPAL-TRANSMISSION-SIG"),
		"webhook_id":        g.webhookID,
		"webhook_event":     event,
	}
	if verify["transmission_sig"] == "" || verify["transmission_id"] == "" {
		return nil, ErrInvalidSignature
	}
	var result struct {
		Status string `json:"verification_status"`
	}
	err = doJSON(ctx, g.http.client, http.MethodPost,
		g.baseURL+"/v1/notifications/verify-webhook-signature", verify,
		map[string]string{"Authorization": "Bearer " + token}, &result)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(result.Status, "SUCCESS") {
		return nil, ErrInvalidSignature
	}
	eventID, _ := event["id"].(string)
	eventType, _ := event["event_type"].(string)
	resource, _ := event["resource"].(map[string]any)
	orderID := ""
	if resource != nil {
		if units, ok := resource["purchase_units"].([]any); ok && len(units) > 0 {
			if u, ok := units[0].(map[string]any); ok {
				orderID, _ = u["reference_id"].(string)
			}
		}
		if orderID == "" {
			orderID, _ = resource["custom_id"].(string)
		}
	}
	if orderID == "" {
		return nil, errors.New("paypal webhook missing order reference")
	}
	status := domain.PayFailed
	upper := strings.ToUpper(eventType)
	if strings.Contains(upper, "COMPLETED") || strings.Contains(upper, "APPROVED") {
		status = domain.PaySucceeded
	}
	if strings.Contains(upper, "REFUNDED") || strings.Contains(upper, "REVERSED") {
		status = domain.PayRefunded
	}
	return &WebhookEvent{EventID: "pp_" + eventID, OrderID: orderID, Status: status}, nil
}

// ---------------- Sandbox（演示，永不发起外部请求） ----------------

type SandboxGateway struct{}

func (SandboxGateway) Info() PaymentProviderInfo {
	return PaymentProviderInfo{
		ID: ProviderSandbox, Name: "演示收银台",
		Description: "立即开通，不扣款，用于体验完整流程",
		Methods:     []string{"demo"}, Currencies: []string{"USD"},
		Live: false, SandboxMode: true,
	}
}

func (SandboxGateway) CreateSession(_ context.Context, order domain.Order, _, _ string) (*CheckoutSession, error) {
	return &CheckoutSession{
		OrderID: order.ID, Provider: ProviderSandbox,
		SessionID: "demo_" + order.ID, CreatedAt: time.Now(),
	}, nil
}

func (SandboxGateway) VerifyWebhook(context.Context, http.Header, []byte) (*WebhookEvent, error) {
	return nil, ErrInvalidSignature // 演示网关不接受外部回调
}

func (SandboxGateway) Capture(context.Context, string) error { return nil }

// ---------------- Registry ----------------

type ProviderRegistry struct {
	gateways map[Provider]HostedGateway
	order    []Provider
}

// NewProviderRegistry 从环境变量装配可用供应商。凭证缺失时只有演示网关。
func NewProviderRegistry() *ProviderRegistry { return NewProviderRegistryWith(nil, os.Getenv) }

// NewProviderRegistryWith 允许测试注入 client 与 env，不触碰真实网络。
func NewProviderRegistryWith(client httpDoer, getenv func(string) string) *ProviderRegistry {
	if getenv == nil {
		getenv = os.Getenv
	}
	r := &ProviderRegistry{
		gateways: map[Provider]HostedGateway{ProviderSandbox: SandboxGateway{}},
		order:    []Provider{ProviderSandbox},
	}
	if id, key := getenv("AIRWALLEX_CLIENT_ID"), getenv("AIRWALLEX_API_KEY"); id != "" && key != "" {
		g := NewAirwallexGateway(client, getenv("AIRWALLEX_API_BASE"), id, key, getenv("AIRWALLEX_WEBHOOK_SECRET"))
		r.gateways[ProviderAirwallex] = g
		r.order = append([]Provider{ProviderAirwallex}, r.order...)
	}
	if id, secret := getenv("PAYPAL_CLIENT_ID"), getenv("PAYPAL_CLIENT_SECRET"); id != "" && secret != "" {
		g := NewPayPalGateway(client, getenv("PAYPAL_API_BASE"), id, secret, getenv("PAYPAL_WEBHOOK_ID"))
		r.gateways[ProviderPayPal] = g
		r.order = append([]Provider{ProviderPayPal}, r.order...)
	}
	return r
}

func (r *ProviderRegistry) List() []PaymentProviderInfo {
	out := make([]PaymentProviderInfo, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.gateways[id].Info())
	}
	return out
}

func (r *ProviderRegistry) Get(id Provider) (HostedGateway, bool) {
	g, ok := r.gateways[id]
	return g, ok
}

func (r *ProviderRegistry) CreateSession(ctx context.Context, id Provider, order domain.Order, returnURL, cancelURL string) (*CheckoutSession, error) {
	g, ok := r.gateways[id]
	if !ok {
		return nil, ErrProviderUnavailable
	}
	return g.CreateSession(ctx, order, returnURL, cancelURL)
}

func (r *ProviderRegistry) VerifyWebhook(ctx context.Context, id Provider, headers http.Header, body []byte) (*WebhookEvent, error) {
	g, ok := r.gateways[id]
	if !ok {
		return nil, ErrProviderUnknown
	}
	return g.VerifyWebhook(ctx, headers, body)
}

func (r *ProviderRegistry) Capture(ctx context.Context, id Provider, sessionID string) error {
	g, ok := r.gateways[id]
	if !ok {
		return ErrProviderUnknown
	}
	return g.Capture(ctx, sessionID)
}
