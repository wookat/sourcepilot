import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { expectNoRootOverflow } from '../utils/assertions';

const ORDER_ID = 'e2e-r120-order-1';

const orderDetail = {
  id: ORDER_ID,
  orderNo: 'SO-R120-P2-1',
  platform: 'manual',
  shopId: 'shop-1',
  shopName: 'e2e-抖店',
  status: 'paid',
  paymentStatus: 'paid',
  reviewStatus: 'passed',
  buyerName: 'e2e-买家',
  receiverName: 'e2e-买家',
  totalAmount: 120,
  payAmount: 120,
  currency: 'CNY',
  items: [],
  createdAt: '2026-08-01T00:00:00Z',
  updatedAt: '2026-08-01T00:00:00Z',
};

const automationTrail = [
  {
    id: 'e2e-r120-log-1',
    ruleId: 'rule-1',
    ruleName: '低额订单自动确认付款',
    orderId: ORDER_ID,
    orderNo: 'SO-R120-P2-1',
    triggerEvent: 'order_created',
    action: 'confirm_payment',
    status: 'success',
    reason: '已自动确认付款（金额 120.00 低于上限 200.00）',
    attempts: 1,
    createdAt: '2026-08-02T00:00:00Z',
    updatedAt: '2026-08-02T00:00:00Z',
  },
  {
    id: 'e2e-r120-log-2',
    ruleId: 'rule-2',
    ruleName: '付款后自动生成采购单',
    orderId: ORDER_ID,
    orderNo: 'SO-R120-P2-1',
    triggerEvent: 'order_paid',
    action: 'generate_procurement',
    status: 'skipped',
    reason: '订单审单状态为待审核/挂起，安全边界禁止自动化',
    attempts: 1,
    createdAt: '2026-08-02T01:00:00Z',
    updatedAt: '2026-08-02T01:00:00Z',
  },
];

async function mockOrderDetail(page: import('@playwright/test').Page) {
  await page.route(`**/api/v1/orders/${ORDER_ID}/automation-logs**`, async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ items: automationTrail })),
    });
  });
  await page.route(`**/api/v1/orders/${ORDER_ID}`, async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok(orderDetail)),
    });
  });
}

test.describe('@round120 P2① 订单详情自动化轨迹', () => {
  test('自动化轨迹 Tab 展示该订单命中的执行记录', async ({ admin, page }) => {
    await mockOrderDetail(page);
    await admin.goto(`/orders/${ORDER_ID}?tab=automation`);
    await expect(page.getByRole('tab', { name: '自动化轨迹' })).toBeVisible({ timeout: 30000 });
    await expect(page.getByText('低额订单自动确认付款')).toBeVisible();
    await expect(page.getByText('已自动确认付款（金额 120.00 低于上限 200.00）')).toBeVisible();
    await expect(page.getByText('成功', { exact: true })).toBeVisible();
    await expect(page.getByText('跳过', { exact: true })).toBeVisible();
    await expect(
      page.getByText('订单审单状态为待审核/挂起，安全边界禁止自动化'),
    ).toBeVisible();
    await expect(page.getByRole('button', { name: '打开自动化执行日志' })).toBeVisible();
    await expectNoRootOverflow(page);
  });
});

const rules = [
  {
    id: 'rule-alive',
    name: 'e2e-发货后自动通知买家',
    node: 'shipped',
    templateId: 'tpl-logistics-1',
    templateName: 'e2e-物流-查询进度',
    templateMissing: false,
    enabled: true,
    platforms: [],
    shopIds: [],
  },
  {
    id: 'rule-deleted-tpl',
    name: 'e2e-引用已删模板的规则',
    node: 'refunded',
    templateId: 'tpl-deleted-1',
    templateName: '',
    templateMissing: true,
    enabled: false,
    platforms: [],
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
];

async function mockBuyerMsgRules(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/customer/buyer-message-rules**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ list: rules, canWrite: true })),
    });
  });
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

