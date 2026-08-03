import { describe, expect, it } from 'vitest';
import { mapCollectErrorMessage, resolveCollectFailureHint } from '../collectErrors';

const BATCH_HINT =
  '该链接单独采集成功，批量失败可能由并发、访问频率或目标站点风控导致。建议降低批量并发或稍后重试。';

describe('resolveCollectFailureHint', () => {
  it('单条任务命中批量场景话术时改用单条话术', () => {
    const hint = resolveCollectFailureHint(BATCH_HINT, false);
    expect(hint).not.toContain('批量');
    expect(hint).toContain('稍后重试');
  });

  it('批量子任务保留批量场景话术', () => {
    expect(resolveCollectFailureHint(BATCH_HINT, true)).toBe(BATCH_HINT);
  });

  it('其他提示原样返回', () => {
    expect(resolveCollectFailureHint('该商品页需要登录后才能访问，请稍后重试或使用登录状态采集。', false)).toBe(
      '该商品页需要登录后才能访问，请稍后重试或使用登录状态采集。',
    );
  });

  it('空值返回空字符串', () => {
    expect(resolveCollectFailureHint(undefined, false)).toBe('');
    expect(resolveCollectFailureHint('  ', true)).toBe('');
  });
});

describe('TENANT_CONTEXT_MISSING 中文映射', () => {
  it('平台管理员（租户 0）提交采集时给出租户切换指引', () => {
    const msg = mapCollectErrorMessage(
      new Error('TENANT_CONTEXT_MISSING: collect requires positive tenant scope'),
      '1688',
    );
    expect(msg).toContain('平台管理员');
    expect(msg).toContain('租户');
    expect(msg).not.toMatch(/TENANT_CONTEXT_MISSING/);
  });
});
