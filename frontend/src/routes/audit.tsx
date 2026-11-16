import { T } from "../i18n";
import { useState } from "react";
import {
  ScanLine,
  Globe,
  ShieldCheck,
  ShieldX,
  ShieldAlert,
  Zap,
  Activity,
  ListChecks,
  Gauge,
} from "lucide-react";
import { Page, PageHeader, Card, Button, TextField, Badge, Callout, Spinner, StatCard } from "../ds";
import { api } from "../api";
import type { SiteScore, AuditCheck, CheckStatus } from "../types";

const STATUS_META: Record<CheckStatus, { label: string; tone: "positive" | "warn" | "negative" | "neutral"; icon: React.ReactNode }> = {
  pass: { label: "通过", tone: "positive", icon: <ShieldCheck className="h-4 w-4" /> },
  partial: { label: "部分", tone: "warn", icon: <ShieldAlert className="h-4 w-4" /> },
  fail: { label: "未通过", tone: "negative", icon: <ShieldX className="h-4 w-4" /> },
  unknown: { label: "未判定", tone: "neutral", icon: <Gauge className="h-4 w-4" /> },
};

export default function AuditPage() {
  const [url, setUrl] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [score, setScore] = useState<SiteScore | null>(null);

  const run = async () => {
    if (!url.trim()) {
      setError("请填写网址");
      return;
    }
    setLoading(true);
    setError(null);
    setScore(null);
    try {
      const result = await api.auditSite(url.trim());
      setScore(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : "检查失败，请稍后重试");
    } finally {
      setLoading(false);
    }
  };

  return (
    <T>
    <Page measure="default">
      <PageHeader
        title="站点就绪度检查"
        description="输入网址，查看站点是否已为 AI 可见性做好准备。免注册，即查即得。"
      />

      <Card className="mb-6 relative">
        <div className="flex flex-col sm:flex-row gap-3">
          <div className="flex-1 relative">
            <Globe className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-ink-subtle pointer-events-none" />
            <TextField
              placeholder="例如 example.com"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && run()}
              error={error ?? undefined}
              aria-label="网址"
            />
          </div>
          <Button onClick={run} loading={loading} disabled={!url.trim()}>
            <ScanLine className="h-4 w-4" />
            开始扫描
          </Button>
        </div>
      </Card>

      {loading && (
        <Card className="py-16">
          <div className="flex flex-col items-center justify-center gap-4">
            <div className="relative">
              <Spinner className="h-8 w-8 text-accent" />
              <div className="absolute inset-0 flex items-center justify-center">
                <ScanLine className="h-4 w-4 text-accent animate-pulse" />
              </div>
            </div>
            <span className="text-sm text-ink-muted">正在抓取站点并检查就绪度...</span>
          </div>
        </Card>
      )}

      {!loading && score && <AuditResult score={score} />}
    </Page>
    </T>
  );
}

