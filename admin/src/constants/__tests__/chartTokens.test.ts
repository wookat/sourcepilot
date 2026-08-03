import { describe, expect, it } from 'vitest';
import {
  chartAxisXLabel,
  chartAxisXTickCount,
  chartTokens,
  formatAmount,
  formatCount,
  formatDateTickShort,
  makeCategoryLabelFilter,
} from '../chartTokens';
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

describe('图表 token（R66）', () => {
  it('Column 单柱宽度上限不超过 40px', () => {
    expect(chartTokens.barMaxWidth).toBeGreaterThan(0);
    expect(chartTokens.barMaxWidth).toBeLessThanOrEqual(40);
  });

  it('x 轴标签公共配置包含自动抽样与自动旋转', () => {
    expect(chartAxisXLabel.labelAutoHide).toBe(true);
    expect(chartAxisXLabel.labelAutoRotate).toBe(true);
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

describe('x 轴刻度抽稀（R77）', () => {
  it('宽屏/紧凑刻度档位在 6–10 之间且紧凑 ≤ 宽屏', () => {
    expect(chartAxisXTickCount.compact).toBeGreaterThanOrEqual(6);
    expect(chartAxisXTickCount.wide).toBeLessThanOrEqual(10);
    expect(chartAxisXTickCount.compact).toBeLessThanOrEqual(chartAxisXTickCount.wide);
  });

  it('makeCategoryLabelFilter：宽屏 30/90 天保留 8–12 个标签且含首末点', () => {
    for (const n of [30, 90]) {
      const filter = makeCategoryLabelFilter(n, chartAxisXTickCount.wide);
      const kept = Array.from({ length: n }, (_, i) => i).filter((i) => filter(undefined, i));
      expect(kept.length).toBeGreaterThanOrEqual(8);
      expect(kept.length).toBeLessThanOrEqual(12);
      expect(kept[0]).toBe(0);
      expect(kept[kept.length - 1]).toBe(n - 1);
    }
  });

  it('makeCategoryLabelFilter：紧凑档 90 天标签数不超过 8，点数少于档位时全保留', () => {
    const compact = makeCategoryLabelFilter(90, chartAxisXTickCount.compact);
    const kept = Array.from({ length: 90 }, (_, i) => i).filter((i) => compact(undefined, i));
    expect(kept.length).toBeLessThanOrEqual(chartAxisXTickCount.compact + 2);
    const few = makeCategoryLabelFilter(5, chartAxisXTickCount.wide);
    expect(Array.from({ length: 5 }, (_, i) => i).every((i) => few(undefined, i))).toBe(true);
  });

  it('formatDateTickShort：YYYY-MM-DD → MM-DD，其余原样', () => {
    expect(formatDateTickShort('2026-08-03')).toBe('08-03');
    expect(formatDateTickShort('2026-08-03T00:00:00Z')).toBe('08-03');
    expect(formatDateTickShort('08-03')).toBe('08-03');
    expect(formatDateTickShort('')).toBe('');
    expect(formatDateTickShort(undefined)).toBe('');
  });
});
