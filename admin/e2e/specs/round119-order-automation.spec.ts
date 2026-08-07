import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import { expectNoRootOverflow } from '../utils/assertions';
import {
  E2E_AUTOMATION_LOG_FAILED_ID,
  E2E_AUTOMATION_RULE_ID,
  e2eOrderAutomationLogs,
  e2eOrderAutomationRules,
} from '../mocks/order-automation';

test.describe('@round119 自动化订单规则管理页', () => {
  test('展示规则触发时机、条件、动作与启停状态', async ({ admin, page }) => {
    await admin.goto('/settings/order-automation-rules');
    await expect(page.getByRole('cell', { name: '低额订单自动确认付款' })).toBeVisible({
      timeout: 20000,
    });
    await expect(page.getByText('金额：≤200 元')).toBeVisible();
    await expect(page.getByRole('cell', { name: '订单创建' })).toBeVisible();
    await expect(page.getByText('自动确认付款', { exact: true })).toBeVisible();
    await expect(page.getByText('自动生成采购单', { exact: true })).toBeVisible();
    await expect(page.getByText('要求审单已通过')).toBeVisible();
  });

  test('新增规则发起 POST 携带触发时机、动作与条件', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'createRule',
      method: 'POST',
      path: /^\/api\/v1\/order-automation-rules$/,
      response: ok({
        ...e2eOrderAutomationRules[0],
        id: 'e2e-automation-rule-new',
        name: '新自动化规则',
      }),
    });
    await admin.goto('/settings/order-automation-rules');
    await page.getByRole('button', { name: '新增规则' }).click();
    const dialog = page.getByRole('dialog', { name: '新增自动化规则' });
    await dialog.getByLabel('规则名称').fill('新自动化规则');
    await dialog.getByLabel('自动动作').click();
    await page.locator('.ant-select-item-option', { hasText: '自动确认付款' }).first().click();
    await dialog.locator('input[id="maxAmount"]').fill('150');
    await dialog.getByRole('button', { name: '保 存' }).click();
    await admin.writeGuard.expectRequestCount('createRule', 1);
    const [call] = admin.writeGuard.calls('createRule');
    expect(call.postDataJSON).toMatchObject({
      name: '新自动化规则',
      priority: 10,
      triggerEvent: 'order_created',
      action: 'confirm_payment',
      maxAmount: 150,
    });
  });

  test('测试跑发起 dry-run 并展示命中数、安全边界跳过数与样本', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'dryRun',
      method: 'POST',
      path: /^\/api\/v1\/order-automation-rules\/dry-run$/,
      response: ok({
        scanned: 100,
        matched: 5,
        blocked: 2,
        samples: [
          {
            orderId: 'o1',
            orderNo: 'SO-AT-DR-1',
            amount: 120,
            reason: '命中规则条件，将执行「自动确认付款」',
            blocked: false,
          },
        ],
      }),
    });
    await admin.goto('/settings/order-automation-rules');
    await page.getByRole('button', { name: '新增规则' }).click();
    const dialog = page.getByRole('dialog', { name: '新增自动化规则' });
    await dialog.getByLabel('规则名称').fill('临时规则');
    await dialog.getByLabel('自动动作').click();
    await page.locator('.ant-select-item-option', { hasText: '自动确认付款' }).first().click();
    await dialog.locator('input[id="maxAmount"]').fill('200');
    await dialog.getByRole('button', { name: '测试跑（dry-run）' }).click();
    await admin.writeGuard.expectRequestCount('dryRun', 1);
    await expect(
      dialog.getByText('测试跑结果：扫描最近 100 单，命中 5 单，其中 2 单将被安全边界跳过'),
    ).toBeVisible();
    await expect(
      dialog.getByText('SO-AT-DR-1（120）：命中规则条件，将执行「自动确认付款」'),
    ).toBeVisible();
  });

  test('启停规则发起 PUT enabled 请求', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'toggleRule',
      method: 'PUT',
      path: new RegExp(`^/api/v1/order-automation-rules/${E2E_AUTOMATION_RULE_ID}$`),
      response: ok({ ...e2eOrderAutomationRules[0], enabled: false }),
    });
    await admin.goto('/settings/order-automation-rules');
    const row = page.getByRole('row', { name: /低额订单自动确认付款/ });
    await row.getByRole('switch').click();
    await admin.writeGuard.expectRequestCount('toggleRule', 1);
    const [call] = admin.writeGuard.calls('toggleRule');
    expect(call.postDataJSON).toEqual({ enabled: false });
  });

  test('readonly 角色新增/编辑禁用', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
      });
    });
    await admin.goto('/settings/order-automation-rules');
    await expect(page.getByRole('button', { name: '新增规则' })).toBeDisabled();
  });
});

