import { describe, expect, it } from 'vitest';
import { activeMobileTabKey, MOBILE_TABS, visibleMobileTabs } from '@/constants/mobileNav';

describe('mobileNav', () => {
  it('底部导航固定为 首页/订单/采购/库存/我的 五个 tab', () => {
    expect(MOBILE_TABS.map((t) => t.title)).toEqual(['首页', '订单', '采购', '库存', '我的']);
  });

  it('admin 角色可见全部 tab', () => {
    expect(visibleMobileTabs('admin').map((t) => t.key)).toEqual([
      'home',
      'orders',
      'procurement',
      'inventory',
      'me',
    ]);
  });

  it('readonly 角色仍可见浏览类 tab（口径与侧栏菜单一致）', () => {
    const keys = visibleMobileTabs('readonly').map((t) => t.key);
    expect(keys).toContain('home');
    expect(keys).toContain('orders');
    expect(keys).toContain('inventory');
    expect(keys).toContain('me');
  });

  it('reviewer 角色看不到订单/库存 tab（无 ORDER_VIEW / INVENTORY_VIEW）', () => {
    const keys = visibleMobileTabs('reviewer').map((t) => t.key);
    expect(keys).not.toContain('orders');
    expect(keys).not.toContain('inventory');
    expect(keys).toContain('home');
    expect(keys).toContain('me');
  });

  it('按路径前缀命中当前 tab', () => {
    expect(activeMobileTabKey('/m/home')).toBe('home');
    expect(activeMobileTabKey('/dashboard/product-operations')).toBe('home');
    expect(activeMobileTabKey('/orders/list')).toBe('orders');
    expect(activeMobileTabKey('/orders/exceptions')).toBe('orders');
    expect(activeMobileTabKey('/procurement/orders')).toBe('procurement');
    expect(activeMobileTabKey('/inventory/alerts')).toBe('inventory');
    expect(activeMobileTabKey('/m/me')).toBe('me');
    expect(activeMobileTabKey('/settings/system')).toBeUndefined();
  });
});
