import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import { dailyStatsCsvBody, dailyStatsResponse } from '../mocks/reports';

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
}

function routeExportCsv(page: import('@playwright/test').Page, opts: { failFirstWith401: boolean }) {
  let calls = 0;
  const getCalls = () => calls;
  void page.route('**/api/v1/orders/stats/daily/export.csv?*', async (route) => {
    calls += 1;
    if (opts.failFirstWith401 && calls === 1) {
      await route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({ code: 40100, message: '登录已过期', data: null }),
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'text/csv; charset=utf-8',
      headers: { 'Content-Disposition': 'attachment; filename="daily-report-30d.csv"' },
      body: dailyStatsCsvBody,
    });
  });
  return getCalls;
}

test.describe('@round90-ux-p2 R90 P2 收口', () => {
  test('it should persist report day range in URL and survive reload', async ({ admin, page }) => {
    const seen: number[] = [];
    await routeReportsApi(page, seen);

    await admin.goto('/orders/reports?days=90');
    await expect(page.getByText('近 90 天合计')).toBeVisible();
    expect(seen).toEqual([90]);

    await page.getByText('近 7 天', { exact: true }).click();
    await expect(page.getByText('近 7 天合计')).toBeVisible();
    await expect(page).toHaveURL(/days=7/);

    await page.reload();
    await expect(page.getByText('近 7 天合计')).toBeVisible();
    expect(seen).toEqual([90, 7, 7]);

    // 默认 30 天时清除 query，保持 URL 干净
    await page.getByText('近 30 天', { exact: true }).click();
    await expect(page.getByText('近 30 天合计')).toBeVisible();
    await expect(page).not.toHaveURL(/days=/);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should fall back to default for invalid days query', async ({ admin, page }) => {
    const seen: number[] = [];
    await routeReportsApi(page, seen);
    await admin.goto('/orders/reports?days=999');
    await expect(page.getByText('近 30 天合计')).toBeVisible();
    expect(seen).toEqual([30]);
  });

  test('it should silently refresh session and retry CSV export on 401', async ({ admin, page }) => {
    admin.consoleGuard.allowForTest(/status of 401/);
    const seen: number[] = [];
    await routeReportsApi(page, seen);
    const exportCalls = routeExportCsv(page, { failFirstWith401: true });
    admin.writeGuard.allow({
      operation: 'auth-refresh',
      method: 'POST',
      path: /\/api\/v1\/auth\/refresh$/,
      response: ok({ token: 'e2e-refreshed-token', expiresAt: 4102444800 }),
    });

    await admin.goto('/orders/reports');
    await expect(page.getByText('近 30 天合计')).toBeVisible();

    await page.getByRole('button', { name: /导出 CSV/ }).click();
    await expect(page.getByText('已导出 CSV')).toBeVisible();
    await admin.writeGuard.expectRequestCount('auth-refresh', 1);
    expect(exportCalls()).toBe(2);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should open unified relogin modal when refresh fails and retry after relogin', async ({ admin, page }) => {
    admin.consoleGuard.allowForTest(/status of 401/);
    const seen: number[] = [];
    await routeReportsApi(page, seen);
    const exportCalls = routeExportCsv(page, { failFirstWith401: true });
    admin.writeGuard.allow({
      operation: 'auth-refresh',
      method: 'POST',
      path: /\/api\/v1\/auth\/refresh$/,
      status: 401,
      response: { code: 40100, message: '登录已过期', data: null },
    });
    admin.writeGuard.allow({
      operation: 'auth-login',
      method: 'POST',
      path: /\/api\/v1\/auth\/login$/,
      response: ok({ token: 'e2e-relogin-token', expiresAt: 4102444800, user: e2eUser }),
    });

    await admin.goto('/orders/reports');
    await expect(page.getByText('近 30 天合计')).toBeVisible();

    await page.getByRole('button', { name: /导出 CSV/ }).click();

    // 统一「登录已过期」页内重登引导（不整页跳转，未保存内容不丢失）
    const modal = page.locator('.ant-modal').filter({ hasText: '登录已过期' });
    await expect(modal).toBeVisible();
    await expect(modal.getByText('页面上未保存的内容不会丢失')).toBeVisible();

    await modal.getByLabel('账号').fill('e2e-user@example.test');
    await modal.getByLabel('密码').fill('e2e-password');
    await modal.getByRole('button', { name: '重新登录' }).click();

    await expect(page.getByText('重新登录成功，可继续当前操作')).toBeVisible();
    await expect(page.getByText('已导出 CSV')).toBeVisible();
    await admin.writeGuard.expectRequestCount('auth-login', 1);
    expect(exportCalls()).toBe(2);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
