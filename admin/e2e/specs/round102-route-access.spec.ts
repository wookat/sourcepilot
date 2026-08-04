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

test.describe('@round102 受限路由统一语义页', () => {
  test('operator 直达受限路由展示统一中文语义页，不泄露存在性', async ({ admin, page }) => {
    await mockOperatorProfile(page);
    await admin.goto('/settings/users');
    await expect(page.getByText('无法访问该页面')).toBeVisible();
    await expect(page.getByText(/页面不存在，或当前账号无权访问/)).toBeVisible();
    await expect(page.getByRole('button', { name: '返回工作台' })).toBeVisible();
  });

  test('operator 访问有权限路由正常渲染', async ({ admin, page }) => {
    await mockOperatorProfile(page);
    await admin.goto('/dashboard');
    await expect(page.getByText('无法访问该页面')).toHaveCount(0);
  });

  test('admin 访问设置路由不被拦截', async ({ admin, page }) => {
    await admin.goto('/settings/users');
    await expect(page.getByText('无法访问该页面')).toHaveCount(0);
  });
});
