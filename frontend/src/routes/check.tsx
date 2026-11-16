import { T } from "../i18n";
import { useEffect, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import {
  Globe, ScanLine, ArrowRight, ShieldCheck, Copy, Check, Link2,
  Zap, AlertTriangle, Activity, ChevronRight, Lock, RefreshCw, ExternalLink,
} from "lucide-react";
import { Card, Button, Badge, Callout, Spinner, cn } from "../ds";
import { api } from "../api";
import type { SiteScore } from "../types";
import { track } from "../lib/tracking";

/** WP-6.3：免注册 / 免邮箱 / 免信用卡的免费检查结果页。 */
export default function CheckPage() {
  const [params] = useSearchParams();
  const initial = params.get("url") || "";
  const [url, setUrl] = useState(initial);
  const [score, setScore] = useState<SiteScore | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [share, setShare] = useState<{ token: string; url: string } | null>(null);
  const [sharing, setSharing] = useState(false);
  const [copied, setCopied] = useState(false);

  const run = async (target: string) => {
    if (!target.trim()) return;
    setLoading(true);
    setError(null);
    setScore(null);
    setShare(null);
    track("check", { source: "check" });
    try {
      const s = await api.auditSite(target.trim());
      setScore(s);
      track("result", { score: Math.round(s.score) });
    } catch (e) {
      setError(e instanceof Error ? e.message : "检查失败，请稍后重试");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (initial) void run(initial);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initial]);

  const makeShare = async () => {
    if (!score) return;
    setSharing(true);
    try {
      const s = await api.createShare(score);
      setShare(s);
      track("share", { source: "check" });
    } catch (e) {
      setError(e instanceof Error ? e.message : "生成分享链接失败");
    } finally {
      setSharing(false);
    }
  };

  const shareURL = share ? `${window.location.origin}${share.url}` : "";
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(shareURL);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  };

  return (
    <T>
    <div className="mx-auto max-w-4xl px-6 py-12">
      <div className="text-center">
        <Badge tone="accent">L0 · 免费站点检查</Badge>
        <h1 className="mt-4 text-3xl sm:text-4xl font-bold text-ink tracking-tight">
          你的站点，为 AI 准备好了吗？
        </h1>
        <p className="mt-3 text-sm text-ink-muted">
          得到 /100 分的站点就绪度与 1 条最高优先级修复建议。
          <span className="inline-flex items-center gap-1.5 ml-1">
            <Lock className="h-3.5 w-3.5" />免注册 · 免邮箱 · 免信用卡
          </span>
        </p>
      </div>

      <Card raised className="mt-8">
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void run(url);
          }}
          className="flex flex-col sm:flex-row gap-3"
        >
          <div className="relative flex-1">
            <Globe className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-ink-subtle pointer-events-none" />
            <input
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="例如 example.com"
              aria-label="网址"
              className="w-full h-11 rounded-ds bg-canvas border border-edge pl-10 pr-3 text-sm text-ink placeholder:text-ink-subtle focus:outline-none focus:border-accent"
            />
          </div>
          <Button type="submit" loading={loading} disabled={!url.trim()}>
            <ScanLine className="h-4 w-4" />开始检查
          </Button>
        </form>
        {error && <p className="mt-3 text-sm text-negative">{error}</p>}
      </Card>

      {loading && (
        <Card className="mt-6 py-14">
          <div className="flex flex-col items-center gap-4">
            <div className="relative">
              <Spinner className="h-8 w-8 text-accent" />
              <ScanLine className="absolute inset-0 m-auto h-4 w-4 text-accent animate-pulse" />
            </div>
            <span className="text-sm text-ink-muted">正在抓取公开页面并检查就绪度…</span>
          </div>
        </Card>
      )}

      {!loading && score && (
        <div className="mt-6 space-y-6">
          {score.source === "unreachable" && (
            <Callout tone="warn" title="站点暂时无法访问">
              各项检查已记录为「未通过」，请确认网址可公开访问后再复测。
            </Callout>
          )}
          <ResultView score={score} />
          <CrawlerVisitPanel score={score} />
          <ProfileNotice score={score} />
          <AdmissionPanel score={score} />

          {/* 分享链接 */}
          <Card>
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
              <div>
                <h2 className="text-base font-semibold text-ink flex items-center gap-2">
                  <Link2 className="h-4 w-4 text-accent" />把结果发给同事
                </h2>
                <p className="mt-1 text-xs text-ink-subtle">生成一条免登录即可打开的只读链接。</p>
              </div>
              <Button variant="secondary" onClick={makeShare} loading={sharing} disabled={!!share}>
                {share ? <Check className="h-4 w-4" /> : <Link2 className="h-4 w-4" />}
                {share ? "已生成" : "生成分享链接"}
              </Button>
            </div>
            {share && (
              <div className="mt-4 flex flex-col sm:flex-row gap-2">
                <input
                  readOnly
                  value={shareURL}
                  aria-label="分享链接"
                  className="flex-1 h-10 rounded-ds bg-canvas border border-edge px-3 text-sm text-ink focus:outline-none"
                />
                <Button variant="secondary" onClick={copy}>
                  {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                  {copied ? "已复制" : "复制"}
                </Button>
              </div>
            )}
          </Card>

          {/* 升级 CTA */}
          <Card className="relative overflow-hidden">
            <div className="pointer-events-none absolute -right-16 -top-16 h-48 w-48 rounded-full bg-[image:var(--ds-gradient-tech)] opacity-10 blur-3xl" />
            <div className="relative flex flex-col sm:flex-row sm:items-center justify-between gap-4">
              <div>
                <h2 className="text-base font-semibold text-ink">想知道 AI 现在到底推荐谁？</h2>
                <p className="mt-1 text-sm text-ink-muted">
                  站点就绪度只说明技术条件。答案级的可见性证据，需要一次真实 buyer question 审计。
                </p>
              </div>
              <Link to="/pricing" className="shrink-0">
                <Button>
                  查看 $39 审计 <ArrowRight className="h-4 w-4" />
                </Button>
              </Link>
            </div>
          </Card>
        </div>
      )}
    </div>
    </T>
  );
}

