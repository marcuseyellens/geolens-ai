import {
  cloneElement, createContext, isValidElement, useCallback, useContext,
  useEffect, useMemo, useState, type ReactElement, type ReactNode,
} from "react";
import { Languages } from "lucide-react";
import { EN } from "./en";
import { EXTRA, LOCALES, SECTION_COPY } from "./extra";

export type Locale =
  | "zh" | "en" | "zh-Hant" | "ja" | "ko" | "es" | "fr" | "de";

const STORAGE_KEY = "geolens-locale";
const AUTO_KEY = "geolens-locale-auto-cache";
const AUTO_TTL = 24 * 60 * 60 * 1000;

/** 中文原文即 key：zh 直接返回原文，其他语言查字典并自动回退英文。 */
function readLocale(): Locale {
  try {
    const saved = window.localStorage.getItem(STORAGE_KEY);
    return LOCALES.some((l) => l.id === saved) ? (saved as Locale) : "zh";
  } catch {
    return "zh";
  }
}

function hasManualLocale(): boolean {
  try {
    const saved = window.localStorage.getItem(STORAGE_KEY);
    return Boolean(saved && LOCALES.some((l) => l.id === saved));
  } catch {
    return false;
  }
}

const COUNTRY_TO_LOCALE: Record<string, Locale> = {
  CN: "zh", SG: "zh", TW: "zh-Hant", HK: "zh-Hant", MO: "zh-Hant",
  JP: "ja", KR: "ko",
  ES: "es", MX: "es", AR: "es", CO: "es", PE: "es", CL: "es", UY: "es", VE: "es",
  FR: "fr", BE: "fr", LU: "fr",
  DE: "de", AT: "de", CH: "de",
};

function localeFromCountry(code: unknown): Locale | null {
  if (typeof code !== "string") return null;
  return COUNTRY_TO_LOCALE[code.toUpperCase()] ?? "en";
}

/**
 * 按请求方 IP 自动识别界面语言。只写自动缓存，不写 STORAGE_KEY，
 * 因此用户在下拉框里选过的语言永远优先，E2E 预置 localStorage 也不受影响。
 * IP 服务不可达时保持默认中文，不猜 navigator.language。
 */
async function detectLocaleByIP(): Promise<Locale | null> {
  const controller = new AbortController();
  const timer = window.setTimeout(() => controller.abort(), 3000);
  try {
    const response = await fetch("https://ipwho.is/", {
      cache: "no-store",
      signal: controller.signal,
    });
    if (!response.ok) return null;
    const data = (await response.json()) as { success?: boolean; country_code?: string };
    if (!data.success) return null;
    return localeFromCountry(data.country_code);
  } catch {
    return null;
  } finally {
    window.clearTimeout(timer);
  }
}

async function resolveAutoLocale(): Promise<Locale | null> {
  try {
    const raw = window.localStorage.getItem(AUTO_KEY);
    if (raw) {
      const cache = JSON.parse(raw) as { locale: Locale; at: number };
      if (Date.now() - cache.at < AUTO_TTL && LOCALES.some((l) => l.id === cache.locale)) {
        return cache.locale;
      }
    }
  } catch {
    // 缓存损坏则重新识别
  }
  const detected = await detectLocaleByIP();
  if (!detected) return null;
  try {
    window.localStorage.setItem(AUTO_KEY, JSON.stringify({ locale: detected, at: Date.now() }));
  } catch {
    // 隐私模式下不缓存，本页仍然生效
  }
  return detected;
}

function persist(locale: Locale) {
  try {
    window.localStorage.setItem(STORAGE_KEY, locale);
  } catch {
    // 隐私模式下不可写，本页切换仍生效
  }
}

export function applyLocale(locale: Locale) {
  const meta = LOCALES.find((l) => l.id === locale);
  document.documentElement.lang = meta?.htmlLang ?? "zh-CN";
}

function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

interface Template {
  re: RegExp;
  value: string;
  holes: number;
}

interface Matcher {
  dict: Record<string, string>;
  templates: Template[];
  fallback: RegExp;
}

/**
 * 每种语言合并 EN 基线与自己的覆盖词条：新增语种只需翻译客户最常看到的
 * 市场化文案，后台与长尾文案自动回退英文，不会出现半中半外文的界面。
 */
const matcherCache = new Map<Locale, Matcher>();

function matcherFor(locale: Locale): Matcher {
  const cached = matcherCache.get(locale);
  if (cached) return cached;
  const dict: Record<string, string> = locale === "zh" ? {} : {
    ...EN, ...(EXTRA[locale] ?? {}), ...(SECTION_COPY[locale] ?? {}),
  };
  const templates: Template[] = Object.entries(dict)
    .filter(([key]) => key.includes("{n}"))
    .map(([key, value]) => {
      const parts = key.split("{n}");
      const pattern = `^${parts.map(escapeRegExp).join("(\\d[\\d,]*(?:\\.\\d+)?)")}$`;
      return { re: new RegExp(pattern), value, holes: parts.length - 1 };
    });
  const fallbackKeys = Object.keys(dict)
    .filter((key) => /[\u4e00-\u9fff]/.test(key) && !key.includes("{n}"))
    .sort((a, b) => b.length - a.length);
  const matcher: Matcher = {
    dict, templates,
    fallback: new RegExp(fallbackKeys.map(escapeRegExp).join("|"), "g"),
  };
  matcherCache.set(locale, matcher);
  return matcher;
}

/** JSX 会把换行与缩进折叠成空格，字典 key 与运行时文本都按同一规则规范化。 */
export function normalizeKey(text: string): string {
  return text.replace(/\s+/g, " ").trim();
}

