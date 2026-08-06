import type { Page } from '@playwright/test';
import { test, expect } from '../fixtures/admin.fixture';
import { ok, paged, fail } from '../mocks/envelope';
import type { NetworkWriteGuard } from '../utils/network-guard';

const OFFSET_TOO_DEEP = 'pagination_offset_too_deep';
const MAX_OFFSET = 10000;

const ORDER_ROWS = [
  {
    id: 'e2e-order-r139-1',
    platform: 'douyin',
    orderNo: 'E2E-SO-R139-1',
    customerName: 'e2e-买家甲',
    status: 'pending',
    paymentStatus: 'paid',
    fulfillmentStatus: 'unfulfilled',
    currency: 'USD',
    totalAmount: 25.5,
    createdAt: '2026-08-01T02:03:04Z',
  },
];

/** 按后端 pagination.NormalizePage 口径：offset > 10000 返回 400 + 稳定 code */
async function routeOrderListWithDepthGuard(page: Page) {
  await page.route('**/api/v1/orders?*', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    const params = new URL(route.request().url()).searchParams;
    const pageNo = Number(params.get('page') || '1');
    const pageSize = Number(params.get('pageSize') || '20');
    if ((pageNo - 1) * pageSize > MAX_OFFSET) {
      await route.fulfill({
        status: 400,
        contentType: 'application/json',
        body: JSON.stringify(fail(OFFSET_TOO_DEEP, 40001, { errorCode: OFFSET_TOO_DEEP })),
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok(paged(ORDER_ROWS, 20000, pageNo, pageSize))),
    });
  });
}

function allowCostEstimates(admin: { writeGuard: NetworkWriteGuard }) {
  admin.writeGuard.allow({
    operation: 'cost-estimates-batch',
    method: 'POST',
    path: /\/api\/v1\/procurement\/cost-estimates\/batch$/,
    response: ok({ items: {} }),
  });
}

test.describe('@round139 P2-1 订单列表深分页可读提示', () => {
  test('offset 超限时给出中文提示而非静默失败', async ({ admin, page }) => {
    admin.consoleGuard.allowForTest(/400 \(Bad Request\)|Request failed with status code 400/);
    await routeOrderListWithDepthGuard(page);
    allowCostEstimates(admin);

    await admin.goto('/orders/list');
    await expect(page.locator('.ant-table-tbody')).toContainText('E2E-SO-R139-1');

    await admin.goto('/orders/list?page=502&pageSize=20');
    await expect(page.locator('.ant-message')).toContainText('页码过深，请缩小筛选范围或降低页码后重试');
  });
});

test.describe('@round139 P2-2 订单行本地规格编号未绑定口径', () => {
  const DETAIL = {
    id: 'e2e-order-r139-detail',
    platform: 'douyin',
    orderNo: 'E2E-SO-R139-D',
    customerName: 'e2e-买家甲',
    status: 'pending',
    paymentStatus: 'paid',
    fulfillmentStatus: 'unfulfilled',
    currency: 'USD',
    totalAmount: 25.5,
    createdAt: '2026-08-01T02:03:04Z',
    items: [
      {
        id: 'e2e-item-unbound',
        orderId: 'e2e-order-r139-detail',
        productTitle: 'e2e-未绑定商品',
        skuCode: 'HAND-INPUT-001',
        quantity: 1,
        unitPrice: 9.9,
        totalPrice: 9.9,
        createdAt: '2026-08-01T02:03:04Z',
        updatedAt: '2026-08-01T02:03:04Z',
      },
      {
        id: 'e2e-item-bound',
        orderId: 'e2e-order-r139-detail',
        productTitle: 'e2e-已绑定商品',
        productSkuId: 'e2e-sku-1',
        skuCode: 'HAND-INPUT-002',
        quantity: 2,
        unitPrice: 5,
        totalPrice: 10,
        createdAt: '2026-08-01T02:03:04Z',
        updatedAt: '2026-08-01T02:03:04Z',
      },
    ],
    tags: [],
  };

  const SKU_MATCHES = {
    items: [
      {
        id: 'e2e-match-unbound',
        orderItemId: 'e2e-item-unbound',
        matchStatus: 'unmatched',
      },
      {
        id: 'e2e-match-bound',
        orderItemId: 'e2e-item-bound',
        matchStatus: 'matched',
        productSkuId: 'e2e-sku-1',
        localSkuCode: 'LOCAL-SKU-001',
      },
    ],
  };

  test('未绑定行显示「未绑定」，已绑定行显示本地规格编号', async ({ admin, page }) => {
    await page.route(`**/api/v1/orders/${DETAIL.id}`, async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok(DETAIL)),
      });
    });
    await page.route(`**/api/v1/orders/${DETAIL.id}/sku-matches`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok(SKU_MATCHES)),
      });
    });
    allowCostEstimates(admin);

    await admin.goto(`/orders/${DETAIL.id}`);
    await page.getByRole('tab', { name: '商品明细' }).click();
    const unboundRow = page.getByRole('row', { name: /e2e-未绑定商品/ });
    await expect(unboundRow).toBeVisible({ timeout: 20000 });
    await expect(unboundRow).toContainText('未绑定');
    await expect(unboundRow).not.toContainText('HAND-INPUT-001');

    const boundRow = page.getByRole('row', { name: /e2e-已绑定商品/ });
    await expect(boundRow).toContainText('LOCAL-SKU-001');
  });
});
