import type { Page } from '@playwright/test';
import { ok, paged } from './envelope';

export const INV_PRODUCT_ID = '11111111-1111-4111-8111-111111111111';
export const INV_SKU_ID = '22222222-2222-4222-8222-222222222222';
export const INV_SKU_ID_LOW = '33333333-3333-4333-8333-333333333333';
export const INV_ORDER_ID = '44444444-4444-4444-8444-444444444444';

const alertRowBase = {
  productId: INV_PRODUCT_ID,
  productTitle: 'E2E 保温杯 500ml',
  publicationCount: 0,
  platformStocks: [],
  lastInventoryChangeAt: '2026-01-02T03:04:05Z',
};

export const centerRows = [
  {
    ...alertRowBase,
    productSkuId: INV_SKU_ID,
    skuCode: 'E2E-SKU-1',
    skuName: '默认规格-1',
    stock: 60,
    availableStock: 60,
    warningStock: 10,
    safetyStock: 2,
    stockStatus: 'normal',
    alertTypes: [],
    skuBindStatus: 'no_publication',
    platformSyncStatus: 'not_synced',
    exceptionCount: 0,
  },
  {
    ...alertRowBase,
    productSkuId: INV_SKU_ID_LOW,
    skuCode: 'E2E-SKU-2',
    skuName: '默认规格-2',
    stock: 2,
    availableStock: 2,
    warningStock: 10,
    safetyStock: 2,
    stockStatus: 'low_stock',
    alertTypes: ['low_stock'],
    skuBindStatus: 'no_publication',
    platformSyncStatus: 'not_synced',
    exceptionCount: 0,
  },
];

export const alertRows = [
  {
    ...alertRowBase,
    productSkuId: INV_SKU_ID_LOW,
    skuCode: 'E2E-SKU-2',
    skuName: '默认规格-2',
    stock: 2,
    warningStock: 10,
    safetyStock: 2,
    stockStatus: 'low_stock',
    alertTypes: ['low_stock'],
  },
];

export const logRows = [
  {
    id: 'aaaaaaa1-0000-4000-8000-000000000001',
    createdAt: '2026-01-02T03:04:05Z',
    productId: INV_PRODUCT_ID,
    productSkuId: INV_SKU_ID,
    productTitle: 'E2E 保温杯 500ml',
    skuCode: 'E2E-SKU-1',
    skuName: '默认规格-1',
    changeType: 'manual_adjust',
    beforeStock: 50,
    afterStock: 60,
    delta: 10,
    reason: 'restock',
    remark: '盘点调整',
  },
  {
    id: 'aaaaaaa1-0000-4000-8000-000000000002',
    createdAt: '2026-01-02T03:04:06Z',
    productId: INV_PRODUCT_ID,
    productSkuId: INV_SKU_ID,
    productTitle: 'E2E 保温杯 500ml',
    skuCode: 'E2E-SKU-1',
    skuName: '默认规格-1',
    changeType: 'order_deduct',
    beforeStock: 60,
    afterStock: 58,
    delta: -2,
    reason: 'order_paid',
    remark: '订单扣减',
    refOrderId: INV_ORDER_ID,
    refOrderNo: 'E2E-SO-0001',
    refOrderItemId: 'bbbbbbb1-0000-4000-8000-000000000001',
  },
];

export const WAREHOUSE_DEFAULT_ID = '55555555-5555-4555-8555-555555555555';
export const WAREHOUSE_SOUTH_ID = '66666666-6666-4666-8666-666666666666';

export const warehouseRows = [
  {
    id: WAREHOUSE_DEFAULT_ID,
    tenantId: 1,
    code: 'default',
    name: '默认仓',
    isDefault: true,
    enabled: true,
    priority: 0,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
  {
    id: WAREHOUSE_SOUTH_ID,
    tenantId: 1,
    code: 'south',
    name: '华南仓',
    isDefault: false,
    enabled: true,
    priority: 10,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
];

export const warehouseSummaryRows = [
  {
    warehouseId: WAREHOUSE_DEFAULT_ID,
    warehouseName: '默认仓',
    code: 'default',
    isDefault: true,
    enabled: true,
    priority: 0,
    totalStock: 58,
    skuCount: 2,
  },
  {
    warehouseId: WAREHOUSE_SOUTH_ID,
    warehouseName: '华南仓',
    code: 'south',
    isDefault: false,
    enabled: true,
    priority: 10,
    totalStock: 4,
    skuCount: 1,
  },
];

export const skuWarehouseStocks = [
  {
    warehouseId: WAREHOUSE_DEFAULT_ID,
    warehouseName: '默认仓',
    isDefault: true,
    enabled: true,
    stock: 56,
  },
  {
    warehouseId: WAREHOUSE_SOUTH_ID,
    warehouseName: '华南仓',
    isDefault: false,
    enabled: true,
    stock: 4,
  },
];

/** 覆盖库存中心相关 GET 接口为固定造数（后注册的 route 优先）。 */
export async function routeInventoryCenterData(page: Page, opts?: { emptyLogs?: boolean }) {
  await page.route('**/api/v1/inventory**', async (route) => {
    const request = route.request();
    if (request.method().toUpperCase() !== 'GET') {
      await route.fallback();
      return;
    }
    const path = new URL(request.url()).pathname;
    if (path === '/api/v1/inventory') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(paged(centerRows))) });
      return;
    }
    if (path === '/api/v1/inventory/alerts') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(paged(alertRows))) });
      return;
    }
    if (path === '/api/v1/inventory/logs') {
      const rows = opts?.emptyLogs ? [] : logRows;
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(paged(rows))) });
      return;
    }
    if (path === '/api/v1/inventory/warehouses') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ list: warehouseRows, total: warehouseRows.length })),
      });
      return;
    }
    if (path === '/api/v1/inventory/warehouses/summary') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ list: warehouseSummaryRows })),
      });
      return;
    }
    if (path === '/api/v1/inventory/warehouses/migration-preview') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            defaultWarehouseExists: true,
            defaultWarehouseId: WAREHOUSE_DEFAULT_ID,
            defaultWarehouseName: '默认仓',
            skuCount: 2,
            totalStock: 62,
            nonDefaultStock: 4,
            defaultDerivedStock: 58,
            orphanWarehouseRows: 0,
            negativeDerivedSkus: 0,
            consistent: true,
          }),
        ),
      });
      return;
    }
    if (path === '/api/v1/inventory/sku-warehouse-stocks') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ list: skuWarehouseStocks, totalStock: 60 })),
      });
      return;
    }
    await route.fallback();
  });
}
