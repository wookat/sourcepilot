/**
 * 全站 HTTP 错误文案兜底：接口失败时优先展示后端 envelope 的结构化中文 message，
 * 无结构化信息时按 HTTP 状态码映射通用中文提示，杜绝 axios 英文原文
 * （如 "Request failed with status code 500"）直出到界面。
 *
 * 专有处理（401 会话守卫重放、403 只读提示、AI 路径 aiFailureNotice 等）优先，
 * 本模块只在错误消息缺失或为 axios/网络层英文原文时兜底改写 error.message，
 * 不改变 error 对象上的 response / status / code / data 等结构，
 * 因此依赖这些字段的既有分支不受影响。
 */

import { mapErrorCodeToUserMessage } from '@/constants/errorMessages';

const STATUS_COPY: Record<number, string> = {
  400: '请求参数有误，请检查后重试',
  401: '登录已过期，请重新登录',
  403: '没有权限执行该操作',
  404: '请求的资源不存在或已删除',
  405: '请求方式不被支持',
  408: '请求超时，请稍后重试',
  409: '操作冲突，请刷新页面后重试',
  413: '提交内容过大，请缩小后重试',
  429: '操作过于频繁，请稍后再试',
};

/** axios / 网络层生成的英文原文，一律不允许直出到界面 */
const RAW_TRANSPORT_MESSAGE =
  /^(request failed with status code \d+|network error|timeout of .+ exceeded|timeout exceeded|request aborted|canceled|cancelled|failed to fetch|load failed|request_failed)$/i;

export function isRawTransportMessage(message: unknown): boolean {
  return typeof message === 'string' && RAW_TRANSPORT_MESSAGE.test(message.trim());
}

/** 按 HTTP 状态码映射通用中文兜底文案 */
export function httpStatusCopy(status?: number): string {
  if (typeof status === 'number') {
    if (STATUS_COPY[status]) return STATUS_COPY[status];
    if (status >= 500) return '服务异常，请稍后再试';
    if (status >= 400) return '请求失败，请稍后重试';
  }
  return '网络异常，请检查网络后重试';
}

type EnvelopeLike = { message?: unknown };

type HttpErrorLike = {
  message?: unknown;
  response?: { status?: number; data?: EnvelopeLike | null };
};

const CJK_RE = /[\u4e00-\u9fff]/;
const ERROR_CODE_RE = /^[A-Z][A-Z0-9_]+$/;

/**
 * 后端已知英文错误原文 → 用户可见中文（集中维护，不散落到各页面）。
 * 按顺序匹配，命中即返回；仅收录会直出到界面的后端 message。
 */
const BACKEND_MESSAGE_COPY: Array<[RegExp, string]> = [
  [/\bai( gateway)?:?\s+not configured\b/i, 'AI Provider 未配置，请到「系统设置 → AI 设置」完成配置后重试'],
  [/^请配置 (base_url|API Key)$/, 'AI Provider 未配置，请到「系统设置 → AI 设置」完成配置后重试'],
  [/only failed tasks can be retried/i, '仅失败状态的任务可以重试，请刷新列表后再试'],
  [/platform does not implement customer messaging/i, '该平台暂不支持客服消息同步'],
  [/customer message permission denied or not configured/i, '平台客服消息权限未开通或店铺凭证未配置'],
  [/customer (chat|message sync) (service )?unavailable/i, '客服服务暂不可用，请稍后再试'],
  [/invalid json body/i, '请求参数有误，请检查后重试'],
];

/** 已知后端英文错误原文翻译为中文；未收录返回空串 */
export function translateBackendErrorText(raw: unknown): string {
  if (typeof raw !== 'string') return '';
  const msg = raw.trim();
  if (!msg) return '';
  for (const [re, copy] of BACKEND_MESSAGE_COPY) {
    if (re.test(msg)) return copy;
  }
  return '';
}

/** 只采纳可直接展示的结构化中文 message；错误码走 ERROR_MAP 映射；英文原文一律不采纳 */
function usableStructuredMessage(raw: unknown): string {
  if (typeof raw !== 'string') return '';
  const msg = raw.trim();
  if (!msg || isRawTransportMessage(msg)) return '';
  const translated = translateBackendErrorText(msg);
  if (translated) return translated;
  if (CJK_RE.test(msg)) return msg;
  if (ERROR_CODE_RE.test(msg)) {
    const mapped = mapErrorCodeToUserMessage(msg);
    if (mapped) return mapped.detail ? `${mapped.title}：${mapped.detail}` : mapped.title;
  }
  return '';
}

function envelopeMessage(error: HttpErrorLike): string {
  return usableStructuredMessage(error?.response?.data?.message);
}

/**
 * 从任意接口错误中提取可直接展示给用户的中文消息：
 * 后端结构化 message > 已有非英文原文的 error.message > 状态码中文兜底 > fallback。
 */
export function extractErrorMessage(error: unknown, fallback?: string): string {
  const err = (error || {}) as HttpErrorLike;
  const fromEnvelope = envelopeMessage(err);
  if (fromEnvelope) return fromEnvelope;
  const own = usableStructuredMessage(err.message);
  if (own) return own;
  if (fallback) return fallback;
  return httpStatusCopy(err.response?.status);
}

/** fetch 直连（如 CSV 导出）失败时读取 envelope 中文 message，否则按状态码兜底 */
export async function responseErrorMessage(resp: {
  status: number;
  json: () => Promise<unknown>;
}): Promise<string> {
  try {
    const body = (await resp.json()) as EnvelopeLike | null;
    const msg = usableStructuredMessage(body?.message);
    if (msg) return msg;
  } catch {
    /* 非 JSON 响应体，走状态码兜底 */
  }
  return httpStatusCopy(resp.status);
}

/**
 * 原地把 error.message 兜底改写为中文（仅当消息缺失或为 axios 英文原文时），
 * 保留 response / config / code 等全部原有字段，供响应拦截器统一挂载。
 */
export function normalizeHttpErrorMessage<T>(error: T): T {
  const err = error as HttpErrorLike;
  if (!err || typeof err !== 'object') return error;
  const own = typeof err.message === 'string' ? err.message.trim() : '';
  if (own && !isRawTransportMessage(own) && CJK_RE.test(own) && !translateBackendErrorText(own)) return error;
  const next =
    envelopeMessage(err) ||
    (own && !isRawTransportMessage(own) ? translateBackendErrorText(own) || own : '') ||
    httpStatusCopy(err.response?.status);
  try {
    err.message = next;
  } catch {
    /* 只读 message（极少数宿主对象）时保持原样 */
  }
  return error;
}
