import { test, expect } from '../fixtures/admin.fixture';
import { ok, fail } from '../mocks/envelope';
import { expectNoRootOverflow } from '../utils/assertions';

const drafts = [
  {
    id: 'draft-1',
    orderId: 'order-1',
    orderNo: 'SO-2026-1001',
    customerName: 'e2e-买家小美',
    node: 'shipped',
    ruleId: 'rule-1',
    templateId: 'tpl-logistics-1',
    templateName: 'e2e-物流-查询进度',
    platform: 'douyin_shop',
    shopId: 'shop-1',
    shopName: 'e2e-抖店',
    content: '您好e2e-买家小美，您的订单 SO-2026-1001 已发货，物流单号 SF888888。',
    missingVars: [],
    status: 'pending',
    conversationId: 'conv-1',
    createdAt: '2026-08-01T00:00:00Z',
    updatedAt: '2026-08-01T00:00:00Z',
  },
  {
    id: 'draft-2',
    orderId: 'order-2',
    orderNo: 'SO-2026-1002',
    customerName: 'e2e-买家小强',
    node: 'shipped',
    ruleId: 'rule-1',
    templateId: 'tpl-logistics-1',
    templateName: 'e2e-物流-查询进度',
    platform: 'douyin_shop',
    shopId: 'shop-1',
    shopName: 'e2e-抖店',
    content: '您好e2e-买家小强，您的订单 SO-2026-1002 已发货，物流单号 {物流单号}。',
    missingVars: ['物流单号'],
    status: 'pending',
    createdAt: '2026-08-01T00:00:00Z',
    updatedAt: '2026-08-01T00:00:00Z',
  },
  {
    id: 'draft-3',
    orderId: 'order-3',
    orderNo: 'SO-2026-1003',
    customerName: 'e2e-买家小丽',
    node: 'refunded',
    ruleId: 'rule-2',
    templateId: 'tpl-refund-1',
    templateName: 'e2e-退款-流程说明',
    platform: 'douyin_shop',
    shopId: 'shop-1',
    shopName: 'e2e-抖店',
    content: 'e2e-买家小丽您好，订单 SO-2026-1003 的退款申请已收到。',
    missingVars: [],
    status: 'sent',
    sentAt: '2026-08-01T01:00:00Z',
    createdAt: '2026-08-01T00:00:00Z',
    updatedAt: '2026-08-01T01:00:00Z',
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
    platforms: [],
    shopIds: [],
  },
  {
    id: 'rule-2',
    name: 'e2e-退款进度自动告知',
    node: 'refunded',
    templateId: 'tpl-refund-1',
    templateName: 'e2e-退款-流程说明',
    enabled: false,
    platforms: ['douyin_shop'],
    shopIds: [],
  },
];

const templates = [
  {
    id: 'tpl-logistics-1',
    groupKey: 'logistics',
    name: 'e2e-物流-查询进度',
    content: '您好{买家昵称}，您的订单 {订单号} 已发货，物流单号 {物流单号}。',
    sortOrder: 1,
    enabled: true,
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

async function mockDrafts(page: import('@playwright/test').Page, canWrite = true) {
  await page.route('**/api/v1/customer/buyer-messages/drafts**', async (route) => {
    const request = route.request();
    if (request.method() !== 'GET') {
      await route.fallback();
      return;
    }
    const url = new URL(request.url());
    const node = url.searchParams.get('node');
    const status = url.searchParams.get('status');
    const keyword = url.searchParams.get('keyword');
    let list = drafts;
    if (node) list = list.filter((d) => d.node === node);
    if (status) list = list.filter((d) => d.status === status);
    if (keyword)
      list = list.filter(
        (d) => d.orderNo.includes(keyword) || d.customerName.includes(keyword) || d.content.includes(keyword),
      );
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ list, total: list.length, page: 1, pageSize: 10, canWrite })),
    });
  });
}

async function mockRules(page: import('@playwright/test').Page, canWrite = true) {
  await page.route('**/api/v1/customer/buyer-message-rules**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ list: rules, canWrite })),
    });
  });
}

async function mockTemplates(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/customer/reply-templates**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ list: templates, canWrite: true })),
    });
  });
}