test.describe('@round119 自动化执行日志页', () => {
  test('展示成功/失败/跳过日志与原因', async ({ admin, page }) => {
    await admin.goto('/orders/automation-logs');
    await expect(page.getByRole('cell', { name: 'SO-E2E-AT-1' })).toBeVisible({ timeout: 20000 });
    await expect(page.getByText('成功', { exact: true })).toBeVisible();
    await expect(page.getByText('失败', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('跳过', { exact: true })).toBeVisible();
    await expect(page.getByText('生成采购单被阻断：SKU 未匹配货源')).toBeVisible();
    await expect(
      page.getByText('订单审单状态为待审核/挂起，安全边界禁止自动化'),
    ).toBeVisible();
  });

  test('失败日志可发起 POST retry', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'retryLog',
      method: 'POST',
      path: new RegExp(`^/api/v1/order-automation-logs/${E2E_AUTOMATION_LOG_FAILED_ID}/retry$`),
      response: ok({ ...e2eOrderAutomationLogs[1], status: 'success', attempts: 4 }),
    });
    await admin.goto('/orders/automation-logs');
    const row = page.getByRole('row', { name: /SO-E2E-AT-2/ });
    await row.getByRole('button', { name: '重试' }).click();
    await admin.writeGuard.expectRequestCount('retryLog', 1);
    await expect(page.getByText('「付款后自动生成采购单」重试成功')).toBeVisible();
  });

  test('readonly 角色重试禁用', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
      });
    });
    await admin.goto('/orders/automation-logs');
    const row = page.getByRole('row', { name: /SO-E2E-AT-2/ });
    await expect(row.getByRole('button', { name: '重试' })).toBeDisabled();
  });

  test('readonly 空列表空态提示店铺权限范围（R150 v24 P2-2 回归）', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
      });
    });
    await page.route('**/api/v1/order-automation-logs**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ items: [], total: 0, page: 1, pageSize: 20, totalPages: 0 })),
      });
    });
    await admin.goto('/orders/automation-logs');
    await expect(page.getByText('暂无执行日志')).toBeVisible({ timeout: 20000 });
    await expect(page.getByText('店铺权限范围导致看不到数据')).toBeVisible();
    // readonly 账号不展示引导动作按钮
    await expect(page.getByRole('button', { name: '前往自动化规则' })).toHaveCount(0);
  });

  test('非 readonly 账号空列表空态保留引导动作与权限提示', async ({ admin, page }) => {
    await page.route('**/api/v1/order-automation-logs**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ items: [], total: 0, page: 1, pageSize: 20, totalPages: 0 })),
      });
    });
    await admin.goto('/orders/automation-logs');
    await expect(page.getByText('暂无执行日志')).toBeVisible({ timeout: 20000 });
    await expect(page.getByText('店铺权限范围导致看不到数据')).toBeVisible();
    await expect(page.getByRole('button', { name: '前往自动化规则' })).toBeVisible();
  });
});

test.describe('@round119 响应式视口', () => {
  const viewports = [
    { width: 1440, height: 900 },
    { width: 1280, height: 800 },
    { width: 1024, height: 768 },
    { width: 768, height: 900 },
    { width: 375, height: 812 },
  ];

  for (const vp of viewports) {
    test(`规则页与日志页在 ${vp.width}x${vp.height} 无根节点横向溢出`, async ({
      admin,
      page,
    }) => {
      await page.setViewportSize(vp);
      await admin.goto('/settings/order-automation-rules');
      await expect(page.getByRole('cell', { name: '低额订单自动确认付款' })).toBeVisible({
        timeout: 20000,
      });
      await expectNoRootOverflow(page);
      await admin.goto('/orders/automation-logs');
      await expect(page.getByRole('cell', { name: 'SO-E2E-AT-1' })).toBeVisible({
        timeout: 20000,
      });
      await expectNoRootOverflow(page);
    });
  }
});
