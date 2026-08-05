import { canAccessPath } from '@/utils/menuAccess';

export type MobileTab = {
  key: string;
  title: string;
  /** 点击跳转路径 */
  path: string;
  /** 命中这些前缀时视为当前 tab */
  match: string[];
};

/** 移动端底部导航（首页 / 订单 / 采购 / 库存 / 我的） */
export const MOBILE_TABS: MobileTab[] = [
  { key: 'home', title: '首页', path: '/m/home', match: ['/m/home', '/dashboard'] },
  { key: 'orders', title: '订单', path: '/orders/list', match: ['/orders'] },
  { key: 'procurement', title: '采购', path: '/procurement/orders', match: ['/procurement'] },
  { key: 'inventory', title: '库存', path: '/inventory/center', match: ['/inventory'] },
  { key: 'me', title: '我的', path: '/m/me', match: ['/m/me'] },
];

/** 按角色权限过滤可见 tab（口径与侧栏菜单一致） */
export function visibleMobileTabs(
  role?: string | null,
  permissions?: string[],
  tenantId?: number,
): MobileTab[] {
  return MOBILE_TABS.filter((tab) => canAccessPath(tab.path, role, permissions, tenantId));
}

/** 当前路径命中的 tab key（未命中返回 undefined，如设置等可浏览页面） */
export function activeMobileTabKey(pathname: string): string | undefined {
  return MOBILE_TABS.find((tab) =>
    tab.match.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`) || pathname.startsWith(`${prefix}?`)),
  )?.key;
}
