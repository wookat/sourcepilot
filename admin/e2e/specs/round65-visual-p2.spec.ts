import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { expectNoRootOverflow } from '../utils/assertions';

function dashboardPayload() {
  return {
    summary: {
      orderExceptions: 0,
      collectFailedCount: 0,
      aiTitleCompletedCount: 0,
      aiDescriptionCompletedCount: 0,
      collectedProductsCount: 100,
      aiTextCompletedCount: 80,
      readinessPassedCount: 40,
    },
    todos: [
      { id: 'todo-1', key: 'publishable', title: '可刊登商品', count: 9, link: '/product/drafts' },
      { id: 'todo-2', key: 'inventory_alerts', title: '库存预警', count: 3, link: '/inventory/alerts' },
      { id: 'todo-3', key: 'collect_failed', title: '采集失败', count: 1, link: '/collect/hub' },
    ],
    funnel: [],
    exceptions: [],
    charts: {},
    quickLinks: [],
    recent: {},
  };
}

function salesStatsPayload() {
  return {
    generatedAt: new Date().toISOString(),
    windows: [
      { key: 'today', orderCount: 12, paidCount: 10, shippedCount: 8, paidAmounts: [] },
      { key: 'last7d', orderCount: 70, paidCount: 60, shippedCount: 50, paidAmounts: [] },
      { key: 'last30d', orderCount: 300, paidCount: 260, shippedCount: 200, paidAmounts: [] },
    ],
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
      body: JSON.stringify(ok(salesStatsPayload())),
    });
  });
}

const exceptionSummary = {
  totalOpen: 5,
  skuUnmatched: 2,
  skuAmbiguous: 0,
  insufficientStock: 1,
  inventoryDeductFailed: 0,
  inventoryRestoreFailed: 0,
  inventorySyncFailed: 2,
  procurementBlocked: 0,
  negativeMargin: 0,
};

async function mockExceptions(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/orders/exceptions?**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ list: [], total: 0, summary: exceptionSummary })),
    });
  });
}

test.describe('@round65-visual-p2 R65 视觉 P2 收口', () => {
  test('首页经营概览窗口标签全部中文，不出现英文枚举直出', async ({ admin, page }) => {
    await mockDashboard(page);
    await admin.goto('/dashboard');
    await expect(page.getByText('经营概览')).toBeVisible();
    await expect(page.getByText('今日', { exact: true })).toBeVisible();
    await expect(page.getByText('近 7 日', { exact: true })).toBeVisible();
    await expect(page.getByText('近 30 日', { exact: true })).toBeVisible();
    await expect(page.getByText('last7d')).toHaveCount(0);
    await expect(page.getByText('last30d')).toHaveCount(0);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('今日待办仅首个待办使用主按钮，其余降级为次按钮', async ({ admin, page }) => {
    await mockDashboard(page);
    await admin.goto('/dashboard');
    await expect(page.getByText('今日待办')).toBeVisible();
    const todoCard = page.locator('.ant-pro-card', { hasText: '今日待办' }).first();
    const primaryButtons = todoCard.locator('.ant-btn-primary');
    await expect(primaryButtons).toHaveCount(1);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('订单异常统计卡使用语义 MetricCard 且 375px 无横向溢出', async ({ admin, page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await mockExceptions(page);
    await admin.goto('/orders/exceptions');
    await expect(page.getByText('未处理总数')).toBeVisible();
    await expect(page.locator('.tm-metric-card')).toHaveCount(8);
    // 有异常的卡展示语义色（danger/warning），零值卡为默认
    await expect(page.locator('.tm-metric-card--danger')).toHaveCount(1);
    await expect(page.locator('.tm-metric-card--warning')).toHaveCount(3);
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('报表图表在 375px 使用紧凑高度', async ({ admin, page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await page.route('**/api/v1/orders/stats/daily?*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            generatedAt: new Date().toISOString(),
            days: 7,
            items: [
              {
                date: '2026-08-01',
                orderCount: 10,
                paidCount: 8,
                shippedCount: 5,
                paidAmounts: [{ currency: 'USD', amount: 100, orders: 8 }],
              },
            ],
          }),
        ),
      });
    });
    await admin.goto('/orders/reports');
    await expect(page.locator('canvas')).toHaveCount(2);
    const heights = await page
      .locator('canvas')
      .evaluateAll((els) => els.map((el) => el.getBoundingClientRect().height));
    for (const h of heights) expect(h).toBeLessThanOrEqual(240);
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('商品草稿无封面时展示图形占位而非灰字直出', async ({ admin, page }) => {
    await page.route('**/api/v1/products?**', async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            list: [
              {
                id: 'e2e-draft-r65',
                title: 'e2e 无图草稿',
                status: 'draft',
                sourcePlatform: '1688',
                createdAt: '2026-08-01T02:03:04Z',
                updatedAt: '2026-08-01T02:03:04Z',
              },
            ],
            pagination: { total: 1, page: 1, pageSize: 20 },
          }),
        ),
      });
    });
    await admin.goto('/product/drafts');
    await expect(page.locator('.ant-table-tbody')).toContainText('e2e 无图草稿');
    const placeholder = page.locator('.product-drafts-table__image-placeholder').first();
    await expect(placeholder).toBeVisible();
    await expect(placeholder).toHaveAttribute('aria-label', '无图');
    await expect(placeholder.locator('.anticon')).toBeVisible();
    await expect(placeholder).not.toContainText('无图');
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('登录页不再渲染空白幽灵装饰卡', async ({ page }) => {
    await page.addInitScript(() => window.localStorage.removeItem('trademind_admin_token'));
    await page.goto('/user/login');
    await expect(page).toHaveURL(/\/user\/login/);
    await expect(page.getByText('贸灵 TradeMind').first()).toBeVisible();
    await expect(page.locator('.decor-card')).toHaveCount(0);
  });
});
