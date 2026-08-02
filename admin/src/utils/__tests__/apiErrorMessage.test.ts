import { describe, expect, it } from 'vitest';
import { extractApiErrorMessage } from '../apiErrorMessage';

describe('extractApiErrorMessage', () => {
  it('prefers backend envelope message over generic axios message', () => {
    const err = {
      message: 'Request failed with status code 400',
      response: { data: { message: 'customerchat: ai not configured' } },
    };
    expect(extractApiErrorMessage(err, '生成失败')).toBe('customerchat: ai not configured');
  });

  it('maps known error codes to friendly text', () => {
    const err = {
      message: 'Request failed with status code 403',
      response: { data: { message: 'DOUYIN_STORE_NOT_AUTHORIZED' } },
    };
    expect(extractApiErrorMessage(err, '提交失败')).toContain('店铺尚未授权');
  });

  it('falls back to error message then caller fallback', () => {
    expect(extractApiErrorMessage({ message: 'network down' }, '提交失败')).toBe('network down');
    expect(extractApiErrorMessage({}, '提交失败')).toBe('提交失败');
    expect(extractApiErrorMessage(undefined, '提交失败')).toBe('提交失败');
  });

  it('hides raw JSON payloads', () => {
    const err = { message: 'Request failed with status code 500', response: { data: { message: '{"trace":"x"}' } } };
    expect(extractApiErrorMessage(err, '操作失败')).toBe('Request failed with status code 500');
  });
});
