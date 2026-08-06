import type { Page, Locator } from '@playwright/test';
import { test, expect } from '../fixtures/admin.fixture';
import { ok, paged } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import { expectNoRootOverflow } from '../utils/assertions';
import type { NetworkWriteGuard } from '../utils/network-guard';
import { E2E_TAG_URGENT_ID, E2E_TAG_VIP_ID, e2eOrderTags } from '../mocks/order-tags';
import { e2eOrderAutomationRules } from '../mocks/order-automation';

const ORDER_ROWS = [
  {
    id: 'e2e-order-r135-1',
    platform: 'douyin',
    orderNo: 'E2E-SO-TAG-1',
    customerName: 'e2e-买家甲',
    status: 'pending',
    paymentStatus: 'paid',
    fulfillmentStatus: 'unfulfilled',
    currency: 'USD',
    totalAmount: 25.5,
    createdAt: '2026-08-01T02:03:04Z',
    tags: [{ id: E2E_TAG_URGENT_ID, name: '加急', color: 'red' }],
  },
  {
    id: 'e2e-order-r135-2',
    platform: 'douyin',
    orderNo: 'E2E-SO-TAG-2',
    customerName: 'e2e-买家乙',
    status: 'pending',
    paymentStatus: 'unpaid',
    fulfillmentStatus: 'unfulfilled',
    currency: 'USD',
    totalAmount: 9.9,
    createdAt: '2026-08-02T02:03:04Z',
    tags: [],
  },
];

/** 按 tagId query 过滤，并记录每次列表请求的 query 参数（URL query 单一来源口径） */
async function routeOrderList(page: Page, seen: URLSearchParams[]) {
  await page.route('**/api/v1/orders?*', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    const params = new URL(route.request().url()).searchParams;
    seen.push(params);
    const tagId = params.get('tagId') || '';
    const rows = ORDER_ROWS.filter((r) => !tagId || r.tags.some((t) => t.id === tagId));
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok(paged(rows))),
    });
  });
}

function allowCostEstimates(admin: { writeGuard: NetworkWriteGuard }) {
  admin.writeGuard.allow({
    operation: 'cost-estimates-batch',
    method: 'POST',
    path: /\/api\/v1\/procurement\/cost-estimates\/batch$/,
    response: ok({ items: {} }),
  });
}

// 打开下拉、点选选项，并确认选中值真正写入（避免下拉动画期间点击落空的竞态）；
// 多选模式下拉选中后不会自动收起，用 Escape 收起。
async function selectAntOption(
  page: Page,
  trigger: Locator,
  optionText: string,
  opts: { multiple?: boolean } = {},
) {
  const dropdown = page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').last();
  const selected = page.locator('.ant-select-selection-item', { hasText: optionText }).first();
  for (let attempt = 0; ; attempt++) {
    if (!(await dropdown.isVisible())) {
      await trigger.click({ force: true });
    }
    try {
      await expect(dropdown).toBeVisible({ timeout: 2500 });
    } catch (error) {
      if (attempt >= 4) throw error;
      continue;
    }
    const option = dropdown.locator('.ant-select-item-option', { hasText: optionText }).first();
    await expect(option).toBeVisible();
    await option.click();
    try {
      await expect(selected).toBeVisible({ timeout: 2500 });
      break;
    } catch (error) {
      if (attempt >= 4) throw error;
    }
  }
  if (opts.multiple) {
    // 多选下拉选中后不会自动收起，用 Escape 收起；单选下拉自动关闭，
    // 不额外按 Escape，避免竞态下误关 Modal。
    await page.keyboard.press('Escape');
  }
  await expect(dropdown).toBeHidden();
}

test.describe('@round135 订单标签管理页', () => {
  test('it should list tenant tags and create a new tag', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'createTag',
      method: 'POST',
      path: /^\/api\/v1\/order-tags$/,
      response: ok({
        id: 'e2e-order-tag-new',
        tenantId: 1,
        name: '赠品单',
        color: 'green',
        createdAt: '2026-01-02T00:00:00Z',
        updatedAt: '2026-01-02T00:00:00Z',
      }),
    });
    await admin.goto('/settings/order-tags');
    await expect(page.getByRole('cell', { name: '加急' })).toBeVisible({ timeout: 20000 });
    await expect(page.getByRole('cell', { name: '大客户' })).toBeVisible();

    await page.getByRole('button', { name: '新增标签' }).click();
    const dialog = page.getByRole('dialog', { name: '新增标签' });
    await expect(dialog).toBeVisible();
    await dialog.getByLabel(/标签名称/).fill('赠品单');
    await selectAntOption(page, dialog.getByLabel('标签颜色'), '绿色');
    await dialog.getByRole('button', { name: '保 存' }).click();
    await admin.writeGuard.expectRequestCount('createTag', 1);
    const [call] = admin.writeGuard.calls('createTag');
    expect(call.postDataJSON).toMatchObject({ name: '赠品单', color: 'green' });
  });

  test('readonly 角色新增/编辑/删除禁用', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
      });
    });
    await admin.goto('/settings/order-tags');
    await expect(page.getByRole('cell', { name: '加急' })).toBeVisible({ timeout: 20000 });
    await expect(page.getByRole('button', { name: '新增标签' })).toBeDisabled();
    await expect(page.getByRole('button', { name: '编辑' }).first()).toBeDisabled();
    await expect(page.getByRole('button', { name: '删除' }).first()).toBeDisabled();
  });
});

