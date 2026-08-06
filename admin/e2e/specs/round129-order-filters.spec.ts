import type { Page } from '@playwright/test';
import { test, expect } from '../fixtures/admin.fixture';
import { ok, paged } from '../mocks/envelope';
import type { NetworkWriteGuard } from '../utils/network-guard';

const ORDER_ROWS = [
  {
    id: 'e2e-order-r129-1',
    platform: 'douyin',
    orderNo: 'E2E-SO-1001',
    customerName: 'e2e-买家甲',
    status: 'pending',
    paymentStatus: 'paid',
    fulfillmentStatus: 'unfulfilled',
    currency: 'USD',
    totalAmount: 25.5,
    createdAt: '2026-08-01T02:03:04Z',
  },
  {
    id: 'e2e-order-r129-2',
    platform: 'douyin',
    orderNo: 'E2E-SO-2002',
    customerName: 'e2e-买家乙',
    status: 'pending',
    paymentStatus: 'unpaid',
    fulfillmentStatus: 'unfulfilled',
    currency: 'USD',
    totalAmount: 9.9,
    createdAt: '2026-08-02T02:03:04Z',
  },
];

/** 按后端 ILIKE 包含语义过滤，并记录每次列表请求的 query 参数 */
async function routeOrderList(page: Page, seen: URLSearchParams[]) {
  await page.route('**/api/v1/orders?*', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    const params = new URL(route.request().url()).searchParams;
    seen.push(params);
    const orderNo = (params.get('orderNo') || '').toLowerCase();
    const customerName = (params.get('customerName') || '').toLowerCase();
    const rows = ORDER_ROWS.filter(
      (r) =>
        (!orderNo || r.orderNo.toLowerCase().includes(orderNo)) &&
        (!customerName || r.customerName.toLowerCase().includes(customerName)),
    );
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok(paged(rows))),
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

test.describe('@round129 订单列表 订单号/客户 筛选（URL query 唯一来源）', () => {
  test('it should filter by 订单号 on submit and write it back to URL', async ({ admin, page }) => {
    const seen: URLSearchParams[] = [];
    await routeOrderList(page, seen);
    allowCostEstimates(admin);

    await admin.goto('/orders/list');
    const tbody = page.locator('.ant-table-tbody');
    await expect(tbody).toContainText('E2E-SO-1001');
    await expect(tbody).toContainText('E2E-SO-2002');

    await page.getByText('展开', { exact: true }).click();
    await page.getByLabel('订单号').fill('1001');
    await page.getByRole('button', { name: '查 询' }).click();

    await expect(page).toHaveURL(/orderNo=1001/);
    await expect(tbody).toContainText('E2E-SO-1001');
    await expect(tbody).not.toContainText('E2E-SO-2002');
    const last = seen[seen.length - 1];
    expect(last.get('orderNo')).toBe('1001');

    // 重置后筛选清空、恢复全量
    await page.getByRole('button', { name: '重 置' }).click();
    await expect(page).not.toHaveURL(/orderNo=/);
    await expect(tbody).toContainText('E2E-SO-2002');
  });

  test('it should filter by 客户 on submit', async ({ admin, page }) => {
    const seen: URLSearchParams[] = [];
    await routeOrderList(page, seen);
    allowCostEstimates(admin);

    await admin.goto('/orders/list');
    const tbody = page.locator('.ant-table-tbody');
    await expect(tbody).toContainText('E2E-SO-1001');

    await page.getByText('展开', { exact: true }).click();
    await page.getByLabel('客户').fill('买家乙');
    await page.getByRole('button', { name: '查 询' }).click();

    await expect(page).toHaveURL(/customerName=/);
    await expect(tbody).toContainText('E2E-SO-2002');
    await expect(tbody).not.toContainText('E2E-SO-1001');
    const last = seen[seen.length - 1];
    expect(last.get('customerName')).toBe('买家乙');
  });

  test('it should honor orderNo deep link and keep it after reload', async ({ admin, page }) => {
    const seen: URLSearchParams[] = [];
    await routeOrderList(page, seen);
    allowCostEstimates(admin);

    await admin.goto('/orders/list?orderNo=2002');
    const tbody = page.locator('.ant-table-tbody');
    await expect(tbody).toContainText('E2E-SO-2002');
    await expect(tbody).not.toContainText('E2E-SO-1001');
    await expect(page.getByLabel('订单号')).toHaveValue('2002');
    expect(seen.some((p) => p.get('orderNo') === '2002')).toBe(true);

    await page.reload();
    await expect(page).toHaveURL(/orderNo=2002/);
    await expect(tbody).toContainText('E2E-SO-2002');
    await expect(tbody).not.toContainText('E2E-SO-1001');
  });
});
