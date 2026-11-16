import { T } from "../i18n";
import { useCallback, useEffect, useState } from "react";
import {
  Radar, ShieldCheck, Layers3, ScanLine, Target, Download,
  CheckCircle2, CircleAlert, ExternalLink, Globe2, FileText, ListChecks,
} from "lucide-react";
import { Card, Badge, Callout, Spinner, Table, Button, cn } from "../ds";
import { api } from "../api";
import { readToken } from "../auth/session";
import type {
  CompetitorStatus, ReportTemplate, EvidenceRecord, ActionBoardItem,
  CompetitorGap, CrawlerVisit, CrawlPath, PromptTargetingView, CompetitorRunView,
} from "../types";

const DEMO_RUN = "demo_run_1";
const DEMO_PROJECT = "demo_project";
const DEMO_AUDIT = "demo_audit_1";
const DEMO_QS = "demo_qs_1";

type CICheck = { name: string; pass: boolean; detail: string };

interface Bundle {
  status: CompetitorStatus;
  reports: ReportTemplate[];
  board: ActionBoardItem[];
  gaps: CompetitorGap[];
  visits: CrawlerVisit[];
  paths: CrawlPath[];
  targeting: PromptTargetingView | null;
  run: CompetitorRunView | null;
  ci: { checks: CICheck[]; all_pass: boolean };
}

