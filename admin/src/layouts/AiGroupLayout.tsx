import { Outlet, history, useLocation } from '@umijs/max';
import { useLayoutEffect } from 'react';
import { useInitialStateModel } from '@/hooks/useInitialStateModel';
import { canAccessPath } from '@/utils/menuAccess';

const PARENT = '/ai';
/** 平台管理员默认落到模板管理；业务租户账号落到自己可访问的 AI 任务页。 */
const PLATFORM_DEFAULT_CHILD = '/ai/prompts';
const TENANT_DEFAULT_CHILD = '/ai/tasks';

export default function AiGroupLayout() {
  const { pathname } = useLocation();
  const { initialState } = useInitialStateModel();
  const user = initialState?.currentUser;
  useLayoutEffect(() => {
    if (pathname === PARENT) {
      const canSeePrompts =
        !user || canAccessPath(PLATFORM_DEFAULT_CHILD, user.role, user.permissions, user.tenantId);
      history.replace(canSeePrompts ? PLATFORM_DEFAULT_CHILD : TENANT_DEFAULT_CHILD);
    }
  }, [pathname, user]);
  return <Outlet />;
}
