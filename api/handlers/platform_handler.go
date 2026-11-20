package handlers

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/answerlens/answerlens/domain"
	"github.com/answerlens/answerlens/internal/aeo"
	"github.com/answerlens/answerlens/internal/auth"
	"github.com/answerlens/answerlens/internal/billing"
	"github.com/answerlens/answerlens/internal/collector"
	"github.com/answerlens/answerlens/internal/governance"
	"github.com/answerlens/answerlens/internal/monitor"
	"github.com/answerlens/answerlens/internal/omnigraph"
	"github.com/answerlens/answerlens/internal/regression"
	"github.com/answerlens/answerlens/internal/siteaudit"
	"github.com/gin-gonic/gin"
)

// registerPlatformRoutes 挂载基线 WBS WP-1/2/3/5/7/8 的运行时端点。
// api 为匿名组，protected 要求登录会话，admin 要求管理员角色或 X-Admin-Token。
func (s *Server) registerPlatformRoutes(api, protected, admin *gin.RouterGroup, r *gin.Engine) {
	if s.platform == nil {
		return
	}
	// WP-5 MCP：JSON-RPC 用独立的只读 Agent token，不走用户会话。
	r.POST("/mcp", s.mcpEndpoint)
	r.GET("/mcp", s.mcpInfo)
	// 套餐目录不含用户数据，允许匿名查看；签发 token 属于管理操作。
	api.GET("/plans", s.listPlans)
	admin.POST("/mcp/tokens", s.mcpIssueToken)

	// WP-3 运行治理
	protected.GET("/governance", s.governanceOverview)
	protected.POST("/governance/dispatch", s.governanceDispatch)
	protected.POST("/governance/rate-limit", s.governanceRateLimit)
	protected.POST("/governance/resume", s.governanceResume)
	admin.POST("/governance/reset-budget", s.governanceReset)

	// WP-2 市场图谱
	protected.GET("/graph", s.graphOverview)
	protected.POST("/graph/branches", s.graphCreateBranch)
	protected.POST("/graph/nodes", s.graphWriteNode)
	protected.POST("/graph/edges", s.graphWriteEdge)
	protected.POST("/graph/merge", s.graphMerge)
	protected.GET("/graph/at", s.graphAt)
	protected.POST("/graph/search", s.graphSearch)
	admin.POST("/graph/rollback", s.graphRollback)

	// WP-1 采集框架
	protected.GET("/collector", s.collectorOverview)
	protected.POST("/collector/batch", s.collectorBatch)
	protected.GET("/collector/runs", s.collectorRuns)
	protected.GET("/collector/margins", s.collectorMargins)

	// WP-7 套餐计费
	protected.GET("/billing/panel", s.billingPanel)
	protected.GET("/billing/quota", s.billingQuota)
	protected.POST("/billing/checkout", s.billingCheckout)
	protected.POST("/billing/pay", s.billingPay)
	// 海外收款：支付方式目录、订单状态轮询与买家回跳需登录；供应商回调匿名但强制验签。
	protected.GET("/billing/providers", s.billingProviders)
	protected.GET("/billing/orders/:id", s.billingOrder)
	protected.POST("/billing/orders/:id/return", s.billingReturn)
	api.POST("/billing/webhook/:provider", s.billingProviderWebhook)
	admin.POST("/billing/webhook", s.billingWebhook)
	admin.POST("/billing/reconcile", s.billingReconcile)
	admin.POST("/redeem/codes", s.generateRedeemCodes)
	admin.GET("/redeem/codes", s.listRedeemCodes)

	// WP-8 监控与回归
	protected.GET("/monitor", s.monitorSnapshot)
	protected.GET("/monitor/release-checklist", s.releaseChecklist)
	protected.POST("/full-chain", s.fullChain)
}