test.describe('@round120 P2③ 规则引用已删除模板保护', () => {
  test('列表标红「模板已删除」、启用开关禁用、编辑弹层提示并强制重选模板', async ({
    admin,
    page,
  }) => {
    await mockBuyerMsgRules(page);
    await admin.goto('/customer/buyer-messages?tab=rules');
    await page.getByRole('tab', { name: /节点规则/ }).click();
    const row = page.locator('.ant-table-tbody tr', { hasText: 'e2e-引用已删模板的规则' });
    await expect(row).toBeVisible({ timeout: 30000 });
    await expect(row.getByText('模板已删除')).toBeVisible();
    await expect(row.getByRole('switch')).toBeDisabled();

    await row.getByRole('button', { name: /编\s*辑/ }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByText('原话术模板已被删除')).toBeVisible();
    // 模板下拉未预填已删模板，直接保存被必填校验拦截（无写请求）
    await dialog.getByRole('button', { name: '确 定' }).click();
    await expect(dialog.locator('#templateId_help').getByText('请选择话术模板')).toBeVisible();
  });
});

const draftWithMissing = {
  id: 'draft-mv-1',
  orderId: 'order-mv-1',
  orderNo: 'SO-R120-MV-1',
  customerName: 'e2e-买家小美',
  node: 'shipped',
  ruleId: 'rule-alive',
  templateId: 'tpl-logistics-1',
  templateName: 'e2e-物流-查询进度',
  platform: 'douyin_shop',
  shopId: 'shop-1',
  shopName: 'e2e-抖店',
  content: '您好e2e-买家小美，您的订单 SO-R120-MV-1 已发货，物流单号 {物流单号}。',
  missingVars: ['物流单号'],
  status: 'pending',
  createdAt: '2026-08-01T00:00:00Z',
  updatedAt: '2026-08-01T00:00:00Z',
};

test.describe('@round120 P2② 草稿补全变量后缺少变量警告重算', () => {
  test('编辑补全变量保存后，列表刷新不再显示「缺少变量」', async ({ admin, page }) => {
    let edited = false;
    await page.route('**/api/v1/customer/buyer-messages/drafts**', async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      const current = edited
        ? {
            ...draftWithMissing,
            content: '您好e2e-买家小美，您的订单 SO-R120-MV-1 已发货，物流单号 SF999999。',
            missingVars: [],
          }
        : draftWithMissing;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ list: [current], total: 1, page: 1, pageSize: 10, canWrite: true })),
      });
    });
    admin.writeGuard.allow({
      operation: 'edit-draft',
      method: 'PUT',
      path: /\/api\/v1\/customer\/buyer-messages\/drafts\/draft-mv-1$/,
      response: ok({
        ...draftWithMissing,
        content: '您好e2e-买家小美，您的订单 SO-R120-MV-1 已发货，物流单号 SF999999。',
        missingVars: [],
      }),
    });

    await admin.goto('/customer/buyer-messages');
    await expect(page.getByText(/缺少变量：\{物流单号\}/)).toBeVisible({ timeout: 30000 });

    const row = page.locator('.ant-table-tbody tr', { hasText: 'SO-R120-MV-1' });
    await row.getByRole('button', { name: /编\s*辑/ }).click();
    const dialog = page.getByRole('dialog');
    await dialog
      .getByLabel('消息内容')
      .fill('您好e2e-买家小美，您的订单 SO-R120-MV-1 已发货，物流单号 SF999999。');
    edited = true;
    await dialog.getByRole('button', { name: '确 定' }).click();
    await admin.writeGuard.expectRequestCount('edit-draft', 1);

    await expect(page.getByText(/缺少变量：/)).toHaveCount(0);
    await expect(
      page.locator('.ant-table-tbody tr', { hasText: 'SO-R120-MV-1' }).getByText(/物流单号 SF999999/),
    ).toBeVisible();
  });
});
