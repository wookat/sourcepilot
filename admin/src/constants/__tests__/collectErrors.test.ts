import { describe, expect, it } from 'vitest';
import {
  mapCollectErrorMessage,
  mapCollectorErrorCodeDetail,
  mapCollectorErrorCodeLabel,
  resolveCollectFailureHint,
} from '../collectErrors';

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

describe('UNSUPPORTED_URL / INVALID_URL 中文映射', () => {
  it('UNSUPPORTED_URL 有中文短标题', () => {
    expect(mapCollectorErrorCodeLabel('UNSUPPORTED_URL')).toBe('链接类型暂不支持');
  });

  it('UNSUPPORTED_URL + 1688 给出 1688 详情页指引', () => {
    const detail = mapCollectorErrorCodeDetail('UNSUPPORTED_URL', '1688');
    expect(detail).toContain('1688');
    expect(detail).toContain('detail.1688.com');
  });

  it('collector 原始英文消息映射为中文（带 source 参数）', () => {
    const msg = mapCollectErrorMessage(new Error('url is not supported by source "1688"'), '1688');
    expect(msg).toContain('detail.1688.com');
    expect(msg).not.toMatch(/not supported/i);
  });

  it('collector 原始英文消息缺少 source 参数时从消息中提取来源', () => {
    const msg = mapCollectErrorMessage('url is not supported by source "1688"');
    expect(msg).toContain('detail.1688.com');
  });

  it('collector INVALID_URL 原始错误映射为中文', () => {
    const msg = mapCollectErrorMessage('INVALID_URL:not_a_1688_product_url', '1688');
    expect(msg).toContain('1688 商品详情页');
  });
});