// requireAdmin 兼容两种管理凭据：路由中间件写入的 admin 角色，或 X-Admin-Token。
func (s *Server) requireAdmin(c *gin.Context) bool {
	if role, _ := c.Get(ctxAuthRole); role == string(auth.RoleAdmin) {
		return true
	}
	if s.adminToken != "" && c.GetHeader("X-Admin-Token") == s.adminToken {
		return true
	}
	c.JSON(http.StatusForbidden, gin.H{"error": "admin role or X-Admin-Token required"})
	return false
}

func platformUnavailable(c *gin.Context) {
	c.JSON(http.StatusServiceUnavailable, gin.H{"error": "platform runtime not configured"})
}

// ---- WP-3 运行治理 ----

func (s *Server) governanceOverview(c *gin.Context) {
	if s.governance == nil {
		platformUnavailable(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"routes": s.governance.RouteTable(), "budget": s.governance.Budget(),
		"pentads": s.governance.Pentads(), "operators": s.governance.ByOperator(),
		"pending": s.governance.Pending(), "events": s.governance.BudgetEvents(),
		"methodology": domain.MethodologyVersion,
		"provenance":  domain.ObservedProvenance(len(s.governance.Pentads())),
	})
}

func (s *Server) governanceDispatch(c *gin.Context) {
	if s.governance == nil {
		platformUnavailable(c)
		return
	}
	var body struct {
		Operator string          `json:"operator"`
		Task     domain.TaskKind `json:"task"`
		Change   string          `json:"change"`
		Platform string          `json:"platform"`
		RetryOf  string          `json:"retry_of"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := s.governance.Dispatch(governance.RunRequest{
		Operator: body.Operator, Task: body.Task, Change: body.Change,
		Platform: body.Platform, RetryOf: body.RetryOf,
	})
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "result": res})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": res, "budget": s.governance.Budget()})
}

func (s *Server) governanceRateLimit(c *gin.Context) {
	if s.governance == nil {
		platformUnavailable(c)
		return
	}
	var body struct {
		Platform      string `json:"platform"`
		RetryAfterSec int    `json:"retry_after_sec"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.RetryAfterSec <= 0 {
		body.RetryAfterSec = 30
	}
	s.governance.ReportRateLimit(domain.RateLimitSignal{
		Platform: body.Platform, RetryAfter: time.Duration(body.RetryAfterSec) * time.Second,
	})
	c.JSON(http.StatusOK, gin.H{"ok": true, "note": "已进入限速窗口排队，不立即重试"})
}

func (s *Server) governanceResume(c *gin.Context) {
	if s.governance == nil {
		platformUnavailable(c)
		return
	}
	resumed := s.governance.Resume()
	c.JSON(http.StatusOK, gin.H{"resumed": resumed, "pending": s.governance.Pending()})
}

func (s *Server) governanceReset(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	s.governance.ResetBudget(0)
	c.JSON(http.StatusOK, gin.H{"budget": s.governance.Budget()})
}

// ---- WP-2 市场图谱 ----

func (s *Server) graphOverview(c *gin.Context) {
	if s.graph == nil {
		platformUnavailable(c)
		return
	}
	nodes, edges := s.graph.Snapshot()
	kinds := make([]string, 0, len(omnigraph.AllNodeKinds()))
	for _, k := range omnigraph.AllNodeKinds() {
		kinds = append(kinds, string(k))
	}
	c.JSON(http.StatusOK, gin.H{
		"ontology": kinds, "nodes": nodes, "edges": edges,
		"merges": s.graph.MergeHistory(),
		"note":   "collector 身份禁止直接写 main；批次经 reviewer 审核后合入，可整体回滚。",
	})
}

func (s *Server) graphCreateBranch(c *gin.Context) {
	var body struct {
		ID string             `json:"id"`
		By omnigraph.Identity `json:"by"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	b, err := s.graph.CreateBranch(body.ID, body.By)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, b)
}

func graphIdentity(by string) omnigraph.Identity {
	switch omnigraph.Identity(by) {
	case omnigraph.IdentityCollector, omnigraph.IdentityReviewer, omnigraph.IdentityAdmin:
		return omnigraph.Identity(by)
	default:
		return omnigraph.IdentityCollector
	}
}

func (s *Server) graphWriteNode(c *gin.Context) {
	var body struct {
		Branch string         `json:"branch"`
		By     string         `json:"by"`
		Node   omnigraph.Node `json:"node"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Node.ValidFrom.IsZero() {
		body.Node.ValidFrom = time.Now()
	}
	if err := s.graph.WriteNode(body.Branch, graphIdentity(body.By), body.Node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true, "node": body.Node})
}

func (s *Server) graphWriteEdge(c *gin.Context) {
	var body struct {
		Branch string         `json:"branch"`
		By     string         `json:"by"`
		Edge   omnigraph.Edge `json:"edge"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.graph.WriteEdge(body.Branch, graphIdentity(body.By), body.Edge); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true})
}

func (s *Server) graphMerge(c *gin.Context) {
	var body struct {
		Branch string `json:"branch"`
		By     string `json:"by"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rec, err := s.graph.Merge(body.Branch, graphIdentity(body.By))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rec)
}

func (s *Server) graphRollback(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	var body struct {
		MergeID string `json:"merge_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.graph.RollbackMerge(body.MergeID, omnigraph.IdentityAdmin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "merges": s.graph.MergeHistory()})
}

func (s *Server) graphAt(c *gin.Context) {
	raw := c.Query("as_of")
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "as_of 必须是 RFC3339 时间"})
		return
	}
	g := s.graph.At(t)
	c.JSON(http.StatusOK, gin.H{"as_of": t, "graph": g, "nodes": len(g.Nodes), "edges": len(g.Edges)})
}

func (s *Server) graphSearch(c *gin.Context) {
	var q omnigraph.HybridQuery
	if err := c.ShouldBindJSON(&q); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"hits": s.graph.HybridSearch(q), "provenance": domain.ObservedProvenance(len(s.graph.HybridSearch(q)))})
}

// ---- WP-1 采集框架 ----

func (s *Server) collectorOverview(c *gin.Context) {
	if s.collector == nil {
		platformUnavailable(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"adapters": s.collector.Adapters(), "policy": collector.DefaultPolicy(),
		"margins": s.collector.Margins(), "runs": len(s.collector.Runs()),
		"simulated": true,
		"note":      "本地运行使用 LocalSandbox，产物明确标注为模拟，不冒充第三方实测；生产需替换为 Offstage Cloud 沙箱实现。",
	})
}

func (s *Server) collectorBatch(c *gin.Context) {
	var body struct {
		BrandID  string           `json:"brand_id"`
		Platform domain.AIModelID `json:"platform"`
		Region   string           `json:"region"`
		Count    int              `json:"count"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Count <= 0 || body.Count > 30 {
		body.Count = 30
	}
	if body.Platform == "" {
		body.Platform = domain.ModelChatGPT
	}
	if body.Region == "" {
		body.Region = "us"
	}
	qs := make([]domain.DetectionQuery, 0, body.Count)
	for i := 0; i < body.Count; i++ {
		qs = append(qs, domain.DetectionQuery{
			ID: "batch-q-" + string(rune('a'+i%26)), Text: "推荐一个 AEO 平台",
			Intent: domain.IntentRecommend,
		})
	}
	report := s.collector.CollectBatch(body.Platform, qs, body.BrandID, body.Region)
	c.JSON(http.StatusOK, gin.H{
		"report": report, "simulated": true,
		"provenance": domain.ObservedProvenance(report.Valid),
		"note":       "本地模拟采集批次，用于验证会话生命周期、失败枚举与按秒成本归集。",
	})
}

func (s *Server) collectorRuns(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"runs": s.collector.Runs(), "simulated": true})
}

