import type { ReactNode } from 'react';
import { Button, Result } from 'antd';
import { history, useLocation } from '@umijs/max';
import { useInitialStateModel } from '@/hooks/useInitialStateModel';
import { canAccessPath } from '@/utils/menuAccess';

export const ROUTE_FALLBACK_SUBTITLE = '页面不存在，或当前账号无权访问。请检查地址是否正确；如需访问请联系管理员开通相应权限。';

/**
 * 布局级路由兜底：受限路由与不存在的路由统一展示同一语义页，
 * 不泄露资源是否存在。
 */
export default function RouteAccessGuard({ children }: { children: ReactNode }) {
  const { pathname } = useLocation();
  const { initialState } = useInitialStateModel();
  const user = initialState?.currentUser;
  if (user && !canAccessPath(pathname, user.role, user.permissions, user.tenantId)) {
    return (
      <Result
        status="404"
        title="无法访问该页面"
        subTitle={ROUTE_FALLBACK_SUBTITLE}
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