test.describe('@round119 买家自动消息', () => {
  test('待发消息工作台：降级说明、节点/状态筛选、缺失变量提示、编辑/标记已发送/忽略（写请求显式拦截）', async ({
    admin,
    page,
  }) => {
    await mockShops(page);
    await mockDrafts(page);
    admin.writeGuard.allow({
      operation: 'edit-draft',
      method: 'PUT',
      path: /\/api\/v1\/customer\/buyer-messages\/drafts\/draft-2$/,
      response: ok({ ...drafts[1], content: '已补全物流单号 SF999999' }),
    });
    admin.writeGuard.allow({
      operation: 'mark-sent',
      method: 'POST',
      path: /\/api\/v1\/customer\/buyer-messages\/drafts\/draft-1\/mark-sent$/,
      response: ok({ ...drafts[0], status: 'sent' }),
    });
    admin.writeGuard.allow({
      operation: 'ignore-draft',
      method: 'POST',
      path: /\/api\/v1\/customer\/buyer-messages\/drafts\/draft-2\/ignore$/,
      response: ok({ ...drafts[1], status: 'ignored' }),
    });

    await admin.goto('/customer/buyer-messages');

    // 降级说明：绝不自动外发（首次访问触发 dev server 编译，放宽等待时间）
    await expect(page.getByText('当前不会自动发送到平台')).toBeVisible({ timeout: 30000 });
    await expect(page.getByText(/绝不自动外发/)).toBeVisible();

    await expect(page.getByRole('link', { name: 'SO-2026-1001' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'SO-2026-1002' })).toBeVisible();

    // 缺失变量提示
    await expect(page.getByText(/缺少变量：\{物流单号\}/)).toBeVisible();

    // 编辑草稿
    const row2 = page.locator('.ant-table-tbody tr', { hasText: 'SO-2026-1002' });
    await expect(row2).toBeVisible();
    await row2.getByRole('button', { name: /编\s*辑/ }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByText('编辑后请在对应平台后台发送给买家')).toBeVisible();
    await dialog.getByLabel('消息内容').fill('已补全物流单号 SF999999');
    await dialog.getByRole('button', { name: '确 定' }).click();
    await admin.writeGuard.expectRequestCount('edit-draft', 1);

    // 单个标记已发送（人工回执确认弹层）
    const row1 = page.locator('.ant-table-tbody tr', { hasText: 'SO-2026-1001' });
    await row1.getByRole('button', { name: '标记已发送' }).click();
    await expect(page.getByText(/确认已在平台后台向买家发送订单 SO-2026-1001/)).toBeVisible();
    await page.getByRole('button', { name: '已发送', exact: true }).click();
    await admin.writeGuard.expectRequestCount('mark-sent', 1);

    // 忽略
    await row2.getByRole('button', { name: /忽\s*略/ }).click();
    await page.getByRole('button', { name: '忽 略' }).click();
    await admin.writeGuard.expectRequestCount('ignore-draft', 1);

    // 节点筛选：退款节点下不再展示已发货订单
    await page.getByRole('combobox').first().click();
    await page.getByTitle('退款').locator('div').click();
    await expect(page.getByRole('link', { name: 'SO-2026-1001' })).toHaveCount(0);
  });

  test('批量标记已发送：仅待发送行可选、批量写请求显式拦截', async ({ admin, page }) => {
    await mockShops(page);
    await mockDrafts(page);
    admin.writeGuard.allow({
      operation: 'batch-mark-sent',
      method: 'POST',
      path: /\/api\/v1\/customer\/buyer-messages\/drafts\/batch-mark-sent$/,
      response: ok({ updated: 2, skipped: 0 }),
    });

    await admin.goto('/customer/buyer-messages');
    await expect(page.getByRole('link', { name: 'SO-2026-1001' })).toBeVisible();

    const batchBtn = page.getByRole('button', { name: /批量标记已发送/ });
    await expect(batchBtn).toBeDisabled();

    await page.getByLabel('选择订单 SO-2026-1001 的草稿').check();
    await page.getByLabel('选择订单 SO-2026-1002 的草稿').check();
    await expect(batchBtn).toBeEnabled();
    await batchBtn.click();
    await admin.writeGuard.expectRequestCount('batch-mark-sent', 1);
  });

  test('节点规则管理：列表、平台/店铺过滤展示、新增规则（写请求显式拦截）', async ({ admin, page }) => {
    await mockShops(page);
    await mockDrafts(page);
    await mockRules(page);
    await mockTemplates(page);
    admin.writeGuard.allow({
      operation: 'create-rule',
      method: 'POST',
      path: /\/api\/v1\/customer\/buyer-message-rules$/,
      response: ok(rules[0]),
    });
    admin.writeGuard.allow({
      operation: 'toggle-rule',
      method: 'PUT',
      path: /\/api\/v1\/customer\/buyer-message-rules\/rule-2$/,
      response: ok({ ...rules[1], enabled: true }),
    });

    await admin.goto('/customer/buyer-messages');
    await page.getByRole('tab', { name: '节点规则' }).click();
    await expect(page.getByText('e2e-发货后自动通知买家')).toBeVisible();
    await expect(page.getByText('全部平台').first()).toBeVisible();
    await expect(page.getByText('全部店铺').first()).toBeVisible();

    // 新增规则
    await page.getByRole('button', { name: '新增节点规则' }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    await dialog.getByLabel('规则名称').fill('e2e-签收后关怀');
    await dialog.getByLabel('订单节点').click();
    await page.getByTitle('已签收').locator('div').click();
    await dialog.getByLabel('话术模板').click();
    await page.getByTitle('e2e-物流-查询进度').locator('div').click();
    await dialog.getByRole('button', { name: '确 定' }).click();
    await admin.writeGuard.expectRequestCount('create-rule', 1);

    // 启停规则
    const ruleRow = page.locator('.ant-table-tbody tr', { hasText: 'e2e-退款进度自动告知' });
    await ruleRow.getByRole('switch').click();
    await admin.writeGuard.expectRequestCount('toggle-rule', 1);
  });

  test('无客服写权限（readonly 口径）时写操作禁用，不发出写请求', async ({ admin, page }) => {
    await mockShops(page);
    await mockDrafts(page, false);
    await mockRules(page, false);
    await mockTemplates(page);

    await admin.goto('/customer/buyer-messages');
    await expect(page.getByRole('link', { name: 'SO-2026-1001' })).toBeVisible();
    await expect(page.getByRole('button', { name: /批量标记已发送/ })).toBeDisabled();
    await expect(page.getByRole('button', { name: '立即生成草稿' })).toBeDisabled();
    const row1 = page.locator('.ant-table-tbody tr', { hasText: 'SO-2026-1001' });
    await expect(row1.getByRole('button', { name: /编\s*辑/ })).toBeDisabled();
    await expect(row1.getByRole('button', { name: '标记已发送' })).toBeDisabled();
    await expect(row1.getByRole('button', { name: /忽\s*略/ })).toBeDisabled();

    await page.getByRole('tab', { name: '节点规则' }).click();
    await expect(page.getByRole('button', { name: '新增节点规则' })).toBeDisabled();
  });

  test('草稿加载失败时展示中文错误与重试', async ({ admin, page }) => {
    await mockShops(page);
    await page.route('**/api/v1/customer/buyer-messages/drafts**', async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(fail('服务器开小差了', 500)),
      });
    });
    await admin.goto('/customer/buyer-messages');
    await expect(page.getByText('加载待发消息失败')).toBeVisible();
    await expect(page.getByRole('button', { name: /重\s*试/ })).toBeVisible();
  });

  const viewports = [
    { name: '1440x900', width: 1440, height: 900 },
    { name: '1280x800', width: 1280, height: 800 },
    { name: '1024x768', width: 1024, height: 768 },
    { name: '768x900', width: 768, height: 900 },
    { name: '375x812', width: 375, height: 812 },
  ];
  for (const vp of viewports) {
    test(`工作台 ${vp.name} 无根节点横向溢出`, async ({ admin, page }) => {
      await mockShops(page);
      await mockDrafts(page);
      await page.setViewportSize({ width: vp.width, height: vp.height });
      await admin.goto('/customer/buyer-messages');
      await expect(page.getByText('当前不会自动发送到平台')).toBeVisible();
      await expectNoRootOverflow(page);
    });
  }
});