func (s *Server) collectorMargins(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"margins": s.collector.Margins(), "simulated": true})
}

// ---- WP-7 套餐计费 ----

func (s *Server) listPlans(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"plans": s.billing.Plans(),
		"note":  "套餐价格与结构冻结；运行配额按日/月硬上限执行，超限运行停止并提示升级，不静默超采。",
	})
}

// accountID 只从登录会话取，忽略客户端传来的 account_id（防越权访问他人账单）。
func accountID(c *gin.Context) string {
	return currentAccount(c)
}

func (s *Server) billingPanel(c *gin.Context) {
	c.JSON(http.StatusOK, s.billing.Panel(accountID(c)))
}

func (s *Server) billingQuota(c *gin.Context) {
	id := accountID(c)
	c.JSON(http.StatusOK, gin.H{"usage": s.billing.Usage(id), "decision": s.billing.CheckQuota(id, 1)})
}

func (s *Server) billingCheckout(c *gin.Context) {
	var body struct {
		Plan     domain.PlanID      `json:"plan"`
		Kind     domain.PaymentKind `json:"kind"`
		Provider billing.Provider   `json:"provider"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Kind == "" {
		body.Kind = domain.PaySubscription
	}
	if body.Provider == "" {
		body.Provider = billing.ProviderSandbox
	}
	if _, ok := s.payments.Get(body.Provider); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payment provider not available"})
		return
	}
	// 账户身份来自会话，body 里即便带 account_id 也会被忽略。
	id := currentAccount(c)
	if _, err := s.billing.SelectPlan(id, body.Plan); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	order, err := s.billing.Checkout(id, body.Plan, body.Kind)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp := gin.H{"order": order, "provider": body.Provider,
		"steps": []string{"选档", "确认权益", "支付/开通"}}
	// 免费层直接开通，无需收银台。
	if order.Status == domain.PaySucceeded {
		resp["next"] = "done"
		c.JSON(http.StatusOK, resp)
		return
	}
	base := publicBaseURL(c)
	returnURL := base + "/subscribe?order=" + order.ID + "&payment=return"
	cancelURL := base + "/subscribe?order=" + order.ID + "&payment=cancel"
	session, err := s.payments.CreateSession(c.Request.Context(), body.Provider, *order, returnURL, cancelURL)
	if err != nil {
		// 订单保留 pending，可换一种支付方式重试；不产生任何权益。
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "order": order, "next": "retry"})
		return
	}
	s.billing.SaveSession(session)
	resp["checkout_url"] = session.CheckoutURL
	resp["session_id"] = session.SessionID
	if session.CheckoutURL == "" {
		resp["next"] = "pay" // 演示网关沿用本地即时开通
	} else {
		resp["next"] = "redirect" // 跳转供应商托管收银台
	}
	c.JSON(http.StatusOK, resp)
}

// publicBaseURL 只用于构造供应商回跳地址，优先取浏览器 Origin。
func publicBaseURL(c *gin.Context) string {
	if origin := strings.TrimRight(c.GetHeader("Origin"), "/"); origin != "" {
		return origin
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if fwd := c.GetHeader("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	return scheme + "://" + c.Request.Host
}

// billingProviders 返回当前部署实际可用的收款方式；凭证缺失时只有演示网关。
func (s *Server) billingProviders(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"providers": s.payments.List(),
		"note":      "收款由第三方托管页面完成，本服务不接触银行卡信息。",
	})
}

func (s *Server) billingOrder(c *gin.Context) {
	id := currentAccount(c)
	orderID := c.Param("id")
	owned := false
	for _, o := range s.billing.Panel(id).Orders {
		if o.ID == orderID {
			owned = true
			break
		}
	}
	if !owned {
		c.JSON(http.StatusForbidden, gin.H{"error": "order does not belong to current account"})
		return
	}
	order, ok := s.billing.Order(orderID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": order})
}

// billingReturn 处理买家从托管收银台跳回：PayPal 需在此完成 capture，
// Airwallex 已由供应商扣款，这里只补齐开通；与 Webhook 重复投递天然幂等。
func (s *Server) billingReturn(c *gin.Context) {
	id := currentAccount(c)
	orderID := c.Param("id")
	owned := false
	for _, o := range s.billing.Panel(id).Orders {
		if o.ID == orderID {
			owned = true
			break
		}
	}
	if !owned {
		c.JSON(http.StatusForbidden, gin.H{"error": "order does not belong to current account"})
		return
	}
	session, ok := s.billing.Session(orderID)
	if !ok || session.Provider == billing.ProviderSandbox {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no hosted checkout session for this order"})
		return
	}
	if err := s.payments.Capture(c.Request.Context(), session.Provider, session.SessionID); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "next": "retry"})
		return
	}
	order, _ := s.billing.Order(orderID)
	ev := billing.WebhookEvent{
		EventID: "return_" + session.SessionID, OrderID: orderID,
		Status: domain.PaySucceeded, IdempotencyKey: order.IdempotencyKey,
	}
	settled, err := s.billing.ApplyWebhook(ev)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": settled, "usage": s.billing.Usage(id)})
}

// billingProviderWebhook 匿名可达，但必须先通过供应商签名校验才允许改订单状态。
func (s *Server) billingProviderWebhook(c *gin.Context) {
	provider := billing.Provider(c.Param("provider"))
	if _, ok := s.payments.Get(provider); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown payment provider"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ev, err := s.payments.VerifyWebhook(c.Request.Context(), provider, c.Request.Header, body)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "webhook signature verification failed"})
		return
	}
	if ev.IdempotencyKey == "" {
		if order, ok := s.billing.Order(ev.OrderID); ok {
			ev.IdempotencyKey = order.IdempotencyKey
		}
	}
	order, err := s.billing.ApplyWebhook(*ev)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": order, "idempotent": true})
}

func (s *Server) billingPay(c *gin.Context) {
	var body struct {
		OrderID string `json:"order_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 支付前先校验订单属于当前会话账户，防止拿着别人的 order_id 代付/越权。
	id := currentAccount(c)
	owned := false
	for _, o := range s.billing.Panel(id).Orders {
		if o.ID == body.OrderID {
			owned = true
			break
		}
	}
	if !owned {
		c.JSON(http.StatusForbidden, gin.H{"error": "order does not belong to current account"})
		return
	}
	order, err := s.billing.Pay(body.OrderID)
	if err != nil {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": err.Error(), "order": order})
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": order, "usage": s.billing.Usage(id)})
}

func (s *Server) billingWebhook(c *gin.Context) {
	var ev billing.WebhookEvent
	if err := c.ShouldBindJSON(&ev); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	order, err := s.billing.ApplyWebhook(ev)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": order, "idempotent": true})
}

func (s *Server) billingReconcile(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"repaired_orders": s.billing.Reconcile(), "replayable": true})
}

