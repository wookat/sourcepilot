import type { Page } from '@playwright/test';
import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';

const SETTINGS_URL = '**/api/v1/settings/report-currency';

const dto = {
  provider: 'manual',
  baseCurrency: 'CNY',
  rates: [
    { currency: 'USD', rate: '7.13' },
    { currency: 'EUR', rate: '7.80' },
  ],
};

async function routeGet(page: Page) {
  await page.route(SETTINGS_URL, async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok(dto)),
    });
  });
}

test.describe('@round158 报表币种设置：未保存变更提示', () => {
  test('it should show unsaved hint after deleting a rate row and clear it after saving', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'save-report-currency',
      method: 'PUT',
      path: /\/api\/v1\/settings\/report-currency$/,
      response: ok({ provider: 'manual', baseCurrency: 'CNY', rates: [{ currency: 'USD', rate: '7.13' }] }),
    });
    await routeGet(page);
    await admin.goto('/settings/report-currency');
    await expect(page.getByPlaceholder('币种（如 USD）')).toHaveCount(2);
    await expect(page.getByTestId('report-currency-unsaved')).toHaveCount(0);

    await page.getByRole('button', { name: '删除该汇率' }).last().click();
    await expect(page.getByPlaceholder('币种（如 USD）')).toHaveCount(1);
    await expect(page.getByTestId('report-currency-unsaved')).toBeVisible();
    await admin.writeGuard.expectRequestCount('save-report-currency', 0);

    await page.getByRole('button', { name: '保存配置' }).click();
    await expect(page.getByText('已保存')).toBeVisible();
    await expect(page.getByTestId('report-currency-unsaved')).toHaveCount(0);
    await admin.writeGuard.expectRequestCount('save-report-currency', 1);
    const call = admin.writeGuard.calls('save-report-currency')[0];
    expect(call.postDataJSON).toEqual({ baseCurrency: 'CNY', rates: [{ currency: 'USD', rate: '7.13' }] });
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should show unsaved hint when editing a rate and clear it after reload', async ({ admin, page }) => {
    await routeGet(page);
    await admin.goto('/settings/report-currency');
    await expect(page.getByPlaceholder('汇率（如 7.13）').first()).toHaveValue('7.13');
    await expect(page.getByTestId('report-currency-unsaved')).toHaveCount(0);

    await page.getByPlaceholder('汇率（如 7.13）').first().fill('7.20');
    await expect(page.getByTestId('report-currency-unsaved')).toBeVisible();

    await page.getByRole('button', { name: '重新加载' }).click();
    await expect(page.getByPlaceholder('汇率（如 7.13）').first()).toHaveValue('7.13');
    await expect(page.getByTestId('report-currency-unsaved')).toHaveCount(0);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
