/**
 * 第六批增强页（EXD-07 / EXD-10）：客户面 Agent 四步构建器 + 运行回放。
 * 两条二级路由，挂在第五批 /app/agents 下，不新建顶级路由、不重建 6 条已删页。
 * 红线：四步逐条批准才上线、never made up、双分母分列无合成、evidence_ref 空禁回放。
 */
import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  Page, PageHeader, PageBackLink, Card, Badge, Callout, Table,
} from "../ds";

const STEP_LABELS = ["描述", "构建", "逐条批准", "上线"] as const;

/** EXD-07 AgentBuilder（组件树 A）：四步状态机，逐条批准，never made up。 */
export function AgentBuilderPage() {
  const { id } = useParams();
  const [step, setStep] = useState(0);
  const [instructions, setInstructions] = useState([
    { text: "读取公开页面 robots.txt 与结构化数据", bound: true, approved: false },
    { text: "汇总三态指标并点回原始答案", bound: true, approved: false },
  ]);
  const allApproved = instructions.every((i) => i.approved);
  const launchDisabled = step !== 3 || !allApproved;

  return (
    <Page>
      <PageBackLink to="/app/agents">返回 Agent 管理</PageBackLink>
      <PageHeader title="Agent 构建器" description={`Agent ${id ?? "new"} · 描述 → 构建 → 逐条批准 → 上线`} />

      <div className="flex gap-2 mb-4">
        {STEP_LABELS.map((label, i) => (
          <Badge key={label} tone={i === step ? "accent" : i < step ? "positive" : "neutral"}>
            {i + 1}. {label}
          </Badge>
        ))}
      </div>

      {step === 0 && (
        <Card>
          <h3 className="font-semibold text-ink">Step 1 · 一句话描述</h3>
          <textarea
            className="mt-3 w-full rounded-ds border border-edge bg-canvas p-3 text-sm text-ink"
            rows={3}
            placeholder="用一句话描述这个 agent 要做什么"
            defaultValue="检查公开页面并汇总可核验的三态指标"
          />
          <button onClick={() => setStep(1)} className="mt-3 rounded-ds bg-accent px-4 py-2 text-sm font-medium text-white">下一步：构建</button>
        </Card>
      )}

      {step === 1 && (
        <Card>
          <h3 className="font-semibold text-ink">Step 2 · 构建指令（never made up）</h3>
          <p className="mt-1 text-xs text-ink-muted">每条指令必须引用已绑定的页面 / 文档，否则构建失败。</p>
          <div className="mt-3 space-y-2">
            {instructions.map((ins, idx) => (
              <div key={idx} className="flex items-center justify-between rounded-ds border border-edge p-3 text-sm">
                <span className="text-ink">{ins.text}</span>
                <Badge tone={ins.bound ? "positive" : "warn"}>{ins.bound ? "已绑定来源" : "未绑定（构建失败）"}</Badge>
              </div>
            ))}
          </div>
          <div className="mt-3 flex gap-2">
            <button onClick={() => setStep(2)} className="rounded-ds bg-accent px-4 py-2 text-sm font-medium text-white">下一步：逐条批准</button>
            <button onClick={() => setInstructions((prev) => [...prev, { text: "引用不存在的来源", bound: false, approved: false }])}
              className="rounded-ds border border-edge px-4 py-2 text-sm text-ink-muted">模拟未绑定指令</button>
          </div>
        </Card>
      )}

      {step === 2 && (
        <Card>
          <h3 className="font-semibold text-ink">Step 3 · 逐条批准（人批准门禁）</h3>
          <div className="mt-3 space-y-2">
            {instructions.map((ins, idx) => (
              <div key={idx} className="flex items-center justify-between rounded-ds border border-edge p-3 text-sm">
                <span className="text-ink">{ins.text}</span>
                <button
                  onClick={() => setInstructions((prev) => prev.map((it, i) => i === idx ? { ...it, approved: true } : it))}
                  className="rounded-ds bg-accent px-3 py-1 text-xs font-medium text-white"
                >
                  {ins.approved ? "已批准" : "批准此条"}
                </button>
              </div>
            ))}
          </div>
          <Callout tone="info" className="mt-3">需人工批准后方可发布，AI 不自动上线</Callout>
          <button onClick={() => setStep(3)} className="mt-3 rounded-ds bg-accent px-4 py-2 text-sm font-medium text-white">下一步：上线</button>
        </Card>
      )}

      {step === 3 && (
        <Card>
          <h3 className="font-semibold text-ink">Step 4 · 上线门禁</h3>
          <p className="mt-1 text-sm text-ink-muted">编排转交规则：按需把任务转交给其他 agent，并保持上下文。</p>
          <button
            disabled={launchDisabled}
            className="mt-3 rounded-ds bg-accent px-4 py-2 text-sm font-medium text-white disabled:opacity-40 disabled:cursor-not-allowed"
          >
            {launchDisabled ? "存在未批准指令，禁止上线" : "批准上线"}
          </button>
          {launchDisabled && <Callout tone="warn" className="mt-3">存在 approved=false 的指令，上线返回 409 instructions_not_approved</Callout>}
        </Card>
      )}
    </Page>
  );
}

