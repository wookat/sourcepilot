import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';

const ORDER_ID = 'a4c1a111-2222-4333-8444-555566667777';

function printSheet() {
  return {
    orderId: ORDER_ID,
    orderNo: 'SO-R99-PRINT-1',
    platform: 'douyin',
    customerName: 'e2e 客户',
    items: [
      { productTitle: 'e2e 商品', skuCode: 'SKU-R99', quantity: 1, unitPrice: 10, totalPrice: 10 },
    ],
    shipments: [],
  };
}

async function mockPrintSheets(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/orders/print/sheets**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ items: [printSheet()] })),
    });
  });
}

test.describe('@round99 打印路由不被 /orders/:id 捕获', () => {
  test('深链 /orders/print?ids= 渲染拣货发货单', async ({ admin, page }) => {
    await mockPrintSheets(page);
    await admin.goto(`/orders/print?ids=${ORDER_ID}`);
    await expect(page.getByRole('heading', { name: /拣货 \/ 发货单/ })).toBeVisible();
    await expect(page.getByText('SO-R99-PRINT-1')).toBeVisible();
    await expect(page.locator('body')).not.toContainText('未找到订单');
  });

  test('别名深链 /orders/print-sheets?ids= 重定向到 /orders/print 并保留参数', async ({ admin, page }) => {
    await mockPrintSheets(page);
    await admin.goto(`/orders/print-sheets?ids=${ORDER_ID}`);
    await expect(page).toHaveURL(new RegExp(`/orders/print\\?ids=${ORDER_ID}`));
    await expect(page.getByRole('heading', { name: /拣货 \/ 发货单/ })).toBeVisible();
    await expect(page.locator('body')).not.toContainText('未找到订单');
  });

  test('刷新后仍停留在打印页', async ({ admin, page }) => {
    await mockPrintSheets(page);
    await admin.goto(`/orders/print-sheets?ids=${ORDER_ID}`);
    await expect(page).toHaveURL(new RegExp(`/orders/print\\?ids=${ORDER_ID}`));
    await page.reload();
    await expect(page.getByRole('heading', { name: /拣货 \/ 发货单/ })).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/orders/print\\?ids=${ORDER_ID}`));
  });
});
