import { T, LocaleToggle } from "../i18n";
import { useEffect, useState, type ReactNode } from "react";
import { Link, useLocation } from "react-router-dom";
import { Radar, Moon, Sun, ArrowRight, ShieldCheck } from "lucide-react";
import { cn, useTheme } from "../ds";
import {
  hasConsentChoice,
  setAnalyticsConsent,
  track,
} from "../lib/tracking";

/** 公开站点外壳：顶部导航、页脚、最小化 cookie 同意。与控制台 AppShell 分离。 */
export function PublicShell({ children }: { children: ReactNode }) {
  const location = useLocation();
  const { theme, toggleTheme } = useTheme();
  const dark = theme === "dark";
  const [consentOpen, setConsentOpen] = useState(() => !hasConsentChoice());

  useEffect(() => {
    track("visit", { path: location.pathname });
  }, [location.pathname]);

  const choose = (granted: boolean) => {
    setAnalyticsConsent(granted);
    setConsentOpen(false);
    if (granted) track("visit", { path: location.pathname });
  };

  return (
    <T>
    <div className="min-h-screen bg-canvas flex flex-col">
      <header className="sticky top-0 z-40 border-b border-edge bg-surface-overlay backdrop-blur-md">
        <div className="mx-auto max-w-6xl px-6 h-16 flex items-center justify-between gap-4">
          <Link to="/" className="flex items-center gap-2.5 shrink-0">
            <span className="relative flex items-center justify-center h-8 w-8 rounded-ds-sm bg-[image:var(--ds-gradient-tech)] shadow-md shadow-accent/25">
              <Radar className="h-4 w-4 text-white" />
            </span>
            <span className="font-bold text-ink tracking-tight">
              GeoLens <span className="text-accent">AI</span>
            </span>
          </Link>

          <nav className="hidden md:flex items-center gap-6 text-sm text-ink-muted">
            <a href="/#how" className="hover:text-ink transition-colors">工作原理</a>
            <Link to="/pricing" className="hover:text-ink transition-colors">定价</Link>
            <Link to="/compare" className="hover:text-ink transition-colors">对比</Link>
            <a href="/#faq" className="hover:text-ink transition-colors">常见问题</a>
          </nav>

          <div className="flex items-center gap-2 shrink-0">
            <button
              onClick={toggleTheme}
              className="h-9 w-9 inline-flex items-center justify-center rounded-ds text-ink-muted hover:text-ink hover:bg-canvas transition-colors"
              aria-label={dark ? "切换浅色" : "切换深色"}
            >
              {dark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
            </button>
            <LocaleToggle />
            <Link
              to="/check"
              className="hidden sm:inline-flex h-9 items-center px-3 rounded-ds text-sm font-medium text-ink-muted hover:text-ink hover:bg-canvas transition-colors"
            >
              免费检查
            </Link>
            <Link
              to="/app"
              className="inline-flex h-9 items-center gap-1.5 px-3.5 rounded-ds text-sm font-medium text-white bg-[image:var(--ds-gradient-accent)] shadow-md shadow-accent/20 hover:opacity-90 transition-opacity"
            >
              进入控制台 <ArrowRight className="h-3.5 w-3.5" />
            </Link>
          </div>
        </div>
      </header>

      <main className="flex-1">{children}</main>

      <footer className="border-t border-edge bg-surface">
        <div className="mx-auto max-w-6xl px-6 py-10 grid grid-cols-1 md:grid-cols-4 gap-8">
          <div className="md:col-span-2">
            <div className="flex items-center gap-2">
              <span className="flex items-center justify-center h-7 w-7 rounded-ds-sm bg-[image:var(--ds-gradient-tech)]">
                <Radar className="h-4 w-4 text-white" />
              </span>
              <span className="font-bold text-ink">GeoLens <span className="text-accent">AI</span></span>
            </div>
            <p className="mt-3 text-sm text-ink-muted max-w-sm">
              面向 AEO 的 AI 可见性检查：先看清 AI 现在推荐谁，再决定改什么。
            </p>
            <p className="mt-3 inline-flex items-center gap-1.5 text-xs text-ink-subtle">
              <ShieldCheck className="h-3.5 w-3.5" />
              技术就绪 ≠ 被推荐。我们只报告可核验的事实，不承诺排名或转化结果。
            </p>
          </div>
          <div>
            <p className="text-sm font-semibold text-ink mb-3">产品</p>
            <ul className="space-y-2 text-sm text-ink-muted">
              <li><Link to="/check" className="hover:text-ink">免费站点检查</Link></li>
              <li><Link to="/pricing" className="hover:text-ink">定价与套餐</Link></li>
              <li><Link to="/compare" className="hover:text-ink">方案对比</Link></li>
              <li><Link to="/app" className="hover:text-ink">控制台</Link></li>
            </ul>
          </div>
          <div>
            <p className="text-sm font-semibold text-ink mb-3">法律与支持</p>
            <ul className="space-y-2 text-sm text-ink-muted">
              <li><Link to="/privacy" className="hover:text-ink">隐私政策</Link></li>
              <li><Link to="/terms" className="hover:text-ink">服务条款</Link></li>
              <li><a href="mailto:support@geolens.ai" className="hover:text-ink">联系我们</a></li>
            </ul>
          </div>
        </div>
        <div className="border-t border-edge">
          <div className="mx-auto max-w-6xl px-6 py-4 flex flex-col sm:flex-row justify-between gap-2 text-xs text-ink-subtle">
            <span>© 2026 GeoLens AI. 保留所有权利。</span>
            <span>结果基于公开页面技术条件，不构成对 AI 推荐结果的因果承诺。</span>
          </div>
        </div>
      </footer>

      {consentOpen && (
        <div className="fixed bottom-0 inset-x-0 z-50 border-t border-edge bg-surface-overlay backdrop-blur-md">
          <div className="mx-auto max-w-6xl px-6 py-4 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
            <p className="text-sm text-ink-muted">
              我们仅使用必要的最小化统计来衡量「访问 → 检查 → 结果」漏斗，不采集个人信息、不使用广告追踪。
              详见 <Link to="/privacy" className="text-accent hover:underline">隐私政策</Link>。
            </p>
            <div className="flex items-center gap-2 shrink-0">
              <button
                onClick={() => choose(false)}
                className={cn(
                  "h-9 px-4 rounded-ds text-sm font-medium border border-edge",
                  "text-ink-muted hover:text-ink hover:bg-canvas transition-colors",
                )}
              >
                拒绝
              </button>
              <button
                onClick={() => choose(true)}
                className="h-9 px-4 rounded-ds text-sm font-medium text-white bg-[image:var(--ds-gradient-accent)] hover:opacity-90 transition-opacity"
              >
                同意统计
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
    </T>
  );
}
