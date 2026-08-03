import { describe, expect, it } from 'vitest';
import { formatRecentItem } from '@/constants/dashboardRecent';
import type { DashboardRecentItem } from '@/services/dashboard';

function item(partial: Partial<DashboardRecentItem>): DashboardRecentItem {
  return {
    type: 'product_publish',
    title: '',
    subtitle: '',
    status: '',
    occurredAt: '2026-08-01T02:03:04Z',
    link: '/product/publish-tasks',
    ...partial,
  };
}

describe('formatRecentItem platform enum mapping', () => {
  it('maps raw platform key in publish task title', () => {
    const { title } = formatRecentItem(item({ type: 'product_publish', title: 'douyin_shop 刊登' }));
    expect(title).toBe('抖店 刊登');
  });

  it('maps raw platform key in failed publish title', () => {
    const { title } = formatRecentItem(
      item({ type: 'failed_publish', title: 'tiktok · 刊登失败' }),
    );
    expect(title).toBe('TikTok Shop · 刊登失败');
  });

  it('maps raw platform key in failed inventory sync title', () => {
    const { title } = formatRecentItem(
      item({ type: 'failed_inventory_sync', title: 'shopee · 库存同步失败' }),
    );
    expect(title).toBe('Shopee · 库存同步失败');
  });

  it('keeps unknown platform title unchanged', () => {
    const { title } = formatRecentItem(
      item({ type: 'product_publish', title: 'unknown_platform 刊登' }),
    );
    expect(title).toBe('unknown_platform 刊登');
  });

  it('maps raw platform key in customer conversation subtitle', () => {
    const { subtitle } = formatRecentItem(
      item({ type: 'customer_conversation', title: 'e2e 买家', subtitle: 'douyin_shop' }),
    );
    expect(subtitle).toBe('抖店');
  });
});