/** 翻译单个字符串；保留首尾空白，支持精确匹配、数字模板与片段兜底。 */
export function translate(text: string, locale: Locale): string {
  if (locale === "zh" || !text) return text;
  const leading = text.match(/^\s*/)?.[0] ?? "";
  const trailing = text.match(/\s*$/)?.[0] ?? "";
  const core = normalizeKey(text);
  if (!core) return text;

  const m = matcherFor(locale);
  const exact = m.dict[core];
  if (exact !== undefined) return leading + exact + trailing;

  for (const tpl of m.templates) {
    const match = tpl.re.exec(core);
    if (!match) continue;
    let out = tpl.value;
    for (let i = 1; i <= tpl.holes; i += 1) {
      out = out.replace("{n}", match[i] ?? "");
    }
    return leading + out + trailing;
  }
  const replaced = core.replace(m.fallback, (match) => m.dict[match] ?? match);
  return replaced === core ? text : leading + replaced + trailing;
}

/** 只翻译展示用途的字符串属性，绝不碰 name / value / id / to / href / key。 */
const DISPLAY_PROPS = [
  "title", "placeholder", "aria-label", "alt", "label", "hint", "value",
  "description", "text", "note", "emptyText", "header",
];
// 原生表单的 value 是数据，绝不能翻译，否则会破坏输入与提交。
const NATIVE_VALUE_TAGS = new Set(["input", "textarea", "select", "option"]);

function translateNode(node: ReactNode, locale: Locale): ReactNode {
  if (locale === "zh") return node;
  if (typeof node === "string") return translate(node, locale);
  if (typeof node === "number" || typeof node === "boolean" || node == null) return node;
  if (Array.isArray(node)) return node.map((child) => translateNode(child, locale));
  if (!isValidElement(node)) return node;

  const element = node as ReactElement<Record<string, unknown>>;
  const props: Record<string, unknown> = {};
  let changed = false;

  if (element.props.children !== undefined) {
    props.children = translateNode(element.props.children as ReactNode, locale);
    changed = true;
  }
  for (const [key, value] of Object.entries(element.props)) {
    if (key === "children") continue;
    if (typeof value === "string") {
      if (key === "value" && typeof element.type === "string" && NATIVE_VALUE_TAGS.has(element.type)) {
        continue;
      }
      if (DISPLAY_PROPS.includes(key)) {
        props[key] = translate(value, locale);
        changed = true;
      }
    } else if (isValidElement(value) || (Array.isArray(value) && value.some(isValidElement))) {
      // PageHeader.actions、EmptyState.action 等把 JSX 放在普通 prop 里，
      // 也要递归翻译，否则按钮文案会漏。
      props[key] = translateNode(value as ReactNode, locale);
      changed = true;
    }
  }
  return changed ? cloneElement(element, props) : node;
}

interface I18nState {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (text: string) => string;
}

const I18nContext = createContext<I18nState | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(readLocale);

  useEffect(() => { applyLocale(locale); }, [locale]);

  useEffect(() => {
    if (hasManualLocale()) return;
    let active = true;
    void resolveAutoLocale().then((detected) => {
      if (active && detected && !hasManualLocale()) setLocaleState(detected);
    });
    return () => { active = false; };
  }, []);

  const setLocale = useCallback((next: Locale) => {
    persist(next);
    setLocaleState(next);
  }, []);

  const t = useCallback((text: string) => translate(text, locale), [locale]);
  const value = useMemo<I18nState>(() => ({ locale, setLocale, t }), [locale, setLocale, t]);
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nState {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error("useI18n must be used inside I18nProvider");
  return ctx;
}

/**
 * 页面级翻译边界。递归遍历本组件渲染的 React 节点树，把文本节点与展示类
 * 字符串属性按字典翻译。React 组件本身是黑盒，因此每个页面与两个外壳都要
 * 在自己的 return 里包一层 T；locale=zh 时直接返回原树，零开销零回归。
 */
export function T({ children }: { children: ReactNode }) {
  const { locale } = useI18n();
  return <>{translateNode(children, locale)}</>;
}

/** 语言选择器：原生 select 保证键盘可达、屏幕阅读器可读、移动端可点。 */
export function LocaleToggle({ compact = false }: { compact?: boolean }) {
  // 该控件渲染在两个外壳的 T 树内部，但组件自身是黑盒，需显式调用 t()。
  const { locale, setLocale, t } = useI18n();
  return (
    <label className={compact
      ? "flex items-center gap-2 px-3 py-2 rounded-ds text-sm text-ink-muted hover:bg-canvas w-full transition-colors"
      : "inline-flex items-center gap-1.5 h-9 px-2 rounded-ds text-xs font-semibold text-ink-muted hover:text-ink hover:bg-canvas transition-colors"}>
      <Languages className="h-4 w-4 shrink-0" aria-hidden="true" />
      {compact && <span className="text-xs">{t("语言")}</span>}
      <select
        data-testid="locale-select"
        aria-label="Interface language"
        title={t("界面语言")}
        value={locale}
        onChange={(e) => setLocale(e.target.value as Locale)}
        className={compact
          ? "flex-1 min-w-0 bg-transparent text-sm text-ink cursor-pointer outline-none focus-visible:ring-2 focus-visible:ring-accent rounded-ds"
          : "bg-transparent text-xs font-semibold text-ink cursor-pointer outline-none focus-visible:ring-2 focus-visible:ring-accent rounded-ds"}
      >
        {LOCALES.map((l) => <option key={l.id} value={l.id}>{t(l.native)}</option>)}
      </select>
    </label>
  );
}
