package billing

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/geolens/geolens/domain"
)

func billingClock() (*time.Time, func() time.Time) {
	t := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	return &t, func() time.Time { return t }
}

// scriptedGateway 前 N 次拒付，之后成功，用于「失败可重试」与 Webhook 对账。
type scriptedGateway struct {
	failuresLeft int
	calls        int
}

func (g *scriptedGateway) Charge(_ domain.Order) (string, error) {
	g.calls++
	if g.failuresLeft > 0 {
		g.failuresLeft--
		return "", ErrPaymentDeclined
	}
	return "evt_scripted_" + string(rune('a'+g.calls)), nil
}

// WP-7.2：免费层无需信用卡；从定价页到可用 ≤ 3 步。
func TestFreeTierNoCardAndThreeStepCheckout(t *testing.T) {
	_, clock := billingClock()
	e := NewEngine(NewLocalGateway(0), clock)
	if _, err := e.SelectPlan("acct-1", domain.PlanFree); err != nil {
		t.Fatal(err)
	}
	order, err := e.Checkout("acct-1", domain.PlanFree, domain.PayOneTime)
	if err != nil {
		t.Fatal(err)
	}
	if order.AmountUSD != 0 || order.Status != domain.PaySucceeded {
		t.Errorf("免费层应零金额且直接可用: %+v", order)
	}
	if plan, _ := e.AccountPlan("acct-1"); plan != domain.PlanFree {
		t.Errorf("免费层权益未开通: %s", plan)
	}
	// 付费档 3 步：SelectPlan → Checkout → Pay
	if _, err := e.SelectPlan("acct-2", domain.PlanGrowth); err != nil {
		t.Fatal(err)
	}
	o, err := e.Checkout("acct-2", domain.PlanGrowth, domain.PaySubscription)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Pay(o.ID); err != nil {
		t.Fatalf("第 3 步支付失败: %v", err)
	}
	if plan, _ := e.AccountPlan("acct-2"); plan != domain.PlanGrowth {
		t.Errorf("3 步后权益未开通: %s", plan)
	}
}

// WP-7.3：三种支付形态、失败重试可恢复、Webhook 幂等可重放、发票自动开具。
func TestPaymentKindsRetryAndIdempotentWebhook(t *testing.T) {
	_, clock := billingClock()
	gw := &scriptedGateway{failuresLeft: 1}
	e := NewEngine(gw, clock)
	for _, tc := range []struct {
		plan domain.PlanID
		kind domain.PaymentKind
	}{
		{domain.PlanOneTime, domain.PayOneTime},
		{domain.PlanStarter, domain.PaySubscription},
		{domain.PlanRescan, domain.PayAddOn},
	} {
		o, err := e.Checkout("acct", tc.plan, tc.kind)
		if err != nil {
			t.Fatal(err)
		}
		if o.Kind != tc.kind {
			t.Errorf("支付形态错误: %s", o.Kind)
		}
	}
	// 第一笔：首次拒付，权益不应开通；重试后成功。
	o1, err := e.Checkout("retry", domain.PlanScale, domain.PaySubscription)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Pay(o1.ID); !errors.Is(err, ErrPaymentDeclined) {
		t.Fatalf("首次应拒付，实际 %v", err)
	}
	if plan, _ := e.AccountPlan("retry"); plan != domain.PlanFree {
		t.Error("支付失败不应开通权益")
	}
	settled, err := e.Pay(o1.ID)
	if err != nil {
		t.Fatalf("重试支付应成功: %v", err)
	}
	if settled.Status != domain.PaySucceeded {
		t.Errorf("重试后订单状态错误: %s", settled.Status)
	}
	if plan, _ := e.AccountPlan("retry"); plan != domain.PlanScale {
		t.Errorf("重试成功后权益未开通: %s", plan)
	}

	// Webhook 重复投递：发票不重复、权益不重复。
	panelBefore := e.Panel("retry")
	event := WebhookEvent{EventID: "evt_dup", OrderID: o1.ID, Status: domain.PaySucceeded}
	_, _ = e.ApplyWebhook(event)
	_, _ = e.ApplyWebhook(event)
	panelAfter := e.Panel("retry")
	if len(panelAfter.Invoices) != len(panelBefore.Invoices) {
		t.Errorf("Webhook 重放导致发票重复: %d -> %d", len(panelBefore.Invoices), len(panelAfter.Invoices))
	}
}

