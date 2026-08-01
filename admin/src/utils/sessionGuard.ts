import {
  AUTH_REFRESH_TOKEN_KEY,
  AUTH_SESSION_LEGACY,
  AUTH_SESSION_MODE_KEY,
  AUTH_TOKEN_EXPIRES_KEY,
  AUTH_TOKEN_KEY,
} from '@/constants/auth';

/**
 * 会话守卫：统一管理 access token 的保存、临期静默续期（POST /api/v1/auth/refresh）
 * 和 401 后的「登录已过期」重新登录协调，避免整页跳转导致表单内容丢失。
 */

export type SessionCredentials = {
  token: string;
  /** unix seconds */
  expiresAt?: number;
  /** legacy_local_storage 模式下后端在响应体返回；secure_session 模式走 HttpOnly cookie */
  refreshToken?: string;
  sessionMode?: string;
};

/** token 剩余有效期低于该阈值时触发静默续期 */
export const TOKEN_REFRESH_THRESHOLD_SECONDS = 5 * 60;

export function saveSessionCredentials(cred: SessionCredentials) {
  localStorage.setItem(AUTH_TOKEN_KEY, cred.token);
  if (typeof cred.expiresAt === 'number' && Number.isFinite(cred.expiresAt) && cred.expiresAt > 0) {
    localStorage.setItem(AUTH_TOKEN_EXPIRES_KEY, String(cred.expiresAt));
  } else {
    localStorage.removeItem(AUTH_TOKEN_EXPIRES_KEY);
  }
  if (cred.refreshToken) {
    localStorage.setItem(AUTH_REFRESH_TOKEN_KEY, cred.refreshToken);
  }
  if (cred.sessionMode) {
    localStorage.setItem(AUTH_SESSION_MODE_KEY, cred.sessionMode);
  }
}

export function clearSessionCredentials() {
  localStorage.removeItem(AUTH_TOKEN_KEY);
  localStorage.removeItem(AUTH_TOKEN_EXPIRES_KEY);
  localStorage.removeItem(AUTH_REFRESH_TOKEN_KEY);
}

/** 已登录且 token 剩余有效期低于阈值（或已过期）时返回 true */
export function shouldRefreshSoon(nowMs: number = Date.now()): boolean {
  if (!localStorage.getItem(AUTH_TOKEN_KEY)) return false;
  const raw = localStorage.getItem(AUTH_TOKEN_EXPIRES_KEY);
  if (!raw) return false;
  const exp = Number(raw);
  if (!Number.isFinite(exp) || exp <= 0) return false;
  return exp * 1000 - nowMs < TOKEN_REFRESH_THRESHOLD_SECONDS * 1000;
}

/** auth 链路自身的 URL，不参与 401 重试 / 临期续期 */
export function isAuthUrl(url: string): boolean {
  return (
    url.includes('/auth/login') || url.includes('/auth/refresh') || url.includes('/auth/register')
  );
}

/** 续期失败后的冷却期，避免每个请求都重复敲 /auth/refresh */
const REFRESH_FAILURE_COOLDOWN_MS = 60 * 1000;

let lastRefreshFailureAt = 0;

/**
 * 当前会话是否具备续期能力：
 * legacy_local_storage 模式且无存储的 refreshToken 时，后端 /auth/refresh 必然 401，直接走重登弹窗。
 */
export function canAttemptRefresh(nowMs: number = Date.now()): boolean {
  if (nowMs - lastRefreshFailureAt < REFRESH_FAILURE_COOLDOWN_MS) return false;
  const mode = localStorage.getItem(AUTH_SESSION_MODE_KEY);
  if (mode === AUTH_SESSION_LEGACY && !localStorage.getItem(AUTH_REFRESH_TOKEN_KEY)) {
    return false;
  }
  return true;
}

async function doRefresh(): Promise<boolean> {
  const refreshToken = localStorage.getItem(AUTH_REFRESH_TOKEN_KEY) || '';
  try {
    // 用原生 fetch 绕开 umi request 拦截器，避免续期请求自身再触发 401 重试链
    const resp = await fetch('/api/v1/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      body: JSON.stringify(refreshToken ? { refreshToken } : {}),
    });
    const json = (await resp.json()) as {
      code: number;
      data?: { token?: string; expiresAt?: number; refreshToken?: string };
    };
    if (!resp.ok || json.code !== 0 || !json.data?.token) return false;
    saveSessionCredentials({
      token: json.data.token,
      expiresAt: json.data.expiresAt,
      refreshToken: json.data.refreshToken,
    });
    return true;
  } catch {
    return false;
  }
}

let refreshInFlight: Promise<boolean> | null = null;

/** 静默续期（single-flight：并发调用共享同一次请求；不具备续期能力或冷却期内直接返回 false） */
export function refreshAccessToken(): Promise<boolean> {
  if (!canAttemptRefresh()) return Promise.resolve(false);
  if (!refreshInFlight) {
    refreshInFlight = doRefresh().then((ok) => {
      if (!ok) lastRefreshFailureAt = Date.now();
      else lastRefreshFailureAt = 0;
      refreshInFlight = null;
      return ok;
    });
  }
  return refreshInFlight;
}

/** 仅供测试：重置续期失败冷却状态 */
export function resetRefreshFailureCooldown() {
  lastRefreshFailureAt = 0;
}

type ReloginHandler = () => Promise<boolean>;

let reloginHandler: ReloginHandler | null = null;
let reloginInFlight: Promise<boolean> | null = null;

/** 由「登录已过期」弹窗组件注册；返回 Promise，重新登录成功 resolve(true) */
export function registerReloginHandler(handler: ReloginHandler | null) {
  reloginHandler = handler;
}

/** 弹出重新登录弹窗并等待结果（single-flight：并发 401 共享同一个弹窗） */
export function requireRelogin(): Promise<boolean> {
  if (!reloginHandler) return Promise.resolve(false);
  if (!reloginInFlight) {
    reloginInFlight = reloginHandler().finally(() => {
      reloginInFlight = null;
    });
  }
  return reloginInFlight;
}

/** 用户放弃重新登录时的兜底：清凭证并整页跳登录页 */
export function redirectToLoginPage() {
  clearSessionCredentials();
  const path = window.location.pathname;
  if (path === '/user/login' || path.startsWith('/user/login')) return;
  window.location.assign(
    `${window.location.origin}/user/login?redirect=${encodeURIComponent(path)}`,
  );
}
