import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import {
  INV_ORDER_ID,
  INV_PRODUCT_ID,
  INV_SKU_ID_LOW,
  routeInventoryCenterData,
} from '../mocks/inventory-center.fixture';
import { expectNoRootOverflow } from '../utils/assertions';

const viewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

async function useReadonlyProfile(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/auth/profile', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
    });
  });
}

test.describe('@inventory 库存中心列表', () => {
  test('shows product, sku, stock, warning and status with quick links', async ({ admin, page }) => {
    await routeInventoryCenterData(page);
    await admin.goto('/inventory');
    // 首次访问可能触发 dev server 按需编译，放宽等待时间
    await expect(page.getByText('库存中心').first()).toBeVisible({ timeout: 20_000 });
    await expect(page.getByRole('link', { name: 'E2E 保温杯 500ml' }).first()).toBeVisible();
    await expect(page.getByText('E2E-SKU-2')).toBeVisible();
    await expect(page.getByText('低库存').first()).toBeVisible();
    await expect(page.getByRole('link', { name: '流水' }).first()).toBeVisible();
    await expect(page.getByRole('link', { name: '扣减记录' }).first()).toBeVisible();
  });

  for (const viewport of viewports) {
    test(`no root overflow at ${viewport.width}x${viewport.height}`, async ({ admin, page }) => {
      await routeInventoryCenterData(page);
      await page.setViewportSize(viewport);
      await admin.goto('/inventory');
      await expect(page.getByText('库存中心').first()).toBeVisible();
      await expectNoRootOverflow(page);
    });
  }
});

test.describe('@inventory 库存流水可追溯', () => {
  test('rows show chinese type/reason, product link and order link', async ({ admin, page }) => {
    await routeInventoryCenterData(page);
    await admin.goto('/inventory/logs');
    await expect(page.getByText('库存流水').first()).toBeVisible();
    await expect(page.getByText('人工修正').first()).toBeVisible();
    await expect(page.getByText('盘点补货').first()).toBeVisible();
    const productLink = page.getByRole('link', { name: 'E2E 保温杯 500ml' }).first();
    await expect(productLink).toHaveAttribute('href', new RegExp(INV_PRODUCT_ID));
    const orderLink = page.getByRole('link', { name: 'E2E-SO-0001' });
    await expect(orderLink).toHaveAttribute('href', new RegExp(INV_ORDER_ID));
  });

  test('empty state shows guidance', async ({ admin, page }) => {
    await routeInventoryCenterData(page, { emptyLogs: true });
    await admin.goto('/inventory/logs');
    await expect(page.getByText('暂无库存流水')).toBeVisible();
  });

  for (const viewport of viewports) {
    test(`no root overflow at ${viewport.width}x${viewport.height}`, async ({ admin, page }) => {
      await routeInventoryCenterData(page);
      await page.setViewportSize(viewport);
      await admin.goto('/inventory/logs');
      await expect(page.getByText('库存流水').first()).toBeVisible();
      await expectNoRootOverflow(page);
    });
  }
});

test.describe('@inventory 手工调整库存', () => {
  const adjustPath = new RegExp(`/api/v1/products/${INV_PRODUCT_ID}/skus/${INV_SKU_ID_LOW}/adjust-stock$`);

  test('cancel produces zero writes', async ({ admin, page }) => {
    await routeInventoryCenterData(page);
    await admin.goto('/inventory/alerts');
    await page.getByRole('button', { name: '调整库存' }).first().click();
    await expect(page.getByRole('dialog').getByText(/调整库存/)).toBeVisible();
    await page.getByRole('dialog').getByRole('button', { name: /取.?消/ }).click();
    await admin.writeGuard.expectRequestCount('adjust-stock', 0);
  });

  test('confirm produces exactly one write and double click does not duplicate', async ({ admin, page }) => {
    await routeInventoryCenterData(page);
    admin.writeGuard.allow({
      operation: 'adjust-stock',
      method: 'POST',
      path: adjustPath,
      response: ok({}),
    });
    await admin.goto('/inventory/alerts');
    await page.getByRole('button', { name: '调整库存' }).first().click();
    const dialog = page.getByRole('dialog').filter({ hasText: '调整库存' });
    await expect(dialog).toBeVisible();
    const okButton = dialog.getByRole('button', { name: /确\s*定|OK/ });
    await okButton.click();
    // 二次确认（敏感操作确认弹窗）
    const sensitiveConfirm = page.getByRole('dialog').filter({ hasText: '人工修正库存' });
    await expect(sensitiveConfirm).toBeVisible();
    const confirmBtn = sensitiveConfirm.getByRole('button', { name: /确\s*定|确认|OK/ }).last();
    await confirmBtn.click();
    await confirmBtn.click({ force: true }).catch(() => undefined);
    await admin.writeGuard.expectRequestCount('adjust-stock', 1);
  });
});

test.describe('@inventory readonly 口径', () => {
  test('alerts hides write actions for readonly user', async ({ admin, page }) => {
    await routeInventoryCenterData(page);
    await useReadonlyProfile(page);
    await admin.goto('/inventory/alerts');
    await expect(page.getByText('库存预警').first()).toBeVisible();
    await expect(page.getByRole('link', { name: '商品' }).first()).toBeVisible();
    await expect(page.getByRole('button', { name: '调整库存' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: '批量设置预警线' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: '批量同步库存' })).toHaveCount(0);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('sync tasks hides retry actions for readonly user', async ({ admin, page }) => {
    await useReadonlyProfile(page);
    await admin.goto('/inventory/sync-tasks');
    await expect(page.getByText('库存同步任务').first()).toBeVisible();
    await expect(page.getByRole('button', { name: /批量重试失败/ })).toHaveCount(0);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