/** WP-6.3 验收：分享链接打开无需登录。 */
export function SharedResultPage() {
  const { token } = useParams<{ token: string }>();
  const [score, setScore] = useState<SiteScore | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!token) return;
    api
      .getShare(token)
      .then(setScore)
      .catch((e) => setError(e instanceof Error ? e.message : "结果不存在或已失效"))
      .finally(() => setLoading(false));
  }, [token]);

  return (
    <T>
    <div className="mx-auto max-w-4xl px-6 py-12">
      <div className="text-center">
        <Badge tone="accent">分享的检查结果</Badge>
        <h1 className="mt-4 text-3xl font-bold text-ink tracking-tight">站点就绪度检查结果</h1>
        <p className="mt-2 text-sm text-ink-muted">这条链接无需登录即可查看，且为只读快照。</p>
      </div>

      {loading && (
        <Card className="mt-8 py-14 flex justify-center"><Spinner className="h-8 w-8 text-accent" /></Card>
      )}
      {!loading && error && (
        <Callout tone="warn" title="无法打开结果" className="mt-8">{error}</Callout>
      )}
      {!loading && score && (
        <div className="mt-8 space-y-6">
          <ResultView score={score} />
          <CrawlerVisitPanel score={score} />
          <ProfileNotice score={score} />
          <AdmissionPanel score={score} />
          <div className="flex flex-col sm:flex-row items-center justify-between gap-3">
            <p className="text-xs text-ink-subtle">结果生成于 {new Date(score.audited_at).toLocaleString()}</p>
            <Link to="/check">
              <Button variant="secondary"><RefreshCw className="h-4 w-4" />检查我自己的站点</Button>
            </Link>
          </div>
        </div>
      )}
    </div>
    </T>
  );
}

