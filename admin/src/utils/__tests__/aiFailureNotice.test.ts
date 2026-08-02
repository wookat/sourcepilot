import { describe, expect, it } from 'vitest';
import { extractAIError } from '../aiFailureNotice';

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
