import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import {
  WAREHOUSE_SOUTH_ID,
  routeInventoryCenterData,
  warehouseRows,
  warehouseSummaryRows,
} from '../mocks/inventory-center.fixture';

const swappedSummaryRows = warehouseSummaryRows.map((r) => ({
  ...r,
  isDefault: r.warehouseId === WAREHOUSE_SOUTH_ID,
}));

test.describe('@inventory R115 默认仓切换', () => {
  test('sets a non-default warehouse as default (mocked write)', async ({ admin, page }) => {
    let switched = false;
    await routeInventoryCenterData(page);
    await page.route('**/api/v1/inventory/warehouses/summary', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ list: switched ? swappedSummaryRows : warehouseSummaryRows })),
      });
    });
    admin.writeGuard.allow({
      operation: 'set-default-warehouse',
      method: 'POST',
      path: /\/api\/v1\/inventory\/warehouses\/[^/]+\/set-default$/,
      response: () => {
        switched = true;
        return ok({ ...warehouseRows[1], isDefault: true });
      },
    });

    await admin.goto('/inventory/warehouses');
    await expect(page.getByText('仓库管理').first()).toBeVisible({ timeout: 60_000 });
    const southRow = page.getByRole('row', { name: /华南仓/ });
    await southRow.getByText('设为默认仓').click();
    await expect(page.getByText(/新默认仓将承接未分仓库存/)).toBeVisible();
    await page.getByRole('button', { name: '设为默认仓' }).click();

    await expect(page.getByText('默认仓已切换为「华南仓」')).toBeVisible();
    await admin.writeGuard.expectRequestCount('set-default-warehouse', 1);

    // 刷新后默认标签移动到新默认仓，锁定文案随之切换。
    await expect(southRow.getByText('默认仓', { exact: true })).toBeVisible();
    await expect(southRow.getByText('默认仓不可删除/停用')).toBeVisible();
    await expect(southRow.getByText('设为默认仓')).toHaveCount(0);
    const oldDefaultRow = page.getByRole('row', { name: /默认仓/ }).first();
    await expect(oldDefaultRow.getByText('设为默认仓')).toBeVisible();
  });

  test('readonly user sees no set-default action', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
      });
    });
    await routeInventoryCenterData(page);
    await admin.goto('/inventory/warehouses');
    await expect(page.getByText('仓库管理').first()).toBeVisible({ timeout: 20_000 });
    await expect(page.getByText('设为默认仓')).toHaveCount(0);
    await expect(page.getByText('只读').first()).toBeVisible();
  });
});
