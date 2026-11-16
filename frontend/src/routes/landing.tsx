import { T } from "../i18n";
import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  Globe, ArrowRight, ScanLine, Radar, ShieldCheck, Zap, Layers,
  ClipboardCheck, Lightbulb, Quote, ChevronDown, CheckCircle2,
  FileSearch, Activity, Lock, Sparkles, ExternalLink,
} from "lucide-react";
import { Card, Badge, Button, cn } from "../ds";
import { track } from "../lib/tracking";
import { NEUTRALITY } from "../lib/neutrality";

const FAQ = [
  {
    q: "这和传统 SEO 工具有什么不同？",
    a: "传统工具看的是搜索结果排名与关键词；GeoLens 看的是 AI 生成式回答里是否出现你的品牌、被放在什么位置、引用了哪些来源。两者度量的对象不同，不能互相替代。",
  },
  {
    q: "免费检查需要注册吗？",
    a: "不需要。输入网址即可拿到 /100 分的站点就绪度与 1 条最高优先级建议，全程无登录、无邮箱、无信用卡。结果还可以生成一条可分享链接。",
  },
  {
    q: "拿到高分，AI 就一定会推荐我吗？",
    a: "不会。技术就绪 ≠ 被推荐。分数只反映公开页面是否具备被 AI 抓取、理解与引用的技术条件，不预测也不承诺任何模型的推荐结果、排名或转化。",
  },
  {
    q: "你们会保存我检查的网址吗？",
    a: "仅在生成分享链接时保存该次检查结果（含网址与分数），用于让链接可复现。埋点只记录「访问 → 检查 → 结果」的匿名事件计数，不采集个人信息，且需你明确同意。",
  },
  {
    q: "$39 一次性审计包含什么？",
    a: "30 个 buyer question（含 5 个品牌核验题）的真实回答、逐条证据下钻、人工复核后的修复清单，以及可直接发布的修复件打包下载。失败的运行不计入分母，也不会收费。",
  },
  {
    q: "订阅可以随时取消吗？",
    a: "可以。订阅按月计费，随时取消，取消后当月配额用到周期结束为止。复扫 add-on 与订阅相互独立。",
  },
];

