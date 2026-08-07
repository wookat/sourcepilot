import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { expectNoRootOverflow } from '../utils/assertions';

const templates = [
  {
    id: 'tpl-logistics-1',
    groupKey: 'logistics',
    name: 'e2e-物流-查询进度',
    content: '您好{买家昵称}，您的订单 {订单号} 已发货，物流单号 {物流单号}。',
    sortOrder: 1,
    enabled: true,
    defaultLanguage: 'zh-CN',
    variants: [
      { language: 'en', content: 'Hi {买家昵称}, your order {订单号} has been shipped, tracking {物流单号}.' },
      { language: 'pt', content: 'Olá {买家昵称}, seu pedido {订单号} foi enviado, rastreio {物流单号}.' },
    ],
    createdAt: '2026-08-01T00:00:00Z',
    updatedAt: '2026-08-01T00:00:00Z',
  },
  {
    id: 'tpl-refund-1',
    groupKey: 'refund',
    name: 'e2e-退款-流程说明',
    content: '{买家昵称}您好，订单 {订单号} 的退款申请已收到。',
    sortOrder: 1,
    enabled: true,
    defaultLanguage: 'zh-CN',
    variants: [],
    createdAt: '2026-08-01T00:00:00Z',
    updatedAt: '2026-08-01T00:00:00Z',
  },
];

const drafts = [
  {
    id: 'draft-en',
    orderId: 'order-1',
    orderNo: 'SO-2026-2001',
    customerName: 'e2e-Buyer-US',
    node: 'shipped',
    ruleId: 'rule-1',
    templateId: 'tpl-logistics-1',
    templateName: 'e2e-物流-查询进度',
    platform: 'shopee',
    shopId: 'shop-1',
    shopName: 'e2e-跨境店',
    content: 'Hi e2e-Buyer-US, your order SO-2026-2001 has been shipped, tracking SF100.',
    missingVars: [],
    status: 'pending',
    language: 'en',
    langSource: 'order_country',
    createdAt: '2026-08-01T00:00:00Z',
    updatedAt: '2026-08-01T00:00:00Z',
  },
  {
    id: 'draft-fallback',
    orderId: 'order-2',
    orderNo: 'SO-2026-2002',
    customerName: 'e2e-买家小云',
    node: 'shipped',
    ruleId: 'rule-1',
    templateId: 'tpl-logistics-1',
    templateName: 'e2e-物流-查询进度',
    platform: 'shopee',
    shopId: 'shop-1',
    shopName: 'e2e-跨境店',
    content: '您好e2e-买家小云，您的订单 SO-2026-2002 已发货，物流单号 SF200。',
    missingVars: [],
    status: 'pending',
    language: 'zh-CN',
    langSource: 'fallback',
    createdAt: '2026-08-01T00:00:00Z',
    updatedAt: '2026-08-01T00:00:00Z',
  },
];

async function mockShops(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/shops**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        ok({
          list: [
            {
              id: 'shop-1',
              platform: 'shopee',
              shopName: 'e2e-跨境店',
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

async function mockTemplates(page: import('@playwright/test').Page, canWrite = true) {
  await page.route('**/api/v1/customer/reply-templates**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ list: templates, canWrite })),
    });
  });
}

async function mockDrafts(page: import('@playwright/test').Page, canWrite = true) {
  await page.route('**/api/v1/customer/buyer-messages/drafts**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ list: drafts, total: drafts.length, page: 1, pageSize: 10, canWrite })),
    });
  });
}

