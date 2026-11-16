import { T } from "../i18n";
import { useEffect, useState } from "react";
import { Plug, Terminal, ShieldCheck, RefreshCw } from "lucide-react";
import { Page, PageHeader, Card, Callout, Badge, Spinner, Button } from "../ds";
import { api } from "../api";
import type { MCPInfo } from "../types";

/** WP-5：MCP 两工具接入说明与 Claude Code 两步自查。 */
export default function MCPDocsPage() {
  const [info, setInfo] = useState<MCPInfo | null>(null);
  const [token, setToken] = useState("");
  const [result, setResult] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.getMCPInfo().then(setInfo).catch((e) => setError(String(e)));
  }, []);

  async function testToolsList() {
    setResult(null);
    try {
      const resp = await fetch("/mcp", {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "tools/list" }),
      });
      const body = await resp.json();
      setResult(JSON.stringify(body, null, 2));
    } catch (e) {
      setResult(e instanceof Error ? e.message : "请求失败");
    }
  }

  return (
    <T>
    <Page measure="default">
      <PageHeader title="MCP 接入" description="用只读令牌在 Claude Code 里查询可见性与证据。" />

      <div className="space-y-6">
        {error && <Callout tone="warn" title="无法加载接入信息">{error}</Callout>}
        {!info && !error && <div className="flex justify-center py-12"><Spinner /></div>}

        {info && (
          <>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              <Card><p className="text-xs text-ink-muted">Server</p><p className="text-sm font-semibold text-ink mt-1">{info.server}</p></Card>
              <Card><p className="text-xs text-ink-muted">版本</p><p className="text-sm font-semibold text-ink mt-1">{info.version}</p></Card>
              <Card><p className="text-xs text-ink-muted">端点</p><p className="text-sm font-semibold text-ink mt-1 font-mono">{info.endpoint}</p></Card>
              <Card><p className="text-xs text-ink-muted">权限</p><p className="text-sm font-semibold text-ink mt-1">{info.read_only ? "只读" : "读写"}</p></Card>
            </div>

            <Card>
              <h2 className="text-base font-semibold text-ink mb-3 flex items-center gap-2">
                <Plug className="h-5 w-5 text-accent" />两个工具
              </h2>
              <div className="space-y-3">
                {info.tools.map((t) => (
                  <div key={t} className="rounded-ds border border-edge p-3">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-sm text-ink">{t}</span>
                      <Badge tone="neutral">只读</Badge>
                    </div>
                    <p className="mt-1 text-xs text-ink-muted">
                      {t === "query_visibility"
                        ? "按 brand / market / model / region 返回逐模型三态摘要，无综合分，每个比例伴随 n_valid。"
                        : "按 run_id 返回原始答案文本与引用 URL，用于两步下钻取证。"}
                    </p>
                  </div>
                ))}
              </div>
              <p className="mt-3 text-xs text-ink-muted">{info.note}</p>
            </Card>

            <Card>
              <h2 className="text-base font-semibold text-ink mb-3 flex items-center gap-2">
                <Terminal className="h-5 w-5 text-accent" />Claude Code 配置
              </h2>
              <pre className="rounded-ds bg-canvas border border-edge p-3 text-xs text-ink overflow-x-auto">{`{
  "mcpServers": {
    "answerlens": {
      "url": "<host>${info.endpoint}",
      "headers": { "Authorization": "Bearer <agent_token>" }
    }
  }
}`}</pre>
              <p className="mt-2 text-xs text-ink-muted">{info.auth}</p>
              <p className="mt-2 text-xs text-ink-muted">
                令牌由管理员通过 {info.methodology === "1.0.0" ? "POST /api/v1/mcp/tokens" : "签发端点"} 分发；
                本地零配置启动时控制台会打印一个只读令牌。
              </p>
            </Card>

            <Card>
              <h2 className="text-base font-semibold text-ink mb-3 flex items-center gap-2">
                <ShieldCheck className="h-5 w-5 text-accent" />两步自查
              </h2>
              <ol className="list-decimal pl-5 text-sm text-ink space-y-2">
                <li>配置 MCP 后调用 <code className="font-mono text-xs">query_visibility</code> 查自家品牌，确认输出逐模型三态、带 n_valid、没有综合分。</li>
                <li>取结果里的 <code className="font-mono text-xs">run_id</code> 调用 <code className="font-mono text-xs">get_evidence</code>，确认能拿到原始答案与引用 URL。</li>
              </ol>
              <p className="mt-3 text-xs text-ink-muted">没有检测数据时先跑一次检测；不得用示例数据冒充自查结果。</p>
            </Card>

            <Card>
              <h2 className="text-base font-semibold text-ink mb-3 flex items-center gap-2">
                <RefreshCw className="h-5 w-5 text-accent" />验证只读令牌
              </h2>
              <div className="flex gap-2">
                <input
                  value={token}
                  onChange={(e) => setToken(e.target.value)}
                  placeholder="agt_..."
                  className="flex-1 rounded-ds border border-edge bg-surface px-3 py-2 text-sm text-ink"
                />
                <Button size="sm" variant="secondary" onClick={testToolsList}>调用 tools/list</Button>
              </div>
              {result && <pre className="mt-3 rounded-ds bg-canvas border border-edge p-3 text-xs text-ink overflow-x-auto max-h-64">{result}</pre>}
            </Card>
          </>
        )}
      </div>
    </Page>
    </T>
  );
}
