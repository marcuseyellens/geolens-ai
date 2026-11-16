export type AIModelID = "chatgpt" | "claude" | "gemini" | "perplexity";

export interface Brand {
  id: string;
  name: string;
  website: string;
  description: string;
  competitors: string[];
  keywords: string[];
  created_at: string;
  updated_at: string;
}

export type QueryIntent = "recommend" | "compare" | "howto" | "best" | "alternative";

export interface DetectionQuery {
  id: string;
  brand_id: string;
  text: string;
  intent: QueryIntent;
  created_at: string;
}

export type MentionPosition = "first" | "top3" | "top5" | "mentioned" | "absent";
export type Sentiment = "positive" | "neutral" | "negative";

export interface DetectionResult {
  id: string;
  query_id: string;
  brand_id: string;
  model: AIModelID;
  response_text: string;
  brand_mentioned: boolean;
  mention_position: MentionPosition;
  sentiment: Sentiment;
  competitors_mentioned: string[];
  score: number;
  response_time_ms: number;
  tokens_used: number;
  detected_at: string;
}

export interface ModelStat {
  total: number;
  mentioned: number;
  mention_rate: number;
  avg_score: number;
}

export interface ReportSummary {
  brand_id: string;
  brand_name: string;
  total_detections: number;
  mention_rate: number;
  avg_score: number;
  avg_position: number;
  model_breakdown: Partial<Record<AIModelID, ModelStat>>;
  trend: { date: string; score: number; mention_rate: number }[];
}

// ---- 诊断报告类型 ----

export type FailureType = 
  | "loading_error"
  | "empty_answer"
  | "rate_limit"
  | "auth_error"
  | "parse_error"
  | "blocked"
  | "timeout";

export type SearchActivity = "on" | "off" | "unknown" | "auto";

export type SourceClass = 
  | "competitor_site"
  | "independent_roundup"
  | "directory"
  | "community"
  | "news_research"
  | "brand_site"
  | "other";

export interface Citation {
  url: string;
  domain: string;
  page_path: string;
  source_class: SourceClass;
  direct_citation: boolean;
}

export interface Mention {
  brand_id: string;
  mentioned: boolean;
  recommended: boolean;
  linked: boolean;
  confused_with?: string;
  rank_in_list: number;
  in_shortlist: boolean;
  is_final_pick: boolean;
  sentiment: Sentiment;
  associated_needs?: string[];
  associated_selling_points?: string[];
}

export interface RunAnswer {
  run_id: string;
  question_id: string;
  question_text: string;
  is_brand_check: boolean;
  intent: QueryIntent;
  question_category: string;
  platform: AIModelID;
  model_version: string;
  region: string;
  locale: string;
  search_activity: SearchActivity;
  forced_search: boolean;
  raw_answer_text: string;
  citations: Citation[];
  valid: boolean;
  failure_type?: FailureType;
  mention: Mention;
  competitor_mentions?: Record<string, number>;
  total_brand_mentions: number;
  observed_on: string;
  collected_at: string;
  latency_ms: number;
  cost: number;
}

export interface MetricSet {
  planned: number;
  usable: number;
  failed: number;
  forced_excluded: number;
  brand_check_excluded: number;
  mentioned: number;
  recommended: number;
  linked: number;
  mention_rate: number;
  recommend_rate: number;
  link_rate: number;
  citation_rate: number;
  share_of_voice: number;
  confusion_count: number;
  sample_note?: string;
}

export interface ModelMetricSet {
  platform: AIModelID;
  metrics: MetricSet;
  positive: number;
  neutral: number;
  negative: number;
  search_on: number;
  search_off: number;
  search_unknown: number;
  search_auto: number;
}

export interface SOVRow {
  brand: string;
  mentions: number;
  share: number;
  run_ids: string[];
}

export interface CitationSample {
  run_id: string;
  url: string;
  intent: string;
  is_direct: boolean;
}

export interface CitationGroup {
  source_class: SourceClass;
  count: number;
  domains: string[];
  evidence: CitationSample[];
}

export interface Comparison {
  matched_questions: number;
  before_usable: number;
  after_usable: number;
  before_mentioned: number;
  after_mentioned: number;
  before_rate: number;
  after_rate: number;
  note: string;
}

export interface NameConfusion {
  run_id: string;
  confused_with: string;
  excerpt: string;
}

export interface ExecutiveSummary {
  current: string;
  gap: string;
  next: string;
}

export interface Visibility {
  brand_id: string;
  buyer: MetricSet;
  models: Record<AIModelID, ModelMetricSet>;
}

export type CheckStatus = "pass" | "partial" | "fail" | "unknown";
export type AuditGroup = "crawlability" | "structured_data" | "content_signals" | "basic_health" | "index_admission";

