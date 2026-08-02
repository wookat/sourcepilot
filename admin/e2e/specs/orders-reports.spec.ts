import { test, expect } from '../fixtures/admin.fixture';
import { expectHeaderContentAligned, expectNoRootOverflow } from '../utils/assertions';
import { dailyStatsCsvBody, dailyStatsResponse } from '../mocks/reports';

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

async function routeReportsApi(page: import('@playwright/test').Page, seen: number[]) {
  await page.route('**/api/v1/orders/stats/daily?*', async (route) => {
    const url = new URL(route.request().url());
    const days = Number(url.searchParams.get('days') ?? '30');
    seen.push(days);
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(dailyStatsResponse(days)),
    });
  });
  await page.route('**/api/v1/orders/stats/daily/export.csv?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'text/csv; charset=utf-8',
      headers: { 'Content-Disposition': 'attachment; filename="daily-report-30d.csv"' },
      body: dailyStatsCsvBody,
    });
  });
}

test.describe('@orders-reports 经营报表', () => {
  test('it should render totals and export CSV for the current range', async ({ admin, page }) => {
    const seen: number[] = [];
    await routeReportsApi(page, seen);
    await admin.goto('/orders/reports');
    await expect(page.getByText('经营报表').first()).toBeVisible();
    await expect(page.getByText('近 30 天合计')).toBeVisible();
    await expect(page.getByText('已付款订单')).toBeVisible();
    expect(seen).toEqual([30]);

    const csvRequest = page.waitForRequest((req) => req.url().includes('/orders/stats/daily/export.csv') && req.method() === 'GET');
    await page.getByRole('button', { name: /导出 CSV/ }).click();
    const req = await csvRequest;
    expect(new URL(req.url()).searchParams.get('days')).toBe('30');
    await expect(page.getByText('已导出 CSV')).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should reload data when switching the day range', async ({ admin, page }) => {
    const seen: number[] = [];
    await routeReportsApi(page, seen);
    await admin.goto('/orders/reports');
    await expect(page.getByText('近 30 天合计')).toBeVisible();
    await page.getByText('近 7 天', { exact: true }).click();
    await expect(page.getByText('近 7 天合计')).toBeVisible();
    expect(seen).toEqual([30, 7]);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  for (const viewport of viewports) {
    test(`it should have no root overflow at ${viewport.width}x${viewport.height}`, async ({ admin, page }) => {
      const seen: number[] = [];
      await routeReportsApi(page, seen);
      await page.setViewportSize(viewport);
      await admin.goto('/orders/reports');
      await expect(page.getByText('近 30 天合计')).toBeVisible();
      await expectNoRootOverflow(page);
      await expectHeaderContentAligned(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }
});