export default function LandingPage() {
  const [url, setUrl] = useState("");
  const [openFaq, setOpenFaq] = useState<number | null>(0);
  const navigate = useNavigate();

  const startCheck = (e: React.FormEvent) => {
    e.preventDefault();
    const value = url.trim();
    if (!value) return;
    track("check", { source: "landing" });
    navigate(`/check?url=${encodeURIComponent(value)}`);
  };

  return (
    <T>
    <div>
      {/* Hero */}
      <section className="relative overflow-hidden border-b border-edge">
        <div className="pointer-events-none absolute inset-0 bg-[image:var(--ds-gradient-surface)]" />
        <div className="pointer-events-none absolute -top-40 left-1/2 -translate-x-1/2 h-[420px] w-[720px] rounded-full bg-[image:var(--ds-gradient-tech)] opacity-15 blur-[120px]" />
        <div className="pointer-events-none absolute inset-0 opacity-[0.04]" style={{
          backgroundImage: "linear-gradient(var(--ds-ink) 1px, transparent 1px), linear-gradient(90deg, var(--ds-ink) 1px, transparent 1px)",
          backgroundSize: "48px 48px",
        }} />
        <div className="relative mx-auto max-w-6xl px-6 pt-20 pb-24 text-center">
          <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full border border-accent/25 bg-accent-subtle text-xs font-medium text-accent">
            <Sparkles className="h-3.5 w-3.5" />
            AEO · Answer Engine Optimization
          </div>
          <h1 className="mt-6 text-4xl sm:text-5xl md:text-6xl font-bold tracking-tight text-ink">
            看懂跨平台差异，{" "}
            <span className="tech-gradient-text">拿到能改的东西</span>
          </h1>
          <p className="mx-auto mt-5 max-w-2xl text-base sm:text-lg text-ink-muted leading-relaxed">
            逐模型并列呈现品牌在 AI 回答里的真实可见性，每条结论都能下钻到原始证据。
            <br className="hidden sm:block" />
            从一个免注册的网址检查开始，全程不需要邮箱和信用卡。
          </p>

          <form onSubmit={startCheck} className="mx-auto mt-9 max-w-2xl">
            <div className="flex flex-col sm:flex-row gap-2 p-2 rounded-ds-lg border border-edge-strong bg-surface shadow-xl shadow-accent/5">
              <div className="relative flex-1">
                <Globe className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-ink-subtle" />
                <input
                  value={url}
                  onChange={(e) => setUrl(e.target.value)}
                  placeholder="输入你的网址，例如 example.com"
                  aria-label="网址"
                  className="w-full h-12 bg-transparent pl-11 pr-3 text-sm text-ink placeholder:text-ink-subtle focus:outline-none"
                />
              </div>
              <Button type="submit" size="lg" disabled={!url.trim()} className="shrink-0">
                <ScanLine className="h-4 w-4" />
                免费检查
                <ArrowRight className="h-4 w-4" />
              </Button>
            </div>
            <div className="mt-3 flex flex-wrap items-center justify-center gap-x-5 gap-y-1.5 text-xs text-ink-subtle">
              <span className="inline-flex items-center gap-1.5"><CheckCircle2 className="h-3.5 w-3.5 text-positive" />免注册</span>
              <span className="inline-flex items-center gap-1.5"><CheckCircle2 className="h-3.5 w-3.5 text-positive" />免邮箱</span>
              <span className="inline-flex items-center gap-1.5"><CheckCircle2 className="h-3.5 w-3.5 text-positive" />免信用卡</span>
              <span className="inline-flex items-center gap-1.5"><ShieldCheck className="h-3.5 w-3.5 text-positive" />结果可分享</span>
            </div>
          </form>

          <div className="mx-auto mt-10 max-w-2xl rounded-ds border border-edge bg-surface/70 px-4 py-3 text-left text-xs leading-relaxed text-ink-muted">
            <div className="flex items-start gap-2">
              <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-accent" />
              <span><strong className="text-ink">技术就绪 ≠ 被推荐。</strong>{NEUTRALITY}</span>
            </div>
            <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-edge pt-2">
              <span>数据来源分三类，分表展示、不合并：</span>
              <span className="text-ink">第一方自述</span>
              <span className="text-ink">第三方实测</span>
              <span className="text-ink">第三方转述</span>
              <Link to="/privacy" className="text-accent hover:underline">口径说明</Link>
            </div>
          </div>
        </div>
      </section>

      {/* 三层漏斗 */}
      <section className="mx-auto max-w-6xl px-6 py-16">
        <SectionEyebrow>从免费到持续监测</SectionEyebrow>
        <h2 className="mt-3 text-2xl sm:text-3xl font-bold text-ink">一条不要求你先付费的路径</h2>
        <p className="mt-2 text-sm text-ink-muted">先看技术条件，再看真实回答，最后才决定是否长期监测。</p>
        <div className="mt-9 grid grid-cols-1 md:grid-cols-3 gap-5">
          <FunnelCard
            step="01" tone="accent" title="免费站点检查" price="$0"
            icon={<ScanLine className="h-5 w-5" />}
            bullets={["免注册 / 免邮箱 / 免信用卡", "/100 分就绪度 + 1 条首要建议", "生成可分享的结果链接"]}
            cta={<Link to="/check" className="inline-flex items-center gap-1.5 text-sm font-medium text-accent hover:underline">立即检查 <ArrowRight className="h-3.5 w-3.5" /></Link>}
          />
          <FunnelCard
            step="02" tone="cyan" title="一次性深度审计" price="$39"
            icon={<FileSearch className="h-5 w-5" />}
            bullets={["30 个 buyer question 真实回答", "逐条证据下钻 + 人工复核", "修复清单与可发布修复件"]}
            cta={<Link to="/pricing" className="inline-flex items-center gap-1.5 text-sm font-medium text-accent hover:underline">查看包含项 <ArrowRight className="h-3.5 w-3.5" /></Link>}
            highlight
          />
          <FunnelCard
            step="03" tone="violet" title="订阅监测" price="$49 起"
            icon={<Activity className="h-5 w-5" />}
            bullets={["按意图持续跑题与配额", "matched comparison 复扫对比", "Growth 起含 MCP / API / 告警"]}
            cta={<Link to="/pricing" className="inline-flex items-center gap-1.5 text-sm font-medium text-accent hover:underline">对比套餐 <ArrowRight className="h-3.5 w-3.5" /></Link>}
          />
        </div>
      </section>

      {/* 工作原理 */}
      <section id="how" className="border-y border-edge bg-surface scroll-mt-20">
        <div className="mx-auto max-w-6xl px-6 py-16">
          <SectionEyebrow>工作原理</SectionEyebrow>
          <h2 className="mt-3 text-2xl sm:text-3xl font-bold text-ink">三步，从网址到可核验的证据</h2>
          <div className="mt-10 grid grid-cols-1 md:grid-cols-3 gap-6">
            <HowStep n="1" icon={<Radar className="h-5 w-5" />} title="输入网址"
              desc="只读取公开页面，检查 robots.txt、llms.txt、结构化数据、可抓取性与页面事实。" />
            <HowStep n="2" icon={<ClipboardCheck className="h-5 w-5" />} title="逐项打分"
              desc="按可抓取性 / 结构化 / 内容事实 / 页面健康四组加权，得到 /100 的站点就绪度。" />
            <HowStep n="3" icon={<Lightbulb className="h-5 w-5" />} title="拿到首要修复"
              desc="给出 1 条最高优先级建议；需要更深的答案级证据时，再进入审计。" />
          </div>
          <p className="mt-8 flex items-start gap-2 text-xs text-ink-subtle">
            <Lock className="h-3.5 w-3.5 mt-0.5 shrink-0" />
            检查只读公开页面，不写入你的站点，也不要求提供任何账号权限。失败运行不计入分母。
          </p>
        </div>
      </section>

      {/* 证据样例 */}
      <section className="mx-auto max-w-6xl px-6 py-16">
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-10 items-center">
          <div>
            <SectionEyebrow>证据优先</SectionEyebrow>
            <h2 className="mt-3 text-2xl sm:text-3xl font-bold text-ink">每个结论都能下钻到原文</h2>
            <p className="mt-4 text-sm text-ink-muted leading-relaxed">
              我们不给「综合分」式的主观判断。报告里的每个比例都由一次真实运行支撑，
              可以一路点到原始 AI 回答、被引用的域名与页面路径。
            </p>
            <ul className="mt-6 space-y-3">
              {[
                "逐模型、逐意图的原始答案与引用",
                "品牌提及 / 推荐 / 链接分别统计，不合并成单一分数",
                "失败、超时、无引用的运行单独标注，不静默计入分母",
              ].map((t) => (
                <li key={t} className="flex items-start gap-2.5 text-sm text-ink">
                  <CheckCircle2 className="h-4 w-4 mt-0.5 text-positive shrink-0" />
                  {t}
                </li>
              ))}
            </ul>
            <Link to="/app" className="mt-7 inline-flex items-center gap-1.5 text-sm font-medium text-accent hover:underline">
              在控制台查看证据下钻 <ExternalLink className="h-3.5 w-3.5" />
            </Link>
          </div>
          <Card raised className="relative">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Quote className="h-4 w-4 text-accent" />
                <span className="text-xs font-medium text-ink-muted">证据样例 · buyer question</span>
              </div>
              <Badge tone="accent">示例</Badge>
            </div>
            <p className="mt-4 text-sm font-medium text-ink">
              “What are the best tools for team knowledge management?”
            </p>
            <div className="mt-3 rounded-ds border border-edge bg-canvas p-4 text-sm text-ink-muted leading-relaxed">
              … 常见的选择包括 <span className="font-medium text-ink">Notion</span>、
              <span className="font-medium text-ink">Confluence</span> 与
              <span className="font-medium text-ink">Obsidian</span> …
            </div>
            <div className="mt-4 space-y-2">
              {[
                { domain: "notion.so", path: "/product", mentioned: true },
                { domain: "a-third-party-review.example", path: "/best-tools", mentioned: false },
              ].map((c) => (
                <div key={c.domain} className="flex items-center gap-2 text-xs">
                  <span className="h-1.5 w-1.5 rounded-full bg-accent" />
                  <span className="text-ink">{c.domain}{c.path}</span>
                  {c.mentioned && <Badge tone="positive">品牌链接</Badge>}
                </div>
              ))}
            </div>
            <div className="mt-4 pt-3 border-t border-edge flex items-center justify-between text-xs text-ink-subtle">
              <span>run_id 可追溯 · 逐项证据</span>
              <span>不承诺排名</span>
            </div>
          </Card>
        </div>
      </section>

      {/* Δ-01~Δ-07：可核验的数据口径 */}
      <section id="methodology" className="border-t border-edge bg-surface">
        <div className="mx-auto max-w-6xl px-6 py-14">
          <SectionEyebrow>可核验的数据口径</SectionEyebrow>
          <h2 className="mt-3 text-2xl sm:text-3xl font-bold text-ink">数字怎么来的，都能查到</h2>
          <p className="mt-3 max-w-3xl text-sm text-ink-muted leading-relaxed">
            每个数字都标注来源类型与方法论版本。可疑批次、跨语言运行、用户自述指标各自隔离，不混进同一条结论。
          </p>
          <div className="mt-8 grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            {[
              { icon: Layers, title: "四数互不折算", text: "计划、有效、失败、可疑批次四数同屏；可疑批次单独计数，不并入有效或失败。" },
              { icon: Globe, title: "跨语言并列观察", text: "不同语言的问题集分别生成、分别计入配额；检索语言未判定与不匹配的运行不进默认分母。" },
              { icon: ShieldCheck, title: "独立参考区", text: "DR / DA 等用户自述的传统 SEO 指标单独成表，明确标注未经核验，不进图表、不参与排序或均值。" },
              { icon: ClipboardCheck, title: "证据分行", text: "平台提示与实际观察分成两行；平台报错只作线索，不据此下收录结论。" },
            ].map((f) => (
              <Card key={f.title} className="h-full">
                <f.icon className="h-5 w-5 text-accent" />
                <h3 className="mt-3 text-sm font-semibold text-ink">{f.title}</h3>
                <p className="mt-1.5 text-xs text-ink-muted leading-relaxed">{f.text}</p>
              </Card>
            ))}
          </div>
          <p className="mt-6 text-center text-xs text-ink-subtle">
            全部增量行为挂在 feature flag 之下，可一键回到上一口径；静态断言与 19 条自动用例在 CI 中执行。
          </p>
        </div>
      </section>

      {/* FAQ */}
      <section id="faq" className="border-t border-edge bg-surface scroll-mt-20">
        <div className="mx-auto max-w-3xl px-6 py-16">
          <SectionEyebrow>常见问题</SectionEyebrow>
          <h2 className="mt-3 text-2xl sm:text-3xl font-bold text-ink">把边界说清楚</h2>
          <div className="mt-8 divide-y divide-edge border-y border-edge">
            {FAQ.map((item, i) => (
              <div key={item.q}>
                <button
                  onClick={() => setOpenFaq(openFaq === i ? null : i)}
                  className="w-full flex items-center justify-between gap-4 py-4 text-left text-sm font-medium text-ink hover:text-accent transition-colors"
                  aria-expanded={openFaq === i}
                >
                  {item.q}
                  <ChevronDown className={cn("h-4 w-4 shrink-0 text-ink-subtle transition-transform", openFaq === i && "rotate-180")} />
                </button>
                {openFaq === i && (
                  <p className="pb-5 -mt-1 text-sm text-ink-muted leading-relaxed">{item.a}</p>
                )}
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* 底部 CTA */}
      <section className="relative overflow-hidden">
        <div className="pointer-events-none absolute inset-0 bg-[image:var(--ds-gradient-tech)] opacity-[0.07]" />
        <div className="relative mx-auto max-w-4xl px-6 py-16 text-center">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-ds bg-[image:var(--ds-gradient-tech)] shadow-lg shadow-accent/25">
            <Zap className="h-6 w-6 text-white" />
          </div>
          <h2 className="mt-5 text-2xl sm:text-3xl font-bold text-ink">先花 10 秒，看看你的站点准备好了没有</h2>
          <p className="mt-3 text-sm text-ink-muted">免注册、免邮箱、免信用卡。结果可分享给同事。</p>
          <div className="mt-7 flex flex-wrap items-center justify-center gap-3">
            <Link to="/check">
              <Button size="lg"><ScanLine className="h-4 w-4" />开始免费检查</Button>
            </Link>
            <Link to="/pricing">
              <Button size="lg" variant="secondary"><Layers className="h-4 w-4" />查看定价</Button>
            </Link>
          </div>
        </div>
      </section>
    </div>
    </T>
  );
}

