import { T } from "../i18n";
import { useState, type FormEvent } from "react";
import { Link, Navigate, useLocation, useNavigate } from "react-router-dom";
import { LogIn, UserPlus, LockKeyhole, ArrowRight, ShieldCheck } from "lucide-react";
import { Card, Button, Callout, Badge } from "../ds";
import { useAuth } from "../auth";

const DEMO_EMAIL = "demo@geolens.ai";
const DEMO_PASSWORD = "demo1234";

/** 只接受站内相对路径，避免 next 参数造成开放重定向。 */
function safeNext(raw: string | null): string {
  if (!raw || !raw.startsWith("/") || raw.startsWith("//")) return "/app";
  return raw;
}

export default function LoginPage() {
  const { user, login, register } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const next = safeNext(new URLSearchParams(location.search).get("next"));

  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  if (user) return <Navigate to={next} replace />;

  async function submit(ev: FormEvent) {
    ev.preventDefault();
    setBusy(true);
    setError(null);
    try {
      if (mode === "login") await login(email.trim(), password);
      else await register(email.trim(), password);
      navigate(next, { replace: true });
    } catch (e) {
      setError(e instanceof Error ? e.message : "操作失败，请重试");
    } finally {
      setBusy(false);
    }
  }

  return (
    <T>
    <div className="mx-auto max-w-6xl px-6 py-16 flex justify-center">
      <div className="w-full max-w-md">
        <div className="text-center">
          <Badge tone="accent">账户</Badge>
          <h1 className="mt-4 text-2xl font-bold text-ink tracking-tight">
            {mode === "login" ? "登录控制台" : "创建账户"}
          </h1>
          <p className="mt-2 text-sm text-ink-muted">
            控制台与订阅需要登录；公开的免费站点检查无需账户。
          </p>
        </div>

        <Card raised className="mt-8 p-6">
          <div className="flex rounded-ds bg-canvas p-1 mb-6">
            <button
              type="button"
              onClick={() => { setMode("login"); setError(null); }}
              className={`flex-1 h-9 rounded-ds-sm text-sm font-medium transition-colors ${mode === "login" ? "bg-surface text-ink shadow-sm" : "text-ink-muted hover:text-ink"}`}
            >
              登录
            </button>
            <button
              type="button"
              onClick={() => { setMode("register"); setError(null); }}
              className={`flex-1 h-9 rounded-ds-sm text-sm font-medium transition-colors ${mode === "register" ? "bg-surface text-ink shadow-sm" : "text-ink-muted hover:text-ink"}`}
            >
              注册
            </button>
          </div>

          <form onSubmit={submit} className="space-y-4">
            <label className="block">
              <span className="text-xs font-medium text-ink-muted">邮箱</span>
              <input
                type="email" required autoComplete="email" value={email}
                onChange={(e) => setEmail(e.target.value)} placeholder="you@example.com"
                className="mt-1.5 w-full h-10 px-3 rounded-ds border border-edge bg-surface text-sm text-ink outline-none focus:border-accent"
              />
            </label>
            <label className="block">
              <span className="text-xs font-medium text-ink-muted">密码</span>
              <input
                type="password" required minLength={8} autoComplete={mode === "login" ? "current-password" : "new-password"}
                value={password} onChange={(e) => setPassword(e.target.value)} placeholder="至少 8 位"
                className="mt-1.5 w-full h-10 px-3 rounded-ds border border-edge bg-surface text-sm text-ink outline-none focus:border-accent"
              />
            </label>

            {error && <Callout tone="negative" title="无法完成">{error}</Callout>}

            <Button type="submit" className="w-full" disabled={busy}>
              {mode === "login" ? <LogIn className="h-4 w-4" /> : <UserPlus className="h-4 w-4" />}
              {busy ? "处理中…" : mode === "login" ? "登录" : "注册并登录"}
              {!busy && <ArrowRight className="h-4 w-4" />}
            </Button>
          </form>

          <div className="mt-6 pt-5 border-t border-edge">
            <div className="flex items-start gap-2 text-xs text-ink-muted">
              <LockKeyhole className="h-4 w-4 mt-0.5 shrink-0 text-accent" />
              <div>
                <p className="font-medium text-ink">本地演示账户</p>
                <p className="mt-1 tabular-nums">{DEMO_EMAIL} / {DEMO_PASSWORD}</p>
                <p className="mt-1">具备管理员角色，可操作预算复位、回滚与令牌签发。</p>
              </div>
            </div>
            <Button
              variant="secondary" size="sm" className="mt-3 w-full"
              onClick={() => { setMode("login"); setEmail(DEMO_EMAIL); setPassword(DEMO_PASSWORD); setError(null); }}
            >
              填入演示账户
            </Button>
          </div>
        </Card>

        <p className="mt-5 flex items-center justify-center gap-1.5 text-xs text-ink-subtle">
          <ShieldCheck className="h-3.5 w-3.5" />
          会话仅用于识别账户，免费检查与分享结果保持匿名。
        </p>
        <p className="mt-3 text-center text-xs text-ink-subtle">
          返回 <Link to="/" className="text-accent hover:underline">首页</Link> ·
          查看 <Link to="/pricing" className="text-accent hover:underline">定价</Link>
        </p>
      </div>
    </div>
    </T>
  );
}