test.describe('@round135 订单列表标签列与按标签筛选（URL query 唯一来源）', () => {
  test('it should render tag column and filter by tag with URL writeback', async ({
    admin,
    page,
  }) => {
    const seen: URLSearchParams[] = [];
    await routeOrderList(page, seen);
    allowCostEstimates(admin);

    await admin.goto('/orders/list');
    const tbody = page.locator('.ant-table-tbody');
    await expect(tbody).toContainText('E2E-SO-TAG-1');
    await expect(tbody).toContainText('E2E-SO-TAG-2');
    await expect(tbody.getByText('加急', { exact: true })).toBeVisible();

    await page.getByText('展开', { exact: true }).click();
    await selectAntOption(page, page.getByLabel('标签', { exact: true }), '加急');
    await page.getByRole('button', { name: '查 询' }).click();

    await expect(page).toHaveURL(new RegExp(`tagId=${E2E_TAG_URGENT_ID}`));
    await expect(tbody).toContainText('E2E-SO-TAG-1');
    await expect(tbody).not.toContainText('E2E-SO-TAG-2');
    const last = seen[seen.length - 1];
    expect(last.get('tagId')).toBe(E2E_TAG_URGENT_ID);

    await page.getByRole('button', { name: '重 置' }).click();
    await expect(page).not.toHaveURL(/tagId=/);
    await expect(tbody).toContainText('E2E-SO-TAG-2');
  });

  test('it should honor tagId deep link and keep it after reload', async ({ admin, page }) => {
    const seen: URLSearchParams[] = [];
    await routeOrderList(page, seen);
    allowCostEstimates(admin);

    await admin.goto(`/orders/list?tagId=${E2E_TAG_URGENT_ID}`);
    const tbody = page.locator('.ant-table-tbody');
    await expect(tbody).toContainText('E2E-SO-TAG-1');
    await expect(tbody).not.toContainText('E2E-SO-TAG-2');
    expect(seen.some((p) => p.get('tagId') === E2E_TAG_URGENT_ID)).toBe(true);

    await page.reload();
    await expect(page).toHaveURL(new RegExp(`tagId=${E2E_TAG_URGENT_ID}`));
    await expect(tbody).toContainText('E2E-SO-TAG-1');
    await expect(tbody).not.toContainText('E2E-SO-TAG-2');
  });

  test('it should batch tag selected orders', async ({ admin, page }) => {
    const seen: URLSearchParams[] = [];
    await routeOrderList(page, seen);
    allowCostEstimates(admin);
    admin.writeGuard.allow({
      operation: 'batchTags',
      method: 'POST',
      path: /^\/api\/v1\/orders\/batch-tags$/,
      response: ok({ orders: 2, tags: 1, applied: 2, removed: 0 }),
    });

    await admin.goto('/orders/list');
    const tbody = page.locator('.ant-table-tbody');
    await expect(tbody).toContainText('E2E-SO-TAG-1');
    await page.getByRole('row', { name: /E2E-SO-TAG-1/ }).getByRole('checkbox').check();
    await page.getByRole('row', { name: /E2E-SO-TAG-2/ }).getByRole('checkbox').check();

    await page.getByRole('button', { name: /批量打标签（2）/ }).click();
    const dialog = page.getByRole('dialog', { name: /批量打标签/ });
    await expect(dialog).toBeVisible();
    await selectAntOption(page, dialog.getByLabel(/^标签/), '大客户', { multiple: true });
    await dialog.getByRole('button', { name: '执 行' }).click();
    await admin.writeGuard.expectRequestCount('batchTags', 1);
    const [call] = admin.writeGuard.calls('batchTags');
    expect(call.postDataJSON).toMatchObject({
      orderIds: ['e2e-order-r135-1', 'e2e-order-r135-2'],
      tagIds: [E2E_TAG_VIP_ID],
      action: 'add',
    });
  });
});

