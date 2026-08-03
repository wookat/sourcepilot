import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { E2E_PRODUCT_ID } from '../mocks/product.fixture';

async function mockPublicationSkus(page: import('@playwright/test').Page) {
  await page.route(`**/api/v1/products/${E2E_PRODUCT_ID}/publication-skus*`, async (route) => {
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
              publicationSkuId: 'e2e-pubsku-r69f',
              publicationId: 'e2e-pub-r69f',
              productSkuId: 'e2e-sku-1',
              shopId: 'e2e-shop-douyin',
              shopName: 'e2e 抖店旗舰店',
              platform: 'douyin_shop',
              externalProductId: 'ext-prod-1',
              externalSkuId: 'ext-sku-1',
              skuCode: 'SKU-R69F',
              platformStock: 5,
              inventorySyncCapability: 'available',
              bindStatus: 'matched',
            },
          ],
        }),
      ),
    });
  });
}

test.describe('@round69-raw-enum-followup 库存同步弹窗平台名收口', () => {
  test('it should map platform enum in inventory sync modal note', async ({ admin, page }) => {
    await mockPublicationSkus(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    await admin.goto(`/product/drafts/${E2E_PRODUCT_ID}?tab=inventory`);
    await page.getByRole('button', { name: '同步库存' }).first().click();
    const note = page.locator('.product-draft-inventory__modal-note', { hasText: '平台：' });
    await expect(note).toBeVisible();
    await expect(note).toContainText('平台：抖店');
    await expect(note).not.toContainText('douyin_shop');
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
