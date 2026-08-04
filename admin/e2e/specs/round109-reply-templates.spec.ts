import { test, expect } from '../fixtures/admin.fixture';
import { ok, fail } from '../mocks/envelope';
import { expectNoRootOverflow } from '../utils/assertions';

const CONV_ID = 'e2e-conv-r109-1';

const templates = [
  {
    id: 'tpl-presale-1',
    groupKey: 'presale',
    name: 'e2e-售前-库存确认',
    content: '亲爱的{买家昵称}您好！您咨询的{商品名}现在有现货。',
    sortOrder: 1,
    enabled: true,
    createdAt: '2026-08-01T00:00:00Z',
    updatedAt: '2026-08-01T00:00:00Z',
  },
  {
    id: 'tpl-presale-2',
    groupKey: 'presale',
    name: 'e2e-售前-尺寸咨询',
    content: '您好{买家昵称}！{商品名}的尺寸请见详情页。',
    sortOrder: 2,
    enabled: true,
    createdAt: '2026-08-01T00:00:00Z',
    updatedAt: '2026-08-01T00:00:00Z',
  },
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

async function mockTemplateList(page: import('@playwright/test').Page, canWrite = true) {
  await page.route('**/api/v1/customer/reply-templates**', async (route) => {
    const request = route.request();
    if (request.method() !== 'GET') {
      await route.fallback();
      return;
    }
    const url = new URL(request.url());
    const group = url.searchParams.get('group');
    const keyword = url.searchParams.get('keyword');
    let list = templates;
    if (group) list = list.filter((t) => t.groupKey === group);
    if (keyword) list = list.filter((t) => t.name.includes(keyword) || t.content.includes(keyword));
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ list, canWrite })),
    });
  });
}

async function mockConversation(page: import('@playwright/test').Page) {
  await page.route(`**/api/v1/customer/conversations/${CONV_ID}**`, async (route) => {
    const request = route.request();
    if (request.method() !== 'GET') {
      await route.fallback();
      return;
    }
    const path = new URL(request.url()).pathname;
    if (path.endsWith('/messages')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            list: [
              {
                id: 'msg-1',
                conversationId: CONV_ID,
                role: 'customer',
                content: '请问什么时候发货？',
                language: 'zh',
                messageType: 'text',
                source: 'imported',
                createdAt: '2026-08-01T01:00:00Z',
              },
            ],
          }),
        ),
      });
      return;
    }
    if (path.endsWith('/ai-suggestions')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ list: [] })),
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        ok({
          id: CONV_ID,
          platform: 'manual',
          customerName: 'e2e-买家小美',
          customerLanguage: 'zh',
          status: 'pending_reply',
          createdAt: '2026-08-01T00:00:00Z',
          updatedAt: '2026-08-01T00:00:00Z',
          canWrite: true,
          orderSummary: {
            id: 'order-1',
            orderNo: 'SO-2026-0001',
            platform: 'manual',
            status: 'paid',
            paymentStatus: 'paid',
            fulfillmentStatus: 'shipped',
            currency: 'CNY',
            totalAmount: 99,
            shipments: [
              { carrier: '顺丰', trackingNo: 'SF888888', status: 'shipped' },
            ],
          },
        }),
      ),
    });
  });
}

