import type {
  Brand,
  DetectionQuery,
  DetectionResult,
  DiagnosticReport,
  ModelInfo,
  ReportSummary,
  SiteScore,
  Visibility,
  FunnelStats,
  SelfChecksResponse,
  AcceptanceResponse,
  QuestionSetResponse,
  GovernanceOverview, GraphOverview, HybridHit, CollectorOverview, BatchReport,
  MarginRow, Entitlement, BillingPanel, QuotaUsage, QuotaDecision, Order,
  MonitorResponse, ReleaseResponse, ChainReport, MCPInfo, PlanID, PaymentKind,
  AuthUser, AuthResponse,
} from "./types";
import { readToken, writeToken, notifyUnauthorized } from "./auth/session";

const BASE = "/api/v1";

async function req<T>(path: string, options?: RequestInit): Promise<T> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  const token = readToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  const resp = await fetch(`${BASE}${path}`, {
    ...options,
    headers: { ...headers, ...(options?.headers as Record<string, string> | undefined) },
  });
  if (resp.status === 204) return undefined as T;
  if (resp.status === 401 && !path.startsWith("/auth/login") && !path.startsWith("/auth/register")) {
    // 会话失效：清令牌并通知 AuthProvider，由路由守卫统一跳登录页。
    writeToken(null);
    notifyUnauthorized();
  }
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(body.error || `HTTP ${resp.status}`);
  }
  return resp.json();
}

