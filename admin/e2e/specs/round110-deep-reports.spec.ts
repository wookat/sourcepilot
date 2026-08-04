import { test, expect } from '../fixtures/admin.fixture';
import { expectHeaderContentAligned, expectNoRootOverflow } from '../utils/assertions';
import {
  inventoryReportResponse,
  procurementReportResponse,
  profitCsvBody,
  profitReportResponse,
} from '../mocks/reports';

const viewports = [
  { width: 1440, height: 900 },
  { width: 1024, height: 768 },
  { width: 375, height: 812 },
];

async function routeDeepReportApis(page: import('@playwright/test').Page, seenDimensions: string[]) {
  await page.route('**/api/v1/reports/profit?*', async (route) => {
    const url = new URL(route.request().url());
    const dimension = url.searchParams.get('dimension') ?? 'order';
    seenDimensions.push(dimension);
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(profitReportResponse(dimension)),
    });
  });
  await page.route('**/api/v1/reports/profit/export.csv?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'text/csv; charset=utf-8',
      headers: { 'Content-Disposition': 'attachment; filename="profit-report.csv"' },
      body: profitCsvBody,
    });
  });
  await page.route('**/api/v1/reports/procurement*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(procurementReportResponse()),
    });
  });
  await page.route('**/api/v1/reports/inventory*', async (route) => {
    const url = new URL(route.request().url());
    const slowDays = Number(url.searchParams.get('slowDays') ?? '30');
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(inventoryReportResponse(slowDays)),
    });
  });
}

test.describe('@round110 深度报表', () => {
  test('利润报表 it should render summary, unconverted warning and export CSV', async ({ admin, page }) => {
    const seen: string[] = [];
    await routeDeepReportApis(page, seen);
    await admin.goto('/orders/reports-profit');
    await expect(page.getByText('利润报表').first()).toBeVisible();
    await expect(page.getByText('已付款订单数')).toBeVisible();
    await expect(page.getByText(/未配置汇率，未折算入本位币合计：EUR/)).toBeVisible();
    await expect(page.getByText('SO-1001')).toBeVisible();
    expect(seen).toEqual(['order']);

    const csvRequest = page.waitForRequest(
      (req) => req.url().includes('/reports/profit/export.csv') && req.method() === 'GET',
    );
    await page.getByRole('button', { name: /导出 CSV/ }).click();
    const req = await csvRequest;
    expect(new URL(req.url()).searchParams.get('dimension')).toBe('order');
    await expect(page.getByText('已导出 CSV')).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('利润报表 it should reload when switching dimension to product', async ({ admin, page }) => {
    const seen: string[] = [];
    await routeDeepReportApis(page, seen);
    await admin.goto('/orders/reports-profit');
    await expect(page.getByText('SO-1001')).toBeVisible();
    await page.getByText('按商品', { exact: true }).click();
    await expect(page.getByText('DEMO 商品 A')).toBeVisible();
    expect(seen).toEqual(['order', 'product']);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('采购报表 it should render summary, lead time and supplier ranking', async ({ admin, page }) => {
    const seen: string[] = [];
    await routeDeepReportApis(page, seen);
    await admin.goto('/orders/reports-procurement');
    await expect(page.getByText('采购报表').first()).toBeVisible();
    await expect(page.getByText('采购单量').first()).toBeVisible();
    await expect(page.getByText('签收时效分布（下单→签收天数）')).toBeVisible();
    await expect(page.getByText('DEMO 供应商甲')).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('库存报表 it should render value, slow-moving and low-stock lists', async ({ admin, page }) => {
    const seen: string[] = [];
    await routeDeepReportApis(page, seen);
    await admin.goto('/orders/reports-inventory');
    await expect(page.getByText('库存报表').first()).toBeVisible();
    await expect(page.getByText('库存价值（CNY，参考进价估）')).toBeVisible();
    await expect(page.getByText('DEMO 滞销商品')).toBeVisible();
    await expect(page.getByText('DEMO 低库存商品')).toBeVisible();
    await page.getByText('60 天无出库', { exact: true }).click();
    await expect(page.getByText('滞销预警（60 天无出库，有库存）')).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  for (const viewport of viewports) {
    test(`利润报表 it should have no root overflow at ${viewport.width}x${viewport.height}`, async ({ admin, page }) => {
      const seen: string[] = [];
      await routeDeepReportApis(page, seen);
      await page.setViewportSize(viewport);
      await admin.goto('/orders/reports-profit');
      await expect(page.getByText('已付款订单数')).toBeVisible();
      await expectNoRootOverflow(page);
      await expectHeaderContentAligned(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }
});
