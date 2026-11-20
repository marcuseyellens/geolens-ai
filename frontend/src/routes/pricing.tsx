import { T } from "../i18n";
import { Link } from "react-router-dom";
import {
  ScanLine, FileSearch, Activity, CheckCircle2, ArrowRight,
  ShieldCheck, Info, Sparkles, Repeat2, Globe,
} from "lucide-react";
import { Card, Button, Badge, Callout, cn } from "../ds";
import { NEUTRALITY } from "../lib/neutrality";

const ONE_TIME = [
  {
    icon: <ScanLine className="h-5 w-5" />,
    title: "L0 · 免费站点检查",
    price: "$0",
    unit: "永久免费",
    desc: "看看公开页面是否具备被 AI 抓取与引用的技术条件。",
    bullets: [
      "免注册 / 免邮箱 / 免信用卡",
      "/100 分站点就绪度 + 1 条首要建议",
      "可生成免登录的分享链接",
    ],
    cta: <Link to="/check"><Button variant="secondary" className="w-full">开始免费检查</Button></Link>,
  },
  {
    icon: <FileSearch className="h-5 w-5" />,
    title: "L2 · 一次性深度审计",
    price: "$39",
    unit: "一次性",
    desc: "用真实 buyer question 看清 AI 现在推荐谁，并拿到可执行的修复件。",
    bullets: [
      "30 题：25 buyer question + 5 brand check",
      "逐条证据下钻到原始答案与引用域名",
      "人工复核 + 修复清单 + 修复件打包下载",
    ],
    cta: (
      <div className="space-y-2">
        <Link to="/subscribe?plan=one_time&kind=one_time">
          <Button className="w-full">购买一次性审计 <ArrowRight className="h-4 w-4" /></Button>
        </Link>
        <Link to="/check" className="block text-center text-xs text-ink-muted hover:text-ink transition-colors">
          或先做免费检查
        </Link>
      </div>
    ),
    highlight: true,
  },
  {
    icon: <Repeat2 className="h-5 w-5" />,
    title: "L3 · 复扫 add-on",
    price: "$19",
    unit: "/ 月",
    desc: "同一组问题重跑，用 matched comparison 看改动前后差异。",
    bullets: [
      "与上次审计完全同题、同意图",
      "before / after 可用样本与提及率对照",
      "与订阅相互独立，可随时停",
    ],
    cta: (
      <Link to="/subscribe?plan=rescan&kind=add_on">
        <Button variant="secondary" className="w-full">订阅复扫 <ArrowRight className="h-4 w-4" /></Button>
      </Link>
    ),
  },
];

const TIERS = [
  {
    name: "Starter", plan: "starter", price: "$49", projects: "1 个项目", quota: "50 prompt / 月",
    features: ["按意图持续跑题", "证据下钻与指标看板", "基础导出", "—", "—"],
  },
  {
    name: "Growth", plan: "growth", price: "$199", projects: "2 个项目", quota: "300 prompt / 月",
    features: ["按意图持续跑题", "证据下钻与指标看板", "自定义导出", "MCP / API 接入", "告警与复扫对比"],
    highlight: true,
  },
  {
    name: "Scale", plan: "scale", price: "$410", projects: "5 个项目", quota: "700 prompt / 月",
    features: ["按意图持续跑题", "证据下钻与指标看板", "自定义导出", "MCP / API 接入", "告警与复扫对比 + 优先支持"],
  },
];