/** 第三批次（CF-01~CF-08）：竞品观察、三口径渲染与只读证据回溯。 */
export default function CompetitorPage() {
  const [data, setData] = useState<Bundle | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [evidence, setEvidence] = useState<EvidenceRecord[] | null>(null);
  const [evidenceError, setEvidenceError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    Promise.all([
      api.getCompetitorStatus(),
      api.getCompetitorReports(),
      api.getActionBoard(DEMO_PROJECT),
      api.getCompetitorGaps(DEMO_PROJECT),
      api.getCrawlerVisits(DEMO_AUDIT),
      api.getCrawlPaths(DEMO_AUDIT),
      api.getTargeting(DEMO_QS),
      api.getCompetitorRun(DEMO_RUN),
      api.getCompetitorCI(),
    ])
      .then(([status, reports, board, gaps, visits, paths, targeting, run, ci]) => {
        if (cancelled) return;
        setData({
          status,
          reports: reports.reports,
          board: board.items,
          gaps: gaps.gaps,
          visits: visits.visits,
          paths: paths.paths,
          targeting,
          run,
          ci,
        });
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : "加载失败");
      })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, []);

  const openEvidence = useCallback(async (runId: string) => {
    setEvidenceError(null);
    try {
      const res = await api.getCompetitorEvidence(runId);
      setEvidence(res.evidence);
    } catch (e) {
      setEvidenceError(e instanceof Error ? e.message : "证据读取失败");
    }
  }, []);

  const download = useCallback(async (format: "csv" | "json") => {
    const token = readToken();
    const res = await fetch(api.exportEvidenceURL(DEMO_RUN, format), {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    });
    if (!res.ok) {
      setEvidenceError("导出失败");
      return;
    }
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `evidence-${DEMO_RUN}.${format}`;
    a.click();
    URL.revokeObjectURL(url);
  }, []);

  return (
    <T>
    <div className="space-y-6">
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div>
          <h1 className="text-2xl font-bold text-ink">竞品观察</h1>
          <p className="mt-1 text-sm text-ink-muted">看清每次回答的证据来源、采集通道与竞品占位。</p>
        </div>
        {data && (
          <Badge tone={data.status.on ? "positive" : "warn"}>
            {data.status.flag}：{data.status.on ? "已启用" : "已关闭"}
          </Badge>
        )}
      </div>

      {loading && <div className="flex justify-center py-12"><Spinner /></div>}
      {error && <Callout tone="negative" title="加载未完成">{error}</Callout>}

      {data && (
        <>
          <Card>
            <h2 className="mb-4 flex items-center gap-2 text-base font-semibold text-ink">
              <Layers3 className="h-5 w-5 text-accent" /> 批次状态
            </h2>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-sm">
              <Fact label="本批次口径" value={data.status.methodology_version} />
              <Fact label="历史口径" value={data.status.legacy_versions.join(" / ")} />
              <Fact label="迁移版本" value={data.status.schema_version} />
              <Fact label="增量批次" value={`${data.status.delta_flag}：${data.status.delta_on ? "已启用" : "已关闭"}`} />
            </div>
            <p className="mt-4 text-xs text-ink-muted">
              两个批次各自独立开关；关闭本批次只回到本批次前的状态，不影响增量批次。
            </p>
          </Card>

          <Callout tone="info" title="观测与行动中立">
            {data.status.neutrality_notice}
          </Callout>

          {!data.status.on && (
            <Callout tone="warn" title="批次已关闭">
              所有竞品批次端点已停用；下方是关闭前保留的说明，不代表当前可用数据。
            </Callout>
          )}

          <Card>
            <h2 className="mb-1 flex items-center gap-2 text-base font-semibold text-ink">
              <ShieldCheck className="h-5 w-5 text-accent" /> 三种口径，分开渲染
            </h2>
            <p className="mb-4 text-xs text-ink-muted">历史报告保留生成时版本，不补默认值、不用新口径重算。</p>
            <div className="grid grid-cols-1 lg:grid-cols-3 gap-3">
              {data.reports.map((rep) => (
                <VersionCard key={rep.report_id} report={rep} />
              ))}
            </div>
          </Card>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <Card>
              <h2 className="mb-4 flex items-center gap-2 text-base font-semibold text-ink">
                <Target className="h-5 w-5 text-accent" /> 今天值得做的 3 件事
              </h2>
              {data.board.length === 0 ? (
                <p className="text-sm text-ink-muted">目前没有待处理的竞品占位信号。</p>
              ) : (
                <ul className="space-y-3">
                  {data.board.map((item, i) => (
                    <li key={item.signal_id} className="rounded-ds border border-edge p-3">
                      <div className="flex items-start justify-between gap-2">
                        <span className="text-sm font-medium text-ink">{i + 1}. {gapLabel(item.gap_type)}</span>
                        <Badge>{item.competitor_brand_id}</Badge>
                      </div>
                      <button
                        type="button"
                        onClick={() => openEvidence(item.run_id)}
                        className="mt-2 inline-flex items-center gap-1 text-xs text-accent hover:underline cursor-pointer"
                      >
                        <ExternalLink className="h-3 w-3" /> 查看原始回答
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </Card>

            <Card>
              <h2 className="mb-1 flex items-center gap-2 text-base font-semibold text-ink">
                <ScanLine className="h-5 w-5 text-accent" /> 原始证据
              </h2>
              <p className="mb-4 text-xs text-ink-muted">只读端点，调用不改变任何历史运行记录。</p>
              {data.run && (
                <div className="mb-3 grid grid-cols-3 gap-2 text-xs">
                  <Fact label="采样轮次" value={String(data.run.sample_rounds)} />
                  <Fact label="采样策略" value={data.run.sample_policy_version} />
                  <Fact label="通道标注" value={data.run.capture_channel_declared ? "已标注" : "未标注"} />
                </div>
              )}
              <div className="flex flex-wrap gap-2">
                <Button variant="secondary" onClick={() => openEvidence(DEMO_RUN)}>
                  <FileText className="h-4 w-4" /> 查看原文
                </Button>
                <Button variant="secondary" onClick={() => download("json")}>
                  <Download className="h-4 w-4" /> 导出 JSON
                </Button>
                <Button variant="secondary" onClick={() => download("csv")}>
                  <Download className="h-4 w-4" /> 导出 CSV
                </Button>
              </div>
              {evidenceError && <p className="mt-3 text-xs text-negative">{evidenceError}</p>}
              {evidence && (
                <div className="mt-4 space-y-3">
                  {evidence.map((e) => (
                    <div key={`${e.run_id}-${e.sample_index}`} className="rounded-ds bg-canvas p-3">
                      <div className="flex items-center justify-between gap-2 text-xs text-ink-muted">
                        <span>采样 {e.sample_index} · {channelLabel(e.capture_channel)}</span>
                        <span>{e.citations.length} 个引用</span>
                      </div>
                      <p className="mt-2 text-sm text-ink">{e.raw_answer_text}</p>
                      {e.citations.map((c, i) => (
                        <a key={i} href={c.url} target="_blank" rel="noreferrer"
                          className="mt-1 block truncate text-xs text-accent hover:underline">{c.url}</a>
                      ))}
                    </div>
                  ))}
                </div>
              )}
            </Card>
          </div>

          <Card>
            <h2 className="mb-1 flex items-center gap-2 text-base font-semibold text-ink">
              <Radar className="h-5 w-5 text-accent" /> 竞品占位信号
            </h2>
            <p className="mb-4 text-xs text-ink-muted">priority_score 只用于本页排序，不进任何指标聚合，也不触发品牌表现告警。</p>
            <Table
              columns={[
                { key: "question", label: "问题" },
                { key: "type", label: "缺口类型" },
                { key: "competitor", label: "竞品" },
                { key: "priority", label: "内部排序" },
                { key: "status", label: "状态" },
              ]}
              rows={data.gaps.map((g) => ({
                question: g.question_id,
                type: gapLabel(g.gap_type),
                competitor: g.competitor_brand_id,
                priority: g.priority_score.toFixed(2),
                status: <Badge tone={g.status === "open" ? "warn" : "positive"}>{g.status}</Badge>,
              }))}
            />
          </Card>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <Card>
              <h2 className="mb-1 flex items-center gap-2 text-base font-semibold text-ink">
                <ShieldCheck className="h-5 w-5 text-accent" /> 爬虫访问证据
              </h2>
              <p className="mb-4 text-xs text-ink-muted">IP 校验后的真实访问记录；UA 单独声明或疑似伪造不作通过依据。</p>
              <Table
                columns={[
                  { key: "page", label: "页面" },
                  { key: "operator", label: "来源" },
                  { key: "verification", label: "核验结果" },
                  { key: "spoof", label: "可判定" },
                ]}
                rows={data.visits.map((v) => ({
                  page: <span className="truncate block max-w-[180px]">{v.page_url}</span>,
                  operator: v.crawler_operator,
                  verification: v.bot_verification,
                  spoof: v.suspected_spoof
                    ? <Badge tone="negative">排除</Badge>
                    : <Badge tone="positive">可用</Badge>,
                }))}
              />
              <h3 className="mt-5 mb-2 text-sm font-semibold text-ink">逐页引用路径</h3>
              <Table
                columns={[
                  { key: "page", label: "页面" },
                  { key: "citation", label: "真实引用 ID" },
                  { key: "run", label: "运行" },
                ]}
                rows={data.paths.map((p) => ({
                  page: <span className="truncate block max-w-[180px]">{p.page_url}</span>,
                  citation: p.citation_id,
                  run: p.run_id,
                }))}
              />
            </Card>

            <Card>
              <h2 className="mb-1 flex items-center gap-2 text-base font-semibold text-ink">
                <Globe2 className="h-5 w-5 text-accent" /> 提问定位配置
              </h2>
              <p className="mb-4 text-xs text-ink-muted">位置与角色只影响提问位置，不改写问题集语言。</p>
              {data.targeting && (
                <div className="grid grid-cols-2 gap-3 text-sm">
                  <Fact label="定位范围" value={data.targeting.location_scope} />
                  <Fact label="位置" value={data.targeting.location_value || "未设定"} />
                  <Fact label="角色" value={data.targeting.persona || "未设定"} />
                  <Fact label="时区" value={data.targeting.timezone || "未设定"} />
                  <Fact label="问题集语言" value={data.targeting.query_locale || "未设定"} />
                  <Fact label="配置版本" value={data.targeting.config_version} />
                </div>
              )}
              <h3 className="mt-5 mb-2 flex items-center gap-2 text-sm font-semibold text-ink">
                <ListChecks className="h-4 w-4 text-accent" /> CI 断言
              </h3>
              <div className="mb-3">
                <Badge tone={data.ci.all_pass ? "positive" : "negative"}>
                  {data.ci.all_pass ? "全部通过" : "存在未通过项"}
                </Badge>
              </div>
              <ul className="max-h-56 space-y-1.5 overflow-auto text-xs">
                {data.ci.checks.map((c) => (
                  <li key={c.name} className="flex items-start gap-2">
                    {c.pass
                      ? <CheckCircle2 className="mt-0.5 h-3.5 w-3.5 shrink-0 text-positive" />
                      : <CircleAlert className="mt-0.5 h-3.5 w-3.5 shrink-0 text-negative" />}
                    <span className={cn(c.pass ? "text-ink-muted" : "text-negative")}>{c.name}</span>
                  </li>
                ))}
              </ul>
            </Card>
          </div>
        </>
      )}
    </div>
    </T>
  );
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <T>
    <div className="rounded-ds bg-canvas p-3">
      <div className="text-xs text-ink-muted">{label}</div>
      <div className="mt-1 font-medium text-ink break-words">{value}</div>
    </div>
    </T>
  );
}

function VersionCard({ report }: { report: ReportTemplate }) {
  const current = report.methodology_version === "1.2";
  return (
    <T>
    <div className={cn("rounded-ds border p-4", current ? "border-accent bg-accent-subtle" : "border-edge")}>
      <div className="flex items-center justify-between">
        <span className="text-sm font-semibold text-ink">口径 {report.methodology_version}</span>
        <Badge tone={current ? "accent" : "warn"}>{current ? "当前" : "旧口径"}</Badge>
      </div>
      <div className="mt-3 grid grid-cols-2 gap-2 text-xs text-ink-muted">
        <span>计划 {report.counts.planned}</span>
        <span>有效 {report.counts.usable}</span>
        <span>失败 {report.counts.failed}</span>
        <span>待复核 {report.counts.suspect}</span>
      </div>
      {report.capture_channel_notice && (
        <p className="mt-3 text-xs text-ink-muted">{report.capture_channel_notice}</p>
      )}
      {report.capture_channel && (
        <p className="mt-1 text-xs text-ink-muted">采集通道：{channelLabel(report.capture_channel)}</p>
      )}
      {report.count_notice && <p className="mt-3 text-xs text-ink-muted">{report.count_notice}</p>}
      {report.neutrality_notice && (
        <p className="mt-3 border-t border-edge pt-3 text-xs leading-relaxed text-ink">
          {report.neutrality_notice}
        </p>
      )}
    </div>
    </T>
  );
}

function channelLabel(c: string): string {
  switch (c) {
    case "real_ui": return "真实界面";
    case "api_sourced": return "API 采集";
    case "hybrid": return "混合采集";
    default: return "采集通道未标注";
  }
}

function gapLabel(g: string): string {
  switch (g) {
    case "competitor_present_brand_absent": return "竞品出现、本品牌缺席";
    case "competitor_cited_brand_not": return "竞品被引用、本品牌未被引用";
    case "competitor_recommended_brand_not": return "竞品被推荐、本品牌未被推荐";
    default: return g;
  }
}
