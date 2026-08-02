import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';

test.describe('@product-draft 草稿列表只读角色写入口', () => {
  test('readonly 角色隐藏「新建草稿」按钮', async ({ page, admin }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
      });
    });
    await admin.goto('/product/drafts');
    await expect(page.getByRole('button', { name: '更多' })).toBeVisible();
    await expect(page.getByRole('button', { name: '新建草稿' })).toHaveCount(0);
  });

  test('admin 角色仍显示「新建草稿」按钮', async ({ page, admin }) => {
    await admin.goto('/product/drafts');
    await expect(page.getByRole('button', { name: '新建草稿' })).toBeVisible();
  });

  test('readonly 登录后经 SPA 路由进入草稿列表不出现「新建草稿」按钮', async ({ page, admin }) => {
    const readonlyUser = { ...e2eUser, role: 'readonly', permissions: [] };
    // 登录响应刻意不带 role/permissions，复现权限初始化时序场景
    admin.writeGuard.allow({
      operation: 'login',
      method: 'POST',
      path: /^\/api\/v1\/auth\/login$/,
      response: ok({
        token: 'e2e-readonly-token',
        expiresAt: Date.now() + 3600_000,
        user: { id: readonlyUser.id, username: readonlyUser.username, displayName: readonlyUser.displayName },
      }),
    });
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(readonlyUser)) });
    });
    await page.addInitScript(() => window.localStorage.removeItem('trademind_admin_token'));

    await page.goto('/user/login');
    await page.getByPlaceholder('请输入邮箱或手机号').fill('readonly@example.test');
    await page.getByPlaceholder('请输入登录密码').fill('readonly123');
    await page.getByRole('button', { name: '登录工作台' }).click();
    await expect(page).not.toHaveURL(/\/user\/login/);

    await page.getByRole('menuitem', { name: /商品$/ }).click();
    await page.getByRole('link', { name: '商品草稿' }).first().click();
    await expect(page).toHaveURL(/\/product\/drafts/);
    await expect(page.getByRole('button', { name: '更多' })).toBeVisible();
    await expect(page.getByRole('button', { name: '新建草稿' })).toHaveCount(0);
  });
});
