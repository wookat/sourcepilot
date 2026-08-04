import { request as umiRequest } from '@umijs/max';
import { message as antdMessage } from 'antd';

/**
 * AUTH_STATE_UNAVAILABLE（secure_session 数据库瞬断 fail-closed）专门处理：
 * 会话本身并未失效，与 401 重登守卫（AUTH_SESSION_REVOKED / token 过期）区分——
 * 不清凭证、不弹重登、不跳登录页，提示「服务暂时不可用」并按指数退避自动重试，
 * 后端恢复后原样返回响应，用户无感续用。
 */

export const AUTH_STATE_UNAVAILABLE = 'AUTH_STATE_UNAVAILABLE';

/** 指数退避重试间隔；总计约 15s，覆盖后端 30s last-known-good 快照窗口内的常见瞬断 */
export const AUTH_STATE_RETRY_DELAYS_MS = [1000, 2000, 4000, 8000];

type HttpErrorLike = {
  response?: { status?: number; data?: { message?: unknown } | null };
  config?: Record<string, unknown> & { url?: string; method?: string };
};

/** umi request 错误对象是否为 401 + AUTH_STATE_UNAVAILABLE */
export function isAuthStateUnavailable(error: unknown): boolean {
  const err = (error || {}) as HttpErrorLike;
  return (
    err.response?.status === 401 && err.response?.data?.message === AUTH_STATE_UNAVAILABLE
  );
}

/** 原生 fetch Response 是否为 401 + AUTH_STATE_UNAVAILABLE（clone 后读 body，不消费原响应） */
export async function isAuthStateUnavailableResponse(resp: Response): Promise<boolean> {
  if (resp.status !== 401) return false;
  try {
    const body = (await resp.clone().json()) as { message?: unknown } | null;
    return body?.message === AUTH_STATE_UNAVAILABLE;
  } catch {
    return false;
  }
}

/** 提示去重窗口：并发多个请求同时命中瞬断时只提示一次 */
const NOTICE_DEDUPE_MS = 15 * 1000;

let lastNoticeAt = 0;

function notifyRetrying(nowMs: number = Date.now()) {
  if (nowMs - lastNoticeAt < NOTICE_DEDUPE_MS) return;
  lastNoticeAt = nowMs;
  void antdMessage.warning('服务暂时不可用，正在自动重试…', 3);
}

/** 仅供测试：重置提示去重状态 */
export function resetAuthStateNoticeDedupe() {
  lastNoticeAt = 0;
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => {
    setTimeout(resolve, ms);
  });
}

/**
 * umi request 链路的退避重试：按原请求配置逐次重放，恢复即返回响应；
 * 重试期间遇到非 UNAVAILABLE 错误立即抛出（如恢复后正常 401 走上层守卫），
 * 重试耗尽仍不可用则抛出最后一次错误（页面按普通失败提示，不登出）。
 */
export async function retryWhileAuthStateUnavailable(
  error: HttpErrorLike,
  delays: number[] = AUTH_STATE_RETRY_DELAYS_MS,
): Promise<unknown> {
  const cfg = error?.config || {};
  notifyRetrying();
  let lastError: unknown = error;
  for (const delay of delays) {
    await sleep(delay);
    try {
      return await umiRequest(String(cfg.url || ''), {
        method: (cfg.method as string) || 'GET',
        data: cfg.data,
        params: cfg.params as Record<string, string | number | boolean | undefined> | undefined,
        headers: { ...((cfg.headers as Record<string, string>) || {}) },
        sessionGuardRetry: true,
        skipErrorHandler: true,
        getResponse: true,
      });
    } catch (e) {
      if (!isAuthStateUnavailable(e)) throw e;
      lastError = e;
    }
  }
  throw lastError;
}

/**
 * 原生 fetch 链路（CSV 导出、备份下载等）的退避重试：
 * 恢复即返回新响应；重试耗尽仍不可用则原样返回最后一次响应，由调用方按失败处理。
 */
export async function retryFetchWhileAuthStateUnavailable(
  doFetch: () => Promise<Response>,
  firstResponse: Response,
  delays: number[] = AUTH_STATE_RETRY_DELAYS_MS,
): Promise<Response> {
  notifyRetrying();
  let last = firstResponse;
  for (const delay of delays) {
    await sleep(delay);
    const resp = await doFetch();
    if (!(await isAuthStateUnavailableResponse(resp))) return resp;
    last = resp;
  }
  return last;
}