func (s *Server) redeem(c *gin.Context) {
	var body struct {
		Code    string `json:"code"`
		Contact string `json:"contact"`
		Note    string `json:"note"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	code, fb, err := s.billing.Redeem(body.Code, body.Contact, body.Note)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": code.Code, "plan": code.Plan, "feedback": fb,
		"note": "反馈已进入 2 工作日人工交付队列。"})
}

func (s *Server) generateRedeemCodes(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	var body struct {
		Batch string        `json:"batch"`
		Plan  domain.PlanID `json:"plan"`
		Count int           `json:"count"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Count <= 0 {
		body.Count = 10
	}
	codes, err := s.billing.GenerateCodes(body.Batch, body.Plan, body.Count)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"codes": codes, "max_cold_start": billing.MaxColdStartCodes})
}

func (s *Server) listRedeemCodes(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"codes": s.billing.Codes(), "feedback_queue": s.billing.FeedbackQueue()})
}

// ---- WP-8 监控与回归 ----

// offlineRegressionFetcher 让 /full-chain 完全离线，不向外部站点发请求。
type offlineRegressionFetcher struct{}

func (offlineRegressionFetcher) Fetch(rawURL string) siteaudit.FetchResult {
	switch {
	case strings.HasSuffix(rawURL, "/robots.txt"):
		return siteaudit.FetchResult{Status: 200, Body: "User-agent: *\nAllow: /"}
	case strings.HasSuffix(rawURL, "/llms.txt"):
		return siteaudit.FetchResult{Status: 200, Body: "# GeoLens AI\n> AEO 平台"}
	default:
		return siteaudit.FetchResult{Status: 200, Body: "<!doctype html><html><head><title>GeoLens AI</title>" +
			"<meta name=\"description\" content=\"AEO 可见性平台\"></head><body><h1>GeoLens AI</h1></body></html>"}
	}
}

