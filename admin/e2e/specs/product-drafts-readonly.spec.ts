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
});
