/**
 * WP-6.6 转化埋点：只记录漏斗事件计数，不采集任何个人信息。
 * 用户拒绝 cookie 时完全降级：不发请求、不写本地存储、不生成指纹。
 */
export type FunnelEvent =
  | "visit"
  | "check"
  | "result"
  | "share"
  | "signup"
  | "checkout_39"
  | "subscribe";

export const FUNNEL_STEPS: FunnelEvent[] = [
  "visit",
  "check",
  "result",
  "share",
  "signup",
  "checkout_39",
  "subscribe",
];

const CONSENT_KEY = "geolens.analytics.consent";

export function hasConsentChoice(): boolean {
  try {
    return window.localStorage.getItem(CONSENT_KEY) !== null;
  } catch {
    return true; // 存储不可用时不再打扰用户
  }
}

export function analyticsGranted(): boolean {
  try {
    return window.localStorage.getItem(CONSENT_KEY) === "granted";
  } catch {
    return false;
  }
}

export function setAnalyticsConsent(granted: boolean): void {
  try {
    window.localStorage.setItem(CONSENT_KEY, granted ? "granted" : "denied");
  } catch {
    // 隐私模式下无法持久化，本次会话仍按用户选择处理
  }
  if (!granted) {
    try {
      window.localStorage.removeItem("geolens.analytics.session");
    } catch {
      // ignore
    }
  }
}

export function track(event: FunnelEvent, meta: Record<string, string | number> = {}): void {
  if (!analyticsGranted()) return;
  const body = JSON.stringify({ event, ...meta, ts: Date.now() });
  try {
    if (navigator.sendBeacon) {
      navigator.sendBeacon(
        "/api/v1/events",
        new Blob([body], { type: "application/json" }),
      );
      return;
    }
  } catch {
    // 降级到 fetch
  }
  void fetch("/api/v1/events", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body,
    keepalive: true,
  }).catch(() => {});
}