test.describe('@round135 订单详情手工打标/去标', () => {
  const DETAIL = {
    id: 'e2e-order-r135-1',
    platform: 'douyin',
    orderNo: 'E2E-SO-TAG-1',
    customerName: 'e2e-买家甲',
    status: 'pending',
    paymentStatus: 'paid',
    fulfillmentStatus: 'unfulfilled',
    currency: 'USD',
    totalAmount: 25.5,
    createdAt: '2026-08-01T02:03:04Z',
    items: [],
    tags: [{ id: E2E_TAG_URGENT_ID, name: '加急', color: 'red' }],
  };

  async function routeOrderDetail(page: Page) {
    await page.route(`**/api/v1/orders/${DETAIL.id}`, async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok(DETAIL)),
      });
    });
  }

  test('it should add and remove tags manually on detail page', async ({ admin, page }) => {
    await routeOrderDetail(page);
    admin.writeGuard.allow({
      operation: 'attachTag',
      method: 'POST',
      path: new RegExp(`/api/v1/orders/${DETAIL.id}/tags$`),
      response: ok({
        tags: [
          { id: E2E_TAG_URGENT_ID, name: '加急', color: 'red' },
          { id: E2E_TAG_VIP_ID, name: '大客户', color: 'gold' },
        ],
      }),
    });
    admin.writeGuard.allow({
      operation: 'detachTag',
      method: 'DELETE',
      path: new RegExp(`/api/v1/orders/${DETAIL.id}/tags/${E2E_TAG_URGENT_ID}$`),
      response: ok({ tags: [{ id: E2E_TAG_VIP_ID, name: '大客户', color: 'gold' }] }),
    });

    await admin.goto(`/orders/${DETAIL.id}`);
    const existing = page.locator('.ant-tag', { hasText: '加急' }).first();
    await expect(existing).toBeVisible({ timeout: 20000 });

    const addTagSelect = page.locator('.ant-descriptions .ant-select', { hasText: '添加标签' });
    const dropdown = page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)').last();
    await addTagSelect.click();
    await expect(dropdown).toBeVisible();
    await dropdown.locator('.ant-select-item-option', { hasText: '大客户' }).first().click();
    await admin.writeGuard.expectRequestCount('attachTag', 1);
    const [attach] = admin.writeGuard.calls('attachTag');
    expect(attach.postDataJSON).toMatchObject({ tagIds: [E2E_TAG_VIP_ID] });
    await expect(page.locator('.ant-tag', { hasText: '大客户' }).first()).toBeVisible();

    await existing.locator('.anticon-close').click();
    await admin.writeGuard.expectRequestCount('detachTag', 1);
    await expect(page.locator('.ant-descriptions .ant-tag', { hasText: '加急' })).toHaveCount(0);
  });

  test('readonly 详情仅展示标签，无添加/移除入口', async ({ admin, page }) => {
    await routeOrderDetail(page);
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
      });
    });
    await admin.goto(`/orders/${DETAIL.id}`);
    const tag = page.locator('.ant-descriptions .ant-tag', { hasText: '加急' }).first();
    await expect(tag).toBeVisible({ timeout: 20000 });
    await expect(tag.locator('.anticon-close')).toHaveCount(0);
    await expect(page.getByText('添加标签', { exact: true })).toHaveCount(0);
  });
});

test.describe('@round135 自动化规则「自动打标签」动作', () => {
  test('新增 add_tag 规则携带 tagIds 参数', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'createRule',
      method: 'POST',
      path: /^\/api\/v1\/order-automation-rules$/,
      response: ok({
        ...e2eOrderAutomationRules[0],
        id: 'e2e-automation-rule-new-tag',
        name: '新自动打标签动作',
        action: 'add_tag',
        tagIds: [E2E_TAG_URGENT_ID],
      }),
    });
    await admin.goto('/settings/order-automation-rules');
    const dialog = page.getByRole('dialog', { name: '新增自动化规则' });
    for (let attempt = 0; ; attempt++) {
      if (!(await dialog.isVisible())) {
        await page.getByRole('button', { name: '新增规则' }).click();
      }
      try {
        await expect(dialog).toBeVisible({ timeout: 2500 });
        break;
      } catch (error) {
        if (attempt >= 4) throw error;
      }
    }
    await dialog.getByLabel('规则名称').fill('新自动打标签动作');
    await selectAntOption(page, dialog.getByLabel('触发时机'), '进入待采购（已付款）');
    await selectAntOption(page, dialog.getByLabel('自动动作'), '自动打标签');
    await selectAntOption(page, dialog.getByLabel(/要添加的标签/), '加急', { multiple: true });
    await dialog.getByRole('button', { name: '保 存' }).click();
    await admin.writeGuard.expectRequestCount('createRule', 1);
    const [call] = admin.writeGuard.calls('createRule');
    expect(call.postDataJSON).toMatchObject({
      name: '新自动打标签动作',
      triggerEvent: 'order_paid',
      action: 'add_tag',
      tagIds: [E2E_TAG_URGENT_ID],
    });
  });
});

test.describe('@round135 响应式视口', () => {
  const viewports = [
    { width: 1440, height: 900 },
    { width: 1280, height: 800 },
    { width: 1024, height: 768 },
    { width: 768, height: 900 },
    { width: 375, height: 812 },
  ];

  for (const vp of viewports) {
    test(`订单标签页在 ${vp.width}x${vp.height} 无根节点横向溢出`, async ({ admin, page }) => {
      await page.setViewportSize(vp);
      await admin.goto('/settings/order-tags');
      await expect(page.getByRole('cell', { name: '加急' })).toBeVisible({ timeout: 20000 });
      await expectNoRootOverflow(page);
    });
  }
});
