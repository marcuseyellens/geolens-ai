import { Routes, Route, Link, useLocation, useNavigate, Navigate } from "react-router-dom";
import {
  LayoutDashboard,
  Crosshair,
  ScanLine,
  Settings,
  Moon,
  Sun,
  Radar,
  BarChart3,
  ShieldCheck,
  ExternalLink,
  CreditCard,
  Activity,
  Cpu,
  Plug,
  LogOut,
  UserRound,
} from "lucide-react";
import { cn, useTheme } from "./ds";
import { AuthProvider, RequireAuth, useAuth } from "./auth";
import { I18nProvider, T, LocaleToggle } from "./i18n";
import DashboardPage from "./routes/dashboard";
import BrandsPage from "./routes/brands";
import BrandDetailPage from "./routes/brand-detail";
import SettingsPage from "./routes/settings";
import DiagnosticReportPage from "./routes/diagnostic-report";
import AuditPage from "./routes/audit";
import GrowthPage from "./routes/growth";
import AcceptancePage from "./routes/acceptance";
import BillingPage from "./routes/billing";
import MonitorPage from "./routes/monitor-page";
import OperationsPage from "./routes/operations";
import MCPDocsPage from "./routes/mcp-docs";
import RedeemPage from "./routes/redeem";
import { PublicShell } from "./routes/public-shell";
import LandingPage from "./routes/landing";
import CheckPage, { SharedResultPage } from "./routes/check";
import PricingPage from "./routes/pricing";
import LegalPage from "./routes/legal";
import ComparePage from "./routes/compare";
import LoginPage from "./routes/login";
import SubscribePage from "./routes/subscribe";
import CompetitorPage from "./routes/competitor-page";
import GuidePage from "./routes/guide";

const navItems = [
  { path: "/app", label: "概览", icon: LayoutDashboard },
  { path: "/app/brands", label: "品牌", icon: Crosshair },
  { path: "/app/audit", label: "站点检查", icon: ScanLine },
  { path: "/app/growth", label: "增长看板", icon: BarChart3 },
  { path: "/app/acceptance", label: "发布验收", icon: ShieldCheck },
  { path: "/app/billing", label: "套餐账单", icon: CreditCard },
  { path: "/app/monitor", label: "监控发布", icon: Activity },
  { path: "/app/competitor", label: "竞品观察", icon: Radar },
  { path: "/app/operations", label: "运行治理", icon: Cpu },
  { path: "/app/mcp", label: "MCP 接入", icon: Plug },
  { path: "/app/settings", label: "设置", icon: Settings },
];

