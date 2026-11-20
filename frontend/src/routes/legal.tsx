import { T } from "../i18n";
import { Link } from "react-router-dom";
import { ShieldCheck, FileText, ArrowRight } from "lucide-react";
import { Card, Badge } from "../ds";

const CONTENT = {
  privacy: {
    eyebrow: "法律文件",
    title: "隐私政策",
    updated: "最后更新：2026-11-14",
    icon: <ShieldCheck className="h-5 w-5" />,
    intro: "AnswerLens 遵循数据最小化原则。我们只保留提供服务所必需的数据，不出售、不共享给广告网络。",
    sections: [
      {
        h: "1. 我们收集什么",
        p: [
          "站点检查：你提交的网址、该次检查的 /100 分结果与逐项证据。仅当你主动点击「生成分享链接」时，这份结果快照才会被持久化，用于让链接可复现。",
          "转化漏斗：仅在你明确同意后，记录「访问 → 发起检查 → 得到结果 → 生成分享」等匿名事件计数与页面路径。事件不含姓名、邮箱、IP 或设备指纹。",
          "Cookie 与本地存储：仅一个键用于记住你的统计同意选择，以及主题偏好。拒绝统计后，我们不会发出任何埋点请求。",
        ],
      },
      {
        h: "2. 我们不收集什么",
        p: [
          "免费站点检查不需要注册、不需要邮箱、不需要信用卡，我们不要求任何账号权限，也不会写入你的站点。",
          "我们不使用跨站广告追踪，不构建用户画像，不把数据用于模型训练。",
        ],
      },
      {
        h: "3. 第三方与 AI 模型",
        p: [
          "深度审计与订阅服务会调用第三方 AI 模型接口生成回答。发送给模型的是 buyer question 文本，不包含你的个人信息。",
          "站点检查只读取目标站点的公开页面（robots.txt、llms.txt、sitemap.xml、首页 HTML 等）。",
        ],
      },
      {
        h: "4. 保留与删除",
        p: [
          "分享快照与漏斗事件保留在自托管存储中，可随时清理。你可以发邮件至 support@geolens.ai 请求删除指定分享链接或全部事件数据。",
        ],
      },
      {
        h: "5. 你的权利",
        p: [
          "你可以随时在浏览器中清除本地存储以撤回统计同意；撤回后不再产生新的埋点事件。",
        ],
      },
    ],
  },
  terms: {
    eyebrow: "法律文件",
    title: "服务条款",
    updated: "最后更新：2026-11-14",
    icon: <FileText className="h-5 w-5" />,
    intro: "使用 AnswerLens 即表示你理解本服务的性质：我们提供可核验的事实与证据，不承诺结果。",
    sections: [
      {
        h: "1. 服务性质",
        p: [
          "AnswerLens 是面向 AEO（Answer Engine Optimization）的可见性检查与诊断服务，输出站点就绪度、真实 AI 回答证据、逐模型与逐意图指标、修复清单与修复件。",
          "技术就绪 ≠ 被推荐。所有分数与指标只描述可观测的技术条件与回答内容，不构成对任何 AI 模型推荐结果、排名、流量或转化的因果承诺。",
        ],
      },
      {
        h: "2. 免费与付费范围",
        p: [
          "L0 免费站点检查无需注册，输出 /100 分与 1 条最高优先级建议，可生成免登录的只读分享链接。",
          "L2 $39 一次性审计包含 30 题（25 buyer question + 5 brand check）、证据下钻、人工复核与修复件；L3 复扫 add-on $19/月；订阅三档为 Starter $49、Growth $199、Scale $410 每月。",
        ],
      },
      {
        h: "3. 运行与配额口径",
        p: [
          "失败、超时、空回答、被拦截或解析失败的运行会被单独标注，不计入指标分母，也不占用可用配额。",
          "报告中的每个比例数字都可下钻到对应的 run_id 与原始答案；我们不提供无证据来源的「综合分」。",
        ],
      },
      {
        h: "4. 使用限制",
        p: [
          "不得使用本服务扫描未获授权的系统、内网地址或用于规避目标站点的访问限制。站点检查内置公网地址与内网地址校验。",
          "修复件由你自行审核后发布；因发布内容引发的第三方主张由你自行负责。",
        ],
      },
      {
        h: "5. 退款与变更",
        p: [
          "订阅按月计费，可随时取消，当月配额用到周期结束。一次性审计一经交付，不支持以「未被 AI 推荐」为由的退款。",
          "套餐价格与权益以定价页及本条款为准，调整会提前在站内公告。",
        ],
      },
    ],
  },
} as const;

export default function LegalPage({ kind }: { kind: "privacy" | "terms" }) {
  const c = CONTENT[kind];
  return (
    <T>
    <div className="mx-auto max-w-3xl px-6 py-14">
      <div className="flex items-center gap-3">
        <span className="flex h-10 w-10 items-center justify-center rounded-ds bg-accent-subtle text-accent">{c.icon}</span>
        <div>
          <Badge tone="accent">{c.eyebrow}</Badge>
          <h1 className="mt-1 text-3xl font-bold text-ink tracking-tight">{c.title}</h1>
        </div>
      </div>
      <p className="mt-4 text-xs text-ink-subtle">{c.updated}</p>
      <p className="mt-4 text-sm text-ink-muted leading-relaxed">{c.intro}</p>

      <Card className="mt-8">
        <div className="space-y-7">
          {c.sections.map((s) => (
            <section key={s.h}>
              <h2 className="text-base font-semibold text-ink">{s.h}</h2>
              {s.p.map((p) => (
                <p key={p} className="mt-2 text-sm text-ink-muted leading-relaxed">{p}</p>
              ))}
            </section>
          ))}
        </div>
      </Card>

      <div className="mt-8 flex flex-wrap items-center justify-between gap-4">
        <Link to={kind === "privacy" ? "/terms" : "/privacy"} className="inline-flex items-center gap-1.5 text-sm text-accent hover:underline">
          查看{kind === "privacy" ? "服务条款" : "隐私政策"} <ArrowRight className="h-3.5 w-3.5" />
        </Link>
        <a href="mailto:support@geolens.ai" className="text-sm text-ink-muted hover:text-ink">support@geolens.ai</a>
      </div>
    </div>
    </T>
  );
}
