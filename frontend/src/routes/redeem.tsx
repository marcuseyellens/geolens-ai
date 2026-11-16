import { T } from "../i18n";
import { useState } from "react";
import { Gift } from "lucide-react";
import { Page, PageHeader, Card, Callout, Button } from "../ds";
import { api } from "../api";
import { NEUTRALITY } from "../lib/neutrality";

/** WP-7.6：无支付、无账号的一次性兑换码自助页。 */
export default function RedeemPage() {
  const [code, setCode] = useState("");
  const [contact, setContact] = useState("");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    setBusy(true); setError(null); setDone(null);
    try {
      const res = await api.redeemCode(code.trim(), contact.trim(), note.trim());
      setDone(`兑换成功：${res.plan}。${res.note}`);
      setCode("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "兑换失败");
    } finally {
      setBusy(false);
    }
  }

  return (
    <T>
    <Page measure="narrow">
      <PageHeader title="兑换码" description="输入兑换码即可开通权益，无需支付。" />
      <div className="space-y-4">
        {done && (
          <Callout tone="positive" title="已兑换">{done}</Callout>
        )}
        {error && <Callout tone="warn" title="无法兑换">{error}</Callout>}

        <Card>
          <div className="flex items-center gap-2 mb-4">
            <Gift className="h-5 w-5 text-accent" />
            <h2 className="text-base font-semibold text-ink">兑换权益</h2>
          </div>
          <div className="space-y-3">
            <label className="block">
              <span className="text-xs text-ink-muted">兑换码</span>
              <input value={code} onChange={(e) => setCode(e.target.value)}
                placeholder="AL-XXXX-0001"
                className="mt-1 w-full rounded-ds border border-edge bg-surface px-3 py-2 text-sm text-ink" />
            </label>
            <label className="block">
              <span className="text-xs text-ink-muted">联系方式（选填）</span>
              <input value={contact} onChange={(e) => setContact(e.target.value)}
                placeholder="用于交付反馈"
                className="mt-1 w-full rounded-ds border border-edge bg-surface px-3 py-2 text-sm text-ink" />
            </label>
            <label className="block">
              <span className="text-xs text-ink-muted">想优先解决的问题（选填）</span>
              <textarea value={note} onChange={(e) => setNote(e.target.value)} rows={3}
                className="mt-1 w-full rounded-ds border border-edge bg-surface px-3 py-2 text-sm text-ink" />
            </label>
            <Button onClick={submit} disabled={busy || !code.trim()} className="w-full">
              {busy ? "处理中…" : "兑换"}
            </Button>
          </div>
        </Card>

        <Callout tone="info" title="说明">
          <p>每个兑换码仅可使用一次。反馈会进入 2 个工作日的人工交付队列。</p>
          <p className="mt-1">{NEUTRALITY}</p>
        </Callout>
      </div>
    </Page>
    </T>
  );
}
