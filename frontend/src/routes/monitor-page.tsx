import { T } from "../i18n";
import { useEffect, useState } from "react";
import { Activity, RefreshCw, AlertTriangle, ListChecks, Play, CheckCircle2, XCircle } from "lucide-react";
import {
  Page, PageHeader, Card, StatCard, Callout, Badge, Spinner, Button, cn,
} from "../ds";
import { api } from "../api";
import type { MonitorResponse, ReleaseResponse, ChainReport } from "../types";

/** WP-8：五块看板、阈值告警、发布检查清单与 L0→L3 全链路回归。 */
export default function MonitorPage() {
  const [monitor, setMonitor] = useState<MonitorResponse | null>(null);
  const [release, setRelease] = useState<ReleaseResponse | null>(null);
  const [chain, setChain] = useState<ChainReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [running, setRunning] = useState(false);
  const [nonce, setNonce] = useState(0);
  const [brandId, setBrandId] = useState("");

  useEffect(() => {
    api.listBrands().then((list) => setBrandId(list[0]?.id || "")).catch(() => {});
  }, []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    Promise.all([api.getMonitor(), api.getReleaseChecklist()])
      .then(([m, r]) => {
        if (cancelled) return;
        setMonitor(m); setRelease(r); setError(null);
      })
      .catch((e) => { if (!cancelled) setError(e instanceof Error ? e.message : "加载失败"); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [nonce]);

  async function runChain() {
    setRunning(true);
    try {
      const res = await api.runFullChain(brandId);
      setChain(res.report);
    } catch (e) {
      setError(e instanceof Error ? e.message : "回归执行失败");
    } finally {
      setRunning(false);
    }
  }

  const alerts = monitor?.snapshot.alerts || [];
  const passed = release?.items.filter((i) => i.status === "pass").length ?? 0;

  return (
    <T>
    <Page measure="default">
      <PageHeader
        title="监控与发布"
        description="五块看板、阈值告警、上线清单与全链路回归。"
        actions={
          <Button variant="secondary" size="sm" onClick={() => setNonce((n) => n + 1)}>
            <RefreshCw className="h-4 w-4" />刷新
          </Button>
        }
      />

      {loading && !monitor ? (
        <div className="flex justify-center py-12"><Spinner /></div>
      ) : error && !monitor ? (
        <Callout tone="warn" title="无法加载监控数据">{error}</Callout>
      ) : (
        <div className="space-y-6">
          {error && <Callout tone="warn" title="刷新失败，显示上次数据">{error}</Callout>}

          <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
            {(monitor?.boards || []).map((b) => (
              <StatCard key={b.key} label={b.label}
                value={`${b.value.toFixed(b.unit === "%" ? 1 : 2)}${b.unit === "USD" ? " USD" : b.unit === "%" ? "%" : ""}`}
                hint={`n_valid ${b.provenance.n_valid}`} icon={<Activity className="h-5 w-5" />} />
            ))}
          </div>

          {/* Δ-05 / EX-12：健康告警与业务告警分栏，suspect 批次不触发品牌表现变化。 */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <Card>
              <h2 className="text-base font-semibold text-ink mb-3 flex items-center gap-2">
                <AlertTriangle className="h-5 w-5 text-accent" />品牌表现告警
              </h2>
              {alerts.length > 0 ? (
                <div className="space-y-2">
                  {alerts.map((a, i) => (
                    <div key={i} className="rounded-ds border border-warn/40 bg-warn/5 p-3 text-sm">
                      <div className="flex items-center gap-2">
                        <Badge tone="warn">{a.metric}</Badge>
                        <span className="text-ink">{a.message}</span>
                      </div>
                      <p className="mt-1 text-xs text-ink-muted">阈值 {a.threshold} · 观测值 {a.value.toFixed(4)}</p>
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-sm text-ink-muted">当前无越阈值告警。阈值由本平台设定，不代表任何外部平台规则。</p>
              )}
            </Card>
            <Card>
              <h2 className="text-base font-semibold text-ink mb-3 flex items-center gap-2">
                <Activity className="h-5 w-5 text-accent" />采集健康告警
              </h2>
              <p className="text-sm text-ink-muted" data-field="collection_health">
                无可疑批次。suspect 批次只在此栏显示「采集健康」告警，已排除出业务指标分母，不触发品牌表现变化事件。
              </p>
              <p className="mt-2 text-xs text-ink-subtle">仅描述观测数值，不作因果推断；阈值由本平台设定。</p>
            </Card>
          </div>

          <Card>
            <h2 className="text-base font-semibold text-ink mb-3 flex items-center gap-2">
              <Play className="h-5 w-5 text-accent" />L0 → L3 全链路回归
            </h2>
            <Button size="sm" variant="secondary" disabled={running} onClick={runChain}>
              {running ? "执行中…" : "运行脚本化回归"}
            </Button>
            {chain && (
              <div className="mt-4 space-y-2">
                <div className="flex items-center gap-2">
                  <Badge tone={chain.passed ? "positive" : "negative"}>
                    {chain.passed ? "通过" : "未通过"}
                  </Badge>
                  <span className="text-xs text-ink-muted">每层数字可下钻到 run_id / 证据 ID</span>
                </div>
                {chain.layers.map((l) => (
                  <div key={l.layer} className="rounded-ds border border-edge p-3">
                    <div className="flex items-center justify-between">
                      <span className="text-sm font-medium text-ink">{l.layer} · {l.name}</span>
                      <span className="text-xs text-ink-muted tabular-nums">
                        {l.evidence_ids.length} 条证据 · n_valid {l.provenance.n_valid}
                      </span>
                    </div>
                    <div className="mt-1 flex flex-wrap gap-1.5">
                      {Object.entries(l.metrics).slice(0, 6).map(([k, v]) => (
                        <Badge key={k} tone="neutral">{k}: {Number(v).toFixed(k.includes("rate") ? 3 : 0)}</Badge>
                      ))}
                    </div>
                  </div>
                ))}
                {chain.issues.length > 0 && (
                  <Callout tone="warn" title="回归发现的问题">
                    <ul className="list-disc pl-5 space-y-1">{chain.issues.map((x, i) => <li key={i}>{x}</li>)}</ul>
                  </Callout>
                )}
              </div>
            )}
          </Card>

          <Card>
            <h2 className="text-base font-semibold text-ink mb-3 flex items-center gap-2">
              <ListChecks className="h-5 w-5 text-accent" />发布检查清单
              <Badge tone={release?.all_passed ? "positive" : "warn"}>{passed}/{release?.items.length ?? 0}</Badge>
            </h2>
            <div className="space-y-1">
              {(release?.items || []).map((it) => (
                <div key={it.id} className={cn(
                  "flex items-start gap-3 py-2 px-2 rounded-ds border-b border-edge last:border-0",
                  it.status !== "pass" && "bg-warn/5",
                )}>
                  <span className="w-8 shrink-0 text-xs text-ink-subtle">{it.id}</span>
                  {it.status === "pass"
                    ? <CheckCircle2 className="h-4 w-4 text-positive shrink-0" />
                    : <XCircle className="h-4 w-4 text-negative shrink-0" />}
                  <div className="flex-1">
                    <p className="text-sm text-ink">{it.title}</p>
                    {it.evidence.length > 0 && (
                      <p className="text-xs text-ink-muted mt-0.5 truncate">{it.evidence.join(" · ")}</p>
                    )}
                  </div>
                </div>
              ))}
            </div>
            <p className="mt-3 text-xs text-ink-muted">{release?.note}</p>
          </Card>

          <Callout tone="info" title="口径说明">
            <p>{monitor?.note}</p>
            <p className="mt-1">所有含数字的组件都标注来源类型、方法论版本与 n_valid；告警只描述观测数值，不做因果推断。</p>
          </Callout>
        </div>
      )}
    </Page>
    </T>
  );
}