/** 控制台外壳：侧边栏 + 内部路由。 */
function AppShell() {
  const location = useLocation();
  const { theme, toggleTheme } = useTheme();
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const dark = theme === "dark";

  async function signOut() {
    await logout();
    navigate("/login", { replace: true });
  }

  return (
    <T>
    <div className="flex min-h-screen bg-canvas">
      <aside className="w-56 shrink-0 border-r border-edge bg-surface flex flex-col relative">
        <div className="absolute top-0 left-0 right-0 h-px bg-[image:var(--ds-gradient-tech)] opacity-60" />

        <Link to="/" className="px-5 py-5 flex items-center gap-2.5 group">
          <div className="relative flex items-center justify-center h-9 w-9 rounded-ds-sm bg-[image:var(--ds-gradient-tech)] shadow-lg shadow-accent/25">
            <Radar className="h-5 w-5 text-white" />
            <div className="absolute inset-0 rounded-ds-sm bg-[image:var(--ds-gradient-tech)] opacity-40 blur-sm -z-10" />
          </div>
          <div>
            <span className="font-bold text-ink tracking-tight">GeoLens</span>
            <span className="text-xs font-medium text-accent ml-1">AI</span>
          </div>
        </Link>

        <nav className="flex-1 px-3 py-2 space-y-1">
          {navItems.map((item) => {
            const active =
              item.path === "/app"
                ? location.pathname === "/app" || location.pathname === "/app/"
                : location.pathname.startsWith(item.path);
            const Icon = item.icon;
            return (
              <Link
                key={item.path}
                to={item.path}
                className={cn(
                  "relative flex items-center gap-2.5 px-3 py-2 rounded-ds text-sm font-medium transition-all",
                  active
                    ? "bg-accent-subtle text-accent"
                    : "text-ink-muted hover:text-ink hover:bg-canvas",
                )}
              >
                {active && (
                  <span className="absolute left-0 top-1/2 -translate-y-1/2 h-5 w-0.5 rounded-r-full bg-[image:var(--ds-gradient-accent)]" />
                )}
                <Icon className={cn("h-4 w-4 transition-transform", active && "scale-110")} />
                {item.label}
              </Link>
            );
          })}
        </nav>

        <div className="px-3 py-3 border-t border-edge space-y-1">
          {user && (
            <div className="px-3 py-2 rounded-ds bg-canvas">
              <div className="flex items-center gap-2 text-xs text-ink-muted">
                <UserRound className="h-3.5 w-3.5 shrink-0" />
                <span className="truncate" title={user.email}>{user.email}</span>
              </div>
              <div className="mt-1 text-[11px] text-ink-subtle">
                {user.role === "admin" ? "管理员" : "普通账户"} · {user.account_id}
              </div>
            </div>
          )}
          <Link
            to="/"
            className="flex items-center gap-2 px-3 py-2 rounded-ds text-sm text-ink-muted hover:text-ink hover:bg-canvas transition-colors"
          >
            <ExternalLink className="h-4 w-4" />
            返回官网
          </Link>
          <button
            onClick={toggleTheme}
            className="flex items-center gap-2 px-3 py-2 rounded-ds text-sm text-ink-muted hover:text-ink hover:bg-canvas w-full transition-colors"
          >
            {dark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
            {dark ? "浅色" : "深色"}
          </button>
          <LocaleToggle compact />
          <button
            onClick={signOut}
            className="flex items-center gap-2 px-3 py-2 rounded-ds text-sm text-ink-muted hover:text-negative hover:bg-canvas w-full transition-colors"
          >
            <LogOut className="h-4 w-4" />
            退出登录
          </button>
        </div>
      </aside>

      <main className="flex-1 overflow-y-auto">
        <Routes>
          <Route path="/" element={<DashboardPage />} />
          <Route path="brands" element={<BrandsPage />} />
          <Route path="brands/:id" element={<BrandDetailPage />} />
          <Route path="brands/:id/diagnostic" element={<DiagnosticReportPage />} />
          <Route path="audit" element={<AuditPage />} />
          <Route path="growth" element={<GrowthPage />} />
          <Route path="acceptance" element={<AcceptancePage />} />
          <Route path="billing" element={<BillingPage />} />
          <Route path="monitor" element={<MonitorPage />} />
          <Route path="competitor" element={<CompetitorPage />} />
          <Route path="operations" element={<OperationsPage />} />
          <Route path="mcp" element={<MCPDocsPage />} />
          <Route path="settings" element={<SettingsPage />} />
          <Route path="*" element={<Navigate to="/app" replace />} />
        </Routes>
      </main>
    </div>
    </T>
  );
}

export default function App() {
  return (
    <I18nProvider>
    <AuthProvider>
      <Routes>
        <Route path="/" element={<PublicShell><LandingPage /></PublicShell>} />
        <Route path="/check" element={<PublicShell><CheckPage /></PublicShell>} />
        <Route path="/r/:token" element={<PublicShell><SharedResultPage /></PublicShell>} />
        <Route path="/pricing" element={<PublicShell><PricingPage /></PublicShell>} />
        <Route path="/compare" element={<PublicShell><ComparePage /></PublicShell>} />
        <Route path="/redeem" element={<PublicShell><RedeemPage /></PublicShell>} />
        <Route path="/guide" element={<PublicShell><GuidePage /></PublicShell>} />
        <Route path="/privacy" element={<PublicShell><LegalPage kind="privacy" /></PublicShell>} />
        <Route path="/terms" element={<PublicShell><LegalPage kind="terms" /></PublicShell>} />
        <Route path="/login" element={<PublicShell><LoginPage /></PublicShell>} />
        <Route path="/subscribe" element={<RequireAuth><PublicShell><SubscribePage /></PublicShell></RequireAuth>} />
        <Route path="/app/*" element={<RequireAuth><AppShell /></RequireAuth>} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </AuthProvider>
    </I18nProvider>
  );
}
