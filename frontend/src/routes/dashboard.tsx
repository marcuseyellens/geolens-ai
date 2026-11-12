import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Page, PageHeader, StatCard, Card, EmptyState, Button, Badge, Spinner, Callout } from "../ds";
import { Target, TrendingUp, Eye, Zap } from "lucide-react";
import { api } from "../api";
import type { Brand, ModelInfo } from "../types";
import { MODEL_LABELS } from "../types";

export default function DashboardPage() {
  const [brands, setBrands] = useState<Brand[]>([]);
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [score, setScore] = useState<{ value: number; count: number } | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const [b, m] = await Promise.all([api.listBrands(), api.listModels()]);
        if (cancelled) return;
        setBrands(b || []);
        setModels(m || []);

        const reports = await Promise.all(
          (b || []).map((brand) => api.getReport(brand.id).catch(() => null)),
        );
        if (cancelled) return;
        let weighted = 0;
        let count = 0;
        for (const report of reports) {
          const n = report?.total_detections ?? 0;
          if (n > 0) {
            weighted += (report?.avg_score ?? 0) * n;
            count += n;
          }
        }
        setScore(count > 0 ? { value: weighted / count, count } : null);
      } catch {
        // 加载失败时保留空状态
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, []);

  const enabledModels = models.filter((m) => m.enabled);
  const totalBrands = brands.length;

  return (
    <Page>
      <PageHeader
        title="概览"
        description="监测你的品牌在各大 AI 模型中的推荐表现"
        actions={
          <Link to="/brands">
            <Button>管理品牌</Button>
          </Link>
        }
      />

      {loading ? (
        <div className="flex justify-center py-12">
          <Spinner />
        </div>
      ) : (
        <>
          {models.some((m) => m.demo) && (
            <Callout tone="warn" title="演示模式" className="mb-6">
              未配置 AI 模型 API Key，检测结果由模拟回答生成，可用于体验完整的报告与图表流程。配置 Key 后重启服务即自动切换为真实检测。
            </Callout>
          )}
          <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
            <StatCard
              label="品牌数"
              value={totalBrands}
              icon={<Target className="h-5 w-5" />}
              hint="正在追踪的品牌"
            />
            <StatCard
              label="已接入模型"
              value={enabledModels.length}
              icon={<Zap className="h-5 w-5" />}
              hint={`共 ${models.length} 个可用`}
            />
            <StatCard
              label="检测维度"
              value="5"
              icon={<Eye className="h-5 w-5" />}
              hint="位置、情感、竞品等"
            />
            <StatCard
              label="AEO 评分"
              value={score ? Math.round(score.value) : "—"}
              icon={<TrendingUp className="h-5 w-5" />}
              hint={score ? `基于 ${score.count} 次检测` : "需检测后生成"}
            />
          </div>

          <Card className="mb-6">
            <h2 className="text-base font-medium text-ink mb-4">已接入的 AI 模型</h2>
            <div className="flex flex-wrap gap-3">
              {models.map((m) => (
                <div
                  key={m.id}
                  className="flex items-center gap-2 px-3 py-2 rounded-ds border border-edge bg-canvas"
                >
                  <span className="text-sm font-medium text-ink">
                    {MODEL_LABELS[m.id] || m.name}
                  </span>
                  <Badge tone={m.enabled ? "positive" : "neutral"}>
                    {m.enabled ? "已接入" : "未配置"}
                  </Badge>
                </div>
              ))}
            </div>
          </Card>

          <Card>
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-base font-medium text-ink">品牌列表</h2>
              <Link to="/brands">
                <Button variant="ghost" size="sm">查看全部</Button>
              </Link>
            </div>
            {brands.length === 0 ? (
              <EmptyState
                title="还没有添加品牌"
                description="添加你的品牌，开始检测在 AI 模型中的推荐表现"
                action={
                  <Link to="/brands">
                    <Button>添加品牌</Button>
                  </Link>
                }
              />
            ) : (
              <div className="space-y-3">
                {brands.slice(0, 5).map((brand) => (
                  <Link
                    key={brand.id}
                    to={`/brands/${brand.id}`}
                    className="flex items-center justify-between p-3 rounded-ds border border-edge hover:border-edge-strong transition-colors"
                  >
                    <div>
                      <p className="text-sm font-medium text-ink">{brand.name}</p>
                      <p className="text-xs text-ink-muted">
                        {brand.website || "无网站"}
                        {brand.competitors?.length > 0 &&
                          ` · ${brand.competitors.length} 个竞品`}
                      </p>
                    </div>
                    <Badge tone="accent">{brand.keywords?.length || 0} 关键词</Badge>
                  </Link>
                ))}
              </div>
            )}
          </Card>
        </>
      )}
    </Page>
  );
}
