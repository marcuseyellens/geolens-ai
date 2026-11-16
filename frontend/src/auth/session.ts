/** 会话令牌的本地存取。独立模块，避免 api.ts 与 AuthProvider 循环依赖。 */
export const TOKEN_KEY = "geolens.auth.token";
export const UNAUTHORIZED_EVENT = "geolens:unauthorized";

export function readToken(): string | null {
  try {
    return window.localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

export function writeToken(token: string | null) {
  try {
    if (token) window.localStorage.setItem(TOKEN_KEY, token);
    else window.localStorage.removeItem(TOKEN_KEY);
  } catch {
    // 隐私模式下 localStorage 不可写：本页仍可用，刷新后需重新登录
  }
}

export function notifyUnauthorized() {
  window.dispatchEvent(new Event(UNAUTHORIZED_EVENT));
}
