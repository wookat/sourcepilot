import type { Page } from '@playwright/test';
import { test, expect } from '../fixtures/admin.fixture';
import { expectNoRootOverflow } from '../utils/assertions';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import {
  dashboardScreenConfigResponse,
  dashboardScreenDefaultCards,
  dashboardScreenResponse,
} from '../mocks/dashboard-screen';

const viewports = [
  { width: 1920, height: 1080 },
  { width: 1280, height: 800 },
  { width: 375, height: 812 },
];

async function routeScreenApi(page: Page, body: unknown = dashboardScreenResponse()) {
  await page.route('**/api/v1/dashboard/screen?*', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
  });
  await page.route('**/api/v1/dashboard/screen', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
  });
}

async function routeConfigApi(page: Page, body: unknown = dashboardScreenConfigResponse()) {
  await page.route('**/api/v1/dashboard/screen/config', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
  });
}

async function useReadonlyProfile(page: Page) {
  await page.route('**/api/v1/auth/profile', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
    });
  });
}

test.describe('@round156 经营大屏多币种折算与自定义指标', () => {
  test('it should show FX conversion note and explicit unconverted amounts excluded from total', async ({
    admin,
    page,
  }) => {
    await routeScreenApi(page);
    await admin.goto('/dashboard/screen');
    const sales = page.getByTestId('screen-kpi-sales');
    await expect(sales).toContainText('45,230.50');
    await expect(page.getByTestId('screen-unconverted-revenue')).toContainText('未折算（不计入合计）：EUR 320.50');
    await page.getByLabel('折算口径说明').first().hover();
    await expect(page.getByText('折算口径：按租户「报表币种设置」的手工汇率折算为本位币')).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should hide disabled cards and honor configured order from the screen response', async ({
    admin,
    page,
  }) => {
    const body = dashboardScreenResponse() as { data: Record<string, unknown> };
    body.data.cards = [
      { key: 'trend', title: '24 小时订单趋势', enabled: true },
      { key: 'kpi_orders', title: '今日订单', enabled: true },
      { key: 'kpi_sales', title: '今日销售额', enabled: false },
      { key: 'kpi_profit', title: '今日毛利', enabled: false },
      { key: 'kpi_alerts', title: '当前告警', enabled: false },
      { key: 'todos', title: '待办事项', enabled: false },
      { key: 'funnel', title: '订单状态漏斗', enabled: false },
      { key: 'alerts', title: '告警滚动列表', enabled: false },
    ];
    await routeScreenApi(page, body);
    await admin.goto('/dashboard/screen');
    await expect(page.getByTestId('screen-trend')).toBeVisible();
    await expect(page.getByTestId('screen-kpi-orders')).toBeVisible();
    await expect(page.getByTestId('screen-kpi-sales')).toHaveCount(0);
    await expect(page.getByTestId('screen-funnel')).toHaveCount(0);
    await expect(page.getByTestId('screen-alerts')).toHaveCount(0);
    const trendBox = await page.getByTestId('screen-trend').boundingBox();
    const ordersBox = await page.getByTestId('screen-kpi-orders').boundingBox();
    expect(trendBox && ordersBox && trendBox.y < ordersBox.y).toBe(true);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should save card selection and order via the config modal', async ({ admin, page }) => {
    await routeScreenApi(page);
    await routeConfigApi(page);
    admin.writeGuard.allow({
      operation: 'save-screen-config',
      method: 'PUT',
      path: /\/api\/v1\/dashboard\/screen\/config$/,
      response: dashboardScreenConfigResponse(),
    });
    await admin.goto('/dashboard/screen');
    await page.getByTestId('screen-config-button').click();
    const dialog = page.getByRole('dialog', { name: '自定义大屏指标' });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByTestId('screen-config-item-kpi_orders')).toBeVisible();

    await dialog.getByLabel('显示 今日毛利').click();
    await dialog.getByLabel('下移 今日订单').click();
    await dialog.getByRole('button', { name: /保\s*存/ }).click();

    await admin.writeGuard.expectRequestCount('save-screen-config', 1);
    const call = admin.writeGuard.calls('save-screen-config')[0];
    const payload = call.postDataJSON as { cards: { key: string; enabled: boolean }[] };
    expect(payload.cards[0].key).toBe('kpi_sales');
    expect(payload.cards[1].key).toBe('kpi_orders');
    expect(payload.cards.find((c) => c.key === 'kpi_profit')?.enabled).toBe(false);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should hide the config entry for readonly users and send no writes', async ({
    admin,
    page,
  }) => {
    await useReadonlyProfile(page);
    await routeScreenApi(page);
    await admin.goto('/dashboard/screen');
    await expect(page.getByTestId('screen-kpi-orders')).toBeVisible();
    await expect(page.getByTestId('screen-config-button')).toHaveCount(0);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  for (const viewport of viewports) {
    test(`it should not overflow with default cards at ${viewport.width}x${viewport.height}`, async ({
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

// 卡片默认布局与后端 defaults 对齐，避免 mock 漂移。
test('mock default cards keep the backend default pool and order', () => {
  expect(dashboardScreenDefaultCards.map((c) => c.key)).toEqual([
    'kpi_orders',
    'kpi_sales',
    'kpi_profit',
    'kpi_alerts',
    'todos',
    'funnel',
    'trend',
    'alerts',
  ]);
});
