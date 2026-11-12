import { Routes, Route, Link, useLocation } from "react-router-dom";
import { LayoutDashboard, Target, FileBarChart, Settings, Moon, Sun } from "lucide-react";
import { cn, useTheme } from "./ds";
import DashboardPage from "./routes/dashboard";
import BrandsPage from "./routes/brands";
import BrandDetailPage from "./routes/brand-detail";
import SettingsPage from "./routes/settings";
import DiagnosticReportPage from "./routes/diagnostic-report";

const navItems = [
  { path: "/", label: "概览", icon: LayoutDashboard },
  { path: "/brands", label: "品牌", icon: Target },
  { path: "/settings", label: "设置", icon: Settings },
];

export default function App() {
  const location = useLocation();
  const { theme, toggleTheme } = useTheme();
  const dark = theme === "dark";

  return (
    <div className="flex min-h-screen bg-canvas">
      {/* Sidebar */}
      <aside className="w-56 shrink-0 border-r border-edge bg-surface flex flex-col">
        <div className="px-5 py-5 flex items-center gap-2">
          <div className="h-8 w-8 rounded-ds-sm bg-accent flex items-center justify-center">
            <FileBarChart className="h-5 w-5 text-white" />
          </div>
          <span className="font-semibold text-ink">GeoLens AI</span>
        </div>
        <nav className="flex-1 px-3 py-2 space-y-1">
          {navItems.map((item) => {
            const active =
              item.path === "/"
                ? location.pathname === "/"
                : location.pathname.startsWith(item.path);
            const Icon = item.icon;
            return (
              <Link
                key={item.path}
                to={item.path}
                className={cn(
                  "flex items-center gap-2.5 px-3 py-2 rounded-ds text-sm font-medium transition-colors",
                  active
                    ? "bg-accent-subtle text-accent"
                    : "text-ink-muted hover:text-ink hover:bg-canvas",
                )}
              >
                <Icon className="h-4 w-4" />
                {item.label}
              </Link>
            );
          })}
        </nav>
        <div className="px-3 py-3 border-t border-edge">
          <button
            onClick={toggleTheme}
            className="flex items-center gap-2 px-3 py-2 rounded-ds text-sm text-ink-muted hover:text-ink hover:bg-canvas w-full"
          >
            {dark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
            {dark ? "浅色" : "深色"}
          </button>
        </div>
      </aside>

      {/* Main */}
      <main className="flex-1 overflow-y-auto">
        <Routes>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/brands" element={<BrandsPage />} />
          <Route path="/brands/:id" element={<BrandDetailPage />} />
          <Route path="/brands/:id/diagnostic" element={<DiagnosticReportPage />} />
          <Route path="/settings" element={<SettingsPage />} />
        </Routes>
      </main>
    </div>
  );
}
