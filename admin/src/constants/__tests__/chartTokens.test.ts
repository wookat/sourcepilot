import { describe, expect, it } from 'vitest';
import { chartTokens, formatAmount, formatCount } from '../chartTokens';
import { themeTokens } from '../layoutTokens';

describe('图表 token（R64）', () => {
  it('系列色首色与主题主色一致', () => {
    expect(chartTokens.seriesColors[0]).toBe(themeTokens.colorPrimary);
  });

  it('系列色均为合法 hex 且不重复', () => {
    const colors = [...chartTokens.seriesColors];
    for (const c of colors) expect(c).toMatch(/^#[0-9a-f]{6}$/i);
    expect(new Set(colors).size).toBe(colors.length);
  });
});

describe('formatCount 千分位', () => {
  it.each([
    [0, '0'],
    [999, '999'],
    [1000, '1,000'],
    [1234567, '1,234,567'],
  ])('%d → %s', (input, expected) => {
    expect(formatCount(input)).toBe(expected);
  });
});

describe('formatAmount 千分位 + 两位小数 + 币种', () => {
  it('无币种时只格式化数字', () => {
    expect(formatAmount(1234.5)).toBe('1,234.50');
  });

  it('带币种时前缀币种代码', () => {
    expect(formatAmount(25.5, 'USD')).toBe('USD 25.50');
    expect(formatAmount(1000000, 'EUR')).toBe('EUR 1,000,000.00');
  });

  it('零与负数', () => {
    expect(formatAmount(0)).toBe('0.00');
    expect(formatAmount(-7.5, 'USD')).toBe('USD -7.50');
  });
});
