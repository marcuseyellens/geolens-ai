// Package billing 实现基线 WBS WP-7「套餐选择与计费」：
// 选档流程（7.2）、支付三形态 + Webhook 幂等对账（7.3）、权益与限流引擎（7.4）、
// 账单与用量面板（7.5）、兑换码（7.6）。
//
// 支付网关是端口（PaymentGateway），本地实现为可注入的确定性网关；Stripe 可替换
// 进来而不改变契约。仓库内不含任何真实凭据，也不发起外部支付请求。
package billing

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/geolens/geolens/domain"
	"github.com/google/uuid"
)

var (
	ErrUnknownPlan      = errors.New("unknown plan")
	ErrOrderNotFound    = errors.New("order not found")
	ErrPaymentDeclined  = errors.New("payment declined")
	ErrCodeInvalid      = errors.New("invalid or already redeemed code")
	ErrCodeQuotaReached = errors.New("cold-start redeem code quota reached (max 10)")
)

const MaxColdStartCodes = 10

// PaymentGateway 是可插拔支付端口；Charge 返回供应商事件 ID。
type PaymentGateway interface {
	Charge(order domain.Order) (eventID string, err error)
}

// LocalGateway 是本地确定性网关，failRate 用于失败重试与对账测试。
type LocalGateway struct {
	mu       sync.Mutex
	failRate float64
	calls    int
	events   map[string]domain.Order
}

func NewLocalGateway(failRate float64) *LocalGateway {
	return &LocalGateway{failRate: failRate, events: map[string]domain.Order{}}
}

func (g *LocalGateway) Charge(order domain.Order) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	if g.failRate > 0 {
		every := int(1/g.failRate + 0.5)
		if every < 1 {
			every = 1
		}
		if g.calls%every == 0 {
			return "", ErrPaymentDeclined
		}
	}
	eventID := "evt_" + uuid.NewString()
	g.events[eventID] = order
	return eventID, nil
}

func (g *LocalGateway) Event(eventID string) (domain.Order, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	o, ok := g.events[eventID]
	return o, ok
}

