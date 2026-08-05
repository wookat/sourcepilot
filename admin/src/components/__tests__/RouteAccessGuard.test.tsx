import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import RouteAccessGuard, { ROUTE_FORBIDDEN_SUBTITLE } from '../RouteAccessGuard';
import NotFoundPage, { NOT_FOUND_SUBTITLE } from '@/pages/404';

let pathname = '/dashboard';
let currentUser: API.CurrentUser | undefined;

vi.mock('@umijs/max', () => ({
  history: { push: vi.fn(), location: { pathname: '/' } },
  useLocation: () => ({ pathname }),
}));

vi.mock('@/hooks/useInitialStateModel', () => ({
  useInitialStateModel: () => ({ initialState: { currentUser }, setInitialState: vi.fn() }),
}));

describe('RouteAccessGuard（权限空态与 404 语义分离）', () => {
  beforeEach(() => {
    pathname = '/dashboard';
    currentUser = undefined;
  });

  it('operator 访问受限路由时展示权限引导空态（非 404）', () => {
    pathname = '/settings/users';
    currentUser = {
      id: '1',
      username: 'op',
      displayName: 'op',
      role: 'operator',
      tenantId: 1,
    };
    render(
      <RouteAccessGuard>
        <div>secret page</div>
      </RouteAccessGuard>,
    );
    expect(screen.queryByText('secret page')).toBeNull();
    expect(screen.getByText('暂无访问权限')).toBeTruthy();
    expect(screen.getByText(ROUTE_FORBIDDEN_SUBTITLE)).toBeTruthy();
    expect(screen.queryByText('页面不存在')).toBeNull();
  });

  it('404 页使用独立的不存在语义文案', () => {
    render(<NotFoundPage />);
    expect(screen.getByText('页面不存在')).toBeTruthy();
    expect(screen.getByText(NOT_FOUND_SUBTITLE)).toBeTruthy();
    expect(screen.queryByText('暂无访问权限')).toBeNull();
  });

  it('有权限的路由正常渲染子内容', () => {
    pathname = '/orders/list';
    currentUser = {
      id: '1',
      username: 'op',
      displayName: 'op',
      role: 'operator',
      tenantId: 1,
    };
    render(
      <RouteAccessGuard>
        <div>orders page</div>
      </RouteAccessGuard>,
    );
    expect(screen.getByText('orders page')).toBeTruthy();
  });

  it('未登录（无 currentUser）时不拦截，交给登录守卫处理', () => {
    pathname = '/settings/users';
    currentUser = undefined;
    render(
      <RouteAccessGuard>
        <div>child</div>
      </RouteAccessGuard>,
    );
    expect(screen.getByText('child')).toBeTruthy();
  });
});