export const api = {
  // ---- 会话 ----
  login: (email: string, password: string) =>
    req<AuthResponse>("/auth/login", { method: "POST", body: JSON.stringify({ email, password }) }),
  register: (email: string, password: string) =>
    req<AuthResponse>("/auth/register", { method: "POST", body: JSON.stringify({ email, password }) }),
  logout: () => req<{ ok: boolean }>("/auth/logout", { method: "POST" }),
  me: () => req<{ user: AuthUser }>("/auth/me"),

  // ---- Brands ----
  listBrands: () => req<Brand[]>("/brands"),
  getBrand: (id: string) => req<Brand>(`/brands/${id}`),
  createBrand: (data: Partial<Brand>) =>
    req<Brand>("/brands", { method: "POST", body: JSON.stringify(data) }),
  updateBrand: (id: string, data: Partial<Brand>) =>
    req<Brand>(`/brands/${id}`, { method: "PUT", body: JSON.stringify(data) }),
  deleteBrand: (id: string) => req<void>(`/brands/${id}`, { method: "DELETE" }),

  // ---- Queries ----
  listQueries: (brandId: string) => req<DetectionQuery[]>(`/brands/${brandId}/queries`),
  createQuery: (brandId: string, data: Partial<DetectionQuery>) =>
    req<DetectionQuery>(`/brands/${brandId}/queries`, {
      method: "POST",
      body: JSON.stringify(data),
    }),
  deleteQuery: (brandId: string, queryId: string) =>
    req<void>(`/brands/${brandId}/queries/${queryId}`, { method: "DELETE" }),

  // ---- Detection ----
  runDetection: (brandId: string, queryId: string) =>
    req<{ results: DetectionResult[] }>(
      `/brands/${brandId}/queries/${queryId}/detect`,
      { method: "POST" },
    ),

  // ---- Reports ----
  getReport: (brandId: string) => req<ReportSummary>(`/brands/${brandId}/report`),
  listResults: (brandId: string) => req<DetectionResult[]>(`/brands/${brandId}/results`),
  
  // ---- Diagnostics ----
  getDiagnosticReport: (brandId: string) =>
    req<DiagnosticReport>(`/brands/${brandId}/diagnose`),
  getVisibility: (brandId: string) =>
    req<Visibility>(`/brands/${brandId}/visibility`),

  // ---- Site audit (免注册) ----
  auditSite: (url: string) =>
    req<SiteScore>("/site-audits", { method: "POST", body: JSON.stringify({ url }) }),

  // ---- WP-6.3 可分享结果 + WP-6.6 漏斗 ----
  createShare: (score: SiteScore) =>
    req<{ token: string; url: string }>("/shares", {
      method: "POST",
      body: JSON.stringify({ score }),
    }),
  getShare: (token: string) => req<SiteScore>(`/shares/${token}`),
  getFunnel: () => req<FunnelStats>("/funnel"),

  // ---- 增量 WBS A-WP-1/7/9 ----
  getSelfChecks: () => req<SelfChecksResponse>("/self-checks"),
  getAcceptance: () => req<AcceptanceResponse>("/acceptance-checklist"),
  generateQuestionSets: (brief: {
    category: string; audience: string; need: string;
    region: string; locale: string; constraint: string;
  }, setType: string, count = 40) =>
    req<QuestionSetResponse>("/question-sets", {
      method: "POST",
      body: JSON.stringify({ brief, set_type: setType, count }),
    }),

  // ---- WP-3 运行治理 ----
  getGovernance: () => req<GovernanceOverview>("/governance"),
  dispatchRun: (body: { operator: string; task: string; change?: string; platform?: string; retry_of?: string }) =>
    req<{ result: { run_id: string; outcome: string }; budget: GovernanceOverview["budget"] }>("/governance/dispatch", {
      method: "POST", body: JSON.stringify(body),
    }),
  reportRateLimit: (platform: string, retryAfterSec = 30) =>
    req<{ ok: boolean }>("/governance/rate-limit", {
      method: "POST", body: JSON.stringify({ platform, retry_after_sec: retryAfterSec }),
    }),
  resumeRuns: () => req<{ resumed: unknown[] }>("/governance/resume", { method: "POST" }),

  // ---- WP-2 市场图谱 ----
  getGraph: () => req<GraphOverview>("/graph"),
  graphCreateBranch: (id: string, by = "collector") =>
    req("/graph/branches", { method: "POST", body: JSON.stringify({ id, by }) }),
  graphWriteNode: (branch: string, by: string, node: Record<string, unknown>) =>
    req("/graph/nodes", { method: "POST", body: JSON.stringify({ branch, by, node }) }),
  graphWriteEdge: (branch: string, by: string, edge: Record<string, unknown>) =>
    req("/graph/edges", { method: "POST", body: JSON.stringify({ branch, by, edge }) }),
  graphMerge: (branch: string, by = "reviewer") =>
    req("/graph/merge", { method: "POST", body: JSON.stringify({ branch, by }) }),
  graphSearch: (q: Record<string, unknown>) =>
    req<{ hits: HybridHit[] }>("/graph/search", { method: "POST", body: JSON.stringify(q) }),
  graphAt: (asOf: string) => req<{ nodes: number; edges: number }>(`/graph/at?as_of=${encodeURIComponent(asOf)}`),

  // ---- WP-1 采集框架 ----
  getCollector: () => req<CollectorOverview>("/collector"),
  runCollectorBatch: (count = 30) =>
    req<{ report: BatchReport; simulated: boolean }>("/collector/batch", {
      method: "POST", body: JSON.stringify({ brand_id: "geolens", platform: "chatgpt", region: "us", count }),
    }),
  getCollectorMargins: () => req<{ margins: MarginRow[] }>("/collector/margins"),

  // ---- WP-7 套餐计费 ----
  getPlans: () => req<{ plans: Entitlement[]; note: string }>("/plans"),
  // 账户身份由服务端从会话解析，前端不再传 account_id。
  getBillingPanel: () => req<BillingPanel>("/billing/panel"),
  getBillingQuota: () => req<{ usage: QuotaUsage; decision: QuotaDecision }>("/billing/quota"),
  checkout: (plan: PlanID, kind: PaymentKind = "subscription") =>
    req<{ order: Order; steps: string[] }>("/billing/checkout", {
      method: "POST", body: JSON.stringify({ plan, kind }),
    }),
  payOrder: (orderId: string) =>
    req<{ order: Order }>("/billing/pay", { method: "POST", body: JSON.stringify({ order_id: orderId }) }),
  redeemCode: (code: string, contact: string, note: string) =>
    req<{ code: string; plan: PlanID; note: string }>("/redeem", {
      method: "POST", body: JSON.stringify({ code, contact, note }),
    }),

  // ---- WP-8 监控与回归 ----
  getMonitor: () => req<MonitorResponse>("/monitor"),
  getReleaseChecklist: () => req<ReleaseResponse>("/monitor/release-checklist"),
  runFullChain: (brandId: string) =>
    req<{ report: ChainReport }>("/full-chain", {
      method: "POST", body: JSON.stringify({ brand_id: brandId }),
    }),

  // ---- WP-5 MCP ----
  getMCPInfo: () => req<MCPInfo>("/mcp/info"),

  // ---- Models ----
  listModels: () => req<ModelInfo[]>("/models"),

  health: () => req<{ status: string }>("/health"),
};
