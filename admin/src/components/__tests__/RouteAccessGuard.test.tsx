import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import RouteAccessGuard, { ROUTE_FALLBACK_SUBTITLE } from '../RouteAccessGuard';

let pathname = '/dashboard';
let currentUser: API.CurrentUser | undefined;

vi.mock('@umijs/max', () => ({
  history: { push: vi.fn(), location: { pathname: '/' } },
  useLocation: () => ({ pathname }),
}));

vi.mock('@/hooks/useInitialStateModel', () => ({
  useInitialStateModel: () => ({ initialState: { currentUser }, setInitialState: vi.fn() }),
}));

describe('RouteAccessGuard（受限路由统一语义页）', () => {
  beforeEach(() => {
    pathname = '/dashboard';
    currentUser = undefined;
  });

  it('operator 访问受限路由时展示统一 404 语义页，不泄露存在性', () => {
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
    expect(screen.getByText('无法访问该页面')).toBeTruthy();
    expect(screen.getByText(ROUTE_FALLBACK_SUBTITLE)).toBeTruthy();
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
