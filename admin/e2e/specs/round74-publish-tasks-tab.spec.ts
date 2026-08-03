import { test, expect } from '../fixtures/admin.fixture';
import { ok, paged } from '../mocks/envelope';

async function mockPublishLists(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/product-publish/tasks?*', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok(paged([]))),
    });
  });
  await page.route('**/api/v1/product-publish/batches?*', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok(paged([]))),
    });
  });
}

test.describe('R74 商品刊登任务 Tab 切换', () => {
  test('切到「子任务」后内容面板展示刊登记录表，刷新后保持', async ({ admin }) => {
    const { page } = admin;
    await mockPublishLists(page);

    await admin.goto('/product/publish-tasks');
    await expect(page.getByText('批量刊登批次', { exact: true })).toBeVisible();

    await page.getByRole('tab', { name: '子任务' }).click();
    await expect(page).toHaveURL(/tab=tasks/);
    const activePane = page.locator('.ant-tabs-tabpane-active');
    await expect(activePane.getByText('刊登记录')).toBeVisible();
    await expect(activePane.getByText('暂无刊登任务')).toBeVisible();

    await page.reload();
    await expect(page).toHaveURL(/tab=tasks/);
    await expect(page.locator('.ant-tabs-tabpane-active').getByText('刊登记录')).toBeVisible();
  });

  test('「刊登批次」空状态使用批次文案，子任务空状态使用任务文案', async ({ admin }) => {
    const { page } = admin;
    await mockPublishLists(page);

    await admin.goto('/product/publish-tasks');
    await expect(page.locator('.ant-tabs-tabpane-active').getByText('暂无刊登批次')).toBeVisible();

    await page.getByRole('tab', { name: '子任务' }).click();
    await expect(page.locator('.ant-tabs-tabpane-active').getByText('暂无刊登任务')).toBeVisible();
  });
});