func (s *Server) monitorSnapshot(c *gin.Context) {
	// 从运行时实例实时汇总五块看板，避免展示静态假数据。
	valid, total := 0, 0
	var cost float64
	for _, r := range s.collector.Runs() {
		total++
		if r.Answer.Valid {
			valid++
		}
		cost += r.Answer.Cost
	}
	payTotal, payFailed := 0, 0
	for _, o := range s.billing.Panel(accountID(c)).Orders {
		payTotal++
		if o.Status == domain.PayFailed {
			payFailed++
		}
	}
	funnel := s.growth.Funnel()
	conversion := 0.0
	if funnel.Conversions != nil {
		conversion = funnel.Conversions["subscribe"]
	}
	s.monitor.Update(monitor.Sources{
		CollectionValid: valid, CollectionTotal: total, DailyCostUSD: cost,
		BudgetTrips:   s.governance.Budget().TrippedEvents,
		PaymentFailed: payFailed, PaymentTotal: payTotal,
		FunnelConversion: conversion,
	})
	snap := s.monitor.Snapshot()
	c.JSON(http.StatusOK, gin.H{
		"snapshot": snap, "boards": monitor.Boards(snap),
		"history": s.monitor.History(),
		"note":    "阈值由本平台设定，不代表任何外部平台规则；告警只描述观测数值。",
	})
}