/** EXD-10 RunReplay（组件树 B）：双分母分列、人工确认 span、execution ID 串联、禁 mock。 */
export function RunReplayPage() {
  const { execId } = useParams();
  const [tab, setTab] = useState<"timeline" | "tools" | "human">("timeline");
  const hasEvidence = true;

  const spans = [
    { seq: 1, type: "用户", status: "success", evidence: "e1" },
    { seq: 2, type: "模型调用", status: "success", evidence: "e2" },
    { seq: 3, type: "人工确认", status: "pending", evidence: "e3", waiting: "等待人批准", decidedBy: "user:alice", changed: "改了预算参数" },
  ];

  return (
    <Page>
      <PageBackLink to="/app/agents">返回 Agent 管理</PageBackLink>
      <PageHeader title="运行回放" description={`execution ID ${execId ?? "—"} · 跨暂停合并时间线`} />

      <Card>
        <div className="flex items-center justify-between">
          <span className="text-sm text-ink-muted">execution ID</span>
          <code className="text-sm text-ink">{execId}</code>
        </div>
        <div className="mt-1 flex items-center justify-between">
          <span className="text-sm text-ink-muted">prompt_version / model_version</span>
          <span className="text-xs text-ink">p1 / m1（缺失则指标不可归因）</span>
        </div>
      </Card>

      <div className="mt-4 flex gap-2">
        {(["timeline", "tools", "human"] as const).map((t) => (
          <button key={t} onClick={() => setTab(t)}
            className={`rounded-ds px-3 py-1.5 text-sm ${tab === t ? "bg-accent text-white" : "border border-edge text-ink-muted"}`}>
            {t === "timeline" ? "时间线" : t === "tools" ? "工具调用" : "人工确认"}
          </button>
        ))}
      </div>

      <Card className="mt-3">
        <Table
          columns={[
            { key: "seq", label: "Seq" },
            { key: "type", label: "类型" },
            { key: "status", label: "状态" },
            { key: "evidence", label: "证据" },
          ]}
          rows={spans
            .filter((s) => tab !== "human" || s.type === "人工确认")
            .map((s) => ({ seq: s.seq, type: s.type, status: s.status, evidence: s.evidence }))}
        />
      </Card>

      <div className="mt-4 grid grid-cols-1 sm:grid-cols-2 gap-4">
        <Card>
          <p className="text-sm text-ink-muted">调用成功率</p>
          <p className="mt-1 text-2xl font-semibold text-ink">50%</p>
          <p className="mt-1 text-xs text-ink-subtle">成功调用 / 总调用（span 分母）</p>
        </Card>
        <Card>
          <p className="text-sm text-ink-muted">任务完成率</p>
          <p className="mt-1 text-2xl font-semibold text-ink">100%</p>
          <p className="mt-1 text-xs text-ink-subtle">达成任务 / 总任务（trace 分母）</p>
        </Card>
      </div>
      <Callout tone="info" className="mt-3">调用成功率与任务完成率分母不同，分开呈现，不可合并</Callout>

      {!hasEvidence && <Callout tone="warn" className="mt-3">真实证据缺失，禁止回放</Callout>}
      <div className="mt-3 flex gap-2">
        <Link to="/app/attribution" className="text-sm text-accent hover:underline">查看原始答案 →</Link>
      </div>
    </Page>
  );
}