// WP-7.3：对账脚本可重放，支付状态与权益状态最终一致。
func TestReconcileIsReplayable(t *testing.T) {
	_, clock := billingClock()
	e := NewEngine(NewLocalGateway(0), clock)
	o, _ := e.Checkout("acct", domain.PlanGrowth, domain.PaySubscription)
	ev := WebhookEvent{EventID: "evt_1", OrderID: o.ID, Status: domain.PaySucceeded}
	for i := 0; i < 3; i++ {
		if _, err := e.ApplyWebhook(ev); err != nil {
			t.Fatal(err)
		}
	}
	if plan, _ := e.AccountPlan("acct"); plan != domain.PlanGrowth {
		t.Error("重放后权益状态不一致")
	}
	if got := len(e.Panel("acct").Invoices); got != 1 {
		t.Errorf("重放应只产生 1 张发票，实际 %d", got)
	}
}

// WP-7.3：对账脚本能补齐漏掉的权益开通与发票，且可重复执行。
func TestReconcileRepairsMissingActivation(t *testing.T) {
	_, clock := billingClock()
	e := NewEngine(NewLocalGateway(0), clock)
	o, _ := e.Checkout("acct", domain.PlanGrowth, domain.PaySubscription)
	_, _ = e.Pay(o.ID)
	// 模拟 Webhook 事件丢失导致权益未落地：订单已支付但激活标记与权益被回退。
	delete(e.activated, o.ID)
	e.accounts["acct"].Plan = domain.PlanFree
	e.invoices["acct"] = nil
	if got := e.Reconcile(); got != 1 {
		t.Fatalf("首次对账应补齐 1 单，实际 %d", got)
	}
	if plan, _ := e.AccountPlan("acct"); plan != domain.PlanGrowth {
		t.Errorf("对账后权益未补齐: %s", plan)
	}
	if len(e.Panel("acct").Invoices) != 1 {
		t.Error("对账后发票未补齐")
	}
	if got := e.Reconcile(); got != 0 {
		t.Errorf("重复对账应为空操作，实际修复 %d 单", got)
	}
	if len(e.Panel("acct").Invoices) != 1 {
		t.Error("重复对账产生了重复发票")
	}
}

// WP-7.4：按日/按月硬上限、超限拒绝并给升级引导、绝不静默超采、失败运行不计。
func TestHardQuotaAndFailedRunsNotBilled(t *testing.T) {
	_, clock := billingClock()
	e := NewEngine(NewLocalGateway(0), clock)
	for i := 0; i < 5; i++ {
		if err := e.Consume("free-acct", 1, false); err != nil {
			t.Fatalf("第 %d 次不应被拒: %v", i+1, err)
		}
	}
	if err := e.Consume("free-acct", 1, false); err == nil {
		t.Fatal("第 6 次应被硬上限拒绝")
	} else if !strings.Contains(err.Error(), "不会静默超采") {
		t.Errorf("拒绝时缺少升级引导: %v", err)
	}
	d := e.CheckQuota("free-acct", 1)
	if d.Allowed || d.UpgradeTo == "" || !strings.Contains(d.Message, "不会静默超采") {
		t.Errorf("超限决策缺少升级指引: %+v", d)
	}
	u := e.Usage("free-acct")
	if u.DailyUsed != 5 || u.DailyLimit != 5 {
		t.Errorf("用量未停在硬上限: %+v", u)
	}
	// 失败运行不计配额，但需可解释地记下来
	if err := e.Consume("free-acct", 3, true); err != nil {
		t.Fatal(err)
	}
	u2 := e.Usage("free-acct")
	if u2.DailyUsed != 5 || u2.FailedNotBilled != 3 {
		t.Errorf("失败运行口径错误: %+v", u2)
	}

	// 订阅档按月硬上限
	e2 := NewEngine(NewLocalGateway(0), clock)
	o, _ := e2.Checkout("sub", domain.PlanStarter, domain.PaySubscription)
	_, _ = e2.Pay(o.ID)
	if err := e2.Consume("sub", 1501, false); err == nil {
		t.Error("超过月配额应被拒绝")
	}
	if e2.Usage("sub").MonthlyUsed != 0 {
		t.Error("被拒的运行不应扣减配额")
	}
}