export interface AuditCheck {
  id: string;
  group: AuditGroup;
  label: string;
  status: CheckStatus;
  evidence: string;
  fix: string;
  // Δ-03 / EX-08：平台提示与实际观察两行独立字段，互不填充。
  platform_hint?: string;
  observed_evidence?: string;
  evidence_source?: string;
  crawler_visit_evidence?: "pass" | "partial" | "fail" | "unknown";
}

// Δ-03 / EX-07：第 9 项「爬虫访问证据」结论，结果页分行渲染。
export interface CrawlerVisitRow {
  verdict: "pass" | "partial" | "fail" | "unknown";
  platform_hint: string;
  observed_evidence: string;
  evidence_source: string;
  disabled?: boolean;
  fix_advice?: string;
}

export type AdmissionStatus = "pass" | "partial" | "fail" | "unknown";

export interface Provenance {
  source_type: string;
  methodology_version: string;
  n_valid: number;
  window?: string;
}

export interface AdmissionCheck {
  detector: string;
  status: AdmissionStatus;
  evidence: string;
  fix: string;
  risk_notice: string;
  source_url: string;
  provenance: Provenance;
}

export interface GroupScore {
  group: AuditGroup;
  label: string;
  weight: number;
  earned: number;
  score: number;
}

export interface SiteScore {
  url: string;
  score: number;
  checks: AuditCheck[];
  groups: GroupScore[];
  top_fix: string;
  caveat: string;
  source: string;
  audited_at: string;
  provenance?: Provenance;
  profile_status?: string;
  admission?: AdmissionCheck[];
  // Δ-03：feature flag on 时返回第 9 项分行结论；off 时字段整体缺失。
  crawler_visit?: CrawlerVisitRow;
}

// ---- 增量 WBS A-WP-1/7/9 的 API 契约 ----
export interface SelfCheckResult {
  id: string;
  name: string;
  passed: boolean;
  details: string[];
}

export interface SelfChecksResponse {
  results: SelfCheckResult[];
  all_passed: boolean;
  provenance: Provenance;
  methodology: string[];
}

export interface AcceptanceItem {
  id: number;
  criterion: string;
  passed: boolean;
  evidence?: string[];
}

export interface AcceptanceResponse {
  items: AcceptanceItem[];
  prices: Record<string, number>;
  provenance: Provenance;
  methodology_version: string;
}

export interface GeneratedQuestion {
  id: string;
  text: string;
  intent: string;
  question_set_type: string;
  is_brand_check: boolean;
}

export interface QuestionSetResponse {
  question_set_type: string;
  questions: GeneratedQuestion[];
  count: number;
  provenance: Provenance;
  note: string;
}

export type ArtifactType = 
  | "llms_txt"
  | "json_ld"
  | "comparison_page"
  | "faq_page"
  | "robots_txt";

export interface FixArtifact {
  type: ArtifactType;
  filename: string;
  target_path: string;
  validation: string;
  content: string;
}

export interface FixItem {
  priority: string;
  problem: string;
  evidence: string[];
  change: string;
  target_metric: string;
  acceptance: string;
  artifact_types: ArtifactType[];
}

export interface DiagnosticReport {
  methodology_version: string;
  brand_id: string;
  brand_name: string;
  generated_at: string;
  executive_summary: ExecutiveSummary;
  site_readiness?: SiteScore;
  ai_visibility: MetricSet;
  buyer_questions: Record<QueryIntent, MetricSet>;
  brand_check: MetricSet;
  model_breakdown: Record<AIModelID, ModelMetricSet>;
  competitor_sov: SOVRow[];
  citation_map: CitationGroup[];
  name_confusion: NameConfusion[];
  fix_list: FixItem[];
  artifacts: FixArtifact[];
  evidence: RunAnswer[];
  comparison?: Comparison;
  methodology_notes: string[];
  // Δ 批次（flag on 时追加）：四数 counts、跨语言并列观察、独立参考区。
  counts?: Counts;
  language_notice?: string;
  reference_only?: UserProvidedMetric[];
  legacy_counts?: boolean;
}

// Δ-05 / EX-12：四数互不折算，suspect 不并入 usable / failed。
export interface Counts {
  planned: number;
  usable: number;
  failed: number;
  suspect: number;
}

// Δ-02 / EX-06：独立参考区，不与业务指标同列求平均。
export interface UserProvidedMetric {
  metric_id: number;
  project_id: number;
  metric_type: string;
  value: number;
  source_type: string;
  disclaimer: string;
  created_at: string;
}

// ---- 第三批次：竞品功能采集（CF-01~CF-08，独立 flag aeo_competitor_20261116）----
export type CaptureChannel = "real_ui" | "api_sourced" | "hybrid" | "unknown";
export type LocationScope = "country" | "region" | "state" | "city" | "none";
export type GapType =
  | "competitor_present_brand_absent"
  | "competitor_cited_brand_not"
  | "competitor_recommended_brand_not";
