import {
  cloneElement, createContext, isValidElement, useCallback, useContext,
  useEffect, useMemo, useState, type ReactElement, type ReactNode,
} from "react";
import { EN } from "./en";

export type Locale = "zh" | "en";

const STORAGE_KEY = "geolens-locale";

/** 中文原文即 key：zh 直接返回原文，en 查字典，未收录时优雅回退中文。 */
function readLocale(): Locale {
  try {
    return window.localStorage.getItem(STORAGE_KEY) === "en" ? "en" : "zh";
  } catch {
    return "zh";
  }
}

function persist(locale: Locale) {
  try {
    window.localStorage.setItem(STORAGE_KEY, locale);
  } catch {
    // 隐私模式下不可写，本页切换仍生效
  }
}

export function applyLocale(locale: Locale) {
  document.documentElement.lang = locale === "zh" ? "zh-CN" : "en";
}

function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

interface Template {
  re: RegExp;
  value: string;
  holes: number;
}

/** 把字典里含 {n} 的条目编译成正则，匹配渲染时已插入数字的文本。 */
const templates: Template[] = Object.entries(EN)
  .filter(([key]) => key.includes("{n}"))
  .map(([key, value]) => {
    const parts = key.split("{n}");
    const pattern = `^${parts.map(escapeRegExp).join("(\\d[\\d,]*(?:\\.\\d+)?)")}$`;
    return { re: new RegExp(pattern), value, holes: parts.length - 1 };
  });

/** JSX 会把换行与缩进折叠成空格，字典 key 与运行时文本都按同一规则规范化。 */
export function normalizeKey(text: string): string {
  return text.replace(/\s+/g, " ").trim();
}

/**
 * 兜底：整串未命中时，把字典里较长的中文片段逐段替换。
 * 用于「上限 $50」「拒绝 3 次」这类数字/货币与中文混排且未单列模板的文案。
 */
const fallbackKeys = Object.keys(EN)
  .filter((key) => /[\u4e00-\u9fff]/.test(key) && !key.includes("{n}"))
  .sort((a, b) => b.length - a.length);
const fallbackPattern = new RegExp(fallbackKeys.map(escapeRegExp).join("|"), "g");

/** 翻译单个字符串；保留首尾空白，支持精确匹配、数字模板与片段兜底。 */
export function translate(text: string, locale: Locale): string {
  if (locale === "zh" || !text) return text;
  const leading = text.match(/^\s*/)?.[0] ?? "";
  const trailing = text.match(/\s*$/)?.[0] ?? "";
  const core = normalizeKey(text);
  if (!core) return text;

  const exact = EN[core];
  if (exact !== undefined) return leading + exact + trailing;

  for (const tpl of templates) {
    const match = tpl.re.exec(core);
    if (!match) continue;
    let out = tpl.value;
    for (let i = 1; i <= tpl.holes; i += 1) {
      out = out.replace("{n}", match[i] ?? "");
    }
    return leading + out + trailing;
  }
  const replaced = core.replace(fallbackPattern, (match) => EN[match] ?? match);
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

/** 语言切换按钮，放在公开页头部与控制台侧栏。 */
export function LocaleToggle({ compact = false }: { compact?: boolean }) {
  // 该控件渲染在两个外壳的 T 树内部，但组件自身是黑盒，需显式调用 t()。
  const { locale, setLocale, t } = useI18n();
  const next: Locale = locale === "zh" ? "en" : "zh";
  return (
    <button
      type="button"
      onClick={() => setLocale(next)}
      aria-label={locale === "zh" ? "Switch to English" : "Switch to Chinese"}
      title={locale === "zh" ? "Switch to English" : "Switch to Chinese"}
      className={compact
        ? "flex items-center gap-2 px-3 py-2 rounded-ds text-sm text-ink-muted hover:text-ink hover:bg-canvas w-full transition-colors"
        : "h-9 px-2.5 inline-flex items-center justify-center rounded-ds text-xs font-semibold text-ink-muted hover:text-ink hover:bg-canvas transition-colors"}
    >
      {compact && <span className="text-xs">{t("语言")}</span>}
      <span className="tabular-nums">{locale === "zh" ? "EN" : "Chinese"}</span>
    </button>
  );
}