// Account 是账户权益状态。PendingPlan 用于「降级下周期生效」。
type Account struct {
	ID              string        `json:"id"`
	Plan            domain.PlanID `json:"plan"`
	PendingPlan     domain.PlanID `json:"pending_plan,omitempty"`
	Period          string        `json:"period"`
	DailyUsed       int           `json:"daily_used"`
	MonthlyUsed     int           `json:"monthly_used"`
	FailedNotBilled int           `json:"failed_not_billed"`
	Orders          []string      `json:"orders"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// RedeemFeedback 是兑换后的反馈回收条目，2 工作日交付入人工队列。
type RedeemFeedback struct {
	Code       string    `json:"code"`
	Contact    string    `json:"contact,omitempty"`
	Note       string    `json:"note"`
	QueuedAt   time.Time `json:"queued_at"`
	DueAt      time.Time `json:"due_at"`
	HumanQueue bool      `json:"human_queue"`
}

// Engine 是计费与权益引擎。
type Engine struct {
	mu       sync.Mutex
	clock    func() time.Time
	gateway  PaymentGateway
	accounts map[string]*Account
	orders   map[string]*domain.Order
	invoices map[string][]domain.Invoice
	codes    map[string]*domain.RedeemCode
	// processedEvents 保证 Webhook 幂等：同一事件重复投递不改变权益。
	processedEvents map[string]bool
	// activated 记录已完成权益开通的订单，供对账脚本补齐漏单。
	activated map[string]bool
	// sessions 记录宿主收银台会话，订单归属校验与买家回跳都依赖它。
	sessions map[string]*CheckoutSession
	feedback []RedeemFeedback
}

func NewEngine(gateway PaymentGateway, clock func() time.Time) *Engine {
	if clock == nil {
		clock = time.Now
	}
	if gateway == nil {
		gateway = NewLocalGateway(0)
	}
	return &Engine{
		clock: clock, gateway: gateway,
		accounts: map[string]*Account{}, orders: map[string]*domain.Order{},
		invoices: map[string][]domain.Invoice{}, codes: map[string]*domain.RedeemCode{},
		processedEvents: map[string]bool{}, activated: map[string]bool{},
		sessions: map[string]*CheckoutSession{},
	}
}

// SaveSession 保存供应商会话；同一订单重复创建会话会覆盖前一条。
func (e *Engine) SaveSession(s *CheckoutSession) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sessions[s.OrderID] = s
}

// Session / Order 供 handler 校验订单归属与查询支付状态，返回副本避免外部改写。
func (e *Engine) Session(orderID string) (*CheckoutSession, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, ok := e.sessions[orderID]
	if !ok {
		return nil, false
	}
	copy := *s
	return &copy, true
}

func (e *Engine) Order(orderID string) (*domain.Order, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	o, ok := e.orders[orderID]
	if !ok {
		return nil, false
	}
	copy := *o
	return &copy, true
}

func periodOf(t time.Time) string { return t.Format("2006-01") }

func (e *Engine) account(id string) *Account {
	a, ok := e.accounts[id]
	if !ok {
		a = &Account{ID: id, Plan: domain.PlanFree, Period: periodOf(e.clock()), Orders: []string{}}
		e.accounts[id] = a
	}
	// 周期滚动：应用待生效的降级、清零月用量
	if a.Period != periodOf(e.clock()) {
		if a.PendingPlan != "" {
			a.Plan = a.PendingPlan
			a.PendingPlan = ""
		}
		a.Period = periodOf(e.clock())
		a.MonthlyUsed = 0
	}
	return a
}

// Plans 返回套餐矩阵（含脚注口径），供定价页与选档使用。
func (e *Engine) Plans() []domain.Entitlement {
	out := make([]domain.Entitlement, 0, len(domain.DefaultEntitlements))
	for _, v := range domain.DefaultEntitlements {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PriceUSD < out[j].PriceUSD })
	return out
}

// ---- WP-7.2 选档流程：从定价页到可用 ≤ 3 步 ----

// SelectPlan 是第 1 步：选定套餐并确认权益，免费层无需信用卡。
func (e *Engine) SelectPlan(accountID string, plan domain.PlanID) (domain.Entitlement, error) {
	ent, ok := domain.DefaultEntitlements[plan]
	if !ok {
		return domain.Entitlement{}, ErrUnknownPlan
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	a := e.account(accountID)
	if a.Plan != plan && plan == domain.PlanFree {
		a.Plan = domain.PlanFree
		a.UpdatedAt = e.clock()
	}
	return ent, nil
}

// Checkout 是第 2、3 步：创建订单 → 支付 → 权益开通。免费层直接开通无需支付。
func (e *Engine) Checkout(accountID string, plan domain.PlanID, kind domain.PaymentKind) (*domain.Order, error) {
	ent, ok := domain.DefaultEntitlements[plan]
	if !ok {
		return nil, ErrUnknownPlan
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	a := e.account(accountID)
	now := e.clock()
	if plan == domain.PlanFree {
		a.Plan = domain.PlanFree
		a.PendingPlan = ""
		a.UpdatedAt = now
		return &domain.Order{
			ID: uuid.NewString(), AccountID: accountID, Plan: plan, Kind: kind,
			AmountUSD: 0, Status: domain.PaySucceeded, CreatedAt: now, SettledAt: &now,
			IdempotencyKey: "free-" + accountID,
		}, nil
	}
	order := &domain.Order{
		ID: uuid.NewString(), AccountID: accountID, Plan: plan, Kind: kind,
		AmountUSD: ent.PriceUSD, Status: domain.PayPending, CreatedAt: now,
		IdempotencyKey: "order-" + uuid.NewString(),
	}
	e.orders[order.ID] = order
	a.Orders = append(a.Orders, order.ID)
	return order, nil
}

// Pay 执行支付；失败可重试（订单保持 pending，不产生权益）。
func (e *Engine) Pay(orderID string) (*domain.Order, error) {
	e.mu.Lock()
	order, ok := e.orders[orderID]
	e.mu.Unlock()
	if !ok {
		return nil, ErrOrderNotFound
	}
	eventID, err := e.gateway.Charge(*order)
	e.mu.Lock()
	defer e.mu.Unlock()
	if err != nil {
		order.Status = domain.PayFailed
		return order, fmt.Errorf("%w: %v", ErrPaymentDeclined, err)
	}
	return e.applyWebhookLocked(WebhookEvent{
		EventID: eventID, OrderID: orderID, Status: domain.PaySucceeded,
		IdempotencyKey: order.IdempotencyKey,
	})
}

// WebhookEvent 是支付供应商回调事件。
type WebhookEvent struct {
	EventID        string               `json:"event_id"`
	OrderID        string               `json:"order_id"`
	Status         domain.PaymentStatus `json:"status"`
	IdempotencyKey string               `json:"idempotency_key"`
}

// ApplyWebhook 是 Webhook 对账入口：可重放、幂等（同一事件重复投递不改变权益）。
func (e *Engine) ApplyWebhook(ev WebhookEvent) (*domain.Order, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.applyWebhookLocked(ev)
}

func (e *Engine) applyWebhookLocked(ev WebhookEvent) (*domain.Order, error) {
	if ev.EventID != "" && e.processedEvents[ev.EventID] {
		o, ok := e.orders[ev.OrderID]
		if ok {
			return o, nil // 幂等重放：直接返回终态，不再重复开通/开票
		}
	}
	order, ok := e.orders[ev.OrderID]
	if !ok {
		return nil, ErrOrderNotFound
	}
	// 订单已处于成功终态：任何重复投递（含不同 EventID 的重放）都不再开通/开票。
	if order.Status == domain.PaySucceeded && ev.Status == domain.PaySucceeded {
		return order, nil
	}
	if ev.IdempotencyKey != "" && order.Status == domain.PaySucceeded && order.IdempotencyKey == ev.IdempotencyKey {
		return order, nil
	}
	now := e.clock()
	order.Status = ev.Status
	if ev.Status == domain.PaySucceeded {
		order.SettledAt = &now
		a := e.account(order.AccountID)
		prev := a.Plan
		a.Plan = order.Plan
		a.UpdatedAt = now
		// 升级即时生效；降级下周期生效（WP-7.4 验收）
		if domain.DefaultEntitlements[order.Plan].PriceUSD < domain.DefaultEntitlements[prev].PriceUSD {
			a.PendingPlan = order.Plan
			a.Plan = prev
		}
		// 发票自动开具
		e.invoices[order.AccountID] = append(e.invoices[order.AccountID], domain.Invoice{
			ID: "inv_" + order.ID, AccountID: order.AccountID, Period: a.Period,
			AmountUSD: order.AmountUSD, CreatedAt: now,
		})
		e.activated[order.ID] = true
	}
	if ev.EventID != "" {
		e.processedEvents[ev.EventID] = true
	}
	return order, nil
}

// Reconcile 是对账脚本：扫描已支付但未完成权益开通的订单并补齐。
// 可重复执行，第二次返回 0；不产生重复发票或重复权益变更。
func (e *Engine) Reconcile() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	ids := make([]string, 0, len(e.orders))
	for id := range e.orders {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	fixed := 0
	for _, id := range ids {
		o := e.orders[id]
		if o.Status != domain.PaySucceeded || e.activated[id] {
			continue
		}
		now := e.clock()
		a := e.account(o.AccountID)
		prev := a.Plan
		a.Plan = o.Plan
		a.UpdatedAt = now
		if domain.DefaultEntitlements[o.Plan].PriceUSD < domain.DefaultEntitlements[prev].PriceUSD {
			a.PendingPlan = o.Plan
			a.Plan = prev
		}
		invoiceID := "inv_" + id
		found := false
		for _, inv := range e.invoices[o.AccountID] {
			if inv.ID == invoiceID {
				found = true
				break
			}
		}
		if !found {
			e.invoices[o.AccountID] = append(e.invoices[o.AccountID], domain.Invoice{
				ID: invoiceID, AccountID: o.AccountID, Period: a.Period,
				AmountUSD: o.AmountUSD, CreatedAt: now,
			})
		}
		e.activated[id] = true
		fixed++
	}
	return fixed
}

// ---- WP-7.4 权益与限流引擎 ----

// quotaDecisionLocked 是配额判定的无锁内核，调用方必须持有 e.mu。
func quotaDecisionLocked(a *Account, runs int) domain.QuotaDecision {
	ent := domain.DefaultEntitlements[a.Plan]
	if ent.DailyQueries > 0 && a.DailyUsed+runs > ent.DailyQueries {
		return domain.QuotaDecision{
			Allowed: false, Reason: "daily_limit", UpgradeTo: domain.PlanStarter,
			Message: domain.QuotaUpgradeMessage,
		}
	}
	if ent.MonthlyRuns > 0 && a.MonthlyUsed+runs > ent.MonthlyRuns {
		return domain.QuotaDecision{
			Allowed: false, Reason: "monthly_limit", UpgradeTo: domain.PlanScale,
			Message: domain.QuotaUpgradeMessage,
		}
	}
	return domain.QuotaDecision{Allowed: true, Message: "运行允许"}
}

// CheckQuota 判定本次运行是否允许；硬上限拒绝并给出升级引导，绝不静默超采。
func (e *Engine) CheckQuota(accountID string, runs int) domain.QuotaDecision {
	e.mu.Lock()
	defer e.mu.Unlock()
	return quotaDecisionLocked(e.account(accountID), runs)
}

// Consume 扣减配额。失败运行不计（PRD §10 默认不计，口径与五元组一致）。
func (e *Engine) Consume(accountID string, runs int, failed bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	a := e.account(accountID)
	if failed {
		a.FailedNotBilled += runs
		return nil
	}
	d := quotaDecisionLocked(a, runs)
	if !d.Allowed {
		return fmt.Errorf("%s (%s)", d.Message, d.Reason)
	}
	a.DailyUsed += runs
	a.MonthlyUsed += runs
	return nil
}

func usageLocked(a *Account) domain.QuotaUsage {
	ent := domain.DefaultEntitlements[a.Plan]
	return domain.QuotaUsage{
		Plan: a.Plan, DailyUsed: a.DailyUsed, DailyLimit: ent.DailyQueries,
		MonthlyUsed: a.MonthlyUsed, MonthlyLimit: ent.MonthlyRuns,
		FailedNotBilled: a.FailedNotBilled, Period: a.Period,
	}
}

// Usage 返回配额余量页数据。
func (e *Engine) Usage(accountID string) domain.QuotaUsage {
	e.mu.Lock()
	defer e.mu.Unlock()
	return usageLocked(e.account(accountID))
}

// AccountPlan 返回当前生效套餐与待生效降级。
func (e *Engine) AccountPlan(accountID string) (current, pending domain.PlanID) {
	e.mu.Lock()
	defer e.mu.Unlock()
	a := e.account(accountID)
	return a.Plan, a.PendingPlan
}

// ---- WP-7.5 账单与用量面板 ----

// BillingPanel 是面板数据；用量口径与五元组一致，失败运行标记不计。
type BillingPanel struct {
	Usage      domain.QuotaUsage `json:"usage"`
	Invoices   []domain.Invoice  `json:"invoices"`
	Orders     []domain.Order    `json:"orders"`
	Note       string            `json:"note"`
	Provenance domain.Provenance `json:"provenance"`
}

func (e *Engine) Panel(accountID string) BillingPanel {
	e.mu.Lock()
	defer e.mu.Unlock()
	orders := []domain.Order{}
	for _, id := range e.account(accountID).Orders {
		if o, ok := e.orders[id]; ok {
			orders = append(orders, *o)
		}
	}
	sort.Slice(orders, func(i, j int) bool { return orders[i].CreatedAt.After(orders[j].CreatedAt) })
	inv := append([]domain.Invoice{}, e.invoices[accountID]...)
	return BillingPanel{
		Usage: usageLocked(e.account(accountID)), Invoices: inv, Orders: orders,
		Note:       "失败运行不计配额、不计账单（PRD §10 默认口径）；用量与运行五元组一致。",
		Provenance: domain.ObservedProvenance(len(orders)),
	}
}

// ---- WP-7.6 兑换码 ----

// GenerateCodes 生成一次性码；冷启动限 10 个以内对齐人工复核产能。
func (e *Engine) GenerateCodes(batch string, plan domain.PlanID, n int) ([]domain.RedeemCode, error) {
	if _, ok := domain.DefaultEntitlements[plan]; !ok {
		return nil, ErrUnknownPlan
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	existing := 0
	for _, c := range e.codes {
		if c.Batch == batch {
			existing++
		}
	}
	if existing+n > MaxColdStartCodes {
		return nil, ErrCodeQuotaReached
	}
	out := []domain.RedeemCode{}
	for i := 0; i < n; i++ {
		c := domain.RedeemCode{
			Code: fmt.Sprintf("AL-%s-%04d", batch, existing+i+1),
			Plan: plan, Batch: batch, CreatedAt: e.clock(),
		}
		e.codes[c.Code] = &c
		out = append(out, c)
	}
	return out, nil
}

// Redeem 无支付、无账号兑换：只需码与可选联系方式；反馈入 2 工作日人工队列。
func (e *Engine) Redeem(code, contact, note string) (domain.RedeemCode, RedeemFeedback, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	c, ok := e.codes[code]
	if !ok || c.Redeemed {
		return domain.RedeemCode{}, RedeemFeedback{}, ErrCodeInvalid
	}
	now := e.clock()
	cp := *c
	cp.Redeemed = true
	cp.RedeemedBy = contact
	cp.RedeemedAt = &now
	e.codes[code] = &cp
	fb := RedeemFeedback{
		Code: code, Contact: contact, Note: note, QueuedAt: now,
		DueAt: now.Add(48 * time.Hour), HumanQueue: true,
	}
	e.feedback = append(e.feedback, fb)
	return cp, fb, nil
}

func (e *Engine) Codes() []domain.RedeemCode {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]domain.RedeemCode, 0, len(e.codes))
	for _, c := range e.codes {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

func (e *Engine) FeedbackQueue() []RedeemFeedback {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]RedeemFeedback{}, e.feedback...)
}