test.describe('@round109 客服话术模板', () => {
  test('模板管理页：分组 Tab、搜索、新增/启停/排序（写请求显式拦截）', async ({ admin, page }) => {
    await mockTemplateList(page);
    admin.writeGuard.allow({
      operation: 'create-template',
      method: 'POST',
      path: /\/api\/v1\/customer\/reply-templates$/,
      response: ok(templates[0]),
    });
    admin.writeGuard.allow({
      operation: 'toggle-template',
      method: 'PUT',
      path: /\/api\/v1\/customer\/reply-templates\/tpl-presale-1$/,
      response: ok({ ...templates[0], enabled: false }),
    });
    admin.writeGuard.allow({
      operation: 'reorder-template',
      method: 'POST',
      path: /\/api\/v1\/customer\/reply-templates\/reorder$/,
      response: ok({ ok: true }),
    });

    await admin.goto('/customer/reply-templates');
    await expect(page.getByText('e2e-售前-库存确认')).toBeVisible();
    await expect(page.getByText('e2e-物流-查询进度')).toBeVisible();

    // 分组 Tab 过滤
    await page.getByRole('tab', { name: '物流' }).click();
    await expect(page.getByText('e2e-物流-查询进度')).toBeVisible();
    await expect(page.getByText('e2e-售前-库存确认')).toHaveCount(0);
    await page.getByRole('tab', { name: '全部' }).click();

    // 搜索
    await page.getByPlaceholder('按名称 / 内容搜索').fill('退款');
    await page.getByPlaceholder('按名称 / 内容搜索').press('Enter');
    await expect(page.getByText('e2e-退款-流程说明')).toBeVisible();
    await expect(page.getByText('e2e-售前-库存确认')).toHaveCount(0);
    await page.getByPlaceholder('按名称 / 内容搜索').clear();
    await page.getByPlaceholder('按名称 / 内容搜索').press('Enter');

    // 新增
    await page.getByRole('button', { name: '新增话术模板' }).click();
    await expect(page.getByRole('dialog')).toBeVisible();
    await page.getByLabel('名称').fill('e2e-新模板');
    await page.getByLabel('内容').fill('您好{买家昵称}');
    await page.getByRole('button', { name: '确 定' }).click();
    await admin.writeGuard.expectRequestCount('create-template', 1);

    // 启停
    const presaleRow = page.locator('.ant-table-tbody tr', { hasText: 'e2e-售前-库存确认' });
    await presaleRow.getByRole('switch').click();
    await admin.writeGuard.expectRequestCount('toggle-template', 1);

    // 排序
    await page.getByRole('button', { name: '下移「e2e-售前-库存确认」' }).click();
    await admin.writeGuard.expectRequestCount('reorder-template', 1);
  });

  test('无客服写权限（readonly 口径）时按钮禁用，不发出写请求', async ({ admin, page }) => {
    await mockTemplateList(page, false);
    await admin.goto('/customer/reply-templates');
    await expect(page.getByText('e2e-售前-库存确认')).toBeVisible();
    await expect(page.getByRole('button', { name: '新增话术模板' })).toBeDisabled();
    const presaleRow = page.locator('.ant-table-tbody tr', { hasText: 'e2e-售前-库存确认' });
    await expect(presaleRow.getByRole('switch')).toBeDisabled();
    await expect(presaleRow.getByRole('button', { name: /编\s*辑/ })).toBeDisabled();
    await expect(presaleRow.getByRole('button', { name: /删\s*除/ })).toBeDisabled();
  });

  test('会话回复框：插入模板并自动填充变量，插入后仍可编辑且不自动外发', async ({ admin, page }) => {
    await mockTemplateList(page);
    await mockConversation(page);

    await admin.goto(`/customer/conversations/${CONV_ID}`);
    await expect(page.getByPlaceholder('编辑或手写回复内容…')).toBeVisible();

    await page.getByRole('button', { name: '插入话术模板' }).click();
    await expect(page.getByRole('dialog', { name: '插入话术模板' })).toBeVisible();

    // 搜索 + 分组
    const dialog = page.getByRole('dialog', { name: '插入话术模板' });
    await dialog.getByRole('tab', { name: '物流' }).click();
    await expect(dialog.getByText('e2e-物流-查询进度')).toBeVisible();
    await expect(dialog.getByText('e2e-售前-库存确认')).toHaveCount(0);

    await dialog.getByRole('button', { name: /插\s*入/ }).first().click();

    // 变量按会话上下文自动填充
    const replyBox = page.getByPlaceholder('编辑或手写回复内容…');
    await expect(replyBox).toHaveValue(
      '您好e2e-买家小美，您的订单 SO-2026-0001 已发货，物流单号 SF888888。',
    );

    // 插入后仍可编辑
    await replyBox.fill('您好e2e-买家小美，已加急处理。');
    await expect(replyBox).toHaveValue('您好e2e-买家小美，已加急处理。');
  });

  test('模板加载失败时展示中文错误与重试', async ({ admin, page }) => {
    await page.route('**/api/v1/customer/reply-templates**', async (route) => {
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
    await admin.goto('/customer/reply-templates');
    await expect(page.getByText('加载话术模板失败')).toBeVisible();
    await expect(page.getByRole('button', { name: /重\s*试/ })).toBeVisible();
  });

  const viewports = [
    { name: '1440x900', width: 1440, height: 900 },
    { name: '1024x768', width: 1024, height: 768 },
    { name: '375x812', width: 375, height: 812 },
  ];
  for (const vp of viewports) {
    test(`模板管理页 ${vp.name} 无根节点横向溢出`, async ({ admin, page }) => {
      await mockTemplateList(page);
      await page.setViewportSize({ width: vp.width, height: vp.height });
      await admin.goto('/customer/reply-templates');
      await expect(page.getByText('e2e-售前-库存确认')).toBeVisible();
      await expectNoRootOverflow(page);
    });
  }
});
