import { useState } from "react";
import { Search, Globe, CheckCircle2, XCircle, AlertCircle, MinusCircle } from "lucide-react";
import { Page, PageHeader, Card, Button, TextField, Badge, Callout, Spinner, StatCard } from "../ds";
import { api } from "../api";
import type { SiteScore, AuditCheck, CheckStatus } from "../types";

const STATUS_META: Record<CheckStatus, { label: string; tone: "positive" | "warn" | "negative"; icon: React.ReactNode }> = {
  pass: { label: "通过", tone: "positive", icon: <CheckCircle2 className="h-3.5 w-3.5" /> },
  partial: { label: "部分", tone: "warn", icon: <MinusCircle className="h-3.5 w-3.5" /> },
  fail: { label: "未通过", tone: "negative", icon: <XCircle className="h-3.5 w-3.5" /> },
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
    <Page measure="default">
      <PageHeader
        title="站点就绪度检查"
        description="输入网址，查看站点是否已为 AI 可见性做好准备。免注册，即查即得。"
      />

      <Card className="mb-6">
        <div className="flex flex-col sm:flex-row gap-3">
          <div className="flex-1">
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
            <Search className="h-4 w-4" />
            开始检查
          </Button>
        </div>
      </Card>

      {loading && (
        <div className="flex items-center justify-center py-16">
          <Spinner className="h-6 w-6" />
          <span className="ml-3 text-ink-muted">正在抓取站点并检查就绪度...</span>
        </div>
      )}

      {!loading && score && <AuditResult score={score} />}
    </Page>
  );
}

function AuditResult({ score }: { score: SiteScore }) {
  const unreachable = score.source === "unreachable";

  return (
    <div className="space-y-6">
      {unreachable && (
        <Callout tone="warn" title="站点无法访问">
          检查已记录各项为「未通过」，但请先确认网址可公开访问后再复测。
        </Callout>
      )}

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <StatCard
          label="就绪度总分"
          value={`${score.score.toFixed(1)} / 100`}
          hint={score.url}
          icon={<Globe className="h-5 w-5" />}
        />
        <StatCard
          label="检查项"
          value={score.checks.length}
          hint={`通过 ${score.checks.filter((c) => c.status === "pass").length} 项`}
          icon={<CheckCircle2 className="h-5 w-5" />}
        />
        <StatCard
          label="首要修复"
          value="P1"
          hint={score.top_fix}
          icon={<AlertCircle className="h-5 w-5" />}
        />
      </div>

      <Card>
        <h2 className="font-semibold text-ink mb-4">分组得分</h2>
        <div className="space-y-4">
          {score.groups.map((g) => (
            <div key={g.group}>
              <div className="flex justify-between text-sm mb-1">
                <span className="text-ink">{g.label}</span>
                <span className="text-ink-muted">
                  {g.earned.toFixed(1)} / {g.weight}
                </span>
              </div>
              <div className="w-full bg-canvas rounded-full h-2">
                <div
                  className="bg-accent h-2 rounded-full transition-all"
                  style={{ width: `${(g.earned / g.weight) * 100}%` }}
                />
              </div>
            </div>
          ))}
        </div>
        <p className="mt-4 text-xs text-ink-subtle">{score.caveat}</p>
      </Card>

      <Card>
        <h2 className="font-semibold text-ink mb-4">逐项检查</h2>
        <div className="space-y-3">
          {score.checks.map((check) => (
            <CheckRow key={check.id} check={check} />
          ))}
        </div>
      </Card>
    </div>
  );
}

function CheckRow({ check }: { check: AuditCheck }) {
  const meta = STATUS_META[check.status];
  return (
    <div className="flex items-start gap-3 py-2 border-b border-edge last:border-0">
      <div className="mt-0.5 shrink-0">{meta.icon}</div>
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
  );
}
