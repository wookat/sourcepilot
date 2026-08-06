import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import { E2E_PRODUCT_ID } from '../mocks/product.fixture';

test.describe('@product-draft R134 P2 草稿详情只读门控与来源枚举', () => {
  test('admin 角色显示「保存基础信息」按钮，来源平台显示中文映射', async ({ page, admin }) => {
    await admin.goto(`/product/drafts/${E2E_PRODUCT_ID}?tab=basic`);
    await expect(page.getByRole('button', { name: '保存基础信息' })).toBeVisible();
    // e2eProduct.source = 'custom'，应显示映射后的中文而不是原始枚举
    await expect(page.getByText('自定义链接').first()).toBeVisible();
    await expect(page.locator('.product-draft-header__meta')).not.toContainText('custom');
  });

  test('readonly 角色隐藏「保存基础信息」按钮并提示只读', async ({ page, admin }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
      });
    });
    await admin.goto(`/product/drafts/${E2E_PRODUCT_ID}?tab=basic`);
    await expect(page.getByText('商品核心信息').first()).toBeVisible();
    await expect(page.getByRole('button', { name: '保存基础信息' })).toHaveCount(0);
    await expect(page.getByText('当前账号处于只读模式').first()).toBeVisible();
  });
});