export type BotVerification =
  | "ip_verified" | "reverse_dns_verified" | "ua_only"
  | "suspected_spoof" | "unverified";

export interface CompetitorStatus {
  flag: string;
  on: boolean;
  delta_flag: string;
  delta_on: boolean;
  methodology_version: string;
  legacy_versions: string[];
  schema_version: string;
  neutrality_notice: string;
  contracts: string[];
  migration: Record<string, unknown>;
}

export interface ReportTemplate {
  report_id: string;
  methodology_version: string;
  counts: Counts;
  count_notice?: string;
  neutrality_notice?: string;
  capture_channel?: CaptureChannel;
  capture_channel_notice?: string;
  source_type: string;
}

export interface EvidenceRecord {
  run_id: string;
  methodology_version: string;
  raw_answer_text: string;
  sample_index: number;
  capture_channel: CaptureChannel;
  citations: { url: string; domain: string; page_path?: string }[];
  source_type: string;
  collected_at: string;
}

export interface ActionBoardItem {
  signal_id: string;
  question_id: string;
  gap_type: GapType;
  competitor_brand_id: string;
  run_id: string;
  detected_at: string;
}

export interface CompetitorGap {
  signal_id: string;
  project_id: string;
  question_id: string;
  run_id: string;
  competitor_brand_id: string;
  gap_type: GapType;
  priority_score: number;
  status: string;
  detected_at: string;
}

export interface CrawlerVisit {
  visit_id: string;
  project_id: string;
  page_url: string;
  crawler_operator: string;
  ua_string: string;
  source_ip: string;
  bot_verification: BotVerification;
  suspected_spoof: boolean;
  observed_at: string;
  evidence_source: string;
}

export interface CrawlPath {
  path_id: string;
  project_id: string;
  page_url: string;
  citation_id: string;
  run_id: string;
  crawled_at: string;
  cited_at: string;
  visit_id?: string;
}

export interface PromptTargetingView {
  target_id?: string;
  question_set_id: string;
  location_scope: LocationScope;
  location_value: string;
  persona: string;
  timezone: string;
  config_version: string;
  query_locale: string;
}

export interface CompetitorRunView {
  run_id: string;
  question_set_id: string;
  project_id: string;
  sample_rounds: number;
  sample_policy_version: string;
  capture_channel_declared: boolean;
  methodology_version: string;
}

export interface ModelInfo {
  id: AIModelID;
  name: string;
  enabled: boolean;
  demo?: boolean;
}

export const MODEL_LABELS: Record<AIModelID, string> = {
  chatgpt: "ChatGPT",
  claude: "Claude",
  gemini: "Gemini",
  perplexity: "Perplexity",
};

export const POSITION_LABELS: Record<MentionPosition, string> = {
  first: "第1位",
  top3: "前3位",
  top5: "前5位",
  mentioned: "被提及",
  absent: "未提及",
};

export const SENTIMENT_LABELS: Record<Sentiment, string> = {
  positive: "正面",
  neutral: "中性",
  negative: "负面",
};

export const INTENT_LABELS: Record<QueryIntent, string> = {
  recommend: "推荐类",
  compare: "对比类",
  howto: "教程类",
  best: "最佳类",
  alternative: "替代品类",
};

// ---- WP-6.6 转化漏斗 ----
export interface FunnelStats {
  steps: string[];
  counts: Record<string, number>;
  conversions: Record<string, number>;
  baselines: Record<string, number>;
  total_events: number;
  unique_days: number;
  since: string;
}

export const FUNNEL_LABELS: Record<string, string> = {
  visit: "访问",
  check: "发起检查",
  result: "得到结果",
  share: "生成分享",
  signup: "注册",
  checkout_39: "购买审计",
  subscribe: "订阅",
};

// ---- 基线 WBS WP-1/2/3/5/7/8 平台运行时 ----

export type TaskKind = "collect" | "judge_mention" | "judge_recommend" | "judge_link" | "generate_fix";
export type PlanID = "free" | "one_time" | "rescan" | "starter" | "growth" | "scale";
export type PaymentKind = "one_time" | "subscription" | "add_on";
export type PaymentStatus = "pending" | "succeeded" | "failed" | "refunded";

export interface RouteRule {
  task: TaskKind;
  channel: "low_cost" | "strong_model";
  model: string;
  estimated_cost: number;
  quality_critical?: boolean;
}

export interface BudgetState {
  limit_usd: number;
  spent_usd: number;
  tripped: boolean;
  rejected_runs: number;
  tripped_events: number;
  tripped_at?: string;
}

export interface RunPentad {
  run_id: string;
  operator: string;
  task: TaskKind;
  change: string;
  cost: number;
  outcome: string;
  channel: string;
  at: string;
  retry_of?: string;
}

