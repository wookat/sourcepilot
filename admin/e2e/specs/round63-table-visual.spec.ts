import { test, expect } from '../fixtures/admin.fixture';
import { routeInventoryCenterData } from '../mocks/inventory-center.fixture';
import { expectNoRootOverflow } from '../utils/assertions';

test.describe('@round63 表格固定列与筛选收起', () => {
  test('375px 库存中心：操作列不固定，数据列可见且可横向滑动', async ({ admin, page }) => {
    await routeInventoryCenterData(page);
    await page.setViewportSize({ width: 375, height: 812 });
    await admin.goto('/inventory');
    await expect(page.getByText('库存中心').first()).toBeVisible({ timeout: 20_000 });
    await expect(page.getByRole('link', { name: 'E2E 保温杯 500ml' }).first()).toBeVisible();
    // 窄屏不应存在固定列（固定操作列会盖住数据列）
    await expect(page.locator('td.ant-table-cell-fix-right')).toHaveCount(0);
    await expect(page.locator('td.ant-table-cell-fix-left')).toHaveCount(0);
    // 表格容器可横向滑动查看被裁切的数据列
    const scrollable = await page
      .locator('.ant-pro-table .ant-table-content, .ant-pro-table .ant-table-body')
      .first()
      .evaluate((el) => el.scrollWidth > el.clientWidth);
    expect(scrollable).toBe(true);
    await expectNoRootOverflow(page);
  });

  test('1440px 库存中心：宽屏保留固定操作列', async ({ admin, page }) => {
    await routeInventoryCenterData(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    await admin.goto('/inventory');
    await expect(page.getByText('库存中心').first()).toBeVisible({ timeout: 20_000 });
    await expect(page.getByRole('link', { name: 'E2E 保温杯 500ml' }).first()).toBeVisible();
    await expect(page.locator('td.ant-table-cell-fix-right').first()).toBeVisible();
  });

  test('列表页筛选默认收起，可手动展开', async ({ admin, page }) => {
    await routeInventoryCenterData(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    await admin.goto('/inventory');
    await expect(page.getByText('库存中心').first()).toBeVisible({ timeout: 20_000 });
    const expand = page.getByRole('button', { name: '展开' }).or(page.getByText('展开').first());
    await expect(expand.first()).toBeVisible();
    // 收起态下仅保留少量筛选项（默认收起只显示首行）
    const visibleItems = await page.locator('.ant-pro-table-search .ant-form-item:visible').count();
    expect(visibleItems).toBeLessThanOrEqual(4);
    await expand.first().click();
    await expect(page.getByText('收起').first()).toBeVisible();
  });
});
