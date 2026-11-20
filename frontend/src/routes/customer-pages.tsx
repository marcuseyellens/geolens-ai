/**
 * 第五批客户可见页面（C-01～C-06 / MOD-01～04）。
 * 10 条路由统一在此实现：市场基线、品牌可见性、竞品、事实库、账户、
 * 财务 AI、付费广告、Agent 管理、溯源数据。
 * 红线：三态分列不出综合分；数字带来源；写操作草案门禁；无因果文案。
 */
import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  Page, PageHeader, Card, Badge, EmptyState, Callout, Spinner, Table,
  TriStateCards, SourcePeek, CoverageBadge, EvidenceBreadcrumb, TraceableFigure,
  type TriStateMetric,
} from "../ds";
import { api } from "../api";
import { useAuth } from "../auth";

type Props = { kind: string };

function useLoad<T>(loader: () => Promise<T>) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    loader().then(setData).catch((e) => setError(e instanceof Error ? e.message : "加载失败"));
  }, []);
  return { data, error };
}

const emptyTri = (): TriStateMetric => ({
  count: 0, nValid: 0, ratio: 0, sourceRef: "", figureStatus: "unverified",
  methodologyVersion: "1.3", verified: false,
});

/** C-02 市场基线看板 */
export function MarketBaselinePage() {
  const { data } = useLoad(() => api.getJson("/market/baseline"));
  const coverage = (data as { coverage?: { questions: number; models: number; regions: number } } | null)?.coverage;
  return (
    <Page>
      <PageHeader title="市场基线" description="分模型、分地区并列观察，不合并为综合分。" />
      <div className="space-y-4">
        <CoverageBadge
          questions={coverage?.questions ?? 0}
          models={coverage?.models ?? 0}
          regions={coverage?.regions ?? 0}
        />
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <Card>
            <h3 className="font-semibold text-ink">Top 品牌榜</h3>
            <EmptyState title="暂无可核验数据" description="新观察写入后显示；不出综合分。" />
          </Card>
          <Card>
            <h3 className="font-semibold text-ink">买家问题分布</h3>
            <EmptyState title="暂无问题分布" description="按问题来源分组，不合并。" />
          </Card>
          <Card>
            <h3 className="font-semibold text-ink">引用来源结构</h3>
            <EmptyState title="暂无引用数据" description="来源结构与 SEO 指标不并排展示。" />
          </Card>
          <Card>
            <h3 className="font-semibold text-ink">动态流</h3>
            <EmptyState title="暂无动态" description="每条动态均带来源标注。" />
          </Card>
        </div>
      </div>
    </Page>
  );
}

/** C-03 品牌可见性详情（核心页） */
export function BrandVisibilityPage() {
  const { brandId } = useParams();
  const [peek, setPeek] = useState<{ key: string; data: TriStateMetric } | null>(null);
  const tri = emptyTri();
  return (
    <Page>
      <PageHeader title="品牌可见性" description="三态分列、数字可溯源；前后对比不证明因果。" />
      <div className="space-y-4">
        <Card>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs text-ink-subtle">品牌 ID</p>
              <p className="font-medium text-ink">{brandId ?? "—"}</p>
            </div>
            <Badge tone="accent">管理层 / 分析 / 管理</Badge>
          </div>
        </Card>
        <h3 className="font-semibold text-ink">三态指标</h3>
        <TriStateCards
          mention={tri} recommendation={tri} link={tri}
          onPeek={(key, data) => setPeek({ key, data })}
        />
        <Callout tone="info">两次独立观察，不证明因果关系。</Callout>
        <Card>
          <h3 className="font-semibold text-ink">对话式分析</h3>
          <p className="mt-1 text-sm text-ink-muted">过程透明：检查上下文 → 调用采集 → 调用解析 → 调用报告；复用同一只读证据端点。</p>
        </Card>
        <div className="flex gap-2">
          <Link to="/competitors" className="text-sm text-accent hover:underline">建竞品监测 →</Link>
          <Link to="/facts" className="text-sm text-accent hover:underline">加入事实库 →</Link>
        </div>
      </div>
      {peek && (
        <SourcePeek
          verbatim="暂无原文证据（新表空启动）"
          citation="—" url="—" algorithm="三态判定" methodologyVersion="1.3"
          onClose={() => setPeek(null)}
        />
      )}
    </Page>
  );
}