export interface OperatorSummary {
  operator: string;
  runs: number;
  cost_usd: number;
  rejected: number;
}

export interface GovernanceOverview {
  routes: RouteRule[];
  budget: BudgetState;
  pentads: RunPentad[];
  operators: OperatorSummary[];
  pending: { run_id: string; resume_at: string }[];
  events: { kind: string; spent_usd: number; limit_usd: number; at: string }[];
  methodology: string;
  provenance: Provenance;
}

export interface GraphOverview {
  ontology: string[];
  nodes: number;
  edges: number;
  merges: { id: string; branch_id: string; operator: string; at: string; rolled_back: boolean }[];
  note: string;
}

export interface HybridHit {
  id: string;
  kind: string;
  label: string;
  struct_score: number;
  text_score: number;
  sim_score: number;
  score: number;
}

export interface MarginRow {
  channel: string;
  platform: string;
  runs: number;
  valid_runs: number;
  cost_usd: number;
}

export interface BatchReport {
  total: number;
  valid: number;
  failed: number;
  success_rate: number;
  cost_usd: number;
  run_ids: string[];
}

export interface CollectorOverview {
  adapters: string[];
  policy: { public_only: boolean; allow_login_state: boolean; allow_paywall_bypass: boolean };
  margins: MarginRow[];
  runs: number;
  simulated: boolean;
  note: string;
}

export interface Entitlement {
  plan: PlanID;
  daily_queries: number;
  monthly_runs: number;
  projects: number;
  one_time_questions: number;
  mcp_enabled: boolean;
  api_enabled: boolean;
  alerts_enabled: boolean;
  custom_export: boolean;
  price_usd: number;
}

export interface QuotaUsage {
  plan: PlanID;
  daily_used: number;
  daily_limit: number;
  monthly_used: number;
  monthly_limit: number;
  failed_not_billed: number;
  period: string;
}

export interface QuotaDecision {
  allowed: boolean;
  reason?: string;
  upgrade_to?: PlanID;
  message: string;
}

/** 海外收款供应商：id 与后端 billing.Provider 一致。 */
export interface PaymentProvider {
  id: "sandbox" | "airwallex" | "paypal";
  name: string;
  description: string;
  methods: string[];
  currencies: string[];
  live: boolean;
  sandbox_mode: boolean;
}

export interface CheckoutResponse {
  order: Order;
  provider: PaymentProvider["id"];
  next: "pay" | "redirect" | "done" | "retry";
  checkout_url?: string;
  session_id?: string;
  steps: string[];
}

export interface Order {
  id: string;
  account_id: string;
  plan: PlanID;
  kind: PaymentKind;
  amount_usd: number;
  status: PaymentStatus;
  created_at: string;
  settled_at?: string;
  idempotency_key: string;
}

export interface Invoice {
  id: string;
  account_id: string;
  period: string;
  amount_usd: number;
  runs_billed: number;
  failed_runs_not_billed: number;
  created_at: string;
}

export interface BillingPanel {
  usage: QuotaUsage;
  invoices: Invoice[];
  orders: Order[];
  note: string;
  provenance: Provenance;
}

export interface BoardRow {
  key: string;
  label: string;
  value: number;
  unit: string;
  provenance: Provenance;
}

export interface MonitorAlert {
  metric: string;
  value: number;
  threshold: number;
  message: string;
}

export interface MonitorSnapshot {
  collection_success_rate: number;
  daily_cost_usd: number;
  budget_trips: number;
  payment_failure_rate: number;
  funnel_conversion: number;
  alerts: MonitorAlert[];
  provenance: Provenance;
  at?: string;
}

export interface MonitorResponse {
  snapshot: MonitorSnapshot;
  boards: BoardRow[];
  history: MonitorSnapshot[];
  note: string;
}

export interface ReleaseCheckItem {
  id: string;
  title: string;
  status: "pass" | "fail";
  evidence: string[];
}

export interface ReleaseResponse {
  items: ReleaseCheckItem[];
  all_passed: boolean;
  note: string;
}

export interface LayerReport {
  layer: string;
  name: string;
  metrics: Record<string, number>;
  evidence_ids: string[];
  provenance: Provenance;
}

export interface ChainReport {
  brand_id: string;
  layers: LayerReport[];
  passed: boolean;
  issues: string[];
  usage: QuotaUsage;
}

export interface MCPInfo {
  server: string;
  version: string;
  endpoint: string;
  auth: string;
  tools: string[];
  read_only: boolean;
  methodology: string;
  note: string;
}

// ---- 会话与鉴权 ----

export type UserRole = "user" | "admin";

export interface AuthUser {
  id: string;
  email: string;
  role: UserRole;
  account_id: string;
}

export interface AuthResponse {
  token: string;
  user: AuthUser;
  expires_at?: string;
}