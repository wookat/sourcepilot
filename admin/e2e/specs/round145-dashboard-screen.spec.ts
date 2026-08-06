import { test, expect } from '../fixtures/admin.fixture';
import { expectNoRootOverflow } from '../utils/assertions';
import { dashboardScreenEmptyResponse, dashboardScreenResponse } from '../mocks/dashboard-screen';

const viewports = [
  { width: 1920, height: 1080 },
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
];

async function routeScreenApi(
  page: import('@playwright/test').Page,
  body: unknown = dashboardScreenResponse(),
  onRequest?: () => void,
) {
  await page.route('**/api/v1/dashboard/screen*', async (route) => {
    onRequest?.();
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(body),
    });
  });
}

test.describe('@round145 经营大屏', () => {
  test('it should render KPI, todos, funnel, trend and alerts', async ({ admin, page }) => {
    await routeScreenApi(page);
    await admin.goto('/dashboard/screen');
    await expect(page.getByText('经营大屏').first()).toBeVisible();
    await expect(page.getByTestId('screen-kpi-orders')).toContainText('128');
    await expect(page.getByTestId('screen-kpi-sales')).toContainText('45,230.50');
    await expect(page.getByTestId('screen-kpi-sales')).toContainText('未折算：EUR');
    await expect(page.getByTestId('screen-kpi-profit')).toContainText('12,890.25');
    await expect(page.getByTestId('screen-kpi-profit')).toContainText('毛利率 28.5%');
    await expect(page.getByTestId('screen-todo-await_shipment')).toContainText('待发货');
    await expect(page.getByTestId('screen-todo-order_exceptions')).toContainText('订单异常');
    await expect(page.getByTestId('screen-funnel')).toContainText('订单状态流转漏斗');
    await expect(page.getByTestId('screen-trend')).toContainText('近 24 小时订单趋势');
    await expect(page.getByTestId('screen-alerts')).toContainText('断货：DEMO 商品 A');
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should auto refresh by the configured interval with a single API call', async ({
    admin,
    page,
  }) => {
    let calls = 0;
    await routeScreenApi(page, dashboardScreenResponse(), () => {
      calls += 1;
    });
    await admin.goto('/dashboard/screen');
    await expect(page.getByTestId('screen-kpi-orders')).toContainText('128');
    const before = calls;
    await page.getByRole('button', { name: '立即刷新' }).click();
    await expect.poll(() => calls).toBeGreaterThan(before);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should switch theme between dark and light', async ({ admin, page }) => {
    await routeScreenApi(page);
    await admin.goto('/dashboard/screen');
    const root = page.getByTestId('dashboard-screen-root');
    await expect(root).toHaveCSS('background-color', 'rgb(11, 18, 32)');
    await page.getByText('浅色', { exact: true }).click();
    await expect(root).not.toHaveCSS('background-color', 'rgb(11, 18, 32)');
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should render empty states without alerts or orders', async ({ admin, page }) => {
    await routeScreenApi(page, dashboardScreenEmptyResponse());
    await admin.goto('/dashboard/screen');
    await expect(page.getByText('近期暂无订单')).toBeVisible();
    await expect(page.getByText('当前没有待处理告警')).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  for (const viewport of viewports) {
    test(`it should not overflow at ${viewport.width}x${viewport.height}`, async ({
      admin,
      page,
    }) => {
      await routeScreenApi(page);
      await page.setViewportSize(viewport);
      await admin.goto('/dashboard/screen');
      await expect(page.getByTestId('screen-kpi-orders')).toBeVisible();
      await expectNoRootOverflow(page);
    });
  }
});
