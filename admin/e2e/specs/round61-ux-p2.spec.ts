import { test, expect } from '../fixtures/admin.fixture';
import { ok, fail } from '../mocks/envelope';
import { expectNoRootOverflow } from '../utils/assertions';

const E2E_PO_ID = 'e2e-purchase-order-0001';
const E2E_SUPPLIER_ID = 'e2e-supplier-0001';
const ISO_CREATED_AT = '2026-08-01T02:03:04Z';

async function mockPurchaseOrders(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/procurement/orders?**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        ok({
          items: [
            {
              id: E2E_PO_ID,
              supplierName: 'e2e 测试供应商',
              status: 'draft',
              totalAmount: 12.5,
              currency: 'CNY',
              externalOrderId: '',
              createdAt: ISO_CREATED_AT,
            },
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        }),
      ),
    });
  });
}

async function mockSuppliers(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/suppliers?**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        ok({
          items: [
            {
              id: E2E_SUPPLIER_ID,
              platform: '1688',
              name: 'e2e 已绑定货源供应商',
              status: 'active',
              createdAt: ISO_CREATED_AT,
            },
          ],
          total: 1,
          page: 1,
          pageSize: 20,
        }),
      ),
    });
  });
}

test.describe('@round61-ux-p2 Round61 UX P2 收口', () => {
  test('采购单列表创建时间本地化展示（不透出 ISO UTC 串）', async ({ admin, page }) => {
    await mockPurchaseOrders(page);
    await admin.goto('/procurement/orders');
    await expect(page.getByText('e2e 测试供应商')).toBeVisible();
    // 本地化后为 YYYY-MM-DD HH:mm:ss，不应出现原始 ISO 串
    await expect(page.locator('.ant-table-tbody')).toContainText(/\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}/);
    await expect(page.locator('.ant-table-tbody')).not.toContainText(ISO_CREATED_AT);
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('供应商删除失败提示中文兜底（不透出英文原始串）', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'delete-supplier',
      method: 'DELETE',
      path: new RegExp(`/api/v1/suppliers/${E2E_SUPPLIER_ID}$`),
      response: fail('conflict: supplier has bound sources', 409),
    });
    await mockSuppliers(page);
    await admin.goto('/sourcing/suppliers');
    await expect(page.getByText('e2e 已绑定货源供应商')).toBeVisible();
    await page.locator('.ant-table').getByText('删除', { exact: true }).click();
    await page.getByRole('button', { name: '确 定' }).click();
    await admin.writeGuard.expectRequestCount('delete-supplier', 1);
    await expect(page.getByText('该供应商已绑定商品货源，请先解绑货源后再删除')).toBeVisible();
    await expect(page.getByText('conflict: supplier has bound sources')).toHaveCount(0);
  });

  test('选品任务空态展示 EmptyState 新手引导', async ({ admin, page }) => {
    await page.route('**/api/v1/selection/tasks**', async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ items: [], total: 0, page: 1, pageSize: 20 })),
      });
    });
    await admin.goto('/selection/tasks');
    await expect(page.getByText('暂无选品任务')).toBeVisible();
    await expect(page.getByText(/导入海外候选商品或关键词/)).toBeVisible();
    // 空态按钮直接打开新建选品任务弹窗
    await page.locator('.tm-empty-state').getByRole('button', { name: '新建选品任务' }).click();
    await expect(page.getByRole('dialog', { name: '新建选品任务' })).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('异常工作台面包屑显示「异常工作台」而非「订单详情」', async ({ admin, page }) => {
    // 面包屑依赖菜单数据；使用 admin 角色默认权限（profile 不带 '*' 通配）还原真实菜单
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ id: 'e2e-user', username: 'e2e-user', role: 'admin', status: 'active', permissions: [] })),
      });
    });
    await admin.goto('/orders/exceptions');
    await expect(page.getByText('订单异常工作台').first()).toBeVisible();
    const breadcrumb = page.locator('.ant-breadcrumb');
    await expect(breadcrumb).toContainText('异常工作台');
    await expect(breadcrumb).not.toContainText('订单详情');
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
