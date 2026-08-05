import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import { expectNoRootOverflow } from '../utils/assertions';
import { e2eOrderAutomationRules } from '../mocks/order-automation';

test.describe('@round126 自动化动作面扩展：规则页展示', () => {
  test('展示自动应用发货规则 / 自动分仓动作及参数标签', async ({ admin, page }) => {
    await admin.goto('/settings/order-automation-rules');
    await expect(page.getByRole('cell', { name: '付款后自动应用发货规则' })).toBeVisible({
      timeout: 20000,
    });
    await expect(page.getByText('自动应用发货规则', { exact: true })).toBeVisible();
    await expect(page.getByText('直接应用物流商', { exact: true })).toBeVisible();
    await expect(page.getByRole('cell', { name: '付款后自动分仓' })).toBeVisible();
    await expect(page.getByText('自动分仓', { exact: true })).toBeVisible();
    await expect(page.getByText('库存充足优先', { exact: true })).toBeVisible();
  });

  test('新增「自动应用发货规则」携带应用方式参数', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'createRule',
      method: 'POST',
      path: /^\/api\/v1\/order-automation-rules$/,
      response: ok({
        ...e2eOrderAutomationRules[2],
        id: 'e2e-automation-rule-new-ship',
        name: '新发货规则动作',
      }),
    });
    await admin.goto('/settings/order-automation-rules');
    await page.getByRole('button', { name: '新增规则' }).click();
    const dialog = page.getByRole('dialog', { name: '新增自动化规则' });
    await dialog.getByLabel('规则名称').fill('新发货规则动作');
    await dialog.getByLabel('触发时机').click({ force: true });
    await page
      .locator('.ant-select-item-option', { hasText: '进入待采购（已付款）' })
      .first()
      .click();
    await dialog.getByLabel('自动动作').click();
    await page.locator('.ant-select-item-option', { hasText: '自动应用发货规则' }).first().click();
    await dialog.getByLabel(/发货规则应用方式/).click({ force: true });
    await page.locator('.ant-select-item-option', { hasText: '直接应用物流商' }).first().click();
    await dialog.getByRole('button', { name: '保 存' }).click();
    await admin.writeGuard.expectRequestCount('createRule', 1);
    const [call] = admin.writeGuard.calls('createRule');
    expect(call.postDataJSON).toMatchObject({
      name: '新发货规则动作',
      triggerEvent: 'order_paid',
      action: 'apply_shipping_rule',
      shippingApplyMode: 'apply',
    });
  });

  test('新增「自动分仓」携带分仓策略参数', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'createRule',
      method: 'POST',
      path: /^\/api\/v1\/order-automation-rules$/,
      response: ok({
        ...e2eOrderAutomationRules[3],
        id: 'e2e-automation-rule-new-wh',
        name: '新自动分仓动作',
      }),
    });
    await admin.goto('/settings/order-automation-rules');
    await page.getByRole('button', { name: '新增规则' }).click();
    const dialog = page.getByRole('dialog', { name: '新增自动化规则' });
    await dialog.getByLabel('规则名称').fill('新自动分仓动作');
    await dialog.getByLabel('触发时机').click({ force: true });
    await page
      .locator('.ant-select-item-option', { hasText: '进入待采购（已付款）' })
      .first()
      .click();
    await dialog.getByLabel('自动动作').click();
    await page.locator('.ant-select-item-option', { hasText: '自动分仓' }).first().click();
    await dialog.getByLabel(/分仓策略/).click({ force: true });
    await page.locator('.ant-select-item-option', { hasText: '库存充足优先' }).first().click();
    await dialog.getByRole('button', { name: '保 存' }).click();
    await admin.writeGuard.expectRequestCount('createRule', 1);
    const [call] = admin.writeGuard.calls('createRule');
    expect(call.postDataJSON).toMatchObject({
      name: '新自动分仓动作',
      triggerEvent: 'order_paid',
      action: 'assign_warehouse',
      warehouseStrategy: 'stock_first',
    });
  });

  test('订单创建事件不提供自动分仓动作（口径与后端一致）', async ({ admin, page }) => {
    await admin.goto('/settings/order-automation-rules');
    await page.getByRole('button', { name: '新增规则' }).click();
    const dialog = page.getByRole('dialog', { name: '新增自动化规则' });
    await dialog.getByLabel('自动动作').click();
    await expect(
      page.locator('.ant-select-item-option', { hasText: '自动应用发货规则' }),
    ).toBeVisible();
    await expect(
      page.locator('.ant-select-item-option', { hasText: '自动分仓' }),
    ).toHaveCount(0);
  });

  test('readonly 角色新增禁用', async ({ admin, page }) => {
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

test.describe('@round126 自动分仓失败留痕日志', () => {
  test('库存不足失败日志可见且可重试', async ({ admin, page }) => {
    await admin.goto('/orders/automation-logs');
    await expect(page.getByRole('cell', { name: 'SO-E2E-AT-4' })).toBeVisible({ timeout: 20000 });
    await expect(
      page.getByText('执行失败（已重试 3 次）：库存不足，无法分配发货仓：SKU-1 需 999 件仅 6 件'),
    ).toBeVisible();
    const row = page.getByRole('row', { name: /SO-E2E-AT-4/ });
    await expect(row.getByRole('button', { name: '重试' })).toBeEnabled();
  });
});

test.describe('@round126 响应式视口', () => {
  const viewports = [
    { width: 1440, height: 900 },
    { width: 1280, height: 800 },
    { width: 1024, height: 768 },
    { width: 768, height: 900 },
    { width: 375, height: 812 },
  ];

  for (const vp of viewports) {
    test(`新动作表单在 ${vp.width}x${vp.height} 无根节点横向溢出`, async ({ admin, page }) => {
      await page.setViewportSize(vp);
      await admin.goto('/settings/order-automation-rules');
      await expect(page.getByRole('cell', { name: '付款后自动分仓' })).toBeVisible({
        timeout: 20000,
      });
      await expectNoRootOverflow(page);
      await page.getByRole('button', { name: '新增规则' }).click();
      const dialog = page.getByRole('dialog', { name: '新增自动化规则' });
      await dialog.getByLabel('触发时机').click({ force: true });
      await page
        .locator('.ant-select-item-option', { hasText: '进入待采购（已付款）' })
        .first()
        .click();
      await dialog.getByLabel('自动动作').click();
      await page.locator('.ant-select-item-option', { hasText: '自动分仓' }).first().click();
      await expect(dialog.getByLabel(/分仓策略/)).toBeVisible();
      await expectNoRootOverflow(page);
    });
  }
});
