import { describe, expect, it } from 'vitest';
import { buildKpiCards } from '../index';
import type { DashboardSummary } from '@/services/dashboard';

describe('buildKpiCards 空值防御（R66）', () => {
  it('summary 缺字段时所有 KPI 卡为有限数字而非 NaN', () => {
    const cards = buildKpiCards({} as DashboardSummary);
    for (const card of cards) {
      expect(Number.isFinite(card.value), `${card.title} 应为有限数字`).toBe(true);
    }
  });

  it('库存异常：预警字段缺失时按可得字段求和', () => {
    const partial = { inventorySyncFailedCount: 2 } as DashboardSummary;
    const card = buildKpiCards(partial).find((c) => c.title === '库存异常');
    expect(card?.value).toBe(2);
  });

  it('库存异常：inventoryAlerts 存在时优先使用聚合值', () => {
    const summary = { inventoryAlerts: 5, inventorySyncFailedCount: 1 } as DashboardSummary;
    const card = buildKpiCards(summary).find((c) => c.title === '库存异常');
    expect(card?.value).toBe(6);
  });
});
