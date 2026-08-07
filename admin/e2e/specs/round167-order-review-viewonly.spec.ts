import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import { e2eReviewWorkbenchRows } from '../mocks/order-review';

const SHOP_VIEW = 'e2e-shop-view-only';
const SHOP_OP = 'e2e-shop-operate';

const rowsWithShops = [
  { ...e2eReviewWorkbenchRows[0], shopId: SHOP_VIEW, shopName: 'View 店' },
  { ...e2eReviewWorkbenchRows[1], shopId: SHOP_OP, shopName: 'Operate 店' },
];

test.describe('@round167 审单工作台 view-only 店铺预禁用', () => {
  test.beforeEach(async ({ page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            ...e2eUser,
            role: 'operator',
            permissions: [],
            storePermissions: [
              { storeId: SHOP_VIEW, platform: 'manual', permissionScope: 'view' },
              { storeId: SHOP_OP, platform: 'manual', permissionScope: 'operate' },
            ],
          }),
        ),
      });
    });
    await page.route('**/api/v1/order-review?*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            items: rowsWithShops,
            total: rowsWithShops.length,
            page: 1,
            pageSize: 20,
            totalPages: 1,
            pendingTotal: rowsWithShops.length,
          }),
        ),
      });
    });
  });

  test('view-only 店铺行放行/拒绝按钮与勾选预禁用，operate 店铺行可操作', async ({
    admin,
    page,
  }) => {
    await admin.goto('/orders/review');
    const viewRow = page.getByRole('row', { name: /SO-E2E-HELD-1/ });
    await expect(viewRow.getByRole('button', { name: '放行' })).toBeDisabled({ timeout: 20000 });
    await expect(viewRow.getByRole('button', { name: '拒绝' })).toBeDisabled();
    await expect(viewRow.getByRole('checkbox')).toBeDisabled();

    const opRow = page.getByRole('row', { name: /SO-E2E-PEND-1/ });
    await expect(opRow.getByRole('button', { name: '放行' })).toBeEnabled();
    await expect(opRow.getByRole('button', { name: '拒绝' })).toBeEnabled();
    await expect(opRow.getByRole('checkbox')).toBeEnabled();
  });
});