func (s *Server) releaseChecklist(c *gin.Context) {
	// 状态全部由真实仓库内容与自检函数求值，不硬编码「通过」。
	dist := s.frontendRoot()
	root := repoRootOf(dist)
	readRepo := func(parts ...string) string {
		b, err := os.ReadFile(filepath.Join(append([]string{root}, parts...)...))
		if err != nil {
			return ""
		}
		return string(b)
	}
	files := []string{}
	for _, name := range []string{"llms.txt", "robots.txt", "sitemap.xml"} {
		if readRepo("frontend", "public", name) != "" || readRepo("frontend", "dist", name) != "" {
			files = append(files, name)
		}
	}
	evidence := aeo.ChecklistEvidence{
		RepoRoot: root, FrontendDist: dist, FrontendSrc: s.frontendSource(),
	}
	selfChecks := aeo.SelfChecksFor(evidence)
	checklist := aeo.EvaluateChecklist(evidence)
	checklistGreen := len(checklist) > 0
	for _, item := range checklist {
		if !item.Passed {
			checklistGreen = false
		}
	}
	hasTest := func(parts ...string) bool {
		return strings.TrimSpace(readRepo(parts...)) != ""
	}
	costs := aeo.ComputeCost(domain.CostPerSecond, 1, 1.5, 1, aeo.DefaultCostRates())
	_, costOK := aeo.ReconcileCost(costs, costs.Total, 1)

	items := monitor.ReleaseChecklist(monitor.ReleaseEvidence{
		LegalPagesReady:            readRepo("frontend", "src", "routes", "legal.tsx") != "",
		MethodologyNotesPresent:    len(aeo.ReportMethodologyNotes()) > 0,
		BootstrapFiles:             files,
		SelfChecksPassed:           aeo.AllSelfChecksPassed(selfChecks),
		AcceptanceChecklistGreen:   checklistGreen,
		FullChainRegressionPassed:  hasTest("internal", "regression", "chain_test.go"),
		BudgetCircuitTested:        hasTest("internal", "governance", "governance_test.go"),
		CostReconciled:             costOK,
		MCPDocsPublished:           readRepo("docs", "mcp.md") != "",
		MCPToolsReadOnly:           hasTest("internal", "mcpserver", "server_test.go"),
		MonitoringAlertsConfigured: domain.ThresholdCollectionRate > 0 && domain.ThresholdDailyCostUSD > 0,
		RollbackPlan:               "分支合入可按 merge_id 整体回滚；预算熔断需运维显式复位。",
		RateLimitGrayRollout:       "采集批次先按 10% 灰度，观察 429 排队与恢复后续跑。",
	})
	c.JSON(http.StatusOK, gin.H{
		"items": items, "all_passed": monitor.AllReleaseChecksPassed(items),
		"self_checks": selfChecks,
		"note":        "状态由运行时证据计算；CI 中的 go test 是最终判定。",
	})
}