function AuditResult({ score }: { score: SiteScore }) {
  const unreachable = score.source === "unreachable";
  const passed = score.checks.filter((c) => c.status === "pass").length;

  return (
    <T>
    <div className="space-y-6">
      {unreachable && (
        <Callout tone="warn" title="站点无法访问">
          检查已记录各项为「未通过」，但请先确认网址可公开访问后再复测。
        </Callout>
      )}

      {/* Score gauge + stats */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <Card raised className="sm:col-span-1">
          <div className="flex items-center gap-4">
            <ScoreRing score={score.score} />
            <div>
              <p className="text-sm text-ink-muted">就绪度总分</p>
              <p className="text-xs text-ink-subtle mt-1 truncate max-w-[140px]">{score.url}</p>
            </div>
          </div>
        </Card>
        <StatCard
          label="检查项"
          value={score.checks.length}
          hint={`通过 ${passed} 项`}
          icon={<ListChecks className="h-5 w-5" />}
        />
        <StatCard
          label="首要修复"
          value="P1"
          hint={score.top_fix}
          icon={<Zap className="h-5 w-5" />}
        />
      </div>

      {/* Group scores with tech-style progress bars */}
      <Card>
        <h2 className="font-semibold text-ink mb-4 flex items-center gap-2">
          <Gauge className="h-5 w-5 text-accent" />
          分组得分
        </h2>
        <div className="space-y-4">
          {score.groups.map((g) => {
            const pct = (g.earned / g.weight) * 100;
            return (
              <div key={g.group}>
                <div className="flex justify-between text-sm mb-1.5">
                  <span className="text-ink font-medium">{g.label}</span>
                  <span className="text-ink-muted tabular-nums">
                    {g.earned.toFixed(1)} / {g.weight}
                  </span>
                </div>
                <div className="relative w-full bg-canvas rounded-full h-2 overflow-hidden">
                  <div
                    className="absolute inset-y-0 left-0 rounded-full bg-[image:var(--ds-gradient-accent)] transition-all duration-500"
                    style={{ width: `${pct}%` }}
                  />
                  {pct > 0 && (
                    <div
                      className="absolute inset-y-0 left-0 rounded-full opacity-50 blur-sm bg-[image:var(--ds-gradient-accent)] transition-all duration-500"
                      style={{ width: `${pct}%` }}
                    />
                  )}
                </div>
              </div>
            );
          })}
        </div>
        <p className="mt-4 text-xs text-ink-subtle">{score.caveat}</p>
      </Card>

      {/* Check list */}
      <Card>
        <h2 className="font-semibold text-ink mb-4 flex items-center gap-2">
          <Activity className="h-5 w-5 text-accent" />
          逐项检查
        </h2>
        <div className="space-y-1">
          {score.checks.map((check) => (
            <CheckRow key={check.id} check={check} />
          ))}
        </div>
      </Card>
    </div>
    </T>
  );
}

function ScoreRing({ score }: { score: number }) {
  const radius = 28;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference - (score / 100) * circumference;
  const color = score >= 70 ? "var(--ds-positive)" : score >= 40 ? "var(--ds-warn)" : "var(--ds-negative)";

  return (
    <T>
    <div className="relative h-20 w-20 shrink-0">
      <svg className="h-20 w-20 -rotate-90" viewBox="0 0 64 64">
        <circle
          cx="32"
          cy="32"
          r={radius}
          fill="none"
          stroke="var(--ds-edge)"
          strokeWidth="4"
        />
        <circle
          cx="32"
          cy="32"
          r={radius}
          fill="none"
          stroke={color}
          strokeWidth="4"
          strokeLinecap="round"
          strokeDasharray={circumference}
          strokeDashoffset={offset}
          className="transition-all duration-500"
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className="text-lg font-bold text-ink tabular-nums">{score.toFixed(0)}</span>
        <span className="text-[10px] text-ink-subtle">/100</span>
      </div>
    </div>
    </T>
  );
}

function CheckRow({ check }: { check: AuditCheck }) {
  const meta = STATUS_META[check.status];
  const toneColors = {
    positive: "text-positive bg-positive-subtle",
    warn: "text-warn bg-warn-subtle",
    negative: "text-negative bg-negative-subtle",
    neutral: "text-ink-muted bg-canvas",
  };
  return (
    <T>
    <div className="flex items-start gap-3 py-2.5 px-2 rounded-ds hover:bg-canvas/50 transition-colors border-b border-edge last:border-0">
      <div className={`mt-0.5 shrink-0 flex items-center justify-center h-7 w-7 rounded-ds-sm ${toneColors[meta.tone]}`}>
        {meta.icon}
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-ink">{check.label}</span>
          <Badge tone={meta.tone}>{meta.label}</Badge>
        </div>
        <p className="mt-0.5 text-xs text-ink-subtle">证据：{check.evidence}</p>
        {check.status !== "pass" && (
          <p className="mt-0.5 text-xs text-ink-muted">修复：{check.fix}</p>
        )}
      </div>
    </div>
    </T>
  );
}
