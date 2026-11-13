import type {
  Brand,
  DetectionQuery,
  DetectionResult,
  DiagnosticReport,
  ModelInfo,
  ReportSummary,
  SiteScore,
  Visibility,
} from "./types";

const BASE = "/api/v1";

async function req<T>(path: string, options?: RequestInit): Promise<T> {
  const resp = await fetch(`${BASE}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  if (resp.status === 204) return undefined as T;
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(body.error || `HTTP ${resp.status}`);
  }
  return resp.json();
}

// ---- Brands ----
export const api = {
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

  // ---- Models ----
  listModels: () => req<ModelInfo[]>("/models"),

  health: () => req<{ status: string }>("/health"),
};
