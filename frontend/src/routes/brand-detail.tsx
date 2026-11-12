import { useEffect, useState } from "react";
import { useParams, Link } from "react-router-dom";
import {
  Page, PageHeader, PageBackLink, Card, StatCard, Button, Select, TextArea, Badge,
  EmptyState, Spinner, Callout, Table,
} from "../ds";
import {
  Plus, Play, Trash2, TrendingUp, Target, Eye, ThumbsUp, FileBarChart,
} from "lucide-react";
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  RadarChart, PolarGrid, PolarAngleAxis, PolarRadiusAxis, Radar, Legend,
} from "recharts";
import { api } from "../api";
import type { Brand, DetectionQuery, DetectionResult, ReportSummary, QueryIntent } from "../types";
import {
  MODEL_LABELS, POSITION_LABELS, SENTIMENT_LABELS, INTENT_LABELS,
} from "../types";

const SUGGESTED_QUERIES: { text: string; intent: QueryIntent }[] = [
  { text: "What are the best tools for project management?", intent: "best" },
  { text: "Can you recommend a note-taking app?", intent: "recommend" },
  { text: "What's a good alternative to {competitor}?", intent: "alternative" },
  { text: "How do I choose between project management tools?", intent: "howto" },
  { text: "Compare the top project management tools", intent: "compare" },
];

