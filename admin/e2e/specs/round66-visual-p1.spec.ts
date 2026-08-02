import { test, expect } from '../fixtures/admin.fixture';
import { ok, paged } from '../mocks/envelope';
import { routeInventoryCenterData } from '../mocks/inventory-center.fixture';
import { expectNoRootOverflow } from '../utils/assertions';

function isoDaysAgo(offset: number): string {
  const d = new Date();
  d.setDate(d.getDate() - offset);
  return d.toISOString().slice(0, 10);
}

function sparseDailyStats(days: number) {
  const items = [];
  for (let i = days - 1; i >= 0; i--) {
    items.push({
      date: isoDaysAgo(i),
      orderCount: i === 0 ? 3 : 0,
      paidCount: i === 0 ? 2 : 0,
      shippedCount: 0,
      paidAmounts: i === 0 ? [{ currency: 'USD', amount: 66.5, orders: 2 }] : [],
    });
  }
  return ok({ generatedAt: new Date().toISOString(), days, items });
}

async function mockOrdersList(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/orders?*', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        ok(
          paged([
            {
              id: 'e2e-order-r66',
              platform: 'douyin_shop',
              orderNo: 'SO-20260801-000123',
              customerName: 'e2e 买家',
              status: 'pending',
              paymentStatus: 'paid',
              fulfillmentStatus: 'unshipped',
              currency: 'USD',
              totalAmount: 25.5,
              createdAt: '2026-08-01T02:03:04Z',
            },
          ]),
        ),
      ),
    });
  });
}

test.describe('@round66-visual-p1 R66 视觉 P1 收口', () => {
  test('it should render sparse-data column chart without overflow (barMaxWidth)', async ({ admin, page }) => {
    await page.route('**/api/v1/orders/stats/daily?*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(sparseDailyStats(7)),
      });
    });
    await admin.goto('/orders/reports');
    await expect(page.getByText('近 30 天合计')).toBeVisible();
    await expect(page.locator('canvas')).toHaveCount(2);
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test.describe('固定列口径：按表格容器宽度而非视口宽度', () => {
    for (const width of [768, 834]) {
      test(`it should disable fixed columns at ${width}px viewport (content area too narrow)`, async ({
        admin,
        page,
      }) => {
        await routeInventoryCenterData(page);
        await page.setViewportSize({ width, height: 900 });
        await admin.goto('/inventory');
        await expect(page.getByText('库存中心').first()).toBeVisible({ timeout: 20_000 });
        await expect(page.getByRole('link', { name: 'E2E 保温杯 500ml' }).first()).toBeVisible();
        await expect(page.locator('td.ant-table-cell-fix-right')).toHaveCount(0);
        await expect(page.locator('td.ant-table-cell-fix-left')).toHaveCount(0);
        await expectNoRootOverflow(page);
      });
    }

    test('it should keep fixed action column at 1440px viewport', async ({ admin, page }) => {
      await routeInventoryCenterData(page);
      await page.setViewportSize({ width: 1440, height: 900 });
      await admin.goto('/inventory');
      await expect(page.getByText('库存中心').first()).toBeVisible({ timeout: 20_000 });
      await expect(page.getByRole('link', { name: 'E2E 保温杯 500ml' }).first()).toBeVisible();
      await expect(page.locator('td.ant-table-cell-fix-right').first()).toBeVisible();
    });
  });

  test('it should keep order number on a single line at 1440px', async ({ admin, page }) => {
    await mockOrdersList(page);
    admin.writeGuard.allow({
      operation: 'cost-estimates-batch',
      method: 'POST',
      path: /\/api\/v1\/procurement\/cost-estimates\/batch$/,
      response: ok(null),
    });
    await page.setViewportSize({ width: 1440, height: 900 });
    await admin.goto('/orders/list');
    // ellipsis 生效时可见文本可能被截断为 SO-20260801-000…，按前缀匹配
    const orderNoText = page.locator('.ant-table-tbody td', { hasText: /SO-20260801-000/ }).first();
    await expect(orderNoText).toBeVisible();
    // 单行渲染：订单号文本高度不超过 1.5 倍行高（折行时约为两倍行高）
    const singleLine = await orderNoText.evaluate((td) => {
      const target = td.querySelector('span, div') ?? td;
      const lineHeight = parseFloat(getComputedStyle(target).lineHeight) || 22;
      return (target as HTMLElement).offsetHeight <= lineHeight * 1.5;
    });
    expect(singleLine).toBe(true);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should map raw platform enum to PlatformTag on AI workbench', async ({ admin, page }) => {
    await page.route('**/api/v1/ai/operation-workbench/todos?*', async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            items: [
              {
                id: 'e2e-todo-1',
                type: 'ai_text_review',
                typeLabel: 'AI 文案复核',
                priority: 'P1',
                priorityLabel: '高',
                productTitle: 'e2e 商品',
                platform: 'douyin_shop',
                shopName: 'e2e-shop',
                title: '标题待复核',
                message: 'AI 生成标题等待人工复核',
                actionLabel: '去复核',
                actionUrl: '/product/drafts',
                sourceType: 'ai_task',
                sourceId: 'e2e-src-1',
                createdAt: '2026-08-01T02:00:00Z',
                updatedAt: '2026-08-01T02:03:04Z',
              },
            ],
            pagination: { page: 1, pageSize: 20, total: 1 },
          }),
        ),
      });
    });
    await admin.goto('/ai/operation-workbench');
    await expect(page.getByText('待办列表')).toBeVisible();
    // 裸枚举 douyin_shop 不再直出，展示 PlatformTag 中文名
    const tbody = page.locator('.ant-table-tbody');
    await expect(tbody.getByText('抖店')).toBeVisible();
    await expect(tbody.getByText('douyin_shop')).toHaveCount(0);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should show 0 instead of NaN when dashboard summary misses fields', async ({ admin, page }) => {
    await page.route('**/api/v1/dashboard/product-operations**', async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            // summary 刻意缺少 lowStockSkus / outOfStockSkus / draftProducts 等字段
            summary: { inventorySyncFailedCount: 0 },
            todos: [],
            funnel: [],
            exceptions: [],
            charts: {},
            quickLinks: [],
            recent: {},
          }),
        ),
      });
    });
    await admin.goto('/dashboard/product-operations');
    await expect(page.getByText('库存异常').first()).toBeVisible({ timeout: 20_000 });
    await expect(page.locator('.ant-pro-page-container')).not.toContainText('NaN');
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
