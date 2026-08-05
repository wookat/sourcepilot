import { describe, expect, it, vi, beforeEach } from 'vitest';

const notificationError = vi.fn();
const notificationDestroy = vi.fn();
vi.mock('antd', () => ({
  notification: {
    error: (...args: unknown[]) => notificationError(...args),
    destroy: (...args: unknown[]) => notificationDestroy(...args),
  },
  Button: () => null,
  Tooltip: () => null,
  Typography: { Text: () => null },
}));
vi.mock('@umijs/max', () => ({ history: { push: vi.fn() } }));

import { dismissAIFailure, extractAIError, notifyAIFailure } from '../aiFailureNotice';

describe('extractAIError', () => {
  it('maps structured errorCode from axios error response envelope to Chinese copy', () => {
    const err = Object.assign(new Error('Request failed with status code 400'), {
      response: {
        status: 400,
        data: { code: 40001, message: '请配置 API Key', data: { errorCode: 'AI_NOT_CONFIGURED' }, traceId: 't-1' },
      },
    });
    const out = extractAIError(err);
    expect(out.errorCode).toBe('AI_NOT_CONFIGURED');
    expect(out.reason).toContain('AI 设置');
    expect(out.traceId).toBe('t-1');
  });

  it('falls back to envelope Chinese message when errorCode is absent', () => {
    const err = Object.assign(new Error('Request failed with status code 400'), {
      response: {
        status: 400,
        data: { code: 40001, message: '模型返回内容为空', data: null },
      },
    });
    expect(extractAIError(err).reason).toBe('模型返回内容为空');
  });

  it('never surfaces the raw axios English fallback message', () => {
    const err = new Error('Request failed with status code 400');
    const out = extractAIError(err);
    expect(out.reason).toBeUndefined();
    expect(out.errorCode).toBeUndefined();
  });

  it('reads ApiRequestError-shaped errors thrown after envelope unwrap', () => {
    const err = Object.assign(new Error('API Key 无效或未授权'), {
      code: 40001,
      data: { errorCode: 'AI_INVALID_KEY' },
      traceId: 't-2',
    });
    const out = extractAIError(err);
    expect(out.errorCode).toBe('AI_INVALID_KEY');
    expect(out.reason).toContain('密钥');
  });

  it('ignores unknown errorCode values and keeps envelope message', () => {
    const err = Object.assign(new Error('Request failed with status code 409'), {
      response: {
        status: 409,
        data: { code: 40001, message: '内容冲突', data: { errorCode: 'AI_CONTENT_APPLY_CONFLICT' } },
      },
    });
    const out = extractAIError(err);
    expect(out.errorCode).toBeUndefined();
    expect(out.reason).toBe('内容冲突');
  });
});

// Round112 P2-1 regression: a stale failure notification must not linger
// after a follow-up success. Same-scope failures share a stable key (new
// failure replaces the old one) and dismissAIFailure clears it on success.
describe('notifyAIFailure / dismissAIFailure', () => {
  beforeEach(() => {
    notificationError.mockClear();
    notificationDestroy.mockClear();
  });

  it('uses a stable per-scope key so repeated failures replace each other', () => {
    notifyAIFailure({ title: '失败一', error: new Error('模型输出无效'), scope: 'title-optimize' });
    notifyAIFailure({ title: '失败二', error: new Error('模型输出无效'), scope: 'title-optimize' });
    const keys = notificationError.mock.calls.map((c) => (c[0] as { key: string }).key);
    expect(keys[0]).toBe('ai-failure-title-optimize');
    expect(keys[1]).toBe(keys[0]);
  });

  it('dismissAIFailure destroys the notification for the same scope', () => {
    notifyAIFailure({ title: '失败', error: new Error('模型输出无效'), scope: 'title-optimize' });
    dismissAIFailure('title-optimize');
    expect(notificationDestroy).toHaveBeenCalledWith('ai-failure-title-optimize');
  });

  it('falls back to the shared global scope when scope is omitted', () => {
    notifyAIFailure({ title: '失败', error: new Error('x') });
    expect((notificationError.mock.calls[0][0] as { key: string }).key).toBe('ai-failure-global');
    dismissAIFailure();
    expect(notificationDestroy).toHaveBeenCalledWith('ai-failure-global');
  });
});
