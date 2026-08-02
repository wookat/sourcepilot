import { describe, expect, it } from 'vitest';
import {
  extractErrorMessage,
  httpStatusCopy,
  isRawTransportMessage,
  normalizeHttpErrorMessage,
  translateBackendErrorText,
} from '../httpErrorCopy';

function axiosError(status: number, data?: unknown) {
  return Object.assign(new Error(`Request failed with status code ${status}`), {
    response: { status, data },
  });
}

describe('isRawTransportMessage', () => {
  it('matches axios / network raw English messages', () => {
    expect(isRawTransportMessage('Request failed with status code 500')).toBe(true);
    expect(isRawTransportMessage('Network Error')).toBe(true);
    expect(isRawTransportMessage('timeout of 30000ms exceeded')).toBe(true);
    expect(isRawTransportMessage('request_failed')).toBe(true);
  });

  it('does not match business messages', () => {
    expect(isRawTransportMessage('商品不存在')).toBe(false);
    expect(isRawTransportMessage('导出数量超过上限')).toBe(false);
  });
});

describe('httpStatusCopy', () => {
  it('maps common status codes to Chinese copy', () => {
    expect(httpStatusCopy(400)).toBe('请求参数有误，请检查后重试');
    expect(httpStatusCopy(403)).toBe('没有权限执行该操作');
    expect(httpStatusCopy(404)).toBe('请求的资源不存在或已删除');
    expect(httpStatusCopy(409)).toBe('操作冲突，请刷新页面后重试');
    expect(httpStatusCopy(500)).toBe('服务异常，请稍后再试');
    expect(httpStatusCopy(502)).toBe('服务异常，请稍后再试');
  });

  it('falls back to network copy when status is missing', () => {
    expect(httpStatusCopy(undefined)).toBe('网络异常，请检查网络后重试');
  });
});

describe('extractErrorMessage', () => {
  it('prefers structured envelope message from the backend', () => {
    const err = axiosError(400, { code: 40001, message: '导出数量超过上限', data: null });
    expect(extractErrorMessage(err)).toBe('导出数量超过上限');
  });

  it('keeps non-transport error.message (e.g. ApiRequestError)', () => {
    expect(extractErrorMessage(new Error('商品不存在'))).toBe('商品不存在');
  });

  it('never returns the raw axios English message', () => {
    expect(extractErrorMessage(axiosError(500))).toBe('服务异常，请稍后再试');
    expect(extractErrorMessage(axiosError(409))).toBe('操作冲突，请刷新页面后重试');
  });

  it('does not surface English envelope messages; falls back to status copy', () => {
    const err = axiosError(400, { code: 40001, message: 'too many ids', data: null });
    expect(extractErrorMessage(err)).toBe('请求参数有误，请检查后重试');
  });

  it('maps error-code envelope messages through ERROR_MAP', () => {
    const err = axiosError(401, { code: 40100, message: 'AUTH_INVALID_CREDENTIALS', data: null });
    expect(extractErrorMessage(err)).toContain('账号或密码');
  });

  it('uses the provided fallback before status copy', () => {
    expect(extractErrorMessage(axiosError(500), '同步失败')).toBe('同步失败');
  });

  it('handles network errors without a response', () => {
    expect(extractErrorMessage(new Error('Network Error'))).toBe('网络异常，请检查网络后重试');
  });
});

describe('translateBackendErrorText', () => {
  it('translates AI-not-configured variants to a friendly settings hint', () => {
    expect(translateBackendErrorText('customerchat: ai not configured')).toContain('AI Provider 未配置');
    expect(translateBackendErrorText('product: ai not configured')).toContain('AI Provider 未配置');
    expect(translateBackendErrorText('ai gateway: not configured')).toContain('AI Provider 未配置');
    expect(translateBackendErrorText('ai gateway not configured')).toContain('AI 设置');
  });

  it('translates customer message sync errors', () => {
    expect(translateBackendErrorText('only failed tasks can be retried')).toBe(
      '仅失败状态的任务可以重试，请刷新列表后再试',
    );
    expect(translateBackendErrorText('platform does not implement customer messaging')).toBe(
      '该平台暂不支持客服消息同步',
    );
    expect(translateBackendErrorText('platform customer message permission denied or not configured')).toBe(
      '平台客服消息权限未开通或店铺凭证未配置',
    );
    expect(translateBackendErrorText('customer chat service unavailable')).toBe(
      '客服服务暂不可用，请稍后再试',
    );
    expect(translateBackendErrorText('customer message sync unavailable')).toBe(
      '客服服务暂不可用，请稍后再试',
    );
  });

  it('returns empty string for unknown or unrelated messages', () => {
    expect(translateBackendErrorText('api key not configured')).toBe('');
    expect(translateBackendErrorText('too many ids')).toBe('');
    expect(translateBackendErrorText('')).toBe('');
    expect(translateBackendErrorText(undefined)).toBe('');
  });
});

describe('extractErrorMessage with translated backend copy', () => {
  it('surfaces the AI settings hint for ai-not-configured envelope errors', () => {
    const err = axiosError(400, { code: 40001, message: 'customerchat: ai not configured', data: null });
    expect(extractErrorMessage(err)).toBe('AI Provider 未配置，请到「系统设置 → AI 设置」完成配置后重试');
  });

  it('surfaces retry copy for customer sync retry rejection', () => {
    const err = axiosError(400, { code: 40001, message: 'only failed tasks can be retried', data: null });
    expect(extractErrorMessage(err)).toBe('仅失败状态的任务可以重试，请刷新列表后再试');
  });
});

describe('normalizeHttpErrorMessage', () => {
  it('rewrites raw axios message with envelope message', () => {
    const err = axiosError(403, { code: 40301, message: '只读账号不可执行写操作', data: null });
    expect(normalizeHttpErrorMessage(err).message).toBe('只读账号不可执行写操作');
  });

  it('rewrites raw axios message with status copy when envelope has none', () => {
    expect(normalizeHttpErrorMessage(axiosError(500)).message).toBe('服务异常，请稍后再试');
    expect(normalizeHttpErrorMessage(axiosError(404)).message).toBe('请求的资源不存在或已删除');
  });

  it('leaves specialised Chinese messages untouched', () => {
    const err = Object.assign(new Error('API Key 无效或未授权'), { response: { status: 400 } });
    expect(normalizeHttpErrorMessage(err).message).toBe('API Key 无效或未授权');
  });

  it('keeps response / status fields intact for downstream guards', () => {
    const err = axiosError(401);
    const out = normalizeHttpErrorMessage(err);
    expect(out.response?.status).toBe(401);
    expect(out).toBe(err);
  });
});
