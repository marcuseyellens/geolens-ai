/**
 * ECF-14 共享组件库：客户可见版数字组件。
 * 强制校验：任何客户可见数字必须带 source_ref 与 figure_status/methodology_version（FC-3）；
 * 缺参渲染层拒绝出数，figure_check_flag=true 时高亮阻断。三态分列、不出综合分（FC-2）。
 */
import { Badge, Card, Callout } from "./index";

export interface FigureProps {
  label: string;
  value: number | string;
  sourceRef?: string;
  /** EXD-01：溯源到原始 AI 答案快照 ID（DC-1）。可选，向后兼容第五批调用。 */
  answerRef?: string;
  figureStatus?: string;
  methodologyVersion?: string;
  figureCheckFlag?: boolean;
  suffix?: string;
}

/** FigureStatusBadge：数字状态徽章；无来源数字黄色高亮阻断。 */
export function FigureStatusBadge({ status, verified }: { status?: string; verified?: boolean }) {
  if (!status) return <Badge tone="warn">未标注</Badge>;
  const tone = status === "actual" || status === "measured" ? "positive" : "warn";
  return <Badge tone={tone as "positive" | "warn"}>{verified === false ? "未验证" : status}</Badge>;
}

/** TraceableFigure：带溯源的数字；缺来源时显示占位并阻断，不出报告数字。 */
export function TraceableFigure(props: FigureProps) {
  const complete = Boolean(props.sourceRef && props.figureStatus && props.methodologyVersion);
  if (props.figureCheckFlag || !complete) {
    return (
      <Callout tone="warn">
        {props.label}：该数字缺少来源或口径，已阻断展示（figure check）。
      </Callout>
    );
  }
  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-baseline gap-2">
        <span className="text-2xl font-semibold text-ink">{props.value}</span>
        {props.suffix && <span className="text-sm text-ink-muted">{props.suffix}</span>}
        <FigureStatusBadge status={props.figureStatus} />
      </div>
      <span className="text-xs text-ink-subtle">
        口径 {props.methodologyVersion} · 来源 {props.sourceRef}
      </span>
    </div>
  );
}

export interface TriStateMetric {
  count: number;
  nValid: number;
  ratio: number;
  sourceRef: string;
  figureStatus: string;
  methodologyVersion: string;
  verified?: boolean;
}

/** TriStateCards：三卡分列，绝不合成综合分。点击就地弹 SourcePeek（不跳页）。 */
export function TriStateCards({
  mention, recommendation, link, onPeek,
}: {
  mention: TriStateMetric;
  recommendation: TriStateMetric;
  link: TriStateMetric;
  onPeek: (metric: string, data: TriStateMetric) => void;
}) {
  const cards = [
    { key: "mention", title: "提及", data: mention },
    { key: "recommendation", title: "推荐", data: recommendation },
    { key: "link", title: "链接", data: link },
  ];
  return (
    <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
      {cards.map((c) => (
        <Card key={c.key} onClick={() => onPeek(c.key, c.data)} className="text-left hover:border-accent cursor-pointer">
          <div className="flex items-center justify-between">
            <span className="text-sm text-ink-muted">{c.title}</span>
            <FigureStatusBadge status={c.data.figureStatus} verified={c.key === "recommendation" ? c.data.verified : undefined} />
          </div>
          <div className="mt-2 text-2xl font-semibold text-ink">
            {Math.round(c.data.ratio * 100)}%
          </div>
          <div className="mt-1 text-xs text-ink-subtle">
            {c.data.count} / 有效 {c.data.nValid} · 点击看来源
          </div>
        </Card>
      ))}
    </div>
  );
}

/** SourcePeek：就地来源卡，不跳独立页。原文 + citation + URL + 算法 + 固定 notice。 */
export function SourcePeek({
  verbatim, citation, url, algorithm, methodologyVersion, answerRef, onClose,
}: {
  verbatim: string; citation: string; url: string; algorithm: string;
  methodologyVersion: string; answerRef?: string; onClose: () => void;
}) {
  const blocked = !answerRef;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-ink/40 p-4" onClick={onClose}>
      <Card className="w-full max-w-lg" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-start justify-between gap-3">
          <h3 className="font-semibold text-ink">来源明细</h3>
          <button className="text-sm text-ink-muted hover:text-ink" onClick={onClose}>收起</button>
        </div>
        {blocked ? (
          <Callout tone="warn">无原始答案来源，该数字已阻断，不进入报告</Callout>
        ) : (
          <>
            <blockquote className="mt-3 border-l-2 border-accent pl-3 text-sm text-ink">“{verbatim}”</blockquote>
            <div className="mt-3 space-y-1 text-xs text-ink-muted">
              <div>引用：{citation}</div>
              <div>链接：{url}</div>
              <div>原始答案：{answerRef}</div>
              <div>算法：{algorithm} · 口径 {methodologyVersion}</div>
            </div>
          </>
        )}
        <p className="mt-3 text-xs text-ink-subtle">引用与提名仅为观测，引用不等于答案依据。</p>
      </Card>
    </div>
  );
}

/** EvidenceBreadcrumb：逐层下钻面包屑。 */
export function EvidenceBreadcrumb({ levels }: { levels: string[] }) {
  return (
    <nav className="flex flex-wrap items-center gap-1 text-xs text-ink-muted">
      {levels.map((l, i) => (
        <span key={i}>
          {i > 0 && <span className="mx-1">/</span>}
          <span className={i === levels.length - 1 ? "text-ink" : ""}>{l}</span>
        </span>
      ))}
    </nav>
  );
}

/** CoverageBadge：覆盖度徽章，带来源。 */
export function CoverageBadge({ questions, models, regions }: { questions: number; models: number; regions: number }) {
  return <Badge tone="accent">覆盖 {questions} 题 · {models} 模型 · {regions} 地区</Badge>;
}