function SectionEyebrow({ children }: { children: React.ReactNode }) {
  return (
    <T>
    <span className="inline-flex items-center gap-2 text-xs font-semibold uppercase tracking-widest text-accent">
      <span className="h-px w-6 bg-[image:var(--ds-gradient-accent)]" />
      {children}
    </span>
    </T>
  );
}

const toneMap: Record<string, string> = {
  accent: "bg-[image:var(--ds-gradient-accent)]",
  cyan: "bg-[image:var(--ds-gradient-cyan)]",
  violet: "bg-[image:var(--ds-gradient-tech)]",
};

function FunnelCard({
  step, tone, title, price, icon, bullets, cta, highlight,
}: {
  step: string; tone: string; title: string; price: string;
  icon: React.ReactNode; bullets: string[]; cta: React.ReactNode; highlight?: boolean;
}) {
  return (
    <T>
    <Card raised className={cn("relative flex flex-col", highlight && "border-accent/40")}>
      {highlight && <span className="absolute top-0 left-0 right-0 h-px bg-[image:var(--ds-gradient-tech)]" />}
      <div className="flex items-start justify-between">
        <span className={cn("flex h-10 w-10 items-center justify-center rounded-ds text-white shadow-md shadow-accent/20", toneMap[tone])}>
          {icon}
        </span>
        <span className="text-xs font-medium text-ink-subtle tabular-nums">STEP {step}</span>
      </div>
      <h3 className="mt-5 text-base font-semibold text-ink">{title}</h3>
      <p className="mt-1 text-2xl font-bold text-ink tracking-tight">{price}</p>
      <ul className="mt-4 space-y-2 flex-1">
        {bullets.map((b) => (
          <li key={b} className="flex items-start gap-2 text-sm text-ink-muted">
            <CheckCircle2 className="h-4 w-4 mt-0.5 text-positive shrink-0" />
            {b}
          </li>
        ))}
      </ul>
      <div className="mt-5 pt-4 border-t border-edge">{cta}</div>
    </Card>
    </T>
  );
}

function HowStep({ n, icon, title, desc }: { n: string; icon: React.ReactNode; title: string; desc: string }) {
  return (
    <T>
    <div className="relative rounded-ds-lg border border-edge bg-canvas p-6">
      <div className="flex items-center justify-between">
        <span className="flex h-10 w-10 items-center justify-center rounded-ds bg-accent-subtle text-accent">
          {icon}
        </span>
        <span className="text-3xl font-bold text-edge-strong tabular-nums">{n}</span>
      </div>
      <h3 className="mt-5 text-base font-semibold text-ink">{title}</h3>
      <p className="mt-2 text-sm text-ink-muted leading-relaxed">{desc}</p>
    </div>
    </T>
  );
}