/** 结果主体：分数 + 首要修复 + 分组 + 关键未通过项。 */
function ResultView({ score }: { score: SiteScore }) {
  const failed = score.checks.filter((c) => c.status === "fail").slice(0, 3);
  return (
    <T>
    <>
      <Card raised>
        <div className="flex flex-col sm:flex-row items-center gap-6">
          <ScoreRing score={score.score} />
          <div className="flex-1 min-w-0 text-center sm:text-left">
            <p className="text-sm text-ink-muted">站点就绪度</p>
            <p className="mt-1 text-sm font-medium text-ink truncate">{score.url}</p>
            <div className="mt-3 inline-flex items-start gap-2 rounded-ds bg-accent-subtle border border-accent/20 px-3 py-2.5 text-left">
              <Zap className="h-4 w-4 text-accent mt-0.5 shrink-0" />
              <div>
                <span className="text-xs font-semibold text-accent">最高优先级建议</span>
                <p className="mt-0.5 text-sm text-ink">{score.top_fix || "暂无明显阻塞项"}</p>
              </div>
            </div>
          </div>
        </div>
        <p className="mt-5 pt-4 border-t border-edge flex items-start gap-2 text-xs text-ink-subtle">
          <ShieldCheck className="h-3.5 w-3.5 mt-0.5 shrink-0" />
          {score.caveat}
        </p>
      </Card>

      <Card>
        <h2 className="text-base font-semibold text-ink mb-4 flex items-center gap-2">
          <Activity className="h-4 w-4 text-accent" />分组得分
        </h2>
        <div className="space-y-4">
          {score.groups.map((g) => {
            const pct = g.weight > 0 ? (g.earned / g.weight) * 100 : 0;
            return (
              <div key={g.group}>
                <div className="flex justify-between text-sm mb-1.5">
                  <span className="text-ink font-medium">{g.label}</span>
                  <span className="text-ink-muted tabular-nums">{g.earned.toFixed(1)} / {g.weight}</span>
                </div>
                <div className="relative w-full bg-canvas rounded-full h-2 overflow-hidden">
                  <div className="absolute inset-y-0 left-0 rounded-full bg-[image:var(--ds-gradient-accent)] transition-all duration-500" style={{ width: `${pct}%` }} />
                  {pct > 0 && <div className="absolute inset-y-0 left-0 rounded-full opacity-50 blur-sm bg-[image:var(--ds-gradient-accent)] transition-all duration-500" style={{ width: `${pct}%` }} />}
                </div>
              </div>
            );
          })}
        </div>
      </Card>

      {failed.length > 0 && (
        <Card>
          <h2 className="text-base font-semibold text-ink mb-3 flex items-center gap-2">
            <AlertTriangle className="h-4 w-4 text-warn" />最值得先修的 {failed.length} 项
          </h2>
          <div className="space-y-2">
            {failed.map((c) => (
              <div key={c.id} className="rounded-ds border border-edge p-3">
                <div className="flex items-center gap-2">
                  <Badge tone="negative">未通过</Badge>
                  <span className="text-sm font-medium text-ink">{c.label}</span>
                </div>
                <p className="mt-1.5 text-xs text-ink-muted">修复：{c.fix}</p>
              </div>
            ))}
          </div>
          <Link to="/app/audit" className="mt-4 inline-flex items-center gap-1 text-sm text-accent hover:underline">
            在控制台查看全部 {score.checks.length} 项证据 <ChevronRight className="h-3.5 w-3.5" />
          </Link>
        </Card>
      )}

      <p className="flex items-start gap-2 text-xs text-ink-subtle">
        <ShieldCheck className="h-3.5 w-3.5 mt-0.5 shrink-0" />
        技术就绪 ≠ 被推荐。分数仅反映公开页面的技术条件，不预测或承诺任何 AI 模型的推荐结果。
      </p>
    </>
    </T>
  );
}

function ScoreRing({ score }: { score: number }) {
  const r = 34;
  const c = 2 * Math.PI * r;
  const offset = c - (Math.max(0, Math.min(100, score)) / 100) * c;
  const color = score >= 70 ? "var(--ds-positive)" : score >= 40 ? "var(--ds-warn)" : "var(--ds-negative)";
  return (
    <T>
    <div className={cn("relative h-24 w-24 shrink-0")}>
      <svg className="h-24 w-24 -rotate-90" viewBox="0 0 80 80">
        <circle cx="40" cy="40" r={r} fill="none" stroke="var(--ds-edge)" strokeWidth="5" />
        <circle cx="40" cy="40" r={r} fill="none" stroke={color} strokeWidth="5" strokeLinecap="round"
          strokeDasharray={c} strokeDashoffset={offset} className="transition-all duration-500" />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className="text-2xl font-bold text-ink tabular-nums">{score.toFixed(0)}</span>
        <span className="text-[10px] text-ink-subtle">/100</span>
      </div>
    </div>
    </T>
  );
}

/**
 * Δ-03 / EX-08：第 9 项「爬虫访问证据」结果页分行。
 * 「平台提示」(platform_hint) 与「实际观察」(observed_evidence) 是两条独立字段，
 * 互不填充；平台提示只作线索，不作 pass/fail 依据。此 DOM 结构被 §6 CI 快照断言。
 */
