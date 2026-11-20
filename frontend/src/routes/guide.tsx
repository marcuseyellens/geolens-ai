import { T } from "../i18n";
import { Link } from "react-router-dom";
import {
  BookOpen,
  Chrome,
  Smartphone,
  Globe2,
  CreditCard,
  ShieldCheck,
  CircleHelp,
  Download,
  ExternalLink,
} from "lucide-react";
import { Page, PageHeader, Card, Callout, Badge } from "../ds";

type Section = {
  id: string;
  title: string;
  icon: typeof Globe2;
  body: React.ReactNode;
};

const sections: Section[] = [
  {
    id: "start",
    title: "快速开始",
    icon: BookOpen,
    body: (
      <div className="space-y-3 text-sm text-ink-muted">
        <p>
          AnswerLens 把「AI 回答里到底提了谁」做成可核验的证据，而不是一个主观分数。
          所有入口共用同一账户与套餐，口径一致。
        </p>
        <div className="grid gap-2 sm:grid-cols-2">
          <Card>
            <p className="font-semibold text-ink">只想查一个网址</p>
            <p className="mt-1">用<Link to="/check" className="text-accent hover:underline">免费站点检查</Link>，免注册。</p>
          </Card>
          <Card>
            <p className="font-semibold text-ink">想跟踪品牌并做诊断</p>
            <p className="mt-1">注册后进入<Link to="/app" className="text-accent hover:underline">控制台</Link>添加品牌。</p>
          </Card>
        </div>
        <p>
          读结果三步：先看有效样本 <code className="rounded bg-canvas px-1">n_valid</code> 是否够 →
          再看逐模型的提及 / 推荐 / 链接三态 → 最后按「下一步」清单修复。
        </p>
      </div>
    ),
  },
  {
    id: "web",
    title: "网页版",
    icon: Globe2,
    body: (
      <div className="overflow-x-auto text-sm">
        <table className="w-full text-left">
          <thead>
            <tr className="border-b border-edge text-ink">
              <th className="py-2 pr-4">模块</th>
              <th className="py-2 pr-4">路径</th>
              <th className="py-2">做什么</th>
            </tr>
          </thead>
          <tbody className="text-ink-muted">
            {[
              ["免注册检查", "/check", "就绪度、技术条件清单、首要修复建议与只读分享链接"],
              ["品牌详情", "/app/brands/:id", "逐模型三态指标、运行记录与原始答案"],
              ["诊断报告", "/app/brands/:id/diagnostic", "多维度诊断与证据导出（Growth 及以上）"],
              ["竞品观察", "/app/competitor", "竞品占位信号与逐页引用路径（Scale）"],
              ["MCP 接入", "/app/mcp", "只读 Agent token 与 tools/list 说明"],
              ["套餐账单", "/app/billing", "订阅、订单与发票"],
            ].map(([name, path, desc]) => (
              <tr key={path} className="border-b border-edge/60">
                <td className="py-2 pr-4 text-ink">{name}</td>
                <td className="py-2 pr-4 font-mono text-xs">{path}</td>
                <td className="py-2">{desc}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <p className="mt-3 text-xs text-ink-subtle">
          界面语言首次按 IP 自动识别（缓存 24 小时），右上角可手动切换 8 种语言；手动选择永远优先。
        </p>
      </div>
    ),
  },
  {
    id: "extension",
    title: "浏览器插件",
    icon: Chrome,
    body: (
      <div className="space-y-3 text-sm text-ink-muted">
        <p>支持 Chrome / Edge / Firefox。独立仓库管理，与网页版共用账户和套餐。</p>
        <ol className="list-decimal pl-5 space-y-1">
          <li>从 GitHub Release 下载插件 zip 并解压。</li>
          <li>Chrome / Edge：打开 <code className="rounded bg-canvas px-1">chrome://extensions</code> → 开发者模式 → 加载已解压的扩展程序。</li>
          <li>Firefox：<code className="rounded bg-canvas px-1">about:debugging</code> → 临时载入附加组件 → 选择 manifest.json。</li>
        </ol>
        <p>
          三个页签：网址检查、本页提到了谁（绿色标记品牌 / 橙色标记竞品，只标记不改原文）、
          解锁更多（5 项加强能力按套餐逐级解锁）。
        </p>
        <Callout tone="info" title="能力门控">
          Starter 解锁批量检查与本地历史；Growth 解锁完整诊断与证据导出；Scale 解锁竞品对比面板。
        </Callout>
      </div>
    ),
  },
  {
    id: "apps",
    title: "iOS 与 Android App",
    icon: Smartphone,
    body: (
      <div className="space-y-3 text-sm text-ink-muted">
        <p>两个 App 由同一套 Web 全功能打包，功能与网页版控制台一致。</p>
        <ul className="list-disc pl-5 space-y-1">
          <li>Android：从 Release 下载 APK，在 Android 8.0+ 设备允许未知来源后安装。</li>
          <li>iOS：Release 提供模拟器构建供开发验证；真机安装需用源码自行签名 Archive。</li>
        </ul>
        <p className="text-xs text-ink-subtle">
          插件的「本页提及高亮」依赖网页 DOM，App 内不提供；其余功能一致。
        </p>
      </div>
    ),
  },
  {
    id: "plans",
    title: "套餐与支付",
    icon: CreditCard,
    body: (
      <div className="space-y-3 text-sm">
        <div className="overflow-x-auto">
          <table className="w-full text-left">
            <thead>
              <tr className="border-b border-edge text-ink">
                <th className="py-2 pr-4">套餐</th>
                <th className="py-2 pr-4">价格</th>
                <th className="py-2">主要权益</th>
              </tr>
            </thead>
            <tbody className="text-ink-muted">
              {[
                ["免费版", "$0", "每天 5 次速查、当前页扫描、基础修复建议"],
                ["Starter", "$49/月", "1500 次/月、批量检查、本地历史、1 个项目"],
                ["Growth", "$199/月", "9000 次/月、完整诊断、证据导出、MCP/API/告警、2 个项目"],
                ["Scale", "$410/月", "21000 次/月、竞品面板、自定义导出、5 个项目"],
              ].map(([name, price, perks]) => (
                <tr key={name} className="border-b border-edge/60">
                  <td className="py-2 pr-4 text-ink">{name}</td>
                  <td className="py-2 pr-4 font-mono text-xs">{price}</td>
                  <td className="py-2">{perks}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="text-ink-muted">
          支付方式：Airwallex（银行卡 / Apple Pay / Google Pay）与 PayPal。真实支付在主站收银台完成，
          回调强制验签、订单幂等激活。详见 <Link to="/pricing" className="text-accent hover:underline">定价页</Link>。
        </p>
      </div>
    ),
  },
  {
    id: "methodology",
    title: "核心概念与口径",
    icon: ShieldCheck,
    body: (
      <div className="grid gap-2 text-sm text-ink-muted sm:grid-cols-2">
        {[
          ["三态指标", "提及 / 推荐 / 链接分别统计，不合成单一分数。"],
          ["有效样本", "每个比例都附 n_valid 分母，过小时不要下结论。"],
          ["四数隔离", "planned / usable / failed / suspect 互不折算。"],
          ["跨语言观察", "不同语言问题集分别生成、分别计费。"],
          ["独立参考区", "DR / DA 等传统 SEO 指标为用户自述、未经核验。"],
          ["历史不重算", "1.0.0 / 1.1 / 1.2 口径并存，老报告永不改写。"],
        ].map(([title, desc]) => (
          <Card key={title}>
            <p className="font-semibold text-ink">{title}</p>
            <p className="mt-1 text-xs">{desc}</p>
          </Card>
        ))}
      </div>
    ),
  },
  {
    id: "faq",
    title: "常见问题",
    icon: CircleHelp,
    body: (
      <div className="space-y-3 text-sm text-ink-muted">
        {[
          ["为什么没有综合分？", "不同模型的采样与措辞不可直接合成一个可靠数字，我们给逐模型、逐意图的可核验比例与证据。"],
          ["就绪度 90 为什么没被推荐？", "技术就绪是必要而非充分条件，报告不承诺模型行为。"],
          ["沙盒支付会扣款吗？", "不会，沙盒只在本地激活演示状态。"],
          ["IP 自动语言会上传浏览数据吗？", "不会，只请求公共 IP 地理服务读取国家码用于选语言。"],
        ].map(([q, a]) => (
          <div key={q}>
            <p className="font-semibold text-ink">{q}</p>
            <p className="mt-1">{a}</p>
          </div>
        ))}
      </div>
    ),
  },
];

/** 产品使用手册：官网内可读版本，与 docs/PRODUCT-GUIDE.md 内容一致。 */
export default function GuidePage() {
  return (
    <T>
      <Page measure="default">
        <PageHeader
          title="产品使用手册"
          description="一份手册讲清网页版、浏览器插件、iOS / Android App 与套餐支付。"
          actions={
            <a
              href="https://github.com/marcuseyellens/geolens-ai/blob/main/docs/PRODUCT-GUIDE.md"
              target="_blank"
              rel="noreferrer"
              className="inline-flex h-9 items-center gap-1.5 rounded-ds border border-edge px-3 text-sm text-ink-muted hover:text-ink"
            >
              <Download className="h-4 w-4" /> GitHub 原文 <ExternalLink className="h-3 w-3" />
            </a>
          }
        />

        <Callout tone="info">
          技术就绪 ≠ 被推荐。我们只报告可核验的事实，不承诺排名或转化结果。
        </Callout>

        <nav className="sticky top-14 z-10 -mx-2 flex gap-2 overflow-x-auto rounded-ds bg-surface-overlay px-2 py-2 backdrop-blur">
          {sections.map((s) => (
            <a
              key={s.id}
              href={`#${s.id}`}
              className="shrink-0 rounded-full border border-edge px-3 py-1 text-xs text-ink-muted hover:text-ink hover:border-accent"
            >
              {s.title}
            </a>
          ))}
        </nav>

        <div className="space-y-8">
          {sections.map((section) => {
            const Icon = section.icon;
            return (
              <section key={section.id} id={section.id} className="scroll-mt-28">
                <h2 className="mb-3 flex items-center gap-2 text-lg font-semibold text-ink">
                  <Icon className="h-5 w-5 text-accent" />
                  {section.title}
                  <Badge>{section.id}</Badge>
                </h2>
                <Card>{section.body}</Card>
              </section>
            );
          })}
        </div>

        <p className="mt-8 text-center text-xs text-ink-subtle">
          遇到手册没覆盖的问题？<a href="mailto:support@geolens.ai" className="text-accent hover:underline">联系我们</a>。
        </p>
      </Page>
    </T>
  );
}
