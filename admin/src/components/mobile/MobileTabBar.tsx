import type { ReactNode } from 'react';
import {
  ContainerOutlined,
  HomeOutlined,
  InboxOutlined,
  ShoppingCartOutlined,
  UserOutlined,
} from '@ant-design/icons';
import { history, useLocation } from '@umijs/max';
import { activeMobileTabKey, visibleMobileTabs } from '@/constants/mobileNav';
import { usePermission } from '@/hooks/usePermission';
import { useWideScreen } from '@/hooks/useWideScreen';

const TAB_ICONS: Record<string, ReactNode> = {
  home: <HomeOutlined />,
  orders: <ContainerOutlined />,
  procurement: <ShoppingCartOutlined />,
  inventory: <InboxOutlined />,
  me: <UserOutlined />,
};

/**
 * 移动端（<768px）底部固定导航。宽屏或未登录时不渲染；
 * tab 按角色权限过滤，口径与侧栏菜单一致。
 */
export default function MobileTabBar() {
  const wide = useWideScreen();
  const { user, role, permissions } = usePermission();
  const location = useLocation();

  if (wide || !user) return null;

  const tabs = visibleMobileTabs(role, permissions, user.tenantId);
  if (!tabs.length) return null;
  const activeKey = activeMobileTabKey(location.pathname);

  return (
    <>
      <div className="tm-mobile-tabbar__spacer" aria-hidden />
      <nav className="tm-mobile-tabbar" aria-label="移动端主导航" data-testid="tm-mobile-tabbar">
        {tabs.map((tab) => {
          const active = tab.key === activeKey;
          return (
            <button
              key={tab.key}
              type="button"
              className={['tm-mobile-tabbar__item', active ? 'tm-mobile-tabbar__item--active' : '']
                .filter(Boolean)
                .join(' ')}
              aria-current={active ? 'page' : undefined}
              onClick={() => history.push(tab.path)}
            >
              <span className="tm-mobile-tabbar__icon">{TAB_ICONS[tab.key]}</span>
              <span className="tm-mobile-tabbar__label">{tab.title}</span>
            </button>
          );
        })}
      </nav>
    </>
  );
}
