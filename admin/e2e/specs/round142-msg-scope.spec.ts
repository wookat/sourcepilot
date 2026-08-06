import type { Page } from '@playwright/test';
import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { expectNoRootOverflow } from '../utils/assertions';

const templates = [
  {
    id: 'tpl-logistics-1',
    groupKey: 'logistics',
    name: 'e2e-物流-查询进度',
    content: '您好{买家昵称}，您的订单 {订单号} 已发货。',
    sortOrder: 1,
    enabled: true,
    createdAt: '2026-08-01T00:00:00Z',
    updatedAt: '2026-08-01T00:00:00Z',
  },
];

const rules = [
  {
    id: 'rule-1',
    name: 'e2e-发货后自动通知买家',
    node: 'shipped',
    templateId: 'tpl-logistics-1',
    templateName: 'e2e-物流-查询进度',
    enabled: true,
    effectiveFrom: '2026-08-01T00:00:00Z',
    backfill: false,
    platforms: [],
    shopIds: [],
  },
  {
    id: 'rule-2',
    name: 'e2e-回溯存量规则',
    node: 'paid',
    templateId: 'tpl-logistics-1',
    templateName: 'e2e-物流-查询进度',
    enabled: true,
    effectiveFrom: null,
    backfill: true,
    platforms: [],
    shopIds: [],
  },
];

async function mockShops(page: Page) {
  await page.route('**/api/v1/shops**', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        ok({
          list: [
            {
              id: 'shop-1',
              platform: 'douyin_shop',
              shopName: 'e2e-抖店',
              status: 'active',
              authStatus: 'authorized',
              updatedAt: '2026-08-01T00:00:00Z',
            },
          ],
          pagination: { page: 1, pageSize: 100, total: 1, totalPages: 1 },
        }),
      ),
    });
  });
}

async function mockDrafts(page: Page) {
  await page.route('**/api/v1/customer/buyer-messages/drafts**', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ list: [], total: 0, page: 1, pageSize: 10, canWrite: true })),
    });
  });
}

async function mockEstimate(page: Page, estimated: number) {
  await page.route('**/api/v1/customer/buyer-message-rules/backfill-estimate**', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ estimated })),
    });
  });
}

async function mockRules(page: Page) {
  await page.route('**/api/v1/customer/buyer-message-rules**', async (route) => {
    // 预估端点由 mockEstimate 处理
    if (route.request().url().includes('backfill-estimate')) return route.fallback();
    if (route.request().method() !== 'GET') return route.fallback();
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ list: rules, canWrite: true })),
    });
  });
}

async function mockTemplates(page: Page) {
  await page.route('**/api/v1/customer/reply-templates**', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ list: templates, canWrite: true })),
    });
  });
}

