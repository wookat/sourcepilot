import type { Page } from '@playwright/test';
import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { dailyStatsResponse } from '../mocks/reports';

const reportCurrencyDto = {
  provider: 'manual',
  baseCurrency: 'CNY',
  rates: [{ currency: 'USD', rate: '7.13' }],
};

async function routeReportCurrencyGet(page: Page) {
  await page.route('**/api/v1/settings/report-currency', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok(reportCurrencyDto)),
    });
  });
}

test.describe('@round93 报表本位币与汇率', () => {
  test('it should show base currency totals and unconverted hint on reports', async ({ admin, page }) => {
    await page.route('**/api/v1/orders/stats/daily?*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(dailyStatsResponse(30)),
      });
    });
    await admin.goto('/orders/reports');
    await expect(page.getByText('近 30 天合计（本位币 CNY）')).toBeVisible();
    await expect(page.getByText('销售额折算合计（CNY）')).toBeVisible();
    await expect(page.getByText('以下币种未配置汇率，未折算入本位币合计：EUR')).toBeVisible();
    await expect(page.getByRole('link', { name: '报表本位币与汇率设置' })).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should load and save the report currency settings', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'save-report-currency',
      method: 'PUT',
      path: /\/api\/v1\/settings\/report-currency$/,
      response: ok(reportCurrencyDto),
    });
    await routeReportCurrencyGet(page);
    await admin.goto('/settings/report-currency');
    await expect(page.getByText('报表本位币与汇率').first()).toBeVisible();
    await expect(page.getByPlaceholder('币种（如 USD）')).toHaveValue('USD');
    await expect(page.getByPlaceholder('汇率（如 7.13）')).toHaveValue('7.13');

    await page.getByRole('button', { name: '保存配置' }).click();
    await expect(page.getByText('已保存')).toBeVisible();
    await admin.writeGuard.expectRequestCount('save-report-currency', 1);
    const call = admin.writeGuard.calls('save-report-currency')[0];
    expect(call.postDataJSON).toEqual({ baseCurrency: 'CNY', rates: [{ currency: 'USD', rate: '7.13' }] });
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should reject an invalid manual rate before submitting', async ({ admin, page }) => {
    await routeReportCurrencyGet(page);
    await admin.goto('/settings/report-currency');
    await expect(page.getByPlaceholder('汇率（如 7.13）')).toHaveValue('7.13');
    await page.getByPlaceholder('汇率（如 7.13）').fill('-1');
    await page.getByRole('button', { name: '保存配置' }).click();
    await expect(page.getByText('正的十进制数，最多 6 位小数')).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
