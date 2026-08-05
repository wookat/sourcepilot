import type { Page } from '@playwright/test';
import { test, expect } from '../fixtures/admin.fixture';
import { expectNoRootOverflow } from '../utils/assertions';
import {
  productOperationDashboardBody,
  profitReportBody,
  salesStatsBody,
} from '../mocks/mobile-home';

const MOBILE_VIEWPORT = { width: 375, height: 812 };

async function mockMobileHomeApis(page: Page) {
  await page.route('**/api/v1/orders/stats/sales*', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(salesStatsBody()) });
  });
  await page.route('**/api/v1/reports/profit*', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    const days = new URL(route.request().url()).searchParams.get('days') === '1' ? 1 : 7;
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(profitReportBody(days)) });
  });
  await page.route('**/api/v1/dashboard/product-operations*', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(productOperationDashboardBody()),
    });
  });
}

test.describe('@round113 移动 H5 轻端（375px）', () => {
  test.use({ viewport: MOBILE_VIEWPORT });

  test('移动首页展示今日/近 7 日指标、关键待办与告警摘要，无横向溢出', async ({ admin, page }) => {
    await mockMobileHomeApis(page);
    await admin.goto('/m/home');

    await expect(page.getByText('移动工作台').first()).toBeVisible();

    // 指标卡：今日 + 近 7 日，订单 / 销售额 / 毛利
    const todayCard = page.getByTestId('tm-mobile-metric-今日');
    await expect(todayCard).toBeVisible();
    await expect(todayCard.getByText('12')).toBeVisible();
    await expect(todayCard.getByText(/3,?200/)).toBeVisible();
    await expect(todayCard.getByText('860')).toBeVisible();

    // 金额完整可读（375px 下不得被截断成「CNY 3…」）
    const amountBox = await todayCard.getByText('CNY 3,200.00').boundingBox();
    const amountScroll = await todayCard
      .getByText('CNY 3,200.00')
      .evaluate((el) => ({ scrollWidth: el.scrollWidth, clientWidth: el.clientWidth }));
    expect(amountBox, '销售额金额可见').not.toBeNull();
    expect(amountScroll.scrollWidth, '销售额金额未被截断').toBeLessThanOrEqual(amountScroll.clientWidth + 1);
    const weekCard = page.getByTestId('tm-mobile-metric-近 7 日');
    await expect(weekCard).toBeVisible();
    await expect(weekCard.getByText('86')).toBeVisible();
    await expect(weekCard.getByText(/6,?400/)).toBeVisible();

    // 关键待办：五项按顺序展示，其他待办不上首页
    await expect(page.getByTestId('tm-mobile-todo-order_await_payment')).toContainText('订单待收款确认');
    await expect(page.getByTestId('tm-mobile-todo-order_await_procurement')).toContainText('订单待采购');
    await expect(page.getByTestId('tm-mobile-todo-procurement_await_receipt')).toContainText('采购单待签收');
    await expect(page.getByTestId('tm-mobile-todo-order_await_shipment')).toContainText('订单待发货');
    await expect(page.getByTestId('tm-mobile-todo-order_exceptions')).toContainText('订单异常');
    await expect(page.getByText('待补 AI 标题')).toHaveCount(0);

    // 告警摘要 + 批量发货入口
    await expect(page.getByTestId('tm-mobile-alerts-entry')).toContainText('3 条待处理');
    await expect(page.getByTestId('tm-mobile-batch-ship-entry')).toBeVisible();

    await expectNoRootOverflow(page);
  });

  test('点击待办跳转对应筛选列表（订单待收款确认 → 订单列表 unpaid）', async ({ admin, page }) => {
    await mockMobileHomeApis(page);
    await admin.goto('/m/home');
    await page.getByTestId('tm-mobile-todo-order_await_payment').click();
    await expect(page).toHaveURL(/\/orders\/list\?payStatus=unpaid/);
  });

  test('底部导航五 tab 可见、触点 ≥44px，可切换到订单/我的', async ({ admin, page }) => {
    await mockMobileHomeApis(page);
    await admin.goto('/m/home');

    const tabbar = page.getByTestId('tm-mobile-tabbar');
    await expect(tabbar).toBeVisible();
    const items = tabbar.locator('.tm-mobile-tabbar__item');
    await expect(items).toHaveCount(5);
    await expect(items.nth(0)).toContainText('首页');
    await expect(items.nth(4)).toContainText('我的');

    for (let i = 0; i < 5; i += 1) {
      const box = await items.nth(i).boundingBox();
      expect(box, `tab ${i} bounding box`).not.toBeNull();
      expect(box!.height, `tab ${i} 触点高度`).toBeGreaterThanOrEqual(44);
    }

    await expect(items.nth(0)).toHaveAttribute('aria-current', 'page');

    await items.nth(1).click();
    await expect(page).toHaveURL(/\/orders\/list/);
    await expect(tabbar.locator('.tm-mobile-tabbar__item').nth(1)).toHaveAttribute('aria-current', 'page');

    await tabbar.locator('.tm-mobile-tabbar__item', { hasText: '我的' }).click();
    await expect(page).toHaveURL(/\/m\/me/);
    await expect(page.getByTestId('tm-mobile-me')).toContainText('退出登录');
  });

  test('订单列表在 375px 下可用且无横向溢出（可浏览降级口径）', async ({ admin, page }) => {
    await mockMobileHomeApis(page);
    await admin.goto('/orders/list');
    await expect(page.getByTestId('tm-mobile-tabbar')).toBeVisible();
    await expectNoRootOverflow(page);
  });
});

test.describe('@round113 桌面回归（1440px）', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  test('桌面视口不渲染移动底部导航，桌面工作台不回退', async ({ admin, page }) => {
    await mockMobileHomeApis(page);
    await admin.goto('/dashboard/product-operations');
    await expect(page.getByTestId('tm-mobile-tabbar')).toHaveCount(0);
    await expect(page.getByText('运营总览').first()).toBeVisible();
    await expectNoRootOverflow(page);
  });
});
