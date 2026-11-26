import { T } from "../i18n";
import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Page, PageHeader, StatCard, Card, EmptyState, Button, Badge, Spinner, Callout } from "../ds";
import { Crosshair, ScanEye, Cpu, Radar, Database, Waypoints, Wallet, Megaphone, Bot } from "lucide-react";
import { api } from "../api";
import type { Brand, ModelInfo } from "../types";
import { MODEL_LABELS } from "../types";

export default function DashboardPage() {
  const [brands, setBrands] = useState<Brand[]>([]);
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const [b, m] = await Promise.all([api.listBrands(), api.listModels()]);
        if (cancelled) return;
        setBrands(b || []);
        setModels(m || []);
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

  // 扩展模块：财务 AI / 付费广告 / Agent 管理不进主导航，仅从此处进入。
  const modules = [
    { path: "/app/finance", label: "财务 AI", desc: "财务健康分区，独立口径", icon: Wallet },
    { path: "/app/ads", label: "付费广告", desc: "广告活动与建议只读", icon: Megaphone },
    { path: "/app/agents", label: "Agent 管理", desc: "Agent 构建与运行回放", icon: Bot },
  ];

  return (
    <T>
    <Page>
      <PageHeader
        title="AEO 工作台"
        description="按品牌可见性 → 竞品监测 → 事实库 → 溯源 的顺序管理你的 AEO 工作"
        actions={
          <Link to="/app/brand/demo">
            <Button>查看品牌可见性</Button>
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
              icon={<Crosshair className="h-5 w-5" />}
              hint="正在追踪的品牌"
            />
            <StatCard
              label="已接入模型"
              value={enabledModels.length}
              icon={<Cpu className="h-5 w-5" />}
              hint={`共 ${models.length} 个可用`}
            />
            <StatCard
              label="检测维度"
              value="5"
              icon={<ScanEye className="h-5 w-5" />}
              hint="位置、情感、竞品等"
            />
            <StatCard
              label="事实条目"
              value="—"
              icon={<Database className="h-5 w-5" />}
              hint="去品牌事实库查看"
            />
          </div>

          <Callout tone="info" className="mb-6">
            本工作台按「提及 / 推荐 / 链接」三态分列展示，不提供综合可见度分；所有数字附有效样本量与口径 1.3。技术就绪 ≠ 被推荐。
          </Callout>

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
              <Link to="/app/brands">
                <Button variant="ghost" size="sm">查看全部</Button>
              </Link>
            </div>
            {brands.length === 0 ? (
              <EmptyState
                title="还没有添加品牌"
                description="添加你的品牌，开始检测在 AI 模型中的推荐表现"
                action={
                  <Link to="/app/brands">
                    <Button>添加品牌</Button>
                  </Link>
                }
              />
            ) : (
              <div className="space-y-3">
                {brands.slice(0, 5).map((brand) => (
                  <Link
                    key={brand.id}
                    to={`/app/brands/${brand.id}`}
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

          <Card className="mt-6">
            <div className="flex items-center gap-2 mb-4">
              <Waypoints className="h-4 w-4 text-accent" />
              <h2 className="text-base font-medium text-ink">AEO 核心链路</h2>
            </div>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              {[
                { path: "/app/brand/demo", label: "品牌可见性", desc: "逐模型三态", icon: Crosshair },
                { path: "/app/competitors", label: "竞品监测", desc: "对手被引用处", icon: Radar },
                { path: "/app/facts", label: "品牌事实库", desc: "可核验事实", icon: Database },
                { path: "/app/attribution", label: "溯源数据", desc: "引用出处", icon: ScanEye },
              ].map((m) => (
                <Link key={m.path} to={m.path} className="flex items-start gap-2 p-3 rounded-ds border border-edge hover:border-edge-strong transition-colors">
                  <m.icon className="h-4 w-4 text-accent shrink-0 mt-0.5" />
                  <div>
                    <p className="text-sm font-medium text-ink">{m.label}</p>
                    <p className="text-xs text-ink-muted mt-0.5">{m.desc}</p>
                  </div>
                </Link>
              ))}
            </div>
          </Card>

          <Card className="mt-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-base font-medium text-ink">扩展模块</h2>
              <span className="text-xs text-ink-subtle">独立于 AEO 核心链路</span>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
              {modules.map((m) => (
                <Link key={m.path} to={m.path} className="flex items-start gap-2 p-3 rounded-ds border border-edge hover:border-edge-strong transition-colors">
                  <m.icon className="h-4 w-4 text-accent shrink-0 mt-0.5" />
                  <div>
                    <p className="text-sm font-medium text-ink">{m.label}</p>
                    <p className="text-xs text-ink-muted mt-0.5">{m.desc}</p>
                  </div>
                </Link>
              ))}
            </div>
          </Card>
        </>
      )}
    </Page>
    </T>
  );
}