// WP-7.4：升级即时生效、降级下周期生效、配额余量页。
func TestUpgradeImmediateDowngradeNextPeriod(t *testing.T) {
	now, clock := billingClock()
	e := NewEngine(NewLocalGateway(0), clock)
	o1, _ := e.Checkout("acct", domain.PlanScale, domain.PaySubscription)
	_, _ = e.Pay(o1.ID)
	if plan, _ := e.AccountPlan("acct"); plan != domain.PlanScale {
		t.Fatal("升级应即时生效")
	}
	o2, _ := e.Checkout("acct", domain.PlanStarter, domain.PaySubscription)
	_, _ = e.Pay(o2.ID)
	if plan, pending := e.AccountPlan("acct"); plan != domain.PlanScale || pending != domain.PlanStarter {
		t.Errorf("降级应在下周期生效，实际 current=%s pending=%s", plan, pending)
	}
	*now = now.AddDate(0, 1, 0)
	if plan, pending := e.AccountPlan("acct"); plan != domain.PlanStarter || pending != "" {
		t.Errorf("新周期未应用降级: current=%s pending=%s", plan, pending)
	}
	if u := e.Usage("acct"); u.MonthlyUsed != 0 || u.MonthlyLimit != 1500 {
		t.Errorf("新周期配额未重置: %+v", u)
	}
}

// WP-7.5：账单面板含历史发票与用量，口径与五元组一致。
func TestBillingPanelInvoicesAndUsage(t *testing.T) {
	_, clock := billingClock()
	e := NewEngine(NewLocalGateway(0), clock)
	o, _ := e.Checkout("acct", domain.PlanGrowth, domain.PaySubscription)
	_, _ = e.Pay(o.ID)
	_ = e.Consume("acct", 2, false)
	_ = e.Consume("acct", 1, true)
	p := e.Panel("acct")
	if len(p.Invoices) != 1 || p.Invoices[0].AmountUSD != 199 {
		t.Errorf("发票缺失或金额错误: %+v", p.Invoices)
	}
	if p.Usage.MonthlyUsed != 2 || p.Usage.FailedNotBilled != 1 {
		t.Errorf("用量口径与五元组不一致: %+v", p.Usage)
	}
	if p.Provenance.NValid != len(p.Orders) || p.Provenance.SourceType != domain.SourceObserved {
		t.Errorf("面板缺少口径标注: %+v", p.Provenance)
	}
}

// WP-7.6：兑换码生成/兑换/反馈回收，冷启动 ≤10 个且 2 工作日入人工队列。
func TestRedeemCodesColdStartAndFeedback(t *testing.T) {
	_, clock := billingClock()
	e := NewEngine(NewLocalGateway(0), clock)
	codes, err := e.GenerateCodes("COLD", domain.PlanGrowth, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 10 {
		t.Fatalf("冷启动应可生成 10 个码，实际 %d", len(codes))
	}
	if _, err := e.GenerateCodes("COLD", domain.PlanGrowth, 1); !errors.Is(err, ErrCodeQuotaReached) {
		t.Errorf("超过人工复核产能应被限制: %v", err)
	}
	got, fb, err := e.Redeem(codes[0].Code, "team@example.com", "希望看竞品对比")
	if err != nil || !got.Redeemed {
		t.Fatalf("兑换失败: %v", err)
	}
	if !fb.HumanQueue || fb.DueAt.Sub(fb.QueuedAt) != 48*time.Hour {
		t.Errorf("反馈未入 2 工作日人工队列: %+v", fb)
	}
	if _, _, err := e.Redeem(codes[0].Code, "", ""); !errors.Is(err, ErrCodeInvalid) {
		t.Error("兑换码不可重复使用")
	}
	if len(e.FeedbackQueue()) != 1 {
		t.Error("反馈队列未记录")
	}
}