/** MOD-04 溯源数据层 */
export function AttributionDeskPage() {
  const { data } = useLoad(() => api.getJson("/attribution/attribution-methods"));
  const methods = (data as { methods?: string[] } | null)?.methods ?? [];
  return (
    <Page>
      <PageHeader title="溯源数据" description="自然语言问数、只读算式、六口径并列；不提供写查询。" />
      <div className="space-y-4">
        <Card>
          <h3 className="font-semibold text-ink">问数</h3>
          <p className="mt-1 text-sm text-ink-muted">问数生成只读 SQL；写语句被拒绝，SQL 可复制。</p>
        </Card>
        <Card>
          <h3 className="font-semibold text-ink">六口径并列</h3>
          <div className="mt-2 flex flex-wrap gap-2">
            {methods.map((m) => <Badge key={m} tone="accent">{m}</Badge>)}
            {methods.length === 0 && <Spinner />}
          </div>
          <p className="mt-2 text-xs text-ink-subtle">六口径只并列，不合成第七种、不出综合分。</p>
        </Card>
        <Card>
          <h3 className="font-semibold text-ink">集成画廊</h3>
          <p className="mt-1 text-sm text-ink-muted">所有连接 scope 恒 read_only。报告错误进入 open 态。</p>
        </Card>
      </div>
    </Page>
  );
}

/** MOD-03 Agent 管理 */
export function AgentConsolePage() {
  return (
    <Page>
      <PageHeader title="Agent 管理" description="清单、运行回放、失败修复、定时任务；回放只用真实证据。" />
      <Card>
        <EmptyState title="暂无 Agent 运行记录" description="新表空启动；失败运行不进有效分母。" />
      </Card>
      <Callout tone="warn" className="mt-4">本操作生成草案，须经人工批准后生效；广告以暂停态上线，财务不自动过账。</Callout>
    </Page>
  );
}

/** MOD-02 付费广告 */
export function AdsWorkbenchPage() {
  return (
    <Page>
      <PageHeader title="付费广告" description="对话式工作台与三步建议；上线只到暂停态，须人工复开。" />
      <Card>
        <h3 className="font-semibold text-ink">三步建议</h3>
        <ol className="mt-2 list-decimal list-inside text-sm text-ink-muted space-y-1">
          <li>审查受众</li><li>调整预算</li><li>批准并暂停上线</li>
        </ol>
      </Card>
      <Callout tone="info" className="mt-4">本操作生成草案，须经人工批准后生效；广告以暂停态上线，财务不自动过账。</Callout>
    </Page>
  );
}

/** MOD-01 财务 AI */
export function FinanceCopilotPage() {
  return (
    <Page>
      <PageHeader title="财务 AI" description="财务健康分属财务域，与可见性指标分区展示，不自动过账。" />
      <Card>
        <h3 className="font-semibold text-ink">财务健康分（finance_health_score）</h3>
        <div className="mt-3">
          <TraceableFigure
            label="财务健康分" value="—" suffix="/100"
            sourceRef="" figureStatus="unverified" methodologyVersion="1.3"
          />
        </div>
      </Card>
      <Card className="mt-4">
        <h3 className="font-semibold text-ink">逐层下钻</h3>
        <EvidenceBreadcrumb levels={["数字", "报表", "账户", "流水", "凭证"]} />
      </Card>
      <Callout tone="warn" className="mt-4">无来源数字黄色高亮阻断，不出报告；财务过账须人工批准。</Callout>
    </Page>
  );
}

/** C-04 竞品监测台 */
export function CompetitorMonitorPage() {
  return (
    <Page>
      <PageHeader title="竞品监测" description="新建监测、变更告警；告警只陈述观测，不做因果归因。" />
      <Card><EmptyState title="暂无监测" description="新建后显示变更告警。" /></Card>
    </Page>
  );
}

/** C-05 品牌事实库 */
export function FactLibraryPage() {
  return (
    <Page>
      <PageHeader title="品牌事实库" description="实体 / 别名 / NAP / 分级事实；入库默认草案，须批准才可见。" />
      <Card><EmptyState title="暂无事实" description="每条事实带来源与证据状态徽章。" /></Card>
    </Page>
  );
}

/** C-06 账户 / 套餐 / 配额 */
export function AccountPlansPage() {
  const { user } = useAuth();
  return (
    <Page>
      <PageHeader title="账户 / 套餐 / 配额" description="只做展示，不改套餐结构与价格。" />
      <Card>
        <p className="text-sm text-ink-muted">账户：{user?.email ?? "—"}</p>
        <div className="mt-3 flex flex-wrap gap-2">
          <Badge tone="accent">Starter · 50 prompt / 月</Badge>
          <Badge tone="accent">Growth · 300 prompt / 月</Badge>
          <Badge tone="accent">Scale · 700 prompt / 月</Badge>
        </div>
      </Card>
    </Page>
  );
}

/** 兜底：页面不存在 */
export function CustomerFallback() {
  return <Page><EmptyState title="页面不存在" action={<Link to="/app" className="text-accent hover:underline">返回控制台</Link>} /></Page>;
}
