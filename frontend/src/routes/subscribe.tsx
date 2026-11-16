import { T } from "../i18n";
import { useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import {
  CreditCard, CheckCircle2, ArrowRight, ArrowLeft, ShieldCheck, LockKeyhole, Info,
} from "lucide-react";
import { Card, Button, Badge, Callout, Spinner, cn } from "../ds";
import { api } from "../api";
import type { Entitlement, Order, PlanID, PaymentKind } from "../types";
import { NEUTRALITY } from "../lib/neutrality";

const PLAN_LABELS: Record<PlanID, string> = {
  free: "免费", one_time: "一次性深度审计", rescan: "复扫 add-on",
  starter: "Starter", growth: "Growth", scale: "Scale",
};

const KIND_LABELS: Record<PaymentKind, string> = {
  one_time: "一次性付款", subscription: "按月订阅", add_on: "按月附加项",
};

const VALID_PLANS: PlanID[] = ["free", "one_time", "rescan", "starter", "growth", "scale"];

function parsePlan(raw: string | null): PlanID {
  return raw && (VALID_PLANS as string[]).includes(raw) ? (raw as PlanID) : "starter";
}

function kindFor(plan: PlanID, raw: string | null): PaymentKind {
  if (raw === "one_time" || raw === "subscription" || raw === "add_on") return raw;
  if (plan === "one_time") return "one_time";
  if (plan === "rescan") return "add_on";
  return "subscription";
}

/** 公开套餐页 CTA 的落地页：选档 → 确认权益 → 支付/开通，需要登录。 */
export default function SubscribePage() {
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const plan = parsePlan(params.get("plan"));
  const kind = kindFor(plan, params.get("kind"));

  const [plans, setPlans] = useState<Entitlement[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [step, setStep] = useState(1);
  const [busy, setBusy] = useState(false);
  const [order, setOrder] = useState<Order | null>(null);

  useEffect(() => {
    let cancelled = false;
    api.getPlans()
      .then((res) => { if (!cancelled) setPlans(res.plans.filter((p) => p.plan !== "free")); })
      .catch((e) => { if (!cancelled) setError(e instanceof Error ? e.message : "套餐加载失败"); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, []);

  const selected = plans.find((p) => p.plan === plan);

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
      const checkout = await api.checkout(plan, kind);
      if (checkout.order.status !== "succeeded") {
        const paid = await api.payOrder(checkout.order.id);
        setOrder(paid.order);
      } else {
        setOrder(checkout.order);
      }
      setStep(4);
    } catch (e) {
      setError(e instanceof Error ? e.message : "开通失败");
    } finally {
      setBusy(false);
    }
  }

  const steps = ["选档", "确认权益", "支付 / 开通"];

  return (
    <T>
    <div className="mx-auto max-w-4xl px-6 py-14">
      <div className="text-center">
        <Badge tone="accent">订阅与开通</Badge>
        <h1 className="mt-4 text-3xl font-bold text-ink tracking-tight">选择套餐并完成开通</h1>
        <p className="mx-auto mt-3 max-w-2xl text-sm text-ink-muted leading-relaxed">
          三步完成：选档、确认权益、支付开通。配额按可用运行计，失败运行不计入配额也不会收费。
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
            <h2 className="text-base font-semibold text-ink mb-1">第 1 步 · 选择套餐</h2>
            <p className="text-xs text-ink-muted mb-5">价格与权益来自运行时套餐目录，不是页面硬编码。</p>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              {plans.map((p) => {
                const active = p.plan === plan;
                return (
                  <button
                    type="button" key={p.plan} onClick={() => pickPlan(p.plan)}
                    className={cn(
                      "text-left rounded-ds border p-4 transition-colors",
                      active ? "border-accent bg-accent-subtle" : "border-edge hover:border-accent/50",
                    )}
                  >
                    <div className="flex items-center justify-between">
                      <span className="text-sm font-semibold text-ink">{PLAN_LABELS[p.plan]}</span>
                      {active && <Badge tone="accent">已选</Badge>}
                    </div>
                    <div className="mt-2 text-2xl font-bold text-ink tabular-nums">${p.price_usd}</div>
                    <div className="mt-2 text-xs text-ink-muted">
                      {p.monthly_runs ? `每月 ${p.monthly_runs} 次运行` :
                        p.daily_queries ? `每日 ${p.daily_queries} 次查询` : `${p.one_time_questions || 1} 项单次权益`}
                      {" · "}{p.projects} 个项目
                    </div>
                  </button>
                );
              })}
            </div>
            <div className="mt-6 flex justify-end">
              <Button onClick={() => setStep(2)} disabled={!selected}>
                下一步：确认权益 <ArrowRight className="h-4 w-4" />
              </Button>
            </div>
          </Card>
        ) : step === 2 ? (
          <Card>
            <h2 className="text-base font-semibold text-ink mb-1">第 2 步 · 确认权益</h2>
            <p className="text-xs text-ink-muted mb-5">以下权益由当前套餐决定，开通即时生效。</p>
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
                  <EntitlementRow label="月度运行配额" value={selected.monthly_runs ? `${selected.monthly_runs} 次 / 月` : "按套餐口径"} />
                  <EntitlementRow label="每日查询" value={selected.daily_queries ? `${selected.daily_queries} 次 / 日` : "不限"} />
                  <EntitlementRow label="项目数" value={`${selected.projects} 个`} />
                  <EntitlementRow label="MCP / API 接入" value={selected.mcp_enabled ? "包含" : "不含"} />
                  <EntitlementRow label="告警" value={selected.alerts_enabled ? "包含" : "不含"} />
                  <EntitlementRow label="定制导出" value={selected.custom_export ? "包含" : "不含"} />
                </ul>
              </div>
            )}
            <Callout tone="info" title="计费口径">
              失败、超时、无引用的运行会被单独标注且不计入分母，也不会占用配额；升级即时生效，降级在下一账期生效。
            </Callout>
            <div className="mt-6 flex justify-between">
              <Button variant="secondary" onClick={() => setStep(1)}>
                <ArrowLeft className="h-4 w-4" /> 返回选档
              </Button>
              <Button onClick={() => setStep(3)}>下一步：支付开通 <ArrowRight className="h-4 w-4" /></Button>
            </div>
          </Card>
        ) : step === 3 ? (
          <Card>
            <h2 className="text-base font-semibold text-ink mb-1">第 3 步 · 支付与开通</h2>
            <p className="text-xs text-ink-muted mb-5">确认后创建订单并完成开通。</p>
            <div className="rounded-ds border border-edge p-5 flex items-center justify-between">
              <div className="flex items-center gap-3">
                <span className="flex h-10 w-10 items-center justify-center rounded-ds bg-[image:var(--ds-gradient-accent)] text-white">
                  <CreditCard className="h-5 w-5" />
                </span>
                <div>
                  <div className="text-sm font-semibold text-ink">{PLAN_LABELS[plan]} · {KIND_LABELS[kind]}</div>
                  <div className="text-xs text-ink-muted mt-0.5">订单将在确认后创建</div>
                </div>
              </div>
              <span className="text-2xl font-bold text-ink tabular-nums">${selected?.price_usd ?? 0}</span>
            </div>
            <div className="mt-4 flex items-start gap-2">
              <LockKeyhole className="h-4 w-4 mt-0.5 shrink-0 text-accent" />
              <p className="text-xs text-ink-muted leading-relaxed">
                当前接入的是本地演示网关：不跳转第三方收银台、不收集银行卡信息、不产生真实扣款。
                正式环境应替换为 Stripe Checkout 与 Webhook 对账。
              </p>
            </div>
            <div className="mt-6 flex justify-between">
              <Button variant="secondary" onClick={() => setStep(2)}>
                <ArrowLeft className="h-4 w-4" /> 返回确认
              </Button>
              <Button onClick={confirm} disabled={busy || !selected}>
                {busy ? "开通中…" : "确认开通"} <ArrowRight className="h-4 w-4" />
              </Button>
            </div>
          </Card>
        ) : (
          <Card>
            <div className="text-center py-6">
              <span className="inline-flex h-14 w-14 items-center justify-center rounded-full bg-positive-subtle text-positive">
                <CheckCircle2 className="h-7 w-7" />
              </span>
              <h2 className="mt-5 text-xl font-bold text-ink">已开通 {PLAN_LABELS[plan]}</h2>
              <p className="mt-2 text-sm text-ink-muted">
                订单 {order?.id.slice(0, 8) ?? "—"}… 已支付，权益即时生效。
              </p>
              <div className="mt-6 flex items-center justify-center gap-3">
                <Button onClick={() => navigate("/app/billing")}>
                  前往账单 <ArrowRight className="h-4 w-4" />
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
            <span>账户身份由登录会话确定，页面与接口都不会接受伪造的 account_id。</span>
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
