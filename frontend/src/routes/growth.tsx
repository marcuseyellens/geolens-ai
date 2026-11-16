import { T } from "../i18n";
import { useEffect, useState } from "react";
import {
  BarChart3, TrendingUp, RefreshCw, Activity, CalendarDays, Share2, ScanLine,
} from "lucide-react";
import {
  Page, PageHeader, Card, StatCard, Callout, Badge, Spinner, Button, cn,
} from "../ds";
import { api } from "../api";
import type { FunnelStats } from "../types";
import { FUNNEL_LABELS } from "../types";
import { FUNNEL_STEPS } from "../lib/tracking";

const CONVERSION_ROWS: { key: string; from: string; to: string; baseline?: number }[] = [
  { key: "visit_to_check", from: "visit", to: "check" },
  { key: "check_to_result", from: "check", to: "result" },
  { key: "result_to_share", from: "result", to: "share" },
  { key: "check_to_checkout_39", from: "check", to: "checkout_39", baseline: 0.08 },
  { key: "checkout_39_to_subscribe", from: "checkout_39", to: "subscribe", baseline: 0.15 },
];

const pct = (v: number) => `${(v * 100).toFixed(1)}%`;

/** WP-6.6：匿名转化漏斗日报看板，含检查→审计 ≥8%、审计→订阅 ≥15% 两条基线。 */
export default function GrowthPage() {
  const [stats, setStats] = useState<FunnelStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    api
      .getFunnel()
      .then((s) => {
        if (cancelled) return;
        setStats(s);
        setError(null);
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : "漏斗数据加载失败");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [nonce]);

  const steps = stats?.steps?.length ? stats.steps : FUNNEL_STEPS;
  const counts = stats?.counts ?? {};
  const conversions = stats?.conversions ?? {};
  const baselines = stats?.baselines ?? {};
  const max = Math.max(1, ...steps.map((s) => counts[s] ?? 0));

  return (
    <T>
    <Page measure="default">
      <PageHeader
        title="增长看板"
        description="匿名漏斗事件与基线对照，用于日报观测。"
        actions={
          <Button variant="secondary" size="sm" onClick={() => setNonce((n) => n + 1)}>
            <RefreshCw className="h-4 w-4" />刷新
          </Button>
        }
      />

      {loading && !stats ? (
        <div className="flex justify-center py-12"><Spinner /></div>
      ) : error && !stats ? (
        <Callout tone="warn" title="无法加载漏斗数据">{error}</Callout>
      ) : (
        <div className="space-y-6">
          {error && <Callout tone="warn" title="刷新失败，显示上次数据">{error}</Callout>}

          <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
            <StatCard
              label="匿名事件总数"
              value={stats?.total_events ?? 0}
              hint={stats?.since ? `自 ${stats.since} 起` : "尚无事件"}
              icon={<Activity className="h-5 w-5" />}
            />
            <StatCard
              label="覆盖天数"
              value={stats?.unique_days ?? 0}
              hint="有事件的天数"
              icon={<CalendarDays className="h-5 w-5" />}
            />
            <StatCard
              label="发起检查"
              value={counts.check ?? 0}
              hint="漏斗第二步"
              icon={<ScanLine className="h-5 w-5" />}
            />
            <StatCard
              label="生成分享"
              value={counts.share ?? 0}
              hint="免登录结果链接"
              icon={<Share2 className="h-5 w-5" />}
            />
          </div>

          <Card>
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-base font-semibold text-ink flex items-center gap-2">
                <BarChart3 className="h-5 w-5 text-accent" />漏斗步骤
              </h2>
              <span className="text-xs text-ink-subtle">仅匿名事件计数</span>
            </div>
            <div className="space-y-3">
              {steps.map((step, i) => {
                const n = counts[step] ?? 0;
                return (
                  <div key={step} className="flex items-center gap-3">
                    <span className="w-24 shrink-0 text-sm text-ink-muted">
                      {FUNNEL_LABELS[step] ?? step}
                    </span>
                    <div className="relative flex-1 h-7 rounded-ds bg-canvas border border-edge overflow-hidden">
                      <div
                        className="absolute inset-y-0 left-0 rounded-ds bg-[image:var(--ds-gradient-accent)] transition-all duration-500"
                        style={{ width: `${(n / max) * 100}%`, opacity: Math.max(0.45, 1 - i * 0.09) }}
                      />
                      <span className="absolute inset-y-0 left-2.5 flex items-center text-xs font-medium text-ink tabular-nums">
                        {n}
                      </span>
                    </div>
                  </div>
                );
              })}
            </div>
          </Card>

          <Card>
            <h2 className="text-base font-semibold text-ink mb-4 flex items-center gap-2">
              <TrendingUp className="h-5 w-5 text-accent" />转化率与基线
            </h2>
            <div className="space-y-1">
              {CONVERSION_ROWS.map((row) => {
                const denom = counts[row.from] ?? 0;
                const value = conversions[row.key] ?? 0;
                const baseline = baselines[row.key] ?? row.baseline;
                const noSample = denom === 0;
                const label = `${FUNNEL_LABELS[row.from] ?? row.from} → ${FUNNEL_LABELS[row.to] ?? row.to}`;
                const status = noSample
                  ? { tone: "neutral" as const, text: "暂无样本" }
                  : baseline === undefined
                    ? { tone: "neutral" as const, text: "观测值" }
                    : value >= baseline
                      ? { tone: "positive" as const, text: "已达基线" }
                      : { tone: "warn" as const, text: "低于基线" };
                return (
                  <div
                    key={row.key}
                    className={cn(
                      "flex items-center gap-3 py-2.5 px-2 rounded-ds border-b border-edge last:border-0",
                      baseline !== undefined && "bg-accent-subtle",
                    )}
                  >
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium text-ink">{label}</p>
                      <p className="mt-0.5 text-xs text-ink-subtle">
                        样本 {denom}{baseline !== undefined && ` · 基线 ${pct(baseline)}`}
                      </p>
                    </div>
                    <span className="text-sm font-semibold text-ink tabular-nums">
                      {noSample ? "—" : pct(value)}
                    </span>
                    <Badge tone={status.tone}>{status.text}</Badge>
                  </div>
                );
              })}
            </div>
          </Card>

          <Callout tone="info" title="口径说明">
            <ul className="space-y-1">
              <li>失败、超时、无引用的运行不计入分母，也不会收费。</li>
              <li>仅记录匿名事件计数，需你同意统计；拒绝时完全不发请求。</li>
              <li>转化率为观测值，只显示变化，不证明任何改动导致了变化。</li>
            </ul>
          </Callout>
        </div>
      )}
    </Page>
    </T>
  );
}