test.describe('@round152 买家消息多语言模板', () => {
  test('话术模板：语言列展示默认语言与变体、编辑弹窗维护语言变体（写请求显式拦截并校验 payload）', async ({
    admin,
    page,
  }) => {
    await mockTemplates(page);
    admin.writeGuard.allow({
      operation: 'update-template',
      method: 'PUT',
      path: /\/api\/v1\/customer\/reply-templates\/tpl-refund-1$/,
      response: ok({ ...templates[1], variants: [{ language: 'es', content: 'Hola {买家昵称}' }] }),
    });

    await admin.goto('/customer/reply-templates');
    await expect(page.getByText('e2e-物流-查询进度')).toBeVisible({ timeout: 30000 });

    // 语言列：默认语言 + 变体语言标签
    const row1 = page.locator('.ant-table-tbody tr', { hasText: 'e2e-物流-查询进度' });
    await expect(row1.getByText('中文（简体）')).toBeVisible();
    await expect(row1.getByText('英语')).toBeVisible();
    await expect(row1.getByText('葡萄牙语')).toBeVisible();

    // 编辑无变体模板并添加西班牙语变体
    const row2 = page.locator('.ant-table-tbody tr', { hasText: 'e2e-退款-流程说明' });
    await row2.getByRole('button', { name: /编\s*辑/ }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByRole('combobox', { name: /默认语言/ })).toBeVisible();
    await expect(dialog.getByText('暂无语言变体，点击下方添加')).toBeVisible();
    await dialog.getByRole('button', { name: '添加语言变体' }).click();
    await dialog.locator('.ant-select-selector', { hasText: '语言' }).last().click();
    await page.getByTitle('西班牙语').locator('div').click();
    await dialog.getByPlaceholder(/该语言的模板正文/).fill('Hola {买家昵称}, pedido {订单号} recibido.');
    await dialog.getByRole('button', { name: '确 定' }).click();
    await admin.writeGuard.expectRequestCount('update-template', 1);
    const call = admin.writeGuard.calls('update-template')[0];
    const body = call.postDataJSON as { defaultLanguage?: string; variants?: { language: string; content: string }[] };
    expect(body.defaultLanguage).toBe('zh-CN');
    expect(body.variants).toEqual([
      { language: 'es', content: 'Hola {买家昵称}, pedido {订单号} recibido.' },
    ]);
  });

  test('待发消息：语言列展示推断来源与回退标注、切换语言重新生成（写请求显式拦截）', async ({ admin, page }) => {
    await mockShops(page);
    await mockDrafts(page);
    admin.writeGuard.allow({
      operation: 'regenerate',
      method: 'POST',
      path: /\/api\/v1\/customer\/buyer-messages\/drafts\/draft-fallback\/regenerate$/,
      response: ok({
        ...drafts[1],
        content: 'Hi e2e-买家小云, your order SO-2026-2002 has been shipped, tracking SF200.',
        language: 'en',
        langSource: 'manual',
      }),
    });

    await admin.goto('/customer/buyer-messages');
    await expect(page.getByRole('link', { name: 'SO-2026-2001' })).toBeVisible({ timeout: 30000 });

    // 正样本：按收货地推断出英语
    const rowEn = page.locator('.ant-table-tbody tr', { hasText: 'SO-2026-2001' });
    await expect(rowEn.getByText('英语')).toBeVisible();

    // 负样本：无法推断时回退默认语言并展示标注
    const rowFb = page.locator('.ant-table-tbody tr', { hasText: 'SO-2026-2002' });
    await expect(rowFb.getByText('中文（简体）')).toBeVisible();
    await expect(rowFb.getByText('无法推断，已回退默认语言')).toBeVisible();

    // 编辑弹窗内切换语言并重新生成（仅改草稿，不外发）
    await rowFb.getByRole('button', { name: /编\s*辑/ }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByText(/仅改草稿，不会向买家发送/)).toBeVisible();
    await dialog.locator('.ant-select-selector').first().click();
    await page
      .locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)')
      .locator('.ant-select-item-option', { hasText: '英语' })
      .first()
      .click();
    await dialog.getByRole('button', { name: '按所选语言重新生成' }).click();
    await admin.writeGuard.expectRequestCount('regenerate', 1);
    const call = admin.writeGuard.calls('regenerate')[0];
    expect(call.postDataJSON).toEqual({ language: 'en' });
    await expect(dialog.getByText(/人工切换/)).toBeVisible();
  });

  test('readonly 口径：重新生成按钮禁用，不发出写请求', async ({ admin, page }) => {
    await mockShops(page);
    await mockDrafts(page, false);

    await admin.goto('/customer/buyer-messages');
    await expect(page.getByRole('link', { name: 'SO-2026-2002' })).toBeVisible({ timeout: 30000 });
    const rowFb = page.locator('.ant-table-tbody tr', { hasText: 'SO-2026-2002' });
    await expect(rowFb.getByRole('button', { name: /编\s*辑/ })).toBeDisabled();
    await admin.writeGuard.expectNoUnexpectedWrites();
  });

  const viewports = [
    { name: '1440x900', width: 1440, height: 900 },
    { name: '1280x800', width: 1280, height: 800 },
    { name: '1024x768', width: 1024, height: 768 },
    { name: '768x900', width: 768, height: 900 },
    { name: '375x812', width: 375, height: 812 },
  ];
  for (const vp of viewports) {
    test(`话术模板页 ${vp.name} 无根节点横向溢出`, async ({ admin, page }) => {
      await mockTemplates(page);
      await page.setViewportSize({ width: vp.width, height: vp.height });
      await admin.goto('/customer/reply-templates');
      await expect(page.getByText('e2e-物流-查询进度')).toBeVisible({ timeout: 30000 });
      await expectNoRootOverflow(page);
    });
  }
});
