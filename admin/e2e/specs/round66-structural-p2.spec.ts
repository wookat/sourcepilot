import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eProduct, E2E_PRODUCT_ID } from '../mocks/product.fixture';
import { expectNoRootOverflow } from '../utils/assertions';

type PW = import('@playwright/test').Page;

async function mockProductDetail(page: PW, overrides: Record<string, unknown> = {}) {
  await page.route(`**/api/v1/products/${E2E_PRODUCT_ID}`, async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ ...e2eProduct, ...overrides })),
    });
  });
}

const e2eSettingsPlatform = {
  platform: 'tiktok',
  name: 'TikTok Shop',
  status: 'available',
  authType: 'oauth',
  capabilities: [],
  authSchema: [],
  settingsGroupKey: 'platform_tiktok',
  appConfigSchema: {
    description: 'E2E 平台应用配置',
    fields: [
      { name: 'app_key', label: 'App Key', type: 'text', required: true, sensitive: false },
      { name: 'app_secret', label: 'App Secret', type: 'password', required: false, sensitive: true },
      { name: 'api_base_url', label: 'API Base URL', type: 'text', required: false, sensitive: false },
      { name: 'redirect_uri', label: 'Redirect URI', type: 'text', required: false, sensitive: false },
      { name: 'timeout_sec', label: 'Timeout', type: 'number', required: false, sensitive: false },
      { name: 'real_api_enabled', label: 'Real API', type: 'switch', required: false, sensitive: false },
      { name: 'order_sync_enabled', label: 'Order Sync', type: 'switch', required: false, sensitive: false },
    ],
  },
};

async function mockPlatformSettings(page: PW) {
  await page.route('**/api/v1/platform/providers**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ list: [e2eSettingsPlatform] })),
    });
  });
  await page.route('**/api/v1/platform/settings/tiktok', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        ok({
          platform: 'tiktok',
          groupKey: 'platform_tiktok',
          schema: e2eSettingsPlatform.appConfigSchema,
          values: { app_key: 'e2e-app-key', timeout_sec: '30', real_api_enabled: 'false', order_sync_enabled: 'false' },
        }),
      ),
    });
  });
}

test.describe('@round66 P2-4 草稿详情采集质量单一提示区', () => {
  test('无采集提示时整合为单一 info 提示区', async ({ admin, page }) => {
    await mockProductDetail(page);
    await admin.goto(`/product/drafts/${E2E_PRODUCT_ID}?tab=basic`);

    const board = page.locator('.product-draft-basic__quality .collect-quality-notice-board');
    await expect(board).toHaveCount(1);
    await expect(page.locator('.product-draft-basic__quality .ant-alert')).toHaveCount(1);
    await expect(board.getByText(/采集质量提示：说明 \d+ 项/)).toBeVisible();
    await expect(board.getByText('当前来源没有独立采集质量规则')).toBeVisible();
  });

  test('有采集质量警告时按优先级汇总且默认展开', async ({ admin, page }) => {
    await mockProductDetail(page, { collectWarnings: ['价格识别可能不完整', '详情图数量偏少'] });
    await admin.goto(`/product/drafts/${E2E_PRODUCT_ID}?tab=basic`);

    const board = page.locator('.product-draft-basic__quality .collect-quality-notice-board');
    await expect(page.locator('.product-draft-basic__quality .ant-alert')).toHaveCount(1);
    await expect(board.getByText(/建议检查 1 项/)).toBeVisible();
    await expect(board.getByText('采集质量提示（2 条）')).toBeVisible();
    await expect(board.getByText('价格识别可能不完整')).toBeVisible();
    await expect(board.getByText('详情图数量偏少')).toBeVisible();
  });

  test('375px 下提示区无横向溢出', async ({ admin, page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await mockProductDetail(page, { collectWarnings: ['价格识别可能不完整'] });
    await admin.goto(`/product/drafts/${E2E_PRODUCT_ID}?tab=basic`);

    await expect(page.locator('.product-draft-basic__quality .ant-alert')).toHaveCount(1);
    await expectNoRootOverflow(page);
  });
});

test.describe('@round66 P2-10 平台设置字段分组折叠', () => {
  test('配置字段按语义分组且默认全部展开', async ({ admin, page }) => {
    await mockPlatformSettings(page);
    await admin.goto('/settings/platforms');

    const groups = page.locator('.platform-config-groups');
    await expect(groups).toBeVisible();
    await expect(groups.getByText('应用凭证（2 项）')).toBeVisible();
    await expect(groups.getByText('接口地址与环境（3 项）')).toBeVisible();
    await expect(groups.getByText('功能开关与灰度（2 项）')).toBeVisible();
    await expect(page.getByLabel('应用 Key')).toBeVisible();
    await expect(page.getByLabel('应用 Key')).toHaveValue('e2e-app-key');
  });

  test('375px 下可折叠分组快速定位，无横向溢出', async ({ admin, page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await mockPlatformSettings(page);
    await admin.goto('/settings/platforms');

    const groups = page.locator('.platform-config-groups');
    await expect(groups.getByText('应用凭证（2 项）')).toBeVisible();
    await expect(page.getByLabel('应用 Key')).toBeVisible();

    await groups.getByText('应用凭证（2 项）').click();
    await expect(page.getByLabel('应用 Key')).toBeHidden();

    await groups.getByText('应用凭证（2 项）').click();
    await expect(page.getByLabel('应用 Key')).toBeVisible();

    await expectNoRootOverflow(page);
  });
});
