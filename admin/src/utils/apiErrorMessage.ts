import { mapErrorCodeToUserMessage } from '@/constants/errorMessages';

type ApiErrorLike = {
  response?: { data?: { message?: string } };
  message?: string;
};

/**
 * 从请求异常中提取后端 envelope 的业务 message，供用户可读展示：
 * 已知错误码走 errorMessages 映射；其余情况优先展示后端原文，
 * 没有可用信息时回退到调用方 fallback。
 */
export function extractApiErrorMessage(error: unknown, fallback: string): string {
  const ax = error as ApiErrorLike;
  const raw = (ax?.response?.data?.message || '').trim();
  const mapped = mapErrorCodeToUserMessage(raw);
  if (mapped) return mapped.detail ? `${mapped.title}：${mapped.detail}` : mapped.title;
  if (raw && !raw.startsWith('{') && !raw.startsWith('[')) return raw;
  const msg = (ax?.message || '').trim();
  return msg || fallback;
}
