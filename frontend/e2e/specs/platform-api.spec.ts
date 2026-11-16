import { test, expect } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

// 受保护端点一律带会话令牌；globalSetup 登录后写入，未登录场景单独断言 401。
const token = fs.readFileSync(
  path.join(process.cwd(), "e2e", ".auth", "token.txt"), "utf8",
).trim();
const auth = () => ({ Authorization: `Bearer ${token}` });

// WP-4.1 / WP-4.2：平台 API 冒烟 + 关键不变量的接口层回归，失败即让 CI 变红。
test("未登录访问受保护端点被拒绝", async ({ request }) => {
  const resp = await request.get("/api/v1/governance");
  expect(resp.status()).toBe(401);
  const forbidden = await request.post("/api/v1/governance/reset-budget");
  expect(forbidden.status()).toBe(401);
});

test("平台运行时端点可用且带口径标注", async ({ request }) => {
  const health = await request.get("/api/v1/health");
  expect(health.ok()).toBeTruthy();

  const gov = await request.get("/api/v1/governance", { headers: auth() });
  expect(gov.ok()).toBeTruthy();
  const govBody = await gov.json();
  expect(govBody.routes.length).toBeGreaterThan(0);
  expect(govBody.provenance.methodology_version).toBeTruthy();

  const plans = await request.get("/api/v1/plans");
  expect((await plans.json()).plans.length).toBe(6);

  const monitor = await request.get("/api/v1/monitor", { headers: auth() });
  expect(monitor.ok()).toBeTruthy();
  expect((await monitor.json()).boards.length).toBe(5);

  const mcp = await request.get("/api/v1/mcp/info");
  expect((await mcp.json()).tools).toEqual(["query_visibility", "get_evidence"]);
});

test("MCP 非法 token 被拒绝", async ({ request }) => {
  const resp = await request.post("/mcp", {
    headers: { Authorization: "Bearer invalid-token", "Content-Type": "application/json" },
    data: { jsonrpc: "2.0", id: 1, method: "tools/list" },
  });
  expect(resp.status()).toBe(401);
});

test("30 题采集批次成功率与成本口径", async ({ request }) => {
  const resp = await request.post("/api/v1/collector/batch", {
    headers: auth(),
    data: { brand_id: "geolens", platform: "chatgpt", region: "us", count: 30 },
  });
  expect(resp.ok()).toBeTruthy();
  const body = await resp.json();
  expect(body.report.total).toBe(30);
  expect(body.report.success_rate).toBeGreaterThanOrEqual(0.8);
  expect(body.report.valid + body.report.failed).toBe(30);
  expect(body.simulated).toBe(true);
});

test("checkout 忽略客户端伪造的 account_id", async ({ request }) => {
  const resp = await request.post("/api/v1/billing/checkout", {
    headers: auth(),
    data: { account_id: "victim", plan: "growth", kind: "subscription" },
  });
  expect(resp.ok()).toBeTruthy();
  const { order } = await resp.json();
  // 账户身份只来自会话，绝不接受 body 里的 account_id。
  expect(order.account_id).toBe("demo-account");
  expect(order.account_id).not.toBe("victim");
});

test("collector 直写 main 被图谱引擎拒绝", async ({ request }) => {
  const resp = await request.post("/api/v1/graph/nodes", {
    headers: auth(),
    data: {
      branch: "main", by: "collector",
      node: { id: "brand:e2e", kind: "Brand", label: "E2E" },
    },
  });
  expect(resp.status()).toBe(400);
  expect((await resp.json()).error).toContain("main");
});

// Δ-01~Δ-07 / §3.1：增量批次的 API 契约回归，flag 全开时断言字段拼写与四数。
test("Δ 批次端点返回四数、分行字段与固定包装", async ({ request }) => {
  const report = await request.get("/v1/reports/report_demo_1", { headers: auth() });
  expect(report.ok()).toBeTruthy();
  const body = await report.json();
  expect(body.methodology_version).toBe("1.1");
  for (const key of ["planned", "usable", "failed", "suspect"]) {
    expect(body.counts, `counts 缺键 ${key}`).toHaveProperty(key);
  }
  expect(body.counts).not.toHaveProperty("overall_score");
  expect(body.language_notice).toBeTruthy();
  expect(Array.isArray(body.reference_only)).toBeTruthy();

  const checks = await request.get("/v1/audits/audit_demo_1/checks", { headers: auth() });
  expect(checks.ok()).toBeTruthy();
  const rows = await checks.json();
  expect(rows[0]).toHaveProperty("platform_hint");
  expect(rows[0]).toHaveProperty("observed_evidence");
  expect(rows[0].platform_hint).not.toBe(rows[0].observed_evidence);

  const tools = await request.get("/v1/mcp/tools?limit=2", { headers: auth() });
  const list = await tools.json();
  expect(list.truncated).toBe(true);
  expect(list.client_notice).toBe("能力集不完整");
  expect(list.returned_count).toBe(2);
  expect(list.total_count).toBeGreaterThan(2);

  const ci = await request.get("/api/v1/delta/ci-assertions", { headers: auth() });
  expect((await ci.json()).all_pass).toBe(true);
});

// 海外收款：无凭证时降级演示网关，匿名回调必须验签，旧流程保持兼容。
test("支付端点：供应商目录、演示降级与回调验签", async ({ request }) => {
  const res = await request.get("/api/v1/billing/providers", { headers: auth() });
  expect(res.ok()).toBeTruthy();
  const { providers } = await res.json();
  expect(Array.isArray(providers)).toBeTruthy();
  expect(providers.some((p: { id: string }) => p.id === "sandbox")).toBeTruthy();
  expect(providers.find((p: { id: string }) => p.id === "sandbox").live).toBe(false);

  const checkout = await request.post("/api/v1/billing/checkout", {
    headers: auth(), data: { plan: "starter", kind: "subscription", provider: "sandbox" },
  });
  expect(checkout.ok()).toBeTruthy();
  const body = await checkout.json();
  expect(body.next).toBe("pay");
  expect(body.checkout_url).toBeFalsy();
  expect(body.order.status).toBe("pending");

  const unsigned = await request.post("/api/v1/billing/webhook/paypal", {
    data: { event_id: "fake", order_id: body.order.id, status: "succeeded" },
  });
  expect([401, 404]).toContain(unsigned.status());
});
