import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';

const operatorUser = {
  ...e2eUser,
  id: 'e2e-operator',
  username: 'e2e-operator',
  displayName: 'E2E 运营',
  role: 'operator',
  tenantId: 1,
};

async function mockOperatorProfile(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/auth/profile', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok(operatorUser)),
    });
  });
}

test.describe('@round102 受限路由权限空态', () => {
  test('operator 直达受限路由展示权限引导空态', async ({ admin, page }) => {
    await mockOperatorProfile(page);
    await admin.goto('/settings/users');
    await expect(page.getByText('暂无访问权限')).toBeVisible();
    await expect(page.getByText(/当前账号没有访问该页面的权限/)).toBeVisible();
    await expect(page.getByRole('button', { name: '返回工作台' })).toBeVisible();
  });

  test('不存在的路由展示 404 语义页（非权限空态）', async ({ admin, page }) => {
    await mockOperatorProfile(page);
    await admin.goto('/route-not-exists-e2e');
    await expect(page.getByText('页面不存在')).toBeVisible();
    await expect(page.getByText('暂无访问权限')).toHaveCount(0);
  });

  test('operator 访问有权限路由正常渲染', async ({ admin, page }) => {
    await mockOperatorProfile(page);
    await admin.goto('/dashboard');
    await expect(page.getByText('暂无访问权限')).toHaveCount(0);
  });

  test('admin 访问设置路由不被拦截', async ({ admin, page }) => {
    await admin.goto('/settings/users');
    await expect(page.getByText('暂无访问权限')).toHaveCount(0);
  });
});
