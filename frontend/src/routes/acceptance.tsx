import { T } from "../i18n";
import { useEffect, useState } from "react";
import { ShieldCheck, RefreshCw, ListChecks, CheckCircle2, XCircle, Info } from "lucide-react";
import {
  Page, PageHeader, Card, StatCard, Callout, Badge, Spinner, Button, cn,
} from "../ds";
import { api } from "../api";
import type { SelfChecksResponse, AcceptanceResponse } from "../types";
import { NEUTRALITY } from "../lib/neutrality";

/** A-WP-9.4：四项自检与增量发布验收清单的实时看板。 */
export default function AcceptancePage() {
  const [selfChecks, setSelfChecks] = useState<SelfChecksResponse | null>(null);
  const [acceptance, setAcceptance] = useState<AcceptanceResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    Promise.all([api.getSelfChecks(), api.getAcceptance()])
      .then(([sc, ac]) => {
        if (cancelled) return;
        setSelfChecks(sc);
        setAcceptance(ac);
        setError(null);
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : "加载失败");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => { cancelled = true; };
  }, [nonce]);

  const passedItems = acceptance?.items.filter((i) => i.passed).length ?? 0;
  const totalItems = acceptance?.items.length ?? 0;
  const greenChecks = selfChecks?.results.filter((r) => r.passed).length ?? 0;

  return (
    <T>
    <Page measure="default">
      <PageHeader
        title="发布验收"
        description="用自家产品检查自家站点，逐条核对增量发布清单。"
        actions={
          <Button variant="secondary" size="sm" onClick={() => setNonce((n) => n + 1)}>
            <RefreshCw className="h-4 w-4" />重新核对
          </Button>
        }
      />

      {loading && !acceptance ? (
        <div className="flex justify-center py-12"><Spinner /></div>
      ) : error && !acceptance ? (
        <Callout tone="warn" title="无法加载验收数据">{error}</Callout>
      ) : (
        <div className="space-y-6">
          {error && <Callout tone="warn" title="刷新失败，显示上次数据">{error}</Callout>}

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <StatCard label="四项自检" value={`${greenChecks}/4`} hint="A-01 / A-03 / A-05 / A-14"
              icon={<ShieldCheck className="h-5 w-5" />} />
            <StatCard label="清单通过" value={`${passedItems}/${totalItems}`} hint="增量发布验收清单"
              icon={<ListChecks className="h-5 w-5" />} />
            <StatCard label="方法论版本" value={acceptance?.methodology_version || "—"}
              hint="所有含数字组件的口径" icon={<Info className="h-5 w-5" />} />
          </div>

          {selfChecks && (
            <Card>
              <h2 className="text-base font-semibold text-ink mb-4 flex items-center gap-2">
                <ShieldCheck className="h-5 w-5 text-accent" />四项自检
              </h2>
              <div className="space-y-2">
                {selfChecks.results.map((r) => (
                  <div key={r.id} className="rounded-ds border border-edge p-3">
                    <div className="flex items-center gap-2">
                      {r.passed
                        ? <CheckCircle2 className="h-4 w-4 text-positive" />
                        : <XCircle className="h-4 w-4 text-negative" />}
                      <span className="text-sm font-medium text-ink">{r.id} · {r.name}</span>
                      <Badge tone={r.passed ? "positive" : "negative"}>
                        {r.passed ? "通过" : "未通过"}
                      </Badge>
                    </div>
                    <ul className="mt-2 space-y-0.5 pl-6 text-xs text-ink-muted list-disc">
                      {r.details.map((d, i) => <li key={i}>{d}</li>)}
                    </ul>
                  </div>
                ))}
              </div>
            </Card>
          )}

          {acceptance && (
            <Card>
              <h2 className="text-base font-semibold text-ink mb-4 flex items-center gap-2">
                <ListChecks className="h-5 w-5 text-accent" />增量发布验收清单
              </h2>
              <div className="space-y-1">
                {acceptance.items.map((item) => (
                  <div key={item.id}
                    className={cn(
                      "flex items-start gap-3 py-2.5 px-2 rounded-ds border-b border-edge last:border-0",
                      !item.passed && "bg-warn/5",
                    )}>
                    <span className="w-6 shrink-0 text-xs text-ink-subtle tabular-nums">{item.id}.</span>
                    <p className="flex-1 text-sm text-ink">{item.criterion}</p>
                    <Badge tone={item.passed ? "positive" : "warn"}>
                      {item.passed ? "通过" : "未通过"}
                    </Badge>
                  </div>
                ))}
              </div>
              <div className="mt-4 border-t border-edge pt-3">
                <p className="text-xs font-medium text-ink mb-2">冻结的套餐结构（A-WP-9.3）</p>
                <div className="flex flex-wrap gap-2">
                  {Object.entries(acceptance.prices).map(([k, v]) => (
                    <Badge key={k} tone="neutral">{k} ${v}</Badge>
                  ))}
                </div>
              </div>
            </Card>
          )}

          <Callout tone="info" title="口径说明">
            <p>{NEUTRALITY}</p>
            <ul className="mt-2 space-y-1 list-disc pl-5">
              {(selfChecks?.methodology || []).map((m, i) => <li key={i}>{m}</li>)}
            </ul>
          </Callout>
        </div>
      )}
    </Page>
    </T>
  );
}