export default function PricingPage() {
  return (
    <T>
    <div className="mx-auto max-w-6xl px-6 py-14">
      <div className="text-center">
        <Badge tone="accent">定价与套餐</Badge>
        <h1 className="mt-4 text-3xl sm:text-4xl font-bold text-ink tracking-tight">
          先用免费的，再决定要不要付费
        </h1>
        <p className="mx-auto mt-3 max-w-2xl text-sm text-ink-muted leading-relaxed">
          L0 完全免费且无需注册；一次性审计与订阅各自独立，均按可核验的运行结果计费，
          <strong className="text-ink">没有结果的检查不收费</strong>。
        </p>
      </div>

      {/* 一次性产品 */}
      <div className="mt-12 grid grid-cols-1 md:grid-cols-3 gap-5">
        {ONE_TIME.map((p) => (
          <Card key={p.title} raised className={cn("relative flex flex-col", p.highlight && "border-accent/40")}>
            {p.highlight && (
              <>
                <span className="absolute top-0 left-0 right-0 h-px bg-[image:var(--ds-gradient-tech)]" />
                <span className="absolute -top-3 right-4"><Badge tone="accent">最常选择</Badge></span>
              </>
            )}
            <div className="flex items-center justify-between">
              <span className="flex h-10 w-10 items-center justify-center rounded-ds bg-[image:var(--ds-gradient-accent)] text-white shadow-md shadow-accent/20">
                {p.icon}
              </span>
              <span className="text-2xl font-bold text-ink tracking-tight">
                {p.price}<span className="ml-1 text-xs font-normal text-ink-subtle">{p.unit}</span>
              </span>
            </div>
            <h2 className="mt-5 text-base font-semibold text-ink">{p.title}</h2>
            <p className="mt-2 text-sm text-ink-muted leading-relaxed min-h-[60px]">{p.desc}</p>
            <ul className="mt-4 space-y-2 flex-1">
              {p.bullets.map((b) => (
                <li key={b} className="flex items-start gap-2 text-sm text-ink-muted">
                  <CheckCircle2 className="h-4 w-4 mt-0.5 text-positive shrink-0" />{b}
                </li>
              ))}
            </ul>
            <div className="mt-6 pt-4 border-t border-edge">{p.cta}</div>
          </Card>
        ))}
      </div>

      {/* 订阅三档 */}
      <div className="mt-14">
        <div className="flex items-center gap-2">
          <Sparkles className="h-5 w-5 text-accent" />
          <h2 className="text-xl font-bold text-ink">L3 · 按月订阅监测</h2>
        </div>
        <p className="mt-2 text-sm text-ink-muted">按成功完成的检查计费，每个数字都能看到原始回答。</p>

        <div className="mt-6 overflow-x-auto">
          <table className="w-full min-w-[720px] border-separate border-spacing-0 text-sm">
            <thead>
              <tr>
                <th className="text-left p-4 font-medium text-ink-muted border-b border-edge w-[200px]">能力</th>
                {TIERS.map((t) => (
                  <th key={t.name} className={cn("p-4 text-center border-b border-edge", t.highlight && "bg-accent-subtle rounded-t-ds")}>
                    <div className="font-semibold text-ink">{t.name}</div>
                    <div className="mt-1 text-xl font-bold text-ink">{t.price}<span className="text-xs font-normal text-ink-subtle"> / 月</span></div>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              <Row label="项目数" values={TIERS.map((t) => t.projects)} />
              <Row label="每月检查次数" values={TIERS.map((t) => t.quota)} />
              {["按意图持续跑题", "证据下钻与指标看板", "自定义导出", "MCP / API 接入", "告警与复扫对比"].map((cap, i) => (
                <Row key={cap} label={cap} values={TIERS.map((t) => t.features[i])} />
              ))}
              <tr>
                <td className="p-4 border-b border-edge text-ink-muted">开始使用</td>
                {TIERS.map((t) => (
                  <td key={t.name} className={cn("p-4 text-center border-b border-edge", t.highlight && "bg-accent-subtle")}>
                    <Link to={`/subscribe?plan=${t.plan}&kind=subscription`}>
                      <Button size="sm" variant={t.highlight ? "primary" : "secondary"} className="w-full">
                        订阅 {t.name} <ArrowRight className="h-3.5 w-3.5" />
                      </Button>
                    </Link>
                  </td>
                ))}
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      {/* 口径说明 */}
      <div className="mt-12 space-y-4">
        <Callout tone="warn" title="计费与结果说明">
          失败、超时、无引用的运行会被单独标注且不计入分母，也不会占用配额。我们提供的是可核验的事实与证据，
          <strong className="text-ink">不提供「综合分」，也不承诺任何模型一定推荐你</strong>。
        </Callout>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          <Note icon={<Info className="h-4 w-4" />} title="价格说明"
            text="$39 一次性审计、$19 复扫 add-on 与三档订阅价格沿用公开定价；正式定价与权益细则以服务条款为准。" />
          <Note icon={<ShieldCheck className="h-4 w-4" />} title="数字从哪来"
            text="每个百分比都能点开看到原始 AI 回答和引用页面。" />
          <Note icon={<Activity className="h-4 w-4" />} title="取消与退还"
            text="订阅按月计费，随时取消，当月配额用到周期结束。一次性审计一经交付不支持因「未被推荐」退款。" />
          {/* D-6 / EX-03：多语言按不同采集任务分别计费，分别计入 prompt quota。 */}
          <Note icon={<Globe className="h-4 w-4" />} title="多语言怎么算"
            text="每个语种单独出题、单独计费，不会把中文问题直接翻译成英文重复使用。" />
        </div>
        <p className="text-center text-xs text-ink-subtle">技术就绪 ≠ 被推荐。所有套餐均不构成对 AI 推荐结果、排名或转化的因果承诺。</p>
        <p className="mx-auto mt-3 max-w-3xl text-center text-xs leading-relaxed text-ink-subtle">{NEUTRALITY}</p>
      </div>
    </div>
    </T>
  );
}

function Row({ label, values }: { label: string; values: string[] }) {
  return (
    <T>
    <tr>
      <td className="p-4 border-b border-edge text-ink-muted">{label}</td>
      {values.map((v, i) => (
        <td key={i} className={cn("p-4 text-center border-b border-edge text-ink", TIERS[i].highlight && "bg-accent-subtle")}>
          {v === "—" ? <span className="text-ink-subtle">—</span> : v}
        </td>
      ))}
    </tr>
    </T>
  );
}

function Note({ icon, title, text }: { icon: React.ReactNode; title: string; text: string }) {
  return (
    <T>
    <Card className="text-left">
      <div className="flex items-center gap-2 text-accent">{icon}<span className="text-sm font-semibold text-ink">{title}</span></div>
      <p className="mt-2 text-xs text-ink-muted leading-relaxed">{text}</p>
    </Card>
    </T>
  );
}
