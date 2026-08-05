import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { expectNoRootOverflow } from '../utils/assertions';

const LONG_REASON =
  '执行失败（已重试 3 次）：生成采购单被阻断：订单行未匹配本地 SKU，请先完成 SKU 匹配；' +
  '请在订单详情完成本地 SKU 匹配并绑定主货源后，再从执行日志手动重试该记录。';

const longReasonLog = {
  id: 'e2e-r122-log-long',
  tenantId: 1,
  ruleId: 'rule-r122',
  ruleName: '付款后自动生成采购单',
  orderId: 'e2e-r122-order-1',
  orderNo: 'SO-R122-P2-1',
  triggerEvent: 'order_paid',
  action: 'generate_procurement',
  status: 'failed',
  reason: LONG_REASON,
  attempts: 3,
  createdAt: '2026-08-02T00:00:00Z',
  updatedAt: '2026-08-02T00:00:00Z',
};

test.describe('@round122 P2① 执行日志结果列移动端不撑高', () => {
  test('375 视口下长原因单行省略且无横向溢出', async ({ admin, page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await page.route('**/api/v1/order-automation-logs**', async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ items: [longReasonLog], total: 1, page: 1, pageSize: 20, totalPages: 1 })),
      });
    });
    await admin.goto('/orders/automation-logs');
    const cell = page.locator('td.ant-table-cell-ellipsis', { hasText: '执行失败（已重试 3 次）' });
    await expect(cell).toBeAttached({ timeout: 30000 });
    await cell.scrollIntoViewIfNeeded();
    await expect(cell).toBeVisible();
    const box = await cell.boundingBox();
    expect(box?.height ?? 0).toBeLessThan(120);
    await expectNoRootOverflow(page);
  });
});

test.describe('@round122 P2④ /purchase/orders 路由别名', () => {
  test('访问 /purchase/orders 重定向到 /procurement/orders', async ({ admin, page }) => {
    await admin.goto('/purchase/orders');
    await expect(page).toHaveURL(/\/procurement\/orders/, { timeout: 30000 });
    await expect(page.getByRole('menuitem', { name: /采购单/ }).first()).toBeVisible();
  });
});

const enabledMissingRule = {
  id: 'rule-r122-missing',
  name: 'e2e-已启用但模板被删的规则',
  node: 'shipped',
  templateId: 'tpl-gone-1',
  templateName: '',
  templateMissing: true,
  enabled: true,
  platforms: [],
  shopIds: [],
};

test.describe('@round122 #256 P2② 已删模板两态', () => {
  test('已启用规则显示「已失效」并允许停用', async ({ admin, page }) => {
    await page.route('**/api/v1/customer/buyer-message-rules**', async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ list: [enabledMissingRule], canWrite: true })),
      });
    });
    admin.writeGuard.allow({
      operation: 'disableRule',
      method: 'PUT',
      path: /\/api\/v1\/customer\/buyer-message-rules\/rule-r122-missing$/,
      response: ok({ ...enabledMissingRule, enabled: false }),
    });
    await admin.goto('/customer/buyer-messages?tab=rules');
    await page.getByRole('tab', { name: /节点规则/ }).click();
    const row = page.locator('.ant-table-tbody tr', { hasText: 'e2e-已启用但模板被删的规则' });
    await expect(row).toBeVisible({ timeout: 30000 });
    await expect(row.getByText('已失效')).toBeVisible();
    const toggle = row.getByRole('switch');
    await expect(toggle).toBeEnabled();
    await toggle.click();
    await admin.writeGuard.expectRequestCount('disableRule', 1);
  });
});