function CrawlerVisitPanel({ score }: { score: SiteScore }) {
  const row = score.crawler_visit;
  if (!row) return null;
  const verdictLabel: Record<string, string> = {
    pass: "通过", partial: "部分", fail: "未通过", unknown: "未判定",
  };
  return (
    <T>
    <Card>
      <h2 className="text-base font-semibold text-ink mb-3 flex items-center gap-2">
        <Activity className="h-4 w-4 text-accent" />爬虫访问证据
      </h2>
      <div className="space-y-2">
        <div className="rounded-ds border border-edge p-3">
          <span className="text-xs font-semibold text-ink-muted">平台提示</span>
          <p className="mt-1 text-sm text-ink" data-field="platform_hint">{row.platform_hint || "—"}</p>
        </div>
        <div className="rounded-ds border border-edge p-3">
          <span className="text-xs font-semibold text-ink-muted">实际观察</span>
          <p className="mt-1 text-sm text-ink" data-field="observed_evidence">{row.observed_evidence || "—"}</p>
        </div>
      </div>
      <p className="mt-3 flex items-center gap-2 text-xs text-ink-subtle">
        <Badge tone={row.verdict === "pass" ? "positive" : row.verdict === "unknown" ? "neutral" : "warn"}>
          {verdictLabel[row.verdict] || "未判定"}
        </Badge>
        证据来源 {row.evidence_source}；平台提示仅为线索，未判定项不进分母、不扣分。
      </p>
      {row.fix_advice && <p className="mt-2 text-xs text-ink-muted">修复：{row.fix_advice}</p>}
    </Card>
    </T>
  );
}

/** A-WP-9.2：档案未核对提示。默认 unknown，不扣分，不代认领。 */
function ProfileNotice({ score }: { score: SiteScore }) {
  const status = score.profile_status || "unknown";
  return (
    <T>
    <Callout tone="info" title="第一方档案未核对">
      <p>当前状态 {status}，不计入 /100 分数。请通过官方入口自行核对品牌信息；本平台不代认领、不代填。</p>
      <p className="mt-1.5">
        三类来源分表展示：第一方自述、第三方实测、第三方转述。
        {score.provenance && (
          <span className="ml-1 text-ink-subtle">
            口径 {score.provenance.methodology_version} · n_valid {score.provenance.n_valid}
          </span>
        )}
      </p>
    </Callout>
    </T>
  );
}

/** A-WP-9.2：索引准入高风险项。每条都带风险提示与来源链接，仍只给 1 条首要建议。 */
function AdmissionPanel({ score }: { score: SiteScore }) {
  const checks = score.admission || [];
  if (checks.length === 0) return null;
  const flagged = checks.filter((c) => c.status === "fail" || c.status === "partial");
  const unknown = checks.filter((c) => c.status === "unknown");
  const tone = (s: string) =>
    s === "fail" ? "negative" : s === "partial" ? "warn" : s === "unknown" ? "neutral" : "positive";
  const label = (s: string) =>
    s === "fail" ? "高风险" : s === "partial" ? "需留意" : s === "unknown" ? "未判定" : "通过";
  return (
    <T>
    <Card>
      <div className="flex items-center justify-between">
        <h2 className="text-base font-semibold text-ink flex items-center gap-2">
          <AlertTriangle className="h-4 w-4 text-warn" />索引准入风险提示
        </h2>
        <span className="text-xs text-ink-subtle">{checks.length} 项检查</span>
      </div>
      <p className="mt-1.5 text-xs text-ink-muted">
        概率性判断，阈值由本平台设定，非任何 AI 平台的收录规则。未判定项不进分母、不扣分。
      </p>

      {flagged.length > 0 ? (
        <div className="mt-4 space-y-2">
          {flagged.map((c) => (
            <div key={c.detector} className="rounded-ds border border-edge p-3">
              <div className="flex items-center gap-2">
                <Badge tone={tone(c.status)}>{label(c.status)}</Badge>
                <span className="text-sm font-medium text-ink">{c.detector}</span>
              </div>
              <p className="mt-1.5 text-xs text-ink-muted">证据：{c.evidence}</p>
              {c.fix && <p className="mt-1 text-xs text-ink-muted">建议：{c.fix}</p>}
              <p className="mt-1.5 text-[11px] text-ink-subtle">{c.risk_notice}</p>
              {c.source_url && (
                <a href={c.source_url} target="_blank" rel="noreferrer"
                  className="mt-1 inline-flex items-center gap-1 text-[11px] text-accent hover:underline">
                  来源 {c.source_url} <ExternalLink className="h-3 w-3" />
                </a>
              )}
            </div>
          ))}
        </div>
      ) : (
        <p className="mt-4 text-sm text-ink-muted">当前未发现索引准入高风险项。</p>
      )}

      {unknown.length > 0 && (
        <p className="mt-3 text-xs text-ink-subtle">
          {unknown.length} 项需真实渲染才能判定（{unknown.map((c) => c.detector).join("、")}），按 unknown 处理。
        </p>
      )}
    </Card>
    </T>
  );
}
