import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import { expectNoRootOverflow } from '../utils/assertions';
import {
  E2E_FINANCE_ORDER_ID,
  E2E_FINANCE_PAYMENT_ID,
  E2E_FINANCE_SHOP_ID,
  e2eFinanceOrdersLookup,
  e2eFinancePayments,
  e2eFinanceShops,
  e2eShopExpenses,
} from '../mocks/finance';

import type { Page } from '@playwright/test';

async function mockShops(page: Page) {
  await page.route('**/api/v1/shops?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok(e2eFinanceShops)),
    });
  });
}

test.describe('@round121 回款与费用页', () => {
  test('回款列表展示金额、对账状态与来源标签', async ({ admin, page }) => {
    await mockShops(page);
    await admin.goto('/orders/finance-payments');
    await expect(page.getByRole('link', { name: 'SO-E2E-FIN-1' })).toBeVisible({ timeout: 20000 });
    await expect(page.getByText('已结清')).toBeVisible();
    await expect(page.getByText('少款', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('手工', { exact: true })).toBeVisible();
    await expect(page.getByText('导入', { exact: true })).toBeVisible();
    await expectNoRootOverflow(page);
  });

  test('登记回款先查订单号再 POST 回款 payload', async ({ admin, page }) => {
    await mockShops(page);
    await page.route('**/api/v1/orders?*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok(e2eFinanceOrdersLookup)),
      });
    });
    admin.writeGuard.allow({
      operation: 'createPayment',
      method: 'POST',
      path: /^\/api\/v1\/finance\/payments$/,
      response: ok({ ...e2eFinancePayments[0], id: 'e2e-finance-payment-new' }),
    });
    await admin.goto('/orders/finance-payments');
    await page.getByRole('button', { name: '登记回款' }).click();
    const dialog = page.getByRole('dialog', { name: '登记回款' });
    await dialog.getByLabel('订单号').fill('SO-E2E-FIN-1');
    await dialog.getByLabel('回款金额').fill('199.5');
    await dialog.getByLabel('手续费').fill('3.99');
    await dialog.getByLabel('回款渠道').fill('平台结算');
    await dialog.getByRole('button', { name: '确 定' }).click();
    await admin.writeGuard.expectRequestCount('createPayment', 1);
    const [call] = admin.writeGuard.calls('createPayment');
    expect(call.postDataJSON).toMatchObject({
      orderId: E2E_FINANCE_ORDER_ID,
      amount: 199.5,
      feeAmount: 3.99,
      channel: '平台结算',
    });
    expect((call.postDataJSON as { receivedAt: string }).receivedAt).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });

  test('删除回款记录发起 DELETE 请求', async ({ admin, page }) => {
    await mockShops(page);
    admin.writeGuard.allow({
      operation: 'deletePayment',
      method: 'DELETE',
      path: new RegExp(`^/api/v1/finance/payments/${E2E_FINANCE_PAYMENT_ID}$`),
      response: ok({ deleted: true }),
    });
    await admin.goto('/orders/finance-payments');
    const row = page.getByRole('row', { name: /SO-E2E-FIN-1/ });
    await row.getByText('删除').click();
    await page.getByRole('button', { name: '确 定' }).click();
    await admin.writeGuard.expectRequestCount('deletePayment', 1);
  });

  test('登记店铺月度费用 POST 携带店铺、月份与费用类型', async ({ admin, page }) => {
    await mockShops(page);
    admin.writeGuard.allow({
      operation: 'createShopExpense',
      method: 'POST',
      path: /^\/api\/v1\/finance\/shop-expenses$/,
      response: ok({ ...e2eShopExpenses[0], id: 'e2e-finance-shop-expense-new' }),
    });
    await admin.goto('/orders/finance-payments');
    await page.getByRole('tab', { name: '店铺月度费用' }).click();
    await page.getByRole('button', { name: '登记店铺月度费用' }).click();
    const dialog = page.getByRole('dialog', { name: '登记店铺月度费用' });
    await dialog.getByLabel('店铺').click();
    await page.locator('.ant-select-item-option', { hasText: 'e2e 店铺A' }).first().click();
    await dialog.getByLabel('月份').click();
    await page.locator('.ant-picker-cell-inner', { hasText: /^8月$/ }).first().click();
    await dialog.getByLabel('费用类型').click();
    await page.locator('.ant-select-item-option', { hasText: '推广费' }).first().click();
    await dialog.getByLabel('费用金额').fill('88');
    await dialog.getByRole('button', { name: '确 定' }).click();
    await admin.writeGuard.expectRequestCount('createShopExpense', 1);
    const [call] = admin.writeGuard.calls('createShopExpense');
    expect(call.postDataJSON).toMatchObject({
      shopId: E2E_FINANCE_SHOP_ID,
      typeCode: 'promotion',
      amount: 88,
    });
    expect((call.postDataJSON as { month: string }).month).toMatch(/^\d{4}-\d{2}$/);
  });

  test('readonly 角色隐藏写操作并提示只读', async ({ admin, page }) => {
    await mockShops(page);
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
      });
    });
    await admin.goto('/orders/finance-payments');
    await expect(page.getByText('当前账号为只读权限，仅可查看回款记录')).toBeVisible({
      timeout: 20000,
    });
    await expect(page.getByRole('button', { name: '登记回款' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'CSV 批量导入' })).toHaveCount(0);
    await expect(page.getByRole('row', { name: /SO-E2E-FIN-1/ }).getByText('删除')).toHaveCount(0);
  });
});

test.describe('@round121 对账差异工作台', () => {
  test('展示汇总统计、差异行与实算/估算毛利', async ({ admin, page }) => {
    await admin.goto('/orders/finance-reconciliation');
    await expect(page.getByRole('link', { name: 'SO-E2E-FIN-1' })).toBeVisible({ timeout: 20000 });
    await expect(page.getByText('毛利差异较大')).toBeVisible();
    await expect(page.getByText('待处理异常')).toBeVisible();
    await expect(page.getByRole('table').getByText('差异较大', { exact: true })).toBeVisible();
    await expectNoRootOverflow(page);
  });

  test('五档视口无根节点横向溢出', async ({ admin, page }) => {
    await admin.goto('/orders/finance-reconciliation');
    await expect(page.getByRole('link', { name: 'SO-E2E-FIN-1' })).toBeVisible({ timeout: 20000 });
    for (const viewport of [
      { width: 1440, height: 900 },
      { width: 1280, height: 800 },
      { width: 1024, height: 768 },
      { width: 768, height: 900 },
      { width: 375, height: 812 },
    ]) {
      await page.setViewportSize(viewport);
      await expectNoRootOverflow(page);
    }
  });
});

test.describe('@round121 对账报表', () => {
  test('按店铺/月份展示回款率与实算 vs 估算毛利差异', async ({ admin, page }) => {
    await admin.goto('/orders/finance-report');
    await expect(page.getByRole('cell', { name: 'e2e 店铺A' })).toBeVisible({ timeout: 20000 });
    await expect(page.getByRole('cell', { name: '2026-08' })).toBeVisible();
    await expect(page.getByText('63.96%')).toBeVisible();
    await expectNoRootOverflow(page);
  });
});
