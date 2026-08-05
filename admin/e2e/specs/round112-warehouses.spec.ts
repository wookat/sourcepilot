import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import {
  INV_SKU_ID_LOW,
  WAREHOUSE_DEFAULT_ID,
  WAREHOUSE_SOUTH_ID,
  routeInventoryCenterData,
  warehouseRows,
} from '../mocks/inventory-center.fixture';
import { expectNoRootOverflow } from '../utils/assertions';

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

async function useReadonlyProfile(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/auth/profile', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
    });
  });
}

test.describe('@inventory R112 仓库管理', () => {
  test('lists warehouses with default lock hint and migration preview', async ({ admin, page }) => {
    await routeInventoryCenterData(page);
    await admin.goto('/inventory/warehouses');
    await expect(page.getByText('仓库管理').first()).toBeVisible({ timeout: 60_000 });
    await expect(page.getByText('默认仓', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('华南仓').first()).toBeVisible();
    await expect(page.getByText('默认仓不可删除/停用')).toBeVisible();

    await page.getByRole('button', { name: '存量迁移预检' }).click();
    await expect(page.getByText('迁移零丢失')).toBeVisible();
    await expect(page.getByText('存量迁移预检').nth(1)).toBeVisible();
  });

  test('creates a warehouse via modal (mocked write)', async ({ admin, page }) => {
    await routeInventoryCenterData(page);
    admin.writeGuard.allow({
      operation: 'create-warehouse',
      method: 'POST',
      path: /\/api\/v1\/inventory\/warehouses$/,
      response: ok({ ...warehouseRows[1], id: 'new-wh-id', code: 'north', name: '华北仓' }),
    });

    await admin.goto('/inventory/warehouses');
    await expect(page.getByText('仓库管理').first()).toBeVisible({ timeout: 20_000 });
    await page.getByRole('button', { name: '新建仓库' }).click();
    await page.getByLabel('仓库名称').fill('华北仓');
    await page.getByRole('button', { name: /保\s*存/ }).click();
    await expect(page.getByText('仓库已创建')).toBeVisible();
    await admin.writeGuard.expectRequestCount('create-warehouse', 1);
  });

  test('readonly user sees no write actions', async ({ admin, page }) => {
    await useReadonlyProfile(page);
    await routeInventoryCenterData(page);
    await admin.goto('/inventory/warehouses');
    await expect(page.getByText('仓库管理').first()).toBeVisible({ timeout: 20_000 });
    await expect(page.getByRole('button', { name: '新建仓库' })).toHaveCount(0);
    await expect(page.getByText('只读').first()).toBeVisible();
  });

  for (const viewport of viewports) {
    test(`no root overflow at ${viewport.width}x${viewport.height}`, async ({ admin, page }) => {
      await routeInventoryCenterData(page);
      await page.setViewportSize(viewport);
      await admin.goto('/inventory/warehouses');
      await expect(page.getByText('仓库管理').first()).toBeVisible({ timeout: 20_000 });
      await expectNoRootOverflow(page);
    });
  }
});

test.describe('@inventory R112 库存中心调拨', () => {
  test('shows per-warehouse breakdown and transfers stock atomically (mocked write)', async ({ admin, page }) => {
    await routeInventoryCenterData(page);
    admin.writeGuard.allow({
      operation: 'transfer-stock',
      method: 'POST',
      path: /\/api\/v1\/inventory\/transfers$/,
      response: ok({
        transferId: 'e2e-transfer-1',
        productSkuId: INV_SKU_ID_LOW,
        fromWarehouseId: WAREHOUSE_DEFAULT_ID,
        fromWarehouseName: '默认仓',
        toWarehouseId: WAREHOUSE_SOUTH_ID,
        toWarehouseName: '华南仓',
        quantity: 2,
        fromBefore: 56,
        fromAfter: 54,
        toBefore: 4,
        toAfter: 6,
        outLogId: 'e2e-log-out',
        inLogId: 'e2e-log-in',
      }),
    });

    await admin.goto('/inventory');
    await expect(page.getByText('库存中心').first()).toBeVisible({ timeout: 60_000 });
    await page.getByText('调拨', { exact: true }).first().click();
    await expect(page.getByText('仓库调拨', { exact: true })).toBeVisible();

    await page.getByLabel('源仓库').click();
    await page.locator('.ant-select-item-option', { hasText: '默认仓（默认） · 库存 56' }).click();
    await page.getByLabel('目标仓库').click();
    await page.locator('.ant-select-item-option', { hasText: '华南仓 · 库存 4' }).last().click();
    await page.getByLabel('调拨数量').fill('2');
    await page.getByRole('button', { name: '确认调拨' }).click();

    await expect(page.getByText(/调拨成功：默认仓 → 华南仓，数量 2/)).toBeVisible();
    await admin.writeGuard.expectRequestCount('transfer-stock', 1);
    const call = admin.writeGuard.calls('transfer-stock')[0];
    expect(call.postDataJSON).toMatchObject({
      productSkuId: expect.any(String),
      fromWarehouseId: WAREHOUSE_DEFAULT_ID,
      toWarehouseId: WAREHOUSE_SOUTH_ID,
      quantity: 2,
    });
  });
});

test.describe('@reports R112 库存报表按仓筛选', () => {
  test('warehouse filter changes report query and subtitle', async ({ admin, page }) => {
    await routeInventoryCenterData(page);
    const seenWarehouseIds: (string | null)[] = [];
    await page.route('**/api/v1/reports/inventory**', async (route) => {
      const url = new URL(route.request().url());
      seenWarehouseIds.push(url.searchParams.get('warehouseId'));
      const wid = url.searchParams.get('warehouseId');
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            generatedAt: '2026-08-01T00:00:00Z',
            slowDays: 30,
            currency: 'CNY',
            warehouseId: wid || undefined,
            warehouseName: wid ? '华南仓' : undefined,
            summary: {
              skuCount: wid ? 1 : 2,
              totalStock: wid ? 4 : 62,
              stockValueCny: wid ? 40 : 620,
              valuedSkuCount: wid ? 1 : 2,
              unvaluedSkuCount: 0,
              lowStockCount: wid ? 0 : 1,
              outOfStockCount: 0,
              slowMovingCount: 0,
            },
            slowMoving: [],
            lowStock: [],
          }),
        ),
      });
    });

    await admin.goto('/orders/reports-inventory');
    await expect(page.getByText('库存报表').first()).toBeVisible({ timeout: 60_000 });
    await expect(page.getByText('全仓汇总').first()).toBeVisible();

    await page.getByLabel('仓库筛选').first().click();
    await page.locator('.ant-select-item-option', { hasText: '华南仓' }).click();
    await expect(page.getByText('当前仓库：华南仓')).toBeVisible();
    expect(seenWarehouseIds).toContain(WAREHOUSE_SOUTH_ID);
  });
});
