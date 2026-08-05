import type { ReactNode } from 'react';
import { Button, Result } from 'antd';
import { history, useLocation } from '@umijs/max';
import { useInitialStateModel } from '@/hooks/useInitialStateModel';
import { canAccessPath } from '@/utils/menuAccess';

/** 权限不足空态副标题（与 404 语义分离，指向可执行的开通引导）。 */
export const ROUTE_FORBIDDEN_SUBTITLE =
  '当前账号没有访问该页面的权限。如需使用，请联系管理员开通相应权限或授权对应店铺。';

/**
 * 布局级路由权限兜底：登录账号访问权限外的已知路由时，展示权限引导空态
 * （区别于不存在路由的 404 页，见 pages/404.tsx）。
 */
export default function RouteAccessGuard({ children }: { children: ReactNode }) {
  const { pathname } = useLocation();
  const { initialState } = useInitialStateModel();
  const user = initialState?.currentUser;
  if (user && !canAccessPath(pathname, user.role, user.permissions, user.tenantId)) {
    return (
      <Result
        status="403"
        title="暂无访问权限"
        subTitle={ROUTE_FORBIDDEN_SUBTITLE}
        extra={
          <Button type="primary" onClick={() => history.push('/dashboard')}>
            返回工作台
          </Button>
        }
      />
    );
  }
  return <>{children}</>;
}