export default function BrandDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [brand, setBrand] = useState<Brand | null>(null);
  const [queries, setQueries] = useState<DetectionQuery[]>([]);
  const [results, setResults] = useState<DetectionResult[]>([]);
  const [report, setReport] = useState<ReportSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [detecting, setDetecting] = useState<string | null>(null);
  const [showQueryForm, setShowQueryForm] = useState(false);
  const [queryForm, setQueryForm] = useState({ text: "", intent: "recommend" as QueryIntent });

  const load = async () => {
    if (!id) return;
    try {
      const [b, q, r, rep] = await Promise.all([
        api.getBrand(id),
        api.listQueries(id),
        api.listResults(id),
        api.getReport(id),
      ]);
      setBrand(b);
      setQueries(q || []);
      setResults(r || []);
      setReport(rep);
    } catch {
      // 保持页面可用，由空状态兜底
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, [id]);

  const addQuery = () => {
    if (!id || !queryForm.text) return;
    api.createQuery(id, { text: queryForm.text, intent: queryForm.intent }).then(() => {
      setQueryForm({ text: "", intent: "recommend" });
      setShowQueryForm(false);
      load();
    });
  };

  const runDetect = (queryId: string) => {
    if (!id) return;
    setDetecting(queryId);
    api
      .runDetection(id, queryId)
      .then(() => load())
      .catch((e) => alert(`检测失败: ${e.message}`))
      .finally(() => setDetecting(null));
  };

  const deleteQuery = (queryId: string) => {
    if (!id || !confirm("确认删除此查询？")) return;
    api.deleteQuery(id, queryId).then(load);
  };

  const generateSuggested = async () => {
    if (!id || !brand) return;
    const comp = brand.competitors?.[0] || "competitors";
    await Promise.all(
      SUGGESTED_QUERIES.map((q) =>
        api.createQuery(id, {
          text: q.text.replace("{competitor}", comp),
          intent: q.intent,
        }),
      ),
    );
    await load();
  };

  if (loading) {
    return (
      <Page>
        <div className="flex justify-center py-12">
          <Spinner />
        </div>
      </Page>
    );
  }

  if (!brand) {
    return (
      <Page>
        <EmptyState title="品牌不存在" />
      </Page>
    );
  }

  const modelData = report?.model_breakdown
    ? Object.entries(report.model_breakdown).map(([model, stat]) => ({
        model: MODEL_LABELS[model as keyof typeof MODEL_LABELS] || model,
        mentionRate: stat.mention_rate,
        score: stat.avg_score,
        total: stat.total,
      }))
    : [];

  const radarData = modelData.map((d) => ({
    model: d.model,
    score: d.score,
    mentionRate: d.mentionRate,
  }));

  const recentResults = results.slice(0, 10);

  return (
    <Page>
      <PageHeader
        title={brand.name}
        description={brand.description || brand.website}
        back={<PageBackLink to="/brands">返回品牌列表</PageBackLink>}
        actions={
          <>
            <Button onClick={() => setShowQueryForm(!showQueryForm)}>
              <Plus className="h-4 w-4" />
              添加检测问题
            </Button>
            <Button variant="primary">
              <Link to={`/brands/${brand.id}/diagnostic`} className="flex items-center">
                <FileBarChart className="h-4 w-4 mr-2" />
                诊断报告
              </Link>
            </Button>
          </>
        }
      />

      {/* Stats Overview */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <StatCard
          label="总检测次数"
          value={report?.total_detections || 0}
          icon={<Target className="h-5 w-5" />}
        />
        <StatCard
          label="品牌提及率"
          value={`${(report?.mention_rate || 0).toFixed(1)}%`}
          icon={<Eye className="h-5 w-5" />}
          hint="在 AI 回答中被提及的比例"
        />
        <StatCard
          label="平均 AEO 得分"
          value={(report?.avg_score || 0).toFixed(1)}
          icon={<TrendingUp className="h-5 w-5" />}
          hint="综合位置和情感"
        />
        <StatCard
          label="平均位置权重"
          value={(report?.avg_position || 0).toFixed(1)}
          icon={<ThumbsUp className="h-5 w-5" />}
          hint="越高越靠前"
        />
      </div>

      {/* Charts */}
      {modelData.length > 0 && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-6">
          <Card>
            <h3 className="text-base font-medium text-ink mb-4">各模型提及率对比</h3>
            <ResponsiveContainer width="100%" height={260}>
              <BarChart data={modelData}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--ds-edge)" />
                <XAxis dataKey="model" tick={{ fill: "var(--ds-ink-muted)", fontSize: 12 }} />
                <YAxis tick={{ fill: "var(--ds-ink-muted)", fontSize: 12 }} unit="%" />
                <Tooltip
                  contentStyle={{
                    backgroundColor: "var(--ds-surface)",
                    border: "1px solid var(--ds-edge)",
                    borderRadius: "var(--ds-radius)",
                  }}
                />
                <Bar dataKey="mentionRate" fill="var(--ds-accent)" name="提及率%" radius={[4, 4, 0, 0]} isAnimationActive={false} />
              </BarChart>
            </ResponsiveContainer>
          </Card>

          <Card>
            <h3 className="text-base font-medium text-ink mb-4">AEO 得分雷达图</h3>
            <ResponsiveContainer width="100%" height={260}>
              <RadarChart data={radarData}>
                <PolarGrid stroke="var(--ds-edge)" />
                <PolarAngleAxis dataKey="model" tick={{ fill: "var(--ds-ink-muted)", fontSize: 12 }} />
                <PolarRadiusAxis angle={90} domain={[0, 100]} tick={{ fill: "var(--ds-ink-subtle)", fontSize: 10 }} />
                <Radar name="AEO 得分" dataKey="score" stroke="var(--ds-accent)" fill="var(--ds-accent)" fillOpacity={0.3} isAnimationActive={false} />
                <Legend wrapperStyle={{ fontSize: 12 }} />
              </RadarChart>
            </ResponsiveContainer>
          </Card>
        </div>
      )}

      {/* Query Form */}
      {showQueryForm && (
        <Card className="mb-6">
          <div className="space-y-4">
            <TextArea
              label="检测问题"
              placeholder="如：What are the best tools for ...?"
              value={queryForm.text}
              onChange={(e) => setQueryForm({ ...queryForm, text: e.target.value })}
            />
            <Select
              label="问题类型"
              className="max-w-[260px]"
              value={queryForm.intent}
              onChange={(e) => setQueryForm({ ...queryForm, intent: e.target.value as QueryIntent })}
              options={Object.entries(INTENT_LABELS).map(([value, label]) => ({ value, label }))}
            />
            <div className="flex gap-2">
              <Button onClick={addQuery} disabled={!queryForm.text}>保存</Button>
              <Button variant="ghost" onClick={() => setShowQueryForm(false)}>取消</Button>
            </div>
          </div>
        </Card>
      )}

      {/* Detection Queries */}
      <Card className="mb-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-base font-medium text-ink">检测问题</h3>
          {queries.length === 0 && (
            <Button variant="ghost" size="sm" onClick={generateSuggested}>
              生成推荐问题
            </Button>
          )}
        </div>

        {queries.length === 0 ? (
          <EmptyState
            title="还没有检测问题"
            description="添加问题或生成推荐问题，然后运行检测"
          />
        ) : (
          <div className="space-y-2">
            {queries.map((q) => (
              <div
                key={q.id}
                className="flex items-center justify-between p-3 rounded-ds border border-edge hover:border-edge-strong"
              >
                <div className="flex-1 min-w-0">
                  <p className="text-sm text-ink truncate">{q.text}</p>
                  <div className="mt-1 flex items-center gap-2">
                    <Badge tone="accent">{INTENT_LABELS[q.intent]}</Badge>
                    <span className="text-xs text-ink-subtle">
                      {new Date(q.created_at).toLocaleDateString()}
                    </span>
                  </div>
                </div>
                <div className="flex items-center gap-2 ml-4">
                  <Button
                    size="sm"
                    onClick={() => runDetect(q.id)}
                    loading={detecting === q.id}
                  >
                    <Play className="h-3.5 w-3.5" />
                    运行检测
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => deleteQuery(q.id)}>
                    <Trash2 className="h-3.5 w-3.5 text-negative" />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>

      {/* Recent Results */}
      <Card>
        <h3 className="text-base font-medium text-ink mb-4">最近检测结果</h3>
        {recentResults.length === 0 ? (
          <EmptyState title="还没有检测结果" description="运行检测后结果将显示在这里" />
        ) : (
          <Table
            columns={[
              { key: "model", label: "模型" },
              { key: "mentioned", label: "被提及" },
              { key: "position", label: "位置" },
              { key: "sentiment", label: "情感" },
              { key: "score", label: "得分" },
              { key: "competitors", label: "竞品提及" },
              { key: "time", label: "时间" },
            ]}
            rows={recentResults.map((r) => ({
              model: <Badge tone="neutral">{MODEL_LABELS[r.model]}</Badge>,
              mentioned: r.brand_mentioned ? (
                <Badge tone="positive">是</Badge>
              ) : (
                <Badge tone="negative">否</Badge>
              ),
              position: <span className="text-sm">{POSITION_LABELS[r.mention_position]}</span>,
              sentiment: (
                <Badge
                  tone={
                    r.sentiment === "positive" ? "positive" :
                    r.sentiment === "negative" ? "negative" : "neutral"
                  }
                >
                  {SENTIMENT_LABELS[r.sentiment]}
                </Badge>
              ),
              score: (
                <span className="font-medium text-ink">{r.score.toFixed(1)}</span>
              ),
              competitors: (
                <div className="flex gap-1 flex-wrap">
                  {r.competitors_mentioned?.map((c) => (
                    <Badge key={c} tone="warn">{c}</Badge>
                  ))}
                </div>
              ),
              time: (
                <span className="text-xs text-ink-subtle">
                  {new Date(r.detected_at).toLocaleString()}
                </span>
              ),
            }))}
          />
        )}
      </Card>

      {/* AI Response Detail */}
      {recentResults.length > 0 && recentResults[0].response_text && (
        <Card className="mt-6">
          <h3 className="text-base font-medium text-ink mb-3">
            最新 AI 回答 — {MODEL_LABELS[recentResults[0].model]}
          </h3>
          <Callout
            tone={recentResults[0].brand_mentioned ? "positive" : "negative"}
            title={recentResults[0].brand_mentioned ? "品牌被提及" : "品牌未被提及"}
          >
            位置：{POSITION_LABELS[recentResults[0].mention_position]} ·
            情感：{SENTIMENT_LABELS[recentResults[0].sentiment]} ·
            得分：{recentResults[0].score.toFixed(1)}
          </Callout>
          <div className="mt-3 p-4 rounded-ds bg-canvas text-sm text-ink-muted max-h-60 overflow-y-auto whitespace-pre-wrap">
            {recentResults[0].response_text}
          </div>
        </Card>
      )}
    </Page>
  );
}
