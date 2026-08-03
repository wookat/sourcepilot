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
let handlerWaiters: Array<(h: ReloginHandler | null) => void> = [];

/** 弹窗组件挂载前（如硬刷新首屏）到达的 401 等待注册完成，超时则按无弹窗处理 */
const RELOGIN_HANDLER_WAIT_MS = 3000;

/** 由「登录已过期」弹窗组件注册；返回 Promise，重新登录成功 resolve(true) */
export function registerReloginHandler(handler: ReloginHandler | null) {
  reloginHandler = handler;
  if (handler) {
    const waiters = handlerWaiters;
    handlerWaiters = [];
    waiters.forEach((w) => w(handler));
  }
}

function waitForReloginHandler(): Promise<ReloginHandler | null> {
  if (reloginHandler) return Promise.resolve(reloginHandler);
  return new Promise((resolve) => {
    const onReady = (h: ReloginHandler | null) => {
      clearTimeout(timer);
      resolve(h);
    };
    const timer = setTimeout(() => {
      handlerWaiters = handlerWaiters.filter((w) => w !== onReady);
      resolve(null);
    }, RELOGIN_HANDLER_WAIT_MS);
    handlerWaiters.push(onReady);
  });
}

/**
 * 弹出重新登录弹窗并等待结果。single-flight 对「弹窗未注册」也生效：
 * 硬刷新时并发 401 全部共享同一次重登引导，不会各自 resolve(false)
 * 后触发多次跳转/多个错误弹窗。
 */
export function requireRelogin(): Promise<boolean> {
  if (!reloginInFlight) {
    reloginInFlight = waitForReloginHandler()
      .then((h) => (h ? h() : false))
      .finally(() => {
        reloginInFlight = null;
      });
  }
  return reloginInFlight;
}

/**
 * 绕开 umi request 的原生 fetch（CSV 导出、备份下载等二进制流）专用会话守卫：
 * 自动附带 Authorization，401 时先静默续期、失败再走统一重登引导，成功后重放一次。
 * 用户放弃重登则整页跳登录页并悬挂 Promise（与 umi 拦截器口径一致，避免裸报错）。
 */
export async function fetchWithSessionGuard(url: string, init: RequestInit = {}): Promise<Response> {
  if (!isAuthUrl(url) && shouldRefreshSoon()) {
    await refreshAccessToken();
  }
  const doFetch = () => {
    const token = localStorage.getItem(AUTH_TOKEN_KEY);
    return fetch(url, {
      ...init,
      headers: {
        ...((init.headers as Record<string, string>) || {}),
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
    });
  };
  const resp = await doFetch();
  if (resp.status !== 401 || isAuthUrl(url)) return resp;
  let ok = await refreshAccessToken();
  if (!ok) ok = await requireRelogin();
  if (!ok) {
    redirectToLoginPage();
    return new Promise<never>(() => {});
  }
  return doFetch();
}

let redirectingToLogin = false;

/** 用户放弃重新登录时的兜底：清凭证并整页跳登录页（并发 401 只跳一次） */
export function redirectToLoginPage() {
  clearSessionCredentials();
  if (redirectingToLogin) return;
  const path = window.location.pathname;
  if (path === '/user/login' || path.startsWith('/user/login')) return;
  redirectingToLogin = true;
  window.location.assign(
    `${window.location.origin}/user/login?redirect=${encodeURIComponent(path)}`,
  );
}

/** 仅供测试：重置登录页跳转去重状态 */
export function resetLoginRedirectGuard() {
  redirectingToLogin = false;
}
