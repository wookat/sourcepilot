import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';

async function useReadonlyProfile(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/auth/profile', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
    });
  });
}

test.describe('@product-draft readonly 写入口', () => {
  test('readonly hides create and more actions on drafts list', async ({ admin, page }) => {
    await useReadonlyProfile(page);
    await admin.goto('/product/drafts');
    await expect(page.getByText('商品草稿').first()).toBeVisible();
    await expect(page.getByRole('button', { name: '新建草稿' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: '更多' })).toHaveCount(0);
  });

  test('admin keeps create and more actions on drafts list', async ({ admin, page }) => {
    await admin.goto('/product/drafts');
    await expect(page.getByRole('button', { name: '新建草稿' })).toBeVisible();
    await expect(page.getByRole('button', { name: '更多' })).toBeVisible();
  });
});
