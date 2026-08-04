import { test, expect } from '../fixtures/admin.fixture';
import { ok, paged } from '../mocks/envelope';
import { expectNoRootOverflow } from '../utils/assertions';

function isoDaysAgo(offset: number): string {
  const d = new Date();
  d.setDate(d.getDate() - offset);
  return d.toISOString().slice(0, 10);
}

function bigDailyStats(days: number) {
  const items = [];
  for (let i = days - 1; i >= 0; i--) {
    items.push({
      date: isoDaysAgo(i),
      orderCount: i === 0 ? 1234 : 100,
      paidCount: i === 0 ? 1000 : 80,
      shippedCount: 0,
      paidAmounts: i === 0 ? [{ currency: 'USD', amount: 12345.5, orders: 1000 }] : [],
    });
  }
  return ok({ generatedAt: new Date().toISOString(), days, items });
}

function emptyDailyStats(days: number) {
  const items = [];
  for (let i = days - 1; i >= 0; i--) {
    items.push({ date: isoDaysAgo(i), orderCount: 0, paidCount: 0, shippedCount: 0, paidAmounts: [] });
  }
  return ok({ generatedAt: new Date().toISOString(), days, items });
}

test.describe('@round64-charts-visual R64 报表图表规范', () => {
  test('it should render totals with thousand separators and both charts', async ({ admin, page }) => {
    await page.route('**/api/v1/orders/stats/daily?*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(bigDailyStats(30)),
      });
    });
    await admin.goto('/orders/reports');
    await expect(page.getByText('近 30 天合计')).toBeVisible();
    // 千分位：订单数 1234 + 100*29 = 4,134；已付款 1000 + 80*29 = 3,320
    await expect(page.getByText('4,134')).toBeVisible();
    await expect(page.getByText('3,320')).toBeVisible();
    // 销售额千分位（Statistic precision=2 分组展示）
    await expect(page.getByText('原币销售额（USD）')).toBeVisible();
    await expect(page.getByText(/12,345\.50/)).toBeVisible();
    // 三张图表 canvas 均渲染（订单数/本位币折算/原币明细）
    await expect(page.locator('canvas')).toHaveCount(3);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should show empty guidance when there are no orders', async ({ admin, page }) => {
    await page.route('**/api/v1/orders/stats/daily?*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(emptyDailyStats(30)),
      });
    });
    await admin.goto('/orders/reports');
    await expect(page.getByText('近 30 天暂无订单')).toBeVisible();
    await expect(page.getByText('近 30 天暂无已付款订单').first()).toBeVisible();
    await expect(page.locator('canvas')).toHaveCount(0);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should keep charts inside viewport at 375px', async ({ admin, page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await page.route('**/api/v1/orders/stats/daily?*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(bigDailyStats(30)),
      });
    });
    await admin.goto('/orders/reports');
    await expect(page.getByText('近 30 天合计')).toBeVisible();
    await expect(page.locator('canvas')).toHaveCount(3);
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should not crash the order list when cost-estimates batch returns data null', async ({ admin, page }) => {
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
                id: 'e2e-order-r64',
                platform: 'douyin',
                orderNo: 'E2E-R64-001',
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
    admin.writeGuard.allow({
      operation: 'cost-estimates-batch',
      method: 'POST',
      path: /\/api\/v1\/procurement\/cost-estimates\/batch$/,
      response: ok(null),
    });
    await admin.goto('/orders/list');
    await expect(page.locator('.ant-table-tbody')).toContainText('E2E-R64-001');
    await admin.writeGuard.expectRequestCount('cost-estimates-batch', 1);
    // data:null 防御后毛利列展示占位而非白屏
    await expect(page.locator('.ant-table-tbody')).toContainText('—');
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
