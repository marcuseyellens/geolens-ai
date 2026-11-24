/**
 * 第六批增强组件（EXD-01/02/03/04/05/09）：挂到第五批已建的 9 条路由上。
 * 不新建顶级路由、不重建 6 条已删页。全部受 flag aeo_seven_products_20261123 控制
 * （由调用方按 flag 决定渲染；flag off 时不挂载 → 回到第五批形态）。
 * 红线：ghost 单列不入 n_valid、三态分列无综合分、来源 answer_ref 必填、份额非分数。
 */
import { Card, Badge, Callout, Table, TraceableFigure } from "../ds";

const METHODOLOGY = "1.3";
const NO_SOURCE = "无原始答案来源，该数字已阻断，不进入报告";

/** EXD-01 引用来源溯源表（NF-39/40）：每条带 answer_ref，ghost 单列。 */
export function CitationSourceTable({ rows }: {
  rows?: Array<{ answerRef: string; source: string; domain: string; kind: "verified" | "ghost" | "unresolved"; excerpt: string }>;
}) {
  const data = rows ?? [];
  const verified = data.filter((r) => r.kind === "verified" && r.answerRef).length;
  const ghost = data.filter((r) => r.kind === "ghost").length;
  return (
    <Card>
      <h3 className="font-semibold text-ink">引用来源溯源表</h3>
      <p className="mt-1 text-xs text-ink-muted">每条来源溯源到原始 AI 答案（answer_ref）；ghost citation 单列，不计入 n_valid。</p>
      <div className="mt-3 grid grid-cols-2 gap-4">
        <div><span className="text-xs text-ink-muted">有效分母 n_valid</span><p className="text-xl font-semibold text-ink">{verified}</p></div>
        <div><span className="text-xs text-ink-muted">ghost citation 率</span><p className="text-xl font-semibold text-ink">{data.length ? Math.round(ghost / data.length * 100) : 0}%</p></div>
      </div>
      <div className="mt-3">
        <Table
          columns={[
            { key: "answerRef", label: "原始答案 answer_ref" },
            { key: "source", label: "来源" },
            { key: "domain", label: "域名" },
            { key: "kind", label: "口径" },
          ]}
          rows={data.map((r) => ({
            answerRef: r.answerRef || <span className="text-warn">{NO_SOURCE}</span>,
            source: r.source, domain: r.domain,
            kind: <Badge tone={r.kind === "verified" ? "positive" : "warn"}>{r.kind}</Badge>,
          }))}
        />
      </div>
    </Card>
  );
}

/** EXD-03 多维对比（NF-42/44）：三态分列，无综合分，份额非分数。 */
export function MultiDimCompare() {
  const empty = { count: 0, nValid: 0, ratio: 0 };
  return (
    <Card>
      <h3 className="font-semibold text-ink">多维可见性对比</h3>
      <p className="mt-1 text-xs text-ink-muted">按模型 / 地区 / 时间切分，三态分列；份额对比不是分数。</p>
      <div className="mt-3 grid grid-cols-1 sm:grid-cols-3 gap-3">
        {(["提及", "推荐", "链接"] as const).map((label) => (
          <div key={label} className="rounded-ds border border-edge p-3">
            <p className="text-xs text-ink-muted">{label}</p>
            <p className="mt-1 text-xl font-semibold text-ink">{Math.round(empty.ratio * 100)}%</p>
            <p className="mt-1 text-xs text-ink-subtle">{empty.count} / 有效 {empty.nValid}</p>
          </div>
        ))}
      </div>
      <Callout tone="info" className="mt-3">两次独立观察，不证明因果关系。</Callout>
    </Card>
  );
}

/** EXD-02 白空格面板（NF-41）：claimant_brands 空 ⇒ 白空格。 */
export function GapPanel() {
  return (
    <Card>
      <h3 className="font-semibold text-ink">白空格 / 内容缺口</h3>
      <p className="mt-1 text-xs text-ink-muted">已认领品牌为空的意图场景 = 白空格，可转内容任务（须人批准）。</p>
      <div className="mt-3"><Table
        columns={[{ key: "intent", label: "意图场景" }, { key: "claimants", label: "已认领品牌" }, { key: "gap", label: "白空格" }]}
        rows={[]}
      /></div>
    </Card>
  );
}

/** EXD-04 AI 广告位看板（NF-43）：每条带 answer_ref。 */
export function AdSpotPanel() {
  return (
    <Card>
      <h3 className="font-semibold text-ink">AI 广告位监测</h3>
      <p className="mt-1 text-xs text-ink-muted">竞品在 AI 答案里的赞助广告，每条溯源到原始答案。</p>
      <div className="mt-3"><Table
        columns={[{ key: "competitor", label: "竞品" }, { key: "model", label: "模型" }, { key: "answerRef", label: "原始答案" }]}
        rows={[]}
      /></div>
    </Card>
  );
}

/** EXD-05 意图问题集分组视图（NF-45）：在 /facts 内嵌，非 /prompts。 */
export function IntentGroupView() {
  return (
    <Card>
      <h3 className="font-semibold text-ink">意图问题集（分组视图）</h3>
      <p className="mt-1 text-xs text-ink-muted">按意图场景分组；本视图内嵌于品牌事实库 /facts，不建独立页。</p>
      <div className="mt-3"><Table
        columns={[{ key: "cluster", label: "意图组" }, { key: "count", label: "问题数" }, { key: "status", label: "状态" }]}
        rows={[]}
      /></div>
    </Card>
  );
}

/** EXD-09 承诺账本（NF-51）：只读，自动勾销，不删除原记录。 */
export function CommitmentLedger() {
  return (
    <Card>
      <h3 className="font-semibold text-ink">承诺账本</h3>
      <p className="mt-1 text-xs text-ink-muted">只读；承诺自动勾销写 settled_at，保留原记录，无编辑 / 发送端点。</p>
      <div className="mt-3"><Table
        columns={[{ key: "text", label: "承诺" }, { key: "owner", label: "负责人" }, { key: "status", label: "状态" }]}
        rows={[]}
      /></div>
    </Card>
  );
}

/** EXD-01 数字阻断演示：无来源数字高亮阻断，不出报告。 */
export function NoSourceBlockDemo() {
  return (
    <TraceableFigure
      label="示例指标" value="—" sourceRef="" figureStatus="unverified" methodologyVersion={METHODOLOGY}
    />
  );
}
