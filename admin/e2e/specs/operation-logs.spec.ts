import { test, expect } from '../fixtures/admin.fixture';
import { expectNoRootOverflow } from '../utils/assertions';

test.describe('@p1 操作日志页', () => {
  test('渲染列表、无写请求、无根节点溢出', async ({ admin, page }) => {
    await admin.goto('/system/operation-logs');
    await expect(page.getByText('操作日志').first()).toBeVisible();
    await expect(page.getByText('e2e-admin').first()).toBeVisible();
    await expect(page.getByText('e2e-operator').first()).toBeVisible();
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('筛选操作人写入 URL 深链，刷新后恢复筛选', async ({ admin, page }) => {
    await admin.goto('/system/operation-logs');
    await expect(page.getByText('e2e-admin').first()).toBeVisible();

    await page.getByRole('textbox', { name: '用户 :' }).fill('e2e-operator');
    await page.getByRole('button', { name: /查\s*询/ }).click();

    await expect(page).toHaveURL(/username=e2e-operator/);
    await expect(page.getByText('e2e-admin')).toHaveCount(0);
    await expect(page.getByText('e2e-operator').first()).toBeVisible();

    await page.reload();
    await expect(page).toHaveURL(/username=e2e-operator/);
    await expect(page.getByText('e2e-operator').first()).toBeVisible();
    await expect(page.getByText('e2e-admin')).toHaveCount(0);
  });

  test('URL 深链直接打开即应用筛选', async ({ admin, page }) => {
    await admin.goto('/system/operation-logs?resource=procurement');
    await expect(page.getByText('e2e-po-1').first()).toBeVisible();
    await expect(page.getByText('e2e-admin')).toHaveCount(0);
    await expect(page).toHaveURL(/resource=procurement/);
  });

  test('375px 视口无横向溢出', async ({ admin, page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await admin.goto('/system/operation-logs');
    await expect(page.getByText('操作日志').first()).toBeVisible();
    await expectNoRootOverflow(page);
  });
});
