import { T } from "../i18n";
import { useEffect, useRef, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import {
  CreditCard, CheckCircle2, ArrowRight, ArrowLeft, ShieldCheck, LockKeyhole,
  Info, Wallet, Landmark, FlaskConical, Loader2,
} from "lucide-react";
import { Card, Button, Badge, Callout, Spinner, cn } from "../ds";
import { api } from "../api";
import type {
  Entitlement, Order, PlanID, PaymentKind, PaymentProvider,
} from "../types";
import { NEUTRALITY } from "../lib/neutrality";

const PLAN_LABELS: Record<PlanID, string> = {
  free: "免费版", one_time: "单次品牌体检", rescan: "复扫加购",
  starter: "Starter", growth: "Growth", scale: "Scale",
};

const KIND_LABELS: Record<PaymentKind, string> = {
  one_time: "一次性付款", subscription: "按月订阅", add_on: "按月加购",
};

const VALID_PLANS: PlanID[] = ["free", "one_time", "rescan", "starter", "growth", "scale"];

const METHOD_LABELS: Record<string, string> = {
  card: "银行卡", apple_pay: "Apple Pay", google_pay: "Google Pay",
  paypal_wallet: "PayPal 余额", pay_later: "Pay Later", demo: "演示",
};

function parsePlan(raw: string | null): PlanID {
  return raw && (VALID_PLANS as string[]).includes(raw) ? (raw as PlanID) : "starter";
}

function kindFor(plan: PlanID, raw: string | null): PaymentKind {
  if (raw === "one_time" || raw === "subscription" || raw === "add_on") return raw;
  if (plan === "one_time") return "one_time";
  if (plan === "rescan") return "add_on";
  return "subscription";
}

function ProviderIcon({ id }: { id: PaymentProvider["id"] }) {
  if (id === "paypal") return <Wallet className="h-5 w-5" />;
  if (id === "airwallex") return <Landmark className="h-5 w-5" />;
  return <FlaskConical className="h-5 w-5" />;
}

/** 套餐页 CTA 的落地页：选套餐 → 确认内容 → 付款开通，需要登录。 */
export default function SubscribePage() {
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const plan = parsePlan(params.get("plan"));
  const kind = kindFor(plan, params.get("kind"));

  const [plans, setPlans] = useState<Entitlement[]>([]);
  const [providers, setProviders] = useState<PaymentProvider[]>([]);
  const [provider, setProvider] = useState<PaymentProvider["id"]>("sandbox");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [step, setStep] = useState(1);
  const [busy, setBusy] = useState(false);
  const [order, setOrder] = useState<Order | null>(null);
  const [pendingOrderId, setPendingOrderId] = useState<string | null>(null);
  const handledReturn = useRef(false);

  useEffect(() => {
    let cancelled = false;
    Promise.all([api.getPlans(), api.getPaymentProviders()])
      .then(([planRes, providerRes]) => {
        if (cancelled) return;
        setPlans(planRes.plans.filter((p) => p.plan !== "free"));
        setProviders(providerRes.providers);
        const live = providerRes.providers.find((p) => p.live);
        if (live) setProvider(live.id);
      })
      .catch((e) => { if (!cancelled) setError(e instanceof Error ? e.message : "套餐加载失败"); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, []);

  // 供应商托管收银台把买家送回本页：完成扣款并开通权益。
  useEffect(() => {
    const payment = params.get("payment");
    const orderId = params.get("order");
    if (!payment || !orderId || handledReturn.current) return;
    handledReturn.current = true;
    if (payment === "cancel") {
      setError("付款已取消，可以重新选择付款方式。");
      return;
    }
    if (payment === "return") {
      setStep(3);
      setBusy(true);
      api.confirmHostedReturn(orderId)
        .then((res) => { setOrder(res.order); setStep(4); })
        .catch((e) => setError(e instanceof Error ? e.message : "付款确认失败"))
        .finally(() => setBusy(false));
    }
  }, [params]);

  // Webhook 可能比页面回跳稍晚，轮询订单状态兜底。
  useEffect(() => {
    if (!pendingOrderId) return;
    const timer = window.setInterval(async () => {
      try {
        const res = await api.getOrder(pendingOrderId);
        if (res.order.status === "succeeded") {
          setOrder(res.order);
          setStep(4);
          setPendingOrderId(null);
        } else if (res.order.status === "failed") {
          setError("付款未完成，请重试或换一种付款方式。");
          setPendingOrderId(null);
        }
      } catch { /* 网络波动时下一轮继续 */ }
    }, 2500);
    return () => window.clearInterval(timer);
  }, [pendingOrderId]);

  const selected = plans.find((p) => p.plan === plan);
  const chosenProvider = providers.find((p) => p.id === provider);

  function pickPlan(next: PlanID) {
    const nextKind = kindFor(next, null);
    setParams({ plan: next, kind: nextKind });
    setStep(1);
    setError(null);
  }

  async function confirm() {
    setBusy(true);
    setError(null);
    try {
      const checkout = await api.checkout(plan, kind, provider);
      if (checkout.next === "done") {
        setOrder(checkout.order);
        setStep(4);
        return;
      }
      if (checkout.next === "pay") {
        const paid = await api.payOrder(checkout.order.id);
        setOrder(paid.order);
        setStep(4);
        return;
      }
      if (checkout.next === "redirect" && checkout.checkout_url) {
        setOrder(checkout.order);
        setPendingOrderId(checkout.order.id);
        const popup = window.open(checkout.checkout_url, "_blank", "noopener,noreferrer");
        if (!popup) window.location.assign(checkout.checkout_url);
        return;
      }
      setError("付款方式暂时不可用，请换一种方式重试。");
    } catch (e) {
      setError(e instanceof Error ? e.message : "开通失败");
    } finally {
      setBusy(false);
    }
  }

  async function confirmPaidManually() {
    if (!pendingOrderId) return;
    setBusy(true);
    try {
      const res = await api.confirmHostedReturn(pendingOrderId);
      setOrder(res.order);
      setStep(4);
      setPendingOrderId(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "暂时还没收到付款结果，请稍后再试。");
    } finally {
      setBusy(false);
    }
  }

  const steps = ["选套餐", "确认内容", "付款开通"];

  return (
    <T>
    <div className="mx-auto max-w-4xl px-6 py-14">
      <div className="text-center">
        <Badge tone="accent">订阅与开通</Badge>
        <h1 className="mt-4 text-3xl font-bold text-ink tracking-tight">开通套餐，立刻开始检查</h1>
        <p className="mx-auto mt-3 max-w-2xl text-sm text-ink-muted leading-relaxed">
          选好套餐，付款后马上生效。没有结果的运行不收费，也不会占用额度。
        </p>
      </div>

      <div className="mt-9 flex items-center justify-center gap-2">
        {steps.map((label, i) => {
          const n = i + 1;
          const done = step > n;
          const active = step === n;
          return (
            <div key={label} className="flex items-center gap-2">
              <span className={cn(
                "inline-flex h-7 w-7 items-center justify-center rounded-full text-xs font-semibold",
                done ? "bg-positive-subtle text-positive" : active ? "bg-[image:var(--ds-gradient-accent)] text-white" : "bg-canvas text-ink-subtle",
              )}>
                {done ? <CheckCircle2 className="h-4 w-4" /> : n}
              </span>
              <span className={cn("text-xs", active ? "font-semibold text-ink" : "text-ink-muted")}>{label}</span>
              {i < steps.length - 1 && <span className="mx-1 h-px w-6 bg-edge" />}
            </div>
          );
        })}
      </div>

      <div className="mt-8 space-y-5">
        {error && <Callout tone="negative" title="操作未完成">{error}</Callout>}

        {loading ? (
          <div className="flex justify-center py-14"><Spinner /></div>
        ) : step === 1 ? (
          <Card>
            <h2 className="text-base font-semibold text-ink mb-1">第 1 步 · 选套餐</h2>
            <p className="text-xs text-ink-muted mb-5">价格和权益实时来自套餐目录。</p>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              {plans.map((p) => {
                const active = p.plan === plan;
                return (
                  <button
                    type="button" key={p.plan} onClick={() => pickPlan(p.plan)}
                    className={cn(
                      "text-left rounded-ds border p-4 transition-colors cursor-pointer",
                      active ? "border-accent bg-accent-subtle" : "border-edge hover:border-accent/50",
                    )}
                  >
                    <div className="flex items-center justify-between">
                      <span className="text-sm font-semibold text-ink">{PLAN_LABELS[p.plan]}</span>
                      {active && <Badge tone="accent">已选</Badge>}
                    </div>
                    <div className="mt-2 text-2xl font-bold text-ink tabular-nums">${p.price_usd}</div>
                    <div className="mt-2 text-xs text-ink-muted">
                      {p.monthly_runs ? `每月 ${p.monthly_runs} 次检查` :
                        p.daily_queries ? `每天 ${p.daily_queries} 次提问` : `${p.one_time_questions || 1} 项单次权益`}
                      {" · "}{p.projects} 个品牌
                    </div>
                  </button>
                );
              })}
            </div>
            <div className="mt-6 flex justify-end">
              <Button onClick={() => setStep(2)} disabled={!selected}>
                下一步：确认内容 <ArrowRight className="h-4 w-4" />
              </Button>
            </div>
          </Card>
        ) : step === 2 ? (
          <Card>
            <h2 className="text-base font-semibold text-ink mb-1">第 2 步 · 确认内容</h2>
            <p className="text-xs text-ink-muted mb-5">付款完成后，下列权益立即生效。</p>
            {selected && (
              <div className="rounded-ds border border-edge overflow-hidden">
                <div className="flex items-center justify-between p-4 bg-canvas">
                  <div>
                    <div className="text-sm font-semibold text-ink">{PLAN_LABELS[selected.plan]}</div>
                    <div className="text-xs text-ink-muted mt-0.5">{KIND_LABELS[kind]}</div>
                  </div>
                  <div className="text-2xl font-bold text-ink tabular-nums">${selected.price_usd}</div>
                </div>
                <ul className="p-4 space-y-2 text-sm text-ink-muted">
                  <EntitlementRow label="每月检查次数" value={selected.monthly_runs ? `${selected.monthly_runs} 次 / 月` : "按套餐"} />
                  <EntitlementRow label="每天提问次数" value={selected.daily_queries ? `${selected.daily_queries} 次 / 天` : "不限"} />
                  <EntitlementRow label="可监测品牌" value={`${selected.projects} 个品牌`} />
                  <EntitlementRow label="AI 工具接入" value={selected.mcp_enabled ? "包含" : "不含"} />
                  <EntitlementRow label="变化提醒" value={selected.alerts_enabled ? "包含" : "不含"} />
                  <EntitlementRow label="定制导出" value={selected.custom_export ? "包含" : "不含"} />
                </ul>
              </div>
            )}
            <Callout tone="info" title="关于收费">
              失败、超时或没有引用的检查会单独标注，不计费也不占用额度。升级立即生效，降级在下一个账期生效。
            </Callout>
            <div className="mt-6 flex justify-between">
              <Button variant="secondary" onClick={() => setStep(1)}>
                <ArrowLeft className="h-4 w-4" /> 返回选套餐
              </Button>
              <Button onClick={() => setStep(3)}>下一步：付款 <ArrowRight className="h-4 w-4" /></Button>
            </div>
          </Card>
        ) : step === 3 ? (
          <Card>
            <h2 className="text-base font-semibold text-ink mb-1">第 3 步 · 付款</h2>
            <p className="text-xs text-ink-muted mb-5">选择付款方式，完成后权益自动开通。</p>
            <div className="rounded-ds border border-edge p-5 flex items-center justify-between">
              <div className="flex items-center gap-3">
                <span className="flex h-10 w-10 items-center justify-center rounded-ds bg-[image:var(--ds-gradient-accent)] text-white">
                  <CreditCard className="h-5 w-5" />
                </span>
                <div>
                  <div className="text-sm font-semibold text-ink">{PLAN_LABELS[plan]} · {KIND_LABELS[kind]}</div>
                  <div className="text-xs text-ink-muted mt-0.5">应付金额</div>
                </div>
              </div>
              <span className="text-2xl font-bold text-ink tabular-nums">${selected?.price_usd ?? 0}</span>
            </div>

            {pendingOrderId ? (
              <div className="mt-6 rounded-ds border border-accent/40 bg-accent-subtle p-5">
                <div className="flex items-start gap-3">
                  <Loader2 className="h-5 w-5 mt-0.5 shrink-0 text-accent animate-spin" />
                  <div>
                    <h3 className="text-sm font-semibold text-ink">正在等待付款结果</h3>
                    <p className="mt-1 text-xs text-ink-muted leading-relaxed">
                      付款页面已在新窗口打开。完成后本页会自动更新；如果已经付完，也可以点下面的按钮立即确认。
                    </p>
                    <div className="mt-4 flex flex-wrap gap-2">
                      <Button onClick={confirmPaidManually} disabled={busy}>
                        我已完成付款 <ArrowRight className="h-4 w-4" />
                      </Button>
                      <Button variant="secondary" onClick={() => setPendingOrderId(null)}>重新选择付款方式</Button>
                    </div>
                  </div>
                </div>
              </div>
            ) : (
              <>
                <h3 className="mt-6 mb-3 text-sm font-semibold text-ink">选择付款方式</h3>
                <div className="space-y-2">
                  {providers.map((p) => {
                    const active = p.id === provider;
                    return (
                      <button
                        type="button" key={p.id} onClick={() => setProvider(p.id)}
                        className={cn(
                          "w-full flex items-center gap-3 rounded-ds border p-4 text-left transition-colors cursor-pointer",
                          active ? "border-accent bg-accent-subtle" : "border-edge hover:border-accent/50",
                        )}
                      >
                        <span className={cn(
                          "flex h-9 w-9 items-center justify-center rounded-ds shrink-0",
                          active ? "bg-[image:var(--ds-gradient-accent)] text-white" : "bg-canvas text-ink-muted",
                        )}>
                          <ProviderIcon id={p.id} />
                        </span>
                        <span className="min-w-0 flex-1">
                          <span className="flex items-center gap-2 flex-wrap">
                            <span className="text-sm font-semibold text-ink">{p.name}</span>
                            {!p.live && <Badge tone="warn">演示 · 不扣款</Badge>}
                            {p.live && p.sandbox_mode && <Badge tone="warn">测试环境</Badge>}
                          </span>
                          <span className="mt-0.5 block text-xs text-ink-muted">{p.description}</span>
                          <span className="mt-0.5 block text-xs text-ink-subtle">
                            {p.methods.map((m) => METHOD_LABELS[m] ?? m).join(" · ")} · {p.currencies.join(" / ")}
                          </span>
                        </span>
                        {active && <CheckCircle2 className="h-5 w-5 shrink-0 text-accent" />}
                      </button>
                    );
                  })}
                </div>
                <div className="mt-4 flex items-start gap-2">
                  <LockKeyhole className="h-4 w-4 mt-0.5 shrink-0 text-accent" />
                  <p className="text-xs text-ink-muted leading-relaxed">
                    {chosenProvider?.live
                      ? "付款在第三方安全页面完成，GeoLens 不接触您的银行卡信息。付款结果由供应商回调确认。"
                      : "演示收银台会立即开通套餐，不会跳转第三方页面，也不会产生真实扣款。"}
                  </p>
                </div>
                <div className="mt-6 flex justify-between">
                  <Button variant="secondary" onClick={() => setStep(2)}>
                    <ArrowLeft className="h-4 w-4" /> 返回确认
                  </Button>
                  <Button onClick={confirm} disabled={busy || !selected || providers.length === 0}>
                    {busy ? "处理中…" : chosenProvider?.live ? "前往安全付款" : "立即开通"} <ArrowRight className="h-4 w-4" />
                  </Button>
                </div>
              </>
            )}
          </Card>
        ) : (
          <Card>
            <div className="text-center py-6">
              <span className="inline-flex h-14 w-14 items-center justify-center rounded-full bg-positive-subtle text-positive">
                <CheckCircle2 className="h-7 w-7" />
              </span>
              <h2 className="mt-5 text-xl font-bold text-ink">已开通 {PLAN_LABELS[plan]}</h2>
              <p className="mt-2 text-sm text-ink-muted">
                订单 {order?.id.slice(0, 8) ?? "—"}… 已完成，现在就可以开始检查了。
              </p>
              <div className="mt-6 flex items-center justify-center gap-3">
                <Button onClick={() => navigate("/app/billing")}>
                  查看账单 <ArrowRight className="h-4 w-4" />
                </Button>
                <Link to="/app">
                  <Button variant="secondary">返回控制台</Button>
                </Link>
              </div>
            </div>
          </Card>
        )}

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="flex items-start gap-2 text-xs text-ink-muted">
            <ShieldCheck className="h-4 w-4 mt-0.5 shrink-0 text-accent" />
            <span>{NEUTRALITY}</span>
          </div>
          <div className="flex items-start gap-2 text-xs text-ink-muted">
            <Info className="h-4 w-4 mt-0.5 shrink-0 text-accent" />
            <span>账户由登录会话确定，页面和接口都不会接受伪造的账户信息。</span>
          </div>
        </div>
      </div>
    </div>
    </T>
  );
}

function EntitlementRow({ label, value }: { label: string; value: string }) {
  return (
    <T>
    <li className="flex items-center justify-between gap-4">
      <span>{label}</span>
      <span className="font-medium text-ink tabular-nums">{value}</span>
    </li>
    </T>
  );
}
