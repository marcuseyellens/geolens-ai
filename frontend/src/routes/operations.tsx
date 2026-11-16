import { T } from "../i18n";
import { useEffect, useState } from "react";
import { RefreshCw, ShieldAlert, Network, ScanLine, Play, Zap } from "lucide-react";
import {
  Page, PageHeader, Card, StatCard, Callout, Badge, Spinner, Button, cn,
} from "../ds";
import { api } from "../api";
import type { GovernanceOverview, GraphOverview, CollectorOverview, BatchReport, MarginRow, HybridHit } from "../types";

/** WP-1/2/3：运行治理、市场图谱与采集框架的运行时控制台。 */
export default function OperationsPage() {
  const [gov, setGov] = useState<GovernanceOverview | null>(null);
  const [graph, setGraph] = useState<GraphOverview | null>(null);
  const [collector, setCollector] = useState<CollectorOverview | null>(null);
  const [batch, setBatch] = useState<BatchReport | null>(null);
  const [margins, setMargins] = useState<MarginRow[]>([]);
  const [hits, setHits] = useState<HybridHit[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);

  async function refresh() {
    setLoading(true);
    try {
      const [g, gr, c, m] = await Promise.all([
        api.getGovernance(), api.getGraph(), api.getCollector(), api.getCollectorMargins(),
      ]);
      setGov(g); setGraph(gr); setCollector(c); setMargins(m.margins); setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { refresh(); }, [nonce]);

  async function act(fn: () => Promise<unknown>, ok: string) {
    try {
      await fn();
      setNotice(ok);
      setNonce((n) => n + 1);
    } catch (e) {
      setNotice(e instanceof Error ? e.message : "操作失败");
    }
  }

  async function demoGraphBranch() {
    const id = `batch/${Date.now()}`;
    await api.graphCreateBranch(id, "collector");
    await api.graphWriteNode(id, "collector", {
      id: `brand:${Date.now()}`, kind: "Brand", label: "GeoLens AEO 平台",
      props: { category: "AEO 平台" },
    });
    await api.graphMerge(id, "reviewer");
  }

  return (
    <T>
    <Page measure="default">
      <PageHeader
        title="运行治理"
        description="成本分级路由、预算熔断、市场图谱合入与采集批次。"
        actions={
          <Button variant="secondary" size="sm" onClick={() => setNonce((n) => n + 1)}>
            <RefreshCw className="h-4 w-4" />刷新
          </Button>
        }
      />

      {loading && !gov ? (
        <div className="flex justify-center py-12"><Spinner /></div>
      ) : error && !gov ? (
        <Callout tone="warn" title="无法加载运行时数据">{error}</Callout>
      ) : (
        <div className="space-y-6">
          {error && <Callout tone="warn" title="刷新失败，显示上次数据">{error}</Callout>}
          {notice && <Callout tone="info" title="操作结果" >{notice}</Callout>}

          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <StatCard label="已用预算" value={`$${(gov?.budget.spent_usd ?? 0).toFixed(3)}`}
              hint={`上限 $${(gov?.budget.limit_usd ?? 0).toFixed(0)}`} icon={<ShieldAlert className="h-5 w-5" />} />
            <StatCard label="熔断状态" value={gov?.budget.tripped ? "已熔断" : "正常"}
              hint={`拒绝 ${gov?.budget.rejected_runs ?? 0} 次`}
              icon={<ShieldAlert className="h-5 w-5" />} />
            <StatCard label="图谱节点" value={graph?.nodes ?? 0}
              hint={`${graph?.edges ?? 0} 条边`} icon={<Network className="h-5 w-5" />} />
            <StatCard label="采集运行" value={collector?.runs ?? 0}
              hint="本地模拟产物" icon={<ScanLine className="h-5 w-5" />} />
          </div>

          <Card>
            <h2 className="text-base font-semibold text-ink mb-3 flex items-center gap-2">
              <ShieldAlert className="h-5 w-5 text-accent" />成本分级路由与预算熔断
            </h2>
            <div className="space-y-1 mb-4">
              {(gov?.routes || []).map((r) => (
                <div key={r.task} className="flex items-center gap-3 py-1.5 border-b border-edge last:border-0 text-sm">
                  <span className="flex-1 text-ink font-mono text-xs">{r.task}</span>
                  <Badge tone={r.channel === "strong_model" ? "positive" : "neutral"}>{r.channel}</Badge>
                  <span className="text-xs text-ink-muted tabular-nums w-24 text-right">${r.estimated_cost.toFixed(3)}/次</span>
                </div>
              ))}
            </div>
            <div className="flex flex-wrap gap-2">
              <Button size="sm" variant="secondary" onClick={() => act(
                () => api.dispatchRun({ operator: "console", task: "collect", change: "manual" }),
                "运行已通过路由层派发；重试同样经过熔断检查。",
              )}><Play className="h-4 w-4" />触发采集运行</Button>
              <Button size="sm" variant="secondary" onClick={() => act(
                () => api.reportRateLimit("chatgpt", 5), "已登记 429 限速窗口，运行排队等待恢复。",
              )}><Zap className="h-4 w-4" />上报 429</Button>
              <Button size="sm" variant="secondary" onClick={() => act(
                () => api.resumeRuns(), "已尝试续跑队列中到达恢复时间的运行。",
              )}>恢复续跑</Button>
            </div>
            <p className="mt-3 text-xs text-ink-muted">
              判定任务强制走强模型通道；预算熔断在路由层强制，重试与恢复都不能绕过。
              待续跑 {gov?.pending.length ?? 0} 条。
            </p>
          </Card>

          <Card>
            <h2 className="text-base font-semibold text-ink mb-3 flex items-center gap-2">
              <ScanLine className="h-5 w-5 text-accent" />采集批次与按秒成本
            </h2>
            <Button size="sm" variant="secondary" onClick={() => act(async () => {
              const res = await api.runCollectorBatch(30);
              setBatch(res.report);
            }, "30 题批次已完成，失败运行已进入枚举，未静默丢弃。")}>
              <Play className="h-4 w-4" />运行 30 题批次
            </Button>
            {batch && (
              <div className="mt-3 grid grid-cols-2 md:grid-cols-4 gap-2">
                <Badge tone="neutral">总数 {batch.total}</Badge>
                <Badge tone="positive">有效 {batch.valid}</Badge>
                <Badge tone="warn">失败 {batch.failed}</Badge>
                <Badge tone="neutral">成功率 {(batch.success_rate * 100).toFixed(1)}%</Badge>
              </div>
            )}
            {margins.length > 0 && (
              <div className="mt-3 space-y-1">
                {margins.map((m) => (
                  <div key={`${m.channel}-${m.platform}`} className="flex items-center gap-2 text-xs text-ink-muted">
                    <span className="flex-1">{m.channel} · {m.platform}</span>
                    <span>{m.valid_runs}/{m.runs} 有效</span>
                    <span className="tabular-nums">${m.cost_usd.toFixed(4)}</span>
                  </div>
                ))}
              </div>
            )}
            <p className="mt-3 text-xs text-ink-muted">{collector?.note}</p>
          </Card>

          <Card>
            <h2 className="text-base font-semibold text-ink mb-3 flex items-center gap-2">
              <Network className="h-5 w-5 text-accent" />市场图谱
            </h2>
            <div className="flex flex-wrap gap-1.5 mb-3">
              {(graph?.ontology || []).map((k) => <Badge key={k} tone="neutral">{k}</Badge>)}
            </div>
            <p className="text-xs text-ink-muted mb-3">{graph?.note}</p>
            <div className="flex flex-wrap gap-2">
              <Button size="sm" variant="secondary" onClick={() => act(
                demoGraphBranch, "采集分支已由 reviewer 合入 main；collector 直写 main 会被拒绝。",
              )}>
                演示分支合入
              </Button>
              <Button size="sm" variant="secondary" onClick={() => act(async () => {
                const res = await api.graphSearch({ text: "AEO", kinds: ["Brand"], limit: 10 });
                setHits(res.hits);
              }, "混合检索已完成（结构 + 全文 + 相似度一次查询）。")}>混合检索</Button>
              <Button size="sm" variant="secondary" onClick={() => act(
                () => api.graphAt(new Date(Date.now() - 30 * 86400000).toISOString()),
                "已按历史时点重算市场基线。",
              )}>时点重算</Button>
            </div>
            {hits.length > 0 && (
              <div className="mt-3 space-y-1">
                {hits.map((h) => (
                  <div key={h.id} className={cn("text-xs flex items-center gap-2 py-1", "text-ink")}>
                    <span className="flex-1">{h.label}</span>
                    <span className="text-ink-muted tabular-nums">{h.score.toFixed(2)}</span>
                  </div>
                ))}
              </div>
            )}
            {(graph?.merges || []).length > 0 && (
              <p className="mt-3 text-xs text-ink-muted">
                合入记录 {graph?.merges.length} 条，可按 merge_id 整体回滚。
              </p>
            )}
          </Card>

          <Card>
            <h2 className="text-base font-semibold text-ink mb-3">运行五元组</h2>
            {(gov?.pentads || []).length > 0 ? (
              <div className="space-y-1 max-h-64 overflow-y-auto">
                {gov?.pentads.slice(0, 20).map((p) => (
                  <div key={p.run_id} className="flex items-center gap-2 py-1.5 border-b border-edge last:border-0 text-xs">
                    <span className="font-mono text-ink-subtle w-20 truncate">{p.run_id.slice(0, 8)}</span>
                    <span className="w-20 truncate text-ink">{p.operator}</span>
                    <span className="flex-1 truncate text-ink-muted">{p.task}</span>
                    <span className="tabular-nums">${p.cost.toFixed(4)}</span>
                    <Badge tone={p.outcome === "ok" ? "positive" : "warn"}>{p.outcome}</Badge>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-ink-muted">暂无运行记录。触发一次采集运行即可看到五元组落库。</p>
            )}
          </Card>

          <Callout tone="info" title="口径说明">
            <p>本地采集使用明确标注的模拟沙箱，不冒充第三方实测数据。阈值由本平台设定，不代表任何外部平台规则。</p>
            <p className="mt-1">方法论版本 {gov?.methodology} · n_valid {gov?.provenance.n_valid}</p>
          </Callout>
        </div>
      )}
    </Page>
    </T>
  );
}