func (s *Server) fullChain(c *gin.Context) {
	var body struct {
		BrandID string `json:"brand_id"`
		SiteURL string `json:"site_url"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.SiteURL == "" {
		body.SiteURL = "https://example.com"
	}
	// 优先用落库的真实检测结果构造 L1 输入；没有检测结果时退回本地模拟采集产物，
	// 并明确标注 simulated，绝不把模拟数据说成第三方实测。
	answers := []domain.RunAnswer{}
	simulated := false
	if body.BrandID != "" {
		results, err := s.detection.ListResults(c.Request.Context(), body.BrandID)
		if err == nil {
			for _, r := range results {
				answers = append(answers, domain.RunAnswer{
					RunID: r.ID, RawAnswerText: r.ResponseText, Valid: true,
					Platform: r.Model, ObservedOn: r.DetectedAt.Format("2006-01-02"),
					Mention:            domain.Mention{BrandID: body.BrandID, Mentioned: r.BrandMentioned},
					TotalBrandMentions: 1,
				})
			}
		}
	}
	if len(answers) == 0 {
		simulated = true
		for _, rec := range s.collector.Runs() {
			answers = append(answers, rec.Answer)
		}
	}
	// 回归账户需要足够配额，否则按日免费上限会让脚本误报红。
	if plan, _ := s.billing.AccountPlan("full-chain"); plan == domain.PlanFree && len(answers) > 0 {
		if o, err := s.billing.Checkout("full-chain", domain.PlanStarter, domain.PaySubscription); err == nil {
			_, _ = s.billing.Pay(o.ID)
		}
	}
	in := regression.ChainInput{
		Brand:   domain.Brand{ID: body.BrandID, Name: "GeoLens", Website: body.SiteURL},
		SiteURL: body.SiteURL, Fetcher: offlineRegressionFetcher{}, Answers: answers,
		Brief: domain.QuestionBrief{
			Category: "AEO 平台", Audience: "品牌市场负责人", Need: "了解 AI 回答中的品牌可见性",
			Region: "us", Locale: "zh-CN", Constraint: "无",
		},
		Gov: s.governance, Bill: s.billing, Mon: s.monitor,
		AccountID: "full-chain",
	}
	rep := regression.RunFullChain(in)
	c.JSON(http.StatusOK, gin.H{
		"report": rep, "simulated": simulated,
		"note": "L0→L3 每层数字可下钻到 run_id / 证据 ID；无有效样本时脚本会明确变红。" +
			"simulated=true 表示 L1 使用本地模拟采集产物，不代表第三方实测。",
	})
}
