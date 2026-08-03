import { test, expect } from '../fixtures/admin.fixture';
import { ok, paged } from '../mocks/envelope';

const ISO_AT = '2026-08-01T02:03:04Z';

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
              id: 'e2e-order-r69',
              platform: 'douyin_shop',
              shopId: 'e2e-shop-douyin',
              shopName: 'e2e 抖店旗舰店',
              shopPlatform: 'douyin_shop',
              orderNo: 'SO-20260801-000069',
              customerName: 'e2e 买家',
              status: 'pending',
              paymentStatus: 'paid',
              fulfillmentStatus: 'unshipped',
              currency: 'USD',
              totalAmount: 25.5,
              createdAt: ISO_AT,
            },
          ]),
        ),
      ),
    });
  });
}

function dashboardPayload() {
  return {
    summary: {},
    todos: [],
    funnel: [],
    exceptions: [],
    charts: {},
    quickLinks: [],
    recent: {
      publishTasks: [
        {
          type: 'product_publish',
          title: 'douyin_shop 刊登',
          subtitle: '已完成',
          status: '已完成',
          occurredAt: ISO_AT,
          link: '/product/publish-tasks?keyword=e2e-publish-r69',
        },
      ],
      failedTasks: [
        {
          type: 'failed_inventory_sync',
          title: 'tiktok · 库存同步失败',
          subtitle: 'e2e mock error',
          status: '失败',
          occurredAt: ISO_AT,
          link: '/inventory/sync-tasks?status=failed',
        },
      ],
    },
  };
}

async function mockDashboard(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/dashboard/product-operations**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok(dashboardPayload())),
    });
  });
  await page.route('**/api/v1/orders/stats/sales**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ windows: [] })),
    });
  });
}

test.describe('@round69-raw-enum R69 raw 平台枚举收口', () => {
  test('it should map shop platform enum in orders list 店铺 column', async ({ admin, page }) => {
    await mockOrdersList(page);
    admin.writeGuard.allow({
      operation: 'cost-estimates-batch',
      method: 'POST',
      path: /\/api\/v1\/procurement\/cost-estimates\/batch$/,
      response: ok(null),
    });
    await page.setViewportSize({ width: 1440, height: 900 });
    await admin.goto('/orders/list');
    await expect(page.getByText('e2e 抖店旗舰店 / 抖店')).toBeVisible();
    await expect(page.locator('.ant-table-tbody', { hasText: 'e2e 抖店旗舰店 / douyin_shop' })).toHaveCount(0);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should map raw platform enums in dashboard 最近动态', async ({ admin, page }) => {
    await mockDashboard(page);
    await admin.goto('/dashboard');
    await expect(page.getByText('最近动态')).toBeVisible();
    await expect(page.getByText('抖店 刊登')).toBeVisible();
    await expect(page.getByText('TikTok Shop · 库存同步失败')).toBeVisible();
    await expect(page.getByText('douyin_shop 刊登')).toHaveCount(0);
    await expect(page.getByText('tiktok · 库存同步失败')).toHaveCount(0);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
