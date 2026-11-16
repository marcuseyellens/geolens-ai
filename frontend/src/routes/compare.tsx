import { T } from "../i18n";
import { Link } from "react-router-dom";
import { CheckCircle2, Minus, ArrowRight, ShieldCheck } from "lucide-react";
import { Card, Badge, Button } from "../ds";

const ROWS: { label: string; geolens: string; manual: string; seo: string }[] = [
  { label: "度量对象", geolens: "AI 生成式回答里的品牌可见性", manual: "人工零散提问的印象", seo: "搜索结果排名与关键词" },
  { label: "原始 AI 回答证据", geolens: "逐条保留，可下钻", manual: "不系统留存", seo: "通常不覆盖" },
  { label: "引用域名 / 页面路径", geolens: "逐条可追溯", manual: "需手工记录", seo: "以外链数据为主" },
  { label: "提及 / 推荐 / 链接拆分", geolens: "分别统计", manual: "难以区分", seo: "不适用" },
  { label: "失败运行标注", geolens: "单独标注，不计分母", manual: "容易混入", seo: "不适用" },
  { label: "免注册试用", geolens: "支持（免费站点检查）", manual: "不适用", seo: "多需注册" },
  { label: "结构化数据 / llms.txt 建议", geolens: "随检查给出", manual: "需自行查资料", seo: "部分覆盖" },
  { label: "起步价格", geolens: "$0 起，$39 一次性审计", manual: "人力成本", seo: "按月订阅" },
];

const FAQ = [
  {
    q: "GeoLens AI 和通用 SEO 工具冲突吗？",
    a: "不冲突，两者度量对象不同。SEO 工具关注搜索结果页的排名与关键词；GeoLens 关注 AI 生成式回答里是否出现你的品牌、位置与引用来源。可以并用。",
  },
  {
    q: "为什么不用「综合分」？",
    a: "因为合并成单一分数会掩盖失败运行、样本偏差与逐模型差异。我们保留逐意图、逐模型的原始指标，每个比例都能下钻到 run_id。",
  },
  {
    q: "检查通过就等于会被 AI 推荐吗？",
    a: "不等于。技术就绪 ≠ 被推荐。站点就绪度只反映公开页面是否可被抓取、理解与引用，不预测推荐结果。",
  },
];

export default function ComparePage() {
  return (
    <T>
    <div className="mx-auto max-w-5xl px-6 py-14">
      <div className="text-center">
        <Badge tone="accent">方案对比</Badge>
        <h1 className="mt-4 text-3xl sm:text-4xl font-bold text-ink tracking-tight">
          和手工自查、通用 SEO 工具有什么不同
        </h1>
        <p className="mx-auto mt-3 max-w-2xl text-sm text-ink-muted leading-relaxed">
          一句话：我们度量的是 AI 回答本身，而不是它之外的代理指标。
        </p>
      </div>

      <div className="mt-10 overflow-x-auto">
        <table className="w-full min-w-[680px] text-sm border-separate border-spacing-0">
          <thead>
            <tr>
              <th className="text-left p-4 font-medium text-ink-muted border-b border-edge w-[170px]">能力</th>
              <th className="p-4 text-center border-b border-edge bg-accent-subtle rounded-t-ds text-ink font-semibold">GeoLens AI</th>
              <th className="p-4 text-center border-b border-edge text-ink font-medium">手工自查</th>
              <th className="p-4 text-center border-b border-edge text-ink font-medium">通用 SEO 工具</th>
            </tr>
          </thead>
          <tbody>
            {ROWS.map((r) => (
              <tr key={r.label}>
                <td className="p-4 border-b border-edge text-ink-muted">{r.label}</td>
                <td className="p-4 border-b border-edge bg-accent-subtle text-center text-ink font-medium">
                  <span className="inline-flex items-center gap-1.5"><CheckCircle2 className="h-3.5 w-3.5 text-positive" />{r.geolens}</span>
                </td>
                <td className="p-4 border-b border-edge text-center text-ink-muted">
                  <span className="inline-flex items-center gap-1.5"><Minus className="h-3.5 w-3.5 text-ink-subtle" />{r.manual}</span>
                </td>
                <td className="p-4 border-b border-edge text-center text-ink-muted">
                  <span className="inline-flex items-center gap-1.5"><Minus className="h-3.5 w-3.5 text-ink-subtle" />{r.seo}</span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="mt-10 grid grid-cols-1 md:grid-cols-2 gap-8">
        <Card>
          <h2 className="text-base font-semibold text-ink">什么时候选 GeoLens AI</h2>
          <ul className="mt-4 space-y-2.5">
            {[
              "你需要知道 AI 回答里到底提了谁、引用了哪个域名",
              "你要把「改动前后」用同题、同口径的 matched comparison 说清楚",
              "你要向下游同事或客户展示可复核的证据，而不是主观分数",
            ].map((t) => (
              <li key={t} className="flex items-start gap-2 text-sm text-ink-muted">
                <CheckCircle2 className="h-4 w-4 mt-0.5 text-positive shrink-0" />{t}
              </li>
            ))}
          </ul>
          <Link to="/check" className="mt-6 inline-block"><Button>免费检查一个网址 <ArrowRight className="h-4 w-4" /></Button></Link>
        </Card>
        <Card>
          <h2 className="text-base font-semibold text-ink">什么时候不必用</h2>
          <ul className="mt-4 space-y-2.5">
            {[
              "你只想看传统搜索排名与外链——那属于 SEO 工具的范围",
              "你期待的是一个保证被 AI 推荐的「开关」——本服务不提供，也不承诺",
              "你的站点尚未公开可访问——先解决可访问性再检查",
            ].map((t) => (
              <li key={t} className="flex items-start gap-2 text-sm text-ink-muted">
                <Minus className="h-4 w-4 mt-0.5 text-ink-subtle shrink-0" />{t}
              </li>
            ))}
          </ul>
          <p className="mt-6 flex items-start gap-2 text-xs text-ink-subtle">
            <ShieldCheck className="h-3.5 w-3.5 mt-0.5 shrink-0" />技术就绪 ≠ 被推荐。
          </p>
        </Card>
      </div>

      <section className="mt-12">
        <h2 className="text-lg font-semibold text-ink">常见问题（FAQ）</h2>
        <div className="mt-4 divide-y divide-edge border-y border-edge">
          {FAQ.map((f) => (
            <div key={f.q} className="py-4">
              <h3 className="text-sm font-medium text-ink">{f.q}</h3>
              <p className="mt-1.5 text-sm text-ink-muted leading-relaxed">{f.a}</p>
            </div>
          ))}
        </div>
      </section>

      <div className="mt-10 text-center">
        <Link to="/pricing" className="inline-flex items-center gap-1.5 text-sm font-medium text-accent hover:underline">
          查看定价与套餐 <ArrowRight className="h-3.5 w-3.5" />
        </Link>
      </div>
    </div>
    </T>
  );
}
