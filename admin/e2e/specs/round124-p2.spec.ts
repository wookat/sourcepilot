import type { Page } from '@playwright/test';
import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { expectNoRootOverflow } from '../utils/assertions';
import {
  productOperationDashboardBody,
  profitReportBody,
  salesStatsBody,
} from '../mocks/mobile-home';

const E2E_PO_ID = 'e2e-po-round124-0001';
const SALES_ORDER_ID = '3f1c2a45-0000-4000-8000-124000000001';
const SALES_ORDER_NO = 'SO-20260801-0007';

async function mockMobileHomeApis(page: Page) {
  await page.route('**/api/v1/orders/stats/sales*', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(salesStatsBody()) });
  });
  await page.route('**/api/v1/reports/profit*', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(profitReportBody(7)) });
  });
  await page.route('**/api/v1/dashboard/product-operations*', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(productOperationDashboardBody()),
    });
  });
}

async function mockPurchaseOrderDetail(page: Page) {
  await page.route(`**/api/v1/procurement/orders/${E2E_PO_ID}`, async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        ok({
          id: E2E_PO_ID,
          supplierId: 'e2e-supplier-0001',
          supplierName: 'e2e 测试供应商',
          sourcePlatform: '1688',
          status: 'draft',
          totalAmount: 19.8,
          currency: 'CNY',
          payStatus: 'unpaid',
          idempotencyKey: 'e2e-key',
          retryCount: 0,
          createdAt: '2026-08-01T02:03:04Z',
          items: [
            {
              id: 'e2e-po-item-0001',
              purchaseOrderId: E2E_PO_ID,
              salesOrderId: SALES_ORDER_ID,
              salesOrderNo: SALES_ORDER_NO,
              localSkuId: 'e2e-local-sku-0001',
              sourceSkuId: 'e2e-source-sku-0001',
              productTitle: 'e2e 商品',
              skuName: '红色 / L',
              quantity: 2,
              expectedPrice: 9.9,
            },
          ],
          events: [],
          logistics: [],
        }),
      ),
    });
  });
}

/** 断点口径：<768 移动模式（底部导航，无侧栏汉堡）；≥768 平板/桌面（侧栏，无底部导航）。 */
const MOBILE_WIDTHS = [375, 767];
const DESKTOP_WIDTHS = [768, 769, 1440];

for (const width of MOBILE_WIDTHS) {
  test.describe(`@round124 ${width}px 移动模式：仅底部导航`, () => {
    test.use({ viewport: { width, height: 812 } });

    test(`底部导航可见且侧栏汉堡不可见（${width}px）`, async ({ admin, page }) => {
      await mockMobileHomeApis(page);
      await admin.goto('/m/home');
      await expect(page.getByTestId('tm-mobile-tabbar')).toBeVisible();
      await expect(page.locator('.ant-pro-global-header-collapsed-button')).toBeHidden();
      await expectNoRootOverflow(page);
    });
  });
}

for (const width of DESKTOP_WIDTHS) {
  test.describe(`@round124 ${width}px 平板/桌面模式：仅侧栏`, () => {
    test.use({ viewport: { width, height: 900 } });

    test(`底部导航不渲染且侧栏可见（${width}px）`, async ({ admin, page }) => {
      await mockMobileHomeApis(page);
      await admin.goto('/dashboard/product-operations');
      await expect(page.getByText('运营总览').first()).toBeVisible();
      await expect(page.getByTestId('tm-mobile-tabbar')).toHaveCount(0);
      await expect(page.locator('.ant-pro-global-header-collapsed-button')).toHaveCount(0);
      await expect(page.locator('.ant-pro-sider').first()).toBeVisible();
      await expectNoRootOverflow(page);
    });
  });
}

test.describe('@round124 采购单详情来源销售订单显示订单号', () => {
  test('概览与明细行展示订单号并链接到订单详情', async ({ admin, page }) => {
    await mockPurchaseOrderDetail(page);
    await admin.goto(`/procurement/orders/${E2E_PO_ID}`);

    const overviewLink = page.locator(`a[href*="/orders/${SALES_ORDER_ID}"]`, {
      hasText: SALES_ORDER_NO,
    });
    await expect(overviewLink.first()).toBeVisible();
    // 概览 + 明细行两处都应展示订单号而非 UUID 前缀
    await expect(overviewLink).toHaveCount(2);
    await expect(page.getByText(SALES_ORDER_ID.slice(0, 8))).toHaveCount(0);
  });
});
