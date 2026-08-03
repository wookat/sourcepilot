import { test, expect } from '../fixtures/admin.fixture';
import { ok, paged } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';

const e2eShop = {
  id: 'e2e-shop-douyin',
  platform: 'douyin_shop',
  shopName: 'e2e-授权店铺',
  status: 'active',
  authStatus: 'authorized',
  updatedAt: '2026-01-01T00:00:00Z',
};

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
    await expect(page.getByText('商品草稿').first()).toBeVisible();
    await expect(page.getByRole('button', { name: '新建草稿' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: '更多' })).toHaveCount(0);
  });

  test('admin 角色仍显示「新建草稿」按钮', async ({ page, admin }) => {
    await admin.goto('/product/drafts');
    await expect(page.getByRole('button', { name: '新建草稿' })).toBeVisible();
  });

  test('operator 角色显示「新建草稿」按钮，弹窗归属店铺必填且只列授权店铺', async ({ page, admin }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'operator' })),
      });
    });
    await page.route('**/api/v1/shops**', async (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok(paged([e2eShop]))),
      });
    });
    admin.writeGuard.allow({
      operation: 'create-draft',
      method: 'POST',
      path: /^\/api\/v1\/products$/,
      response: ok({ id: 'e2e-product-new', title: 'e2e-operator-draft' }),
    });

    await admin.goto('/product/drafts');
    await page.getByRole('button', { name: '新建草稿' }).click();
    await expect(page.getByText('新建商品草稿')).toBeVisible();

    // 未选店铺提交：前端必填校验拦截，不发写请求
    await page.getByRole('textbox', { name: '* 标题' }).fill('e2e-operator-draft');
    await page.getByRole('button', { name: '确 认' }).or(page.getByRole('button', { name: '确 定' })).first().click();
    await expect(page.getByText('请选择归属店铺').first()).toBeVisible();
    await admin.writeGuard.expectRequestCount('create-draft', 0);

    // 选择授权店铺后提交成功，POST payload 带 shopId
    await page.getByLabel('归属店铺').click();
    await page.getByText('e2e-授权店铺 (抖店)').click();
    await page.getByRole('button', { name: '确 认' }).or(page.getByRole('button', { name: '确 定' })).first().click();
    await expect(page.getByText('草稿已创建')).toBeVisible();
    await admin.writeGuard.expectRequestCount('create-draft', 1);
    const reqs = admin.writeGuard.calls('create-draft');
    expect((reqs[0].postDataJSON as { shopId?: string }).shopId).toBe('e2e-shop-douyin');
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
    await expect(page.getByText('商品草稿').first()).toBeVisible();
    await expect(page.getByRole('button', { name: '新建草稿' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: '更多' })).toHaveCount(0);
  });
});