test.describe('@round142 买家自动消息规则生效范围', () => {
  test('默认不回溯：开关默认关，创建规则不触发预估，生效范围列展示', async ({ admin, page }) => {
    await mockShops(page);
    await mockDrafts(page);
    await mockEstimate(page, 33);
    await mockRules(page);
    await mockTemplates(page);
    admin.writeGuard.allow({
      operation: 'create-rule',
      method: 'POST',
      path: /\/api\/v1\/customer\/buyer-message-rules$/,
      response: ok(rules[0]),
    });

    await admin.goto('/customer/buyer-messages');
    await page.getByRole('tab', { name: '节点规则' }).click({ timeout: 30000 });

    // 生效范围列：默认规则「仅新订单」，回溯规则「回溯存量」
    const row1 = page.locator('.ant-table-tbody tr', { hasText: 'e2e-发货后自动通知买家' });
    await expect(row1.getByText('仅新订单')).toBeVisible();
    const row2 = page.locator('.ant-table-tbody tr', { hasText: 'e2e-回溯存量规则' });
    await expect(row2.getByText('回溯存量', { exact: true })).toBeVisible();

    // 新增规则：回溯开关默认关，直接保存不弹确认、不调预估
    await page.getByRole('button', { name: '新增节点规则' }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText('回溯存量订单', { exact: true })).toBeVisible();
    await expect(dialog.getByTestId('buyer-msg-backfill-switch')).not.toBeChecked();
    await expect(dialog.getByText(/默认关闭.*不回溯存量订单/)).toBeVisible();

    await dialog.getByLabel('规则名称').fill('e2e-签收后关怀');
    await dialog.getByLabel('订单节点').click();
    await page.getByTitle('已签收').locator('div').click();
    await dialog.getByLabel('话术模板').click();
    await page.getByTitle('e2e-物流-查询进度').locator('div').click();
    await dialog.getByRole('button', { name: '确 定' }).click();
    await expect(page.getByText(/确认回溯存量订单/)).toHaveCount(0);
    await admin.writeGuard.expectRequestCount('create-rule', 1);
  });

  test('开启回溯存量：先展示预估数量并需确认，确认后才发保存请求', async ({ admin, page }) => {
    await mockShops(page);
    await mockDrafts(page);
    await mockEstimate(page, 33);
    await mockRules(page);
    await mockTemplates(page);
    admin.writeGuard.allow({
      operation: 'create-rule-backfill',
      method: 'POST',
      path: /\/api\/v1\/customer\/buyer-message-rules$/,
      response: ok(rules[1]),
    });

    await admin.goto('/customer/buyer-messages');
    await page.getByRole('tab', { name: '节点规则' }).click({ timeout: 30000 });
    await page.getByRole('button', { name: '新增节点规则' }).click();
    const dialog = page.getByRole('dialog');
    await dialog.getByLabel('规则名称').fill('e2e-付款回溯');
    await dialog.getByLabel('订单节点').click();
    await page.getByTitle('已付款').locator('div').click();
    await dialog.getByLabel('话术模板').click();
    await page.getByTitle('e2e-物流-查询进度').locator('div').click();
    await dialog.getByTestId('buyer-msg-backfill-switch').click();
    await dialog.getByRole('button', { name: '确 定' }).click();

    // 确认弹层展示预估数量；取消不发写请求
    const confirm = page.locator('.ant-modal-confirm');
    await expect(confirm.locator('.ant-modal-confirm-title')).toHaveText('确认回溯存量订单？');
    await expect(confirm.getByText(/约 33 笔存量订单/)).toBeVisible();
    await confirm.getByRole('button', { name: '取 消' }).click();
    await admin.writeGuard.expectRequestCount('create-rule-backfill', 0);

    // 再次提交并确认 → 发出保存请求
    await dialog.getByRole('button', { name: '确 定' }).click();
    await page
      .locator('.ant-modal-confirm')
      .getByRole('button', { name: /确认回溯（约 33 条）/ })
      .click();
    await admin.writeGuard.expectRequestCount('create-rule-backfill', 1);
  });
});

/** 断点口径（与 round124 一致）：<768 移动模式；≥768 侧栏模式。 */
test.describe('@round142 768px 客服菜单侧栏直达', () => {
  test.use({ viewport: { width: 768, height: 900 } });

  test('768px 侧栏含客服入口且可直达客服中心，无底部导航、无溢出', async ({ admin, page }) => {
    await page.route('**/api/v1/customer/dashboard**', async (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            pendingReplyCount: 0,
            todayNewMessages: 0,
            aiSuggestionPendingCount: 0,
            sendFailureCount: 0,
            unauthorizedShopCount: 0,
            syncTaskFailureCount: 0,
            openConversationCount: 0,
          }),
        ),
      });
    });
    await mockShops(page);
    await admin.goto('/dashboard');

    await expect(page.getByTestId('tm-mobile-tabbar')).toHaveCount(0);
    const sider = page.locator('.ant-pro-sider').first();
    await expect(sider).toBeVisible();

    // 侧栏客服菜单直达客服中心
    const customerMenu = sider.locator('.ant-menu-submenu', { hasText: '客服' }).first();
    await expect(customerMenu).toBeVisible();
    await customerMenu.hover();
    await page.getByRole('menuitem', { name: '客服中心' }).click();
    await expect(page).toHaveURL(/\/customer\/hub/);
    await expectNoRootOverflow(page);
  });
});

test.describe('@round142 移动端「我的」客服中心入口', () => {
  test.use({ viewport: { width: 375, height: 812 } });

  test('375px 我的页提供客服中心直达入口', async ({ admin, page }) => {
    await page.route('**/api/v1/customer/dashboard**', async (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            pendingReplyCount: 0,
            todayNewMessages: 0,
            aiSuggestionPendingCount: 0,
            sendFailureCount: 0,
            unauthorizedShopCount: 0,
            syncTaskFailureCount: 0,
            openConversationCount: 0,
          }),
        ),
      });
    });
    await mockShops(page);
    await admin.goto('/m/me');
    await expect(page.getByTestId('tm-mobile-me')).toBeVisible();
    await page.getByRole('button', { name: /客服中心/ }).click();
    await expect(page).toHaveURL(/\/customer\/hub/);
    await expectNoRootOverflow(page);
  });
});
