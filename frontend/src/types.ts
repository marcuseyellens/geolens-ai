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

export type CheckStatus = "pass" | "partial" | "fail";
export type AuditGroup = "crawlability" | "structured_data" | "content_signals" | "basic_health";

export interface AuditCheck {
  id: string;
  group: AuditGroup;
  label: string;
  status: CheckStatus;
  evidence: string;
  fix: string;
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