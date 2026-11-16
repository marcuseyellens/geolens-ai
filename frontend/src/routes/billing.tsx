import { T } from "../i18n";
import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { CreditCard, RefreshCw, Info, CheckCircle2, XCircle, ArrowRight } from "lucide-react";
import {
  Page, PageHeader, Card, StatCard, Callout, Badge, Spinner, Button, cn,
} from "../ds";
import { api } from "../api";
import type { BillingPanel, Entitlement, PlanID, Order } from "../types";
import { NEUTRALITY } from "../lib/neutrality";

const PLAN_LABELS: Record<PlanID, string> = {
  free: "免费", one_time: "单次报告", rescan: "复扫", starter: "Starter", growth: "Growth", scale: "Scale",
};

/** WP-7：选档、权益、配额余量与账单用量。 */
export default function BillingPage() {
  const [plans, setPlans] = useState<Entitlement[]>([]);
  const [panel, setPanel] = useState<BillingPanel | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    Promise.all([api.getPlans(), api.getBillingPanel()])
      .then(([p, b]) => {
        if (cancelled) return;
        setPlans(p.plans); setPanel(b); setError(null);
      })
      .catch((e) => { if (!cancelled) setError(e instanceof Error ? e.message : "加载失败"); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [nonce]);

  // 套餐开通统一走 /subscribe 三步流程，账单页只展示用量与订单。
  const subscribeHref = (plan: PlanID) => {
    const kind = plan === "one_time" ? "one_time" : plan === "rescan" ? "add_on" : "subscription";
    return `/subscribe?plan=${plan}&kind=${kind}`;
  };

  const usage = panel?.usage;
  const monthlyPct = usage && usage.monthly_limit > 0
    ? Math.min(100, (usage.monthly_used / usage.monthly_limit) * 100) : 0;

  return (
    <T>
    <Page measure="default">
      <PageHeader
        title="套餐与账单"
        description="选择套餐、查看配额余量与账单历史。"
        actions={
          <Button variant="secondary" size="sm" onClick={() => setNonce((n) => n + 1)}>
            <RefreshCw className="h-4 w-4" />刷新
          </Button>
        }
      />

      {loading && !panel ? (
        <div className="flex justify-center py-12"><Spinner /></div>
      ) : error && !panel ? (
        <Callout tone="warn" title="无法加载账单数据">{error}</Callout>
      ) : (
        <div className="space-y-6">
          {error && <Callout tone="warn" title="刷新失败，显示上次数据">{error}</Callout>}

          <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
            <StatCard label="当前套餐" value={usage ? PLAN_LABELS[usage.plan] : "—"}
              hint={usage?.period || ""} icon={<CreditCard className="h-5 w-5" />} />
            <StatCard label="本月运行" value={usage ? `${usage.monthly_used} / ${usage.monthly_limit || "不限"}` : "—"}
              hint="成功运行计入配额" icon={<CheckCircle2 className="h-5 w-5" />} />
            <StatCard label="失败未计" value={usage?.failed_not_billed ?? 0}
              hint="失败运行不计配额" icon={<XCircle className="h-5 w-5" />} />
            <StatCard label="发票" value={panel?.invoices.length ?? 0}
              hint="支付成功后自动开具" icon={<Info className="h-5 w-5" />} />
          </div>

          {usage && usage.monthly_limit > 0 && (
            <Card>
              <div className="flex items-center justify-between mb-2">
                <span className="text-sm font-medium text-ink">月度配额余量</span>
                <span className="text-xs text-ink-muted tabular-nums">
                  {usage.monthly_used} / {usage.monthly_limit}
                </span>
              </div>
              <div className="h-2 rounded-full bg-canvas overflow-hidden">
                <div className="h-full bg-[image:var(--ds-gradient-accent)]" style={{ width: `${monthlyPct}%` }} />
              </div>
              <p className="mt-2 text-xs text-ink-muted">{domainQuotaNote(usage.plan)}</p>
            </Card>
          )}

          <Card>
            <h2 className="text-base font-semibold text-ink mb-4">套餐矩阵</h2>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
              {plans.map((p) => {
                const active = usage?.plan === p.plan;
                return (
                  <div key={p.plan} className={cn(
                    "rounded-ds border p-4 flex flex-col gap-2",
                    active ? "border-accent bg-accent-subtle" : "border-edge",
                  )}>
                    <div className="flex items-center justify-between">
                      <span className="text-sm font-semibold text-ink">{PLAN_LABELS[p.plan]}</span>
                      {active && <Badge tone="positive">当前</Badge>}
                    </div>
                    <span className="text-2xl font-bold text-ink tabular-nums">${p.price_usd}</span>
                    <ul className="text-xs text-ink-muted space-y-1 flex-1">
                      <li>{p.monthly_runs ? `每月 ${p.monthly_runs} 次运行` : p.daily_queries ? `每日 ${p.daily_queries} 次查询` : `${p.one_time_questions || 1} 项单次权益`}</li>
                      <li>{p.projects} 个项目 · MCP {p.mcp_enabled ? "可用" : "不含"}</li>
                      <li>告警 {p.alerts_enabled ? "含" : "不含"} · 定制导出 {p.custom_export ? "含" : "不含"}</li>
                    </ul>
                    {active ? (
                      <Button size="sm" variant="secondary" disabled>已生效</Button>
                    ) : (
                      <Link to={subscribeHref(p.plan)}>
                        <Button size="sm" variant="primary" className="w-full">
                          前往订阅 <ArrowRight className="h-3.5 w-3.5" />
                        </Button>
                      </Link>
                    )}
                  </div>
                );
              })}
            </div>
            <p className="mt-3 text-xs text-ink-muted">升级即时生效；降级在下一个账期生效。免费层无需信用卡。</p>
          </Card>

          <Card>
            <h2 className="text-base font-semibold text-ink mb-4">账单与订单</h2>
            {panel && panel.orders.length > 0 ? (
              <div className="space-y-2">
                {panel.orders.map((o: Order) => (
                  <div key={o.id} className="flex items-center gap-3 py-2 border-b border-edge last:border-0 text-sm">
                    <span className="flex-1 text-ink">{PLAN_LABELS[o.plan]} · {o.kind}</span>
                    <span className="text-ink-muted tabular-nums">${o.amount_usd}</span>
                    <Badge tone={o.status === "succeeded" ? "positive" : o.status === "failed" ? "negative" : "neutral"}>
                      {o.status}
                    </Badge>
                  </div>
                ))}
                <p className="text-xs text-ink-muted pt-2">发票 {panel.invoices.length} 张，按账期自动开具。</p>
              </div>
            ) : (
              <p className="text-sm text-ink-muted">暂无订单。选择一个套餐即可完成三步开通。</p>
            )}
          </Card>

          <Callout tone="info" title="口径说明">
            <p>{NEUTRALITY}</p>
            <p className="mt-1">{panel?.note}</p>
          </Callout>
        </div>
      )}
    </Page>
    </T>
  );
}

function domainQuotaNote(plan: PlanID): string {
  if (plan === "free") return "免费层按日限次；达到上限后运行停止并提示升级，不静默超采。";
  return "按月运行配额执行硬上限；失败运行不计配额、不计账单。";
}
