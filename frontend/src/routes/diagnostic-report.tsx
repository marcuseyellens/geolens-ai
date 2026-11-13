import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { 
  Card, Badge, Spinner, Callout, Table, Button 
} from "../ds";
import { 
  AlertTriangle, Globe, Users, Link as LinkIcon, Copy, Download,
  Eye, X, ExternalLink, ChevronRight, Search
} from "lucide-react";
import { 
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  PieChart, Pie, Cell
} from "recharts";
import { api } from "../api";
import type { 
  DiagnosticReport, FixItem, FixArtifact, GroupScore, RunAnswer
} from "../types";

export default function DiagnosticReportPage() {
  const { id } = useParams<{ id: string }>();
  const [report, setReport] = useState<DiagnosticReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [evidenceOpen, setEvidenceOpen] = useState(false);

  useEffect(() => {
    if (!id) return;
    
    const loadReport = async () => {
      try {
        setLoading(true);
        const data = await api.getDiagnosticReport(id);
        setReport(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : "加载报告失败");
      } finally {
        setLoading(false);
      }
    };

    loadReport();
  }, [id]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Spinner className="w-8 h-8" />
        <span className="ml-3">正在生成诊断报告...</span>
      </div>
    );
  }

  if (error) {
    return (
      <Callout tone="negative">
        <h3 className="font-medium">加载失败</h3>
        <p>{error}</p>
      </Callout>
    );
  }

  if (!report) {
    return (
      <Callout tone="info">
        <h3 className="font-medium">暂无数据</h3>
        <p>该品牌还没有生成诊断报告</p>
      </Callout>
    );
  }

  return (
    <div className="space-y-6">
      {/* 头部信息 */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-ink">AI 可见性诊断报告</h1>
          <p className="text-ink-muted mt-1">
            品牌：{report.brand_name} | 生成时间：{new Date(report.generated_at).toLocaleString()}
          </p>
        </div>
        <Badge>{report.methodology_version}</Badge>
      </div>

      {/* 执行摘要（F-501.1） */}
      <Card raised>
        <h2 className="text-lg font-semibold mb-4">执行摘要</h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <SummaryBlock label="现状" text={report.executive_summary.current} tone="accent" />
          <SummaryBlock label="差距" text={report.executive_summary.gap} tone="warn" />
          <SummaryBlock label="下一步" text={report.executive_summary.next} tone="positive" />
        </div>
      </Card>

      {/* 方法论说明 */}
      <Card>
        <h2 className="text-lg font-semibold mb-3">方法论说明</h2>
        <ul className="space-y-2 text-sm text-ink-muted">
          {report.methodology_notes.map((note, i) => (
            <li key={i} className="flex items-start">
              <span className="inline-block w-1.5 h-1.5 rounded-full bg-accent mt-2 mr-2 flex-shrink-0"></span>
              {note}
            </li>
          ))}
        </ul>
      </Card>

      {/* 核心指标概览 */}
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">核心指标概览</h2>
        <Button variant="secondary" size="sm" onClick={() => setEvidenceOpen(true)}>
          <Eye className="h-4 w-4" />
          看证据
        </Button>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <div className="flex items-center justify-between">
            <p className="text-sm text-ink-muted">Buyer Question 提及率</p>
          </div>
          <p className="mt-2 text-2xl font-semibold text-ink">
            {(report.ai_visibility.mention_rate * 100).toFixed(1)}%
          </p>
          <p className="mt-1 text-xs text-ink-subtle">
            {report.ai_visibility.mentioned}/{report.ai_visibility.usable} 有效答案
          </p>
        </Card>
        <Card>
          <div className="flex items-center justify-between">
            <p className="text-sm text-ink-muted">推荐率</p>
          </div>
          <p className="mt-2 text-2xl font-semibold text-ink">
            {(report.ai_visibility.recommend_rate * 100).toFixed(1)}%
          </p>
          <p className="mt-1 text-xs text-ink-subtle">
            {report.ai_visibility.recommended}/{report.ai_visibility.usable} 有效答案
          </p>
        </Card>
        <Card>
          <div className="flex items-center justify-between">
            <p className="text-sm text-ink-muted">链接率</p>
          </div>
          <p className="mt-2 text-2xl font-semibold text-ink">
            {(report.ai_visibility.link_rate * 100).toFixed(1)}%
          </p>
          <p className="mt-1 text-xs text-ink-subtle">
            {report.ai_visibility.linked}/{report.ai_visibility.usable} 有效答案
          </p>
        </Card>
        <Card>
          <div className="flex items-center justify-between">
            <p className="text-sm text-ink-muted">引用率</p>
          </div>
          <p className="mt-2 text-2xl font-semibold text-ink">
            {(report.ai_visibility.citation_rate * 100).toFixed(1)}%
          </p>
          <p className="mt-1 text-xs text-ink-subtle">
            {(report.ai_visibility.citation_rate * report.ai_visibility.usable).toFixed(0)}/{report.ai_visibility.usable} 有效答案
          </p>
        </Card>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* 站点就绪度 */}
        {report.site_readiness && (
          <Card>
            <h2 className="text-lg font-semibold mb-4 flex items-center">
              <Globe className="w-5 h-5 mr-2" />
              站点就绪度 /100
            </h2>
            
            <div className="mb-4">
              <div className="text-3xl font-bold">{report.site_readiness.score.toFixed(1)}</div>
              <div className="text-sm text-ink-muted mt-1">{report.site_readiness.caveat}</div>
            </div>
            
            <div className="space-y-3">
              {report.site_readiness.groups.map((group: GroupScore) => (
                <div key={group.group}>
                  <div className="flex justify-between text-sm mb-1">
                    <span>{group.label}</span>
                    <span>{group.earned.toFixed(1)}/{group.weight}</span>
                  </div>
                  <div className="w-full bg-canvas rounded-full h-2">
                    <div 
                      className="bg-accent h-2 rounded-full" 
                      style={{ width: `${(group.earned / group.weight) * 100}%` }}
                    ></div>
                  </div>
                </div>
              ))}
            </div>
            
            {report.site_readiness.top_fix && (
              <div className="mt-4 p-3 bg-warning-subtle rounded-ds">
                <h3 className="font-medium text-warning flex items-center">
                  <AlertTriangle className="w-4 h-4 mr-1" />
                  首要修复建议
                </h3>
                <p className="text-sm mt-1">{report.site_readiness.top_fix}</p>
              </div>
            )}
          </Card>
        )}

        {/* Share of Voice */}
        <Card>
          <h2 className="text-lg font-semibold mb-4 flex items-center">
            <Users className="w-5 h-5 mr-2" />
            Share of Voice
          </h2>
          
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={report.competitor_sov}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="brand" />
                <YAxis />
                <Tooltip />
                <Bar dataKey="mentions" fill="#3b82f6" />
              </BarChart>
            </ResponsiveContainer>
          </div>
          
          <Table 
            columns={[
              { key: "brand", label: "品牌" },
              { key: "mentions", label: "提及次数" },
              { key: "share", label: "占比 %" },
            ]}
            rows={report.competitor_sov.map(row => ({
              ...row,
              share: (row.share * 100).toFixed(1)
            }))}
          />
        </Card>
      </div>

      {/* 按意图分类的指标 */}
      <Card>
        <h2 className="text-lg font-semibold mb-4">按提问意图分类</h2>
        
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {Object.entries(report.buyer_questions).map(([intent, metrics]) => (
            <div key={intent} className="border border-edge rounded-ds p-4">
              <h3 className="font-medium capitalize mb-2">{intent}</h3>
              <div className="space-y-2 text-sm">
                <div className="flex justify-between">
                  <span>提及率</span>
                  <span className="font-medium">{(metrics.mention_rate * 100).toFixed(1)}%</span>
                </div>
                <div className="flex justify-between">
                  <span>推荐率</span>
                  <span className="font-medium">{(metrics.recommend_rate * 100).toFixed(1)}%</span>
                </div>
                <div className="flex justify-between">
                  <span>有效答案</span>
                  <span className="font-medium">{metrics.usable}</span>
                </div>
              </div>
            </div>
          ))}
        </div>
      </Card>

      {/* 引用地图 */}
      <Card>
        <h2 className="text-lg font-semibold mb-4 flex items-center">
          <LinkIcon className="w-5 h-5 mr-2" />
          引用地图
        </h2>
        
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* 图表 */}
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={report.citation_map}
                  cx="50%"
                  cy="50%"
                  labelLine={false}
                  outerRadius={80}
                  fill="#8884d8"
                  dataKey="count"
                  nameKey="source_class"
                  label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
                >
                  {report.citation_map.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={getColor(index)} />
                  ))}
                </Pie>
                <Tooltip />
              </PieChart>
            </ResponsiveContainer>
          </div>
          
          {/* 列表 */}
          <div className="space-y-4">
            {report.citation_map.map((group) => (
              <div key={group.source_class} className="border-b border-edge pb-3 last:border-0 last:pb-0">
                <h3 className="font-medium capitalize mb-1">{group.source_class.replace('_', ' ')}</h3>
                <div className="text-sm text-ink-muted mb-2">
                  引用 {group.count} 次，来自 {group.domains.length} 个域名
                </div>
                <div className="flex flex-wrap gap-1">
                  {group.domains.slice(0, 5).map((domain, i) => (
                    <Badge key={i} tone="accent">{domain}</Badge>
                  ))}
                  {group.domains.length > 5 && (
                    <Badge tone="accent">+{group.domains.length - 5} 更多</Badge>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      </Card>

      {/* 名称混淆 */}
      {report.name_confusion.length > 0 && (
        <Card>
          <h2 className="text-lg font-semibold mb-4">名称混淆</h2>
          
          <Table 
            columns={[
              { key: "confused_with", label: "混淆对象" },
              { key: "excerpt", label: "上下文" },
            ]}
            rows={report.name_confusion.map(item => ({
              confused_with: item.confused_with,
              excerpt: item.excerpt
            }))}
          />
        </Card>
      )}

      {/* 修复建议清单 */}
      <Card>
        <h2 className="text-lg font-semibold mb-4">修复建议清单</h2>
        
        <div className="space-y-4">
          {report.fix_list.map((item: FixItem, index) => (
            <div key={index} className="border border-edge rounded-ds p-4">
              <div className="flex items-start justify-between">
                <div className="flex items-center">
                  <Badge tone="accent">
                    {item.priority}
                  </Badge>
                  <h3 className="font-medium ml-2">{item.problem}</h3>
                </div>
                <div className="text-sm text-ink-muted">{item.target_metric}</div>
              </div>
              
              <p className="mt-2 text-sm">{item.change}</p>
              
              <div className="mt-3 flex flex-wrap gap-2">
                {item.artifact_types.map((type, i) => (
                  <Badge key={i} tone="accent">
                    {type}
                  </Badge>
                ))}
              </div>
            </div>
          ))}
        </div>
      </Card>

      {/* 成品文件 */}
      <Card>
        <h2 className="text-lg font-semibold mb-4">成品文件</h2>
        
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {report.artifacts.map((artifact: FixArtifact, index) => (
            <div key={index} className="border border-edge rounded-ds p-4">
              <div className="flex items-center justify-between">
                <h3 className="font-medium">{artifact.filename}</h3>
                <Badge tone="accent">{artifact.type}</Badge>
              </div>
              
              <p className="text-sm text-ink-muted mt-2">{artifact.target_path}</p>
              
              <div className="mt-3 flex gap-2">
                <button className="text-xs flex items-center text-accent hover:underline">
                  <Copy className="w-3 h-3 mr-1" />
                  复制
                </button>
                <button className="text-xs flex items-center text-accent hover:underline">
                  <Download className="w-3 h-3 mr-1" />
                  下载
                </button>
              </div>
              
              <details className="mt-3 text-xs">
                <summary className="cursor-pointer text-ink-muted">验证说明</summary>
                <p className="mt-1 text-ink-muted">{artifact.validation}</p>
              </details>
            </div>
          ))}
        </div>
      </Card>

      {/* 证据下钻抽屉（F-304 / DoD-4） */}
      {evidenceOpen && (
        <EvidenceDrawer
          evidence={report.evidence}
          brandName={report.brand_name}
          onClose={() => setEvidenceOpen(false)}
        />
      )}
    </div>
  );
}

// 执行摘要三段展示
function SummaryBlock({
  label,
  text,
  tone,
}: {
  label: string;
  text: string;
  tone: "accent" | "warn" | "positive";
}) {
  return (
    <div className="rounded-ds border border-edge p-4">
      <Badge tone={tone}>{label}</Badge>
      <p className="mt-2 text-sm text-ink">{text}</p>
    </div>
  );
}

// 证据下钻抽屉：两次点击可达原文（第一次打开抽屉，第二次打开 citation 链接）
function EvidenceDrawer({
  evidence,
  brandName,
  onClose,
}: {
  evidence: RunAnswer[];
  brandName: string;
  onClose: () => void;
}) {
  const [query, setQuery] = useState("");
  const filtered = evidence.filter((e) => {
    if (!query.trim()) return true;
    const q = query.toLowerCase();
    return (
      e.question_text.toLowerCase().includes(q) ||
      e.platform.toLowerCase().includes(q) ||
      e.intent.toLowerCase().includes(q)
    );
  });

  return (
    <div className="fixed inset-0 z-50 flex justify-end">
      <div className="absolute inset-0 bg-black/30" onClick={onClose} />
      <div className="relative w-full max-w-2xl bg-surface h-full shadow-xl flex flex-col">
        <div className="flex items-center justify-between px-6 py-4 border-b border-edge">
          <div>
            <h2 className="font-semibold text-ink">证据下钻</h2>
            <p className="text-xs text-ink-muted mt-0.5">
              {brandName} · {evidence.length} 条检测证据
            </p>
          </div>
          <button onClick={onClose} className="text-ink-muted hover:text-ink" aria-label="关闭">
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="px-6 py-3 border-b border-edge">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-ink-subtle" />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="按问题、模型或意图筛选"
              className="w-full pl-9 pr-3 py-2 text-sm rounded-ds bg-canvas border border-edge text-ink focus:outline-none focus:border-accent"
            />
          </div>
        </div>

        <div className="flex-1 overflow-y-auto px-6 py-4 space-y-4">
          {filtered.length === 0 && (
            <p className="text-sm text-ink-muted text-center py-8">无匹配证据</p>
          )}
          {filtered.map((e) => (
            <div key={e.run_id} className="rounded-ds border border-edge p-4">
              <div className="flex items-center gap-2 flex-wrap">
                <Badge tone="accent">{e.platform}</Badge>
                <Badge tone={e.mention.mentioned ? "positive" : "negative"}>
                  {e.mention.mentioned ? "已提及" : "未提及"}
                </Badge>
                {e.mention.recommended && <Badge tone="positive">推荐</Badge>}
                {e.mention.linked && <Badge tone="positive">已链接</Badge>}
                <span className="text-xs text-ink-subtle">{e.intent}</span>
              </div>
              <p className="mt-2 text-sm font-medium text-ink">{e.question_text}</p>
              <p className="mt-2 text-xs text-ink-muted leading-relaxed line-clamp-4">
                {e.raw_answer_text}
              </p>
              {e.citations.length > 0 && (
                <div className="mt-3 space-y-1">
                  {e.citations.map((c, i) => (
                    <a
                      key={i}
                      href={c.url}
                      target="_blank"
                      rel="noreferrer"
                      className="flex items-center gap-1.5 text-xs text-accent hover:underline"
                    >
                      <ExternalLink className="h-3 w-3 shrink-0" />
                      <span className="truncate">{c.domain}{c.page_path}</span>
                      {c.direct_citation && (
                        <Badge tone="positive">品牌链接</Badge>
                      )}
                    </a>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

// 辅助函数
function getColor(index: number): string {
  const COLORS = ["#0088FE", "#00C49F", "#FFBB28", "#FF8042", "#8884D8", "#82ca9d"];
  return COLORS[index % COLORS.length];
}