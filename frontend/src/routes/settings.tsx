import { useEffect, useState } from "react";
import { Page, PageHeader, Card, Badge, Callout } from "../ds";
import { api } from "../api";
import type { ModelInfo } from "../types";
import { MODEL_LABELS } from "../types";

const MODEL_ENV_KEYS: Record<string, string> = {
  chatgpt: "OPENAI_API_KEY",
  claude: "ANTHROPIC_API_KEY",
  gemini: "GEMINI_API_KEY",
  perplexity: "PERPLEXITY_API_KEY",
};

const MODEL_GUIDES: Record<string, string> = {
  chatgpt: "https://platform.openai.com/api-keys",
  claude: "https://console.anthropic.com/settings/keys",
  gemini: "https://aistudio.google.com/apikey",
  perplexity: "https://www.perplexity.ai/settings/api",
};

export default function SettingsPage() {
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.listModels().then(setModels).catch(() => {}).finally(() => setLoading(false));
  }, []);

  return (
    <Page measure="narrow">
      <PageHeader title="设置" description="管理 AI 模型接入和配置" />

      {models.some((m) => m.demo) && (
        <Callout tone="warn" title="当前为演示模式" className="mb-4">
          未检测到任何 AI 模型 API Key，系统正在使用模拟回答生成检测结果，以便完整体验报告与图表。配置 API Key 并重启服务后即自动切换为真实检测。
        </Callout>
      )}

      <Callout tone="info" title="如何配置 API Key" className="mb-6">
        在启动后端服务时，通过环境变量设置各模型的 API Key。例如：
        <br />
        <code className="text-xs bg-canvas px-2 py-0.5 rounded mt-1 inline-block">
          OPENAI_API_KEY=sk-... ./geolens
        </code>
      </Callout>

      <Card>
        <h3 className="text-base font-medium text-ink mb-4">AI 模型状态</h3>
        {loading ? (
          <p className="text-sm text-ink-muted">加载中...</p>
        ) : (
          <div className="space-y-3">
            {models.map((m) => (
              <div
                key={m.id}
                className="flex items-center justify-between p-3 rounded-ds border border-edge"
              >
                <div>
                  <p className="text-sm font-medium text-ink">
                    {MODEL_LABELS[m.id] || m.name}
                  </p>
                  <p className="text-xs text-ink-subtle mt-0.5">
                    环境变量：{MODEL_ENV_KEYS[m.id]}
                  </p>
                </div>
                <div className="flex items-center gap-3">
                  <Badge tone={m.enabled ? "positive" : "neutral"}>
                    {m.enabled ? "已配置" : "未配置"}
                  </Badge>
                  <a
                    href={MODEL_GUIDES[m.id]}
                    target="_blank"
                    rel="noopener"
                    className="text-xs text-accent hover:underline"
                  >
                    获取 Key
                  </a>
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>

      <Card className="mt-6">
        <h3 className="text-base font-medium text-ink mb-3">关于 GeoLens AI</h3>
        <p className="text-sm text-ink-muted">
          GeoLens AI 是一个 AEO/GEO（AI Engine Optimization）平台，帮助你监测和优化品牌在大语言模型中的推荐表现。
        </p>
        <p className="text-sm text-ink-muted mt-2">
          检测维度包括：品牌是否被提及、提及位置、情感倾向、竞品提及情况，并生成综合 AEO 得分。
        </p>
      </Card>
    </Page>
  );
}
