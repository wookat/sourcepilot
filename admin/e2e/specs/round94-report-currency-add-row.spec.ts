import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';

const SETTINGS_URL = '**/api/v1/settings/report-currency';

const dto = { provider: 'manual', baseCurrency: 'CNY', rates: [{ currency: 'USD', rate: '7.15' }] };

test.describe('R94 报表币种设置：添加汇率行', () => {
  test('加载完成前「添加币种汇率」禁用，加载后首次点击即新增行', async ({ admin }) => {
    const { page } = admin;
    let release!: () => void;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    await page.route(SETTINGS_URL, async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      await gate;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok(dto)),
      });
    });

    await admin.goto('/settings/report-currency');
    const addButton = page.getByRole('button', { name: '添加币种汇率' });
    // GET 未返回时按钮禁用，点击不会产生随后被 setFieldsValue 覆盖的行
    await expect(addButton).toBeDisabled();

    release();
    await expect(addButton).toBeEnabled();
    await expect(page.getByPlaceholder('币种（如 USD）')).toHaveCount(1);

    await addButton.click();
    await expect(page.getByPlaceholder('币种（如 USD）')).toHaveCount(2);
  });
});
