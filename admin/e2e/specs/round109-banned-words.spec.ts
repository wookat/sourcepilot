import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import { E2E_PRODUCT_ID } from '../mocks/product.fixture';
import { e2eBannedWordCategories } from '../mocks/banned-words';

test.describe('@round109 违禁词库设置页', () => {
  test('展示预置/自定义词与分类启停开关', async ({ admin, page }) => {
    await admin.goto('/settings/banned-words');
    await expect(page.getByText('分类启停')).toBeVisible();
    await expect(page.getByRole('cell', { name: '广告法极限词' })).toBeVisible();
    await expect(page.getByRole('switch', { name: '启停分类 医疗功效词' })).toBeVisible();
    await expect(page.getByRole('cell', { name: '最佳', exact: true })).toBeVisible();
    await expect(page.getByRole('cell', { name: '预置', exact: true })).toBeVisible();
    await expect(page.getByRole('cell', { name: '祖传', exact: true })).toBeVisible();
  });

  test('新增自定义违禁词发起 POST 且成功后刷新列表', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'createBannedWord',
      method: 'POST',
      path: /^\/api\/v1\/banned-words$/,
      response: ok({
        id: 'e2e-banned-word-new',
        word: '全网首发',
        category: 'custom',
        level: 'forbidden',
        isPreset: false,
        enabled: true,
      }),
    });
    await admin.goto('/settings/banned-words');
    await page.getByRole('button', { name: '新增自定义违禁词' }).click();
    const dialog = page.getByRole('dialog', { name: '新增自定义违禁词' });
    await dialog.getByLabel('违禁词').fill('全网首发');
    await dialog.getByRole('button', { name: '确 定' }).click();
    await admin.writeGuard.expectRequestCount('createBannedWord', 1);
    const [call] = admin.writeGuard.calls('createBannedWord');
    expect(call.postDataJSON).toMatchObject({ word: '全网首发', level: 'forbidden' });
  });

  test('分类停用发起 PUT categories 请求', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'toggleCategory',
      method: 'PUT',
      path: /^\/api\/v1\/banned-words\/categories\/medical$/,
      response: ok({ ...e2eBannedWordCategories[2], enabled: false }),
    });
    await admin.goto('/settings/banned-words');
    await page.getByRole('switch', { name: '启停分类 医疗功效词' }).click();
    await admin.writeGuard.expectRequestCount('toggleCategory', 1);
    const [call] = admin.writeGuard.calls('toggleCategory');
    expect(call.postDataJSON).toEqual({ enabled: false });
  });

  test('预置词删除按钮禁用', async ({ admin, page }) => {
    await admin.goto('/settings/banned-words');
    const presetRow = page.getByRole('row', { name: /最佳/ });
    await expect(presetRow.getByRole('button', { name: '删除' })).toBeDisabled();
    const customRow = page.getByRole('row', { name: /祖传/ });
    await expect(customRow.getByRole('button', { name: '删除' })).toBeEnabled();
  });

  test('readonly 角色新增与启停控件禁用', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
      });
    });
    await admin.goto('/settings/banned-words');
    await expect(page.getByRole('button', { name: '新增自定义违禁词' })).toBeDisabled();
    await expect(page.getByRole('switch', { name: '启停分类 医疗功效词' })).toBeDisabled();
  });
});

test.describe('@round109 草稿详情合规检测', () => {
  test('发布检查页展示合规检测入口并高亮命中', async ({ admin, page }) => {
    await admin.goto(`/product/drafts/${E2E_PRODUCT_ID}?tab=readiness`);
    await expect(page.getByText('合规检测（违禁词）')).toBeVisible();
    await page.getByRole('button', { name: '开始合规检测' }).click();
    await expect(page.getByText('禁止级命中').first()).toBeVisible();
    await expect(page.getByText('命中明细')).toBeVisible();
    await expect(page.getByText('「最佳」')).toBeVisible();
    // 高亮片段：mark 元素包含命中词
    await expect(page.locator('mark', { hasText: '最佳' })).toBeVisible();
    await expect(page.locator('mark', { hasText: '祖传' })).toBeVisible();
    await expect(page.getByText('可改为「优选」「精选」等非绝对化表述。')).toBeVisible();
  });
});

test.describe('@round109 草稿列表批量违禁词检测', () => {
  test('批量检测发起 check-batch 请求并展示结果', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'batchBannedWords',
      method: 'POST',
      path: /^\/api\/v1\/products\/banned-words\/check-batch$/,
      response: ok({
        list: [
          {
            productId: E2E_PRODUCT_ID,
            status: 'blocked',
            statusLabel: '存在禁止级违禁词',
            forbiddenCount: 1,
            warningCount: 0,
            hits: [
              {
                word: '最佳',
                field: 'title',
                fieldLabel: '商品标题',
                category: 'ad_extreme',
                categoryLabel: '广告法极限词',
                level: 'forbidden',
                levelLabel: '禁止',
                positions: [{ start: 2, end: 4 }],
              },
            ],
          },
        ],
      }),
    });
    await admin.goto('/product/drafts');
    await page.getByRole('checkbox', { name: /Select all/i }).or(page.locator('.ant-table-thead input[type="checkbox"]')).first().check();
    await page.getByRole('button', { name: '批量违禁词检测' }).click();
    const drawer = page.getByRole('dialog').or(page.locator('.ant-drawer-content'));
    await expect(drawer.getByText('批量违禁词检测').first()).toBeVisible();
    await page.getByRole('button', { name: '开始检测' }).click();
    await admin.writeGuard.expectRequestCount('batchBannedWords', 1);
    const [call] = admin.writeGuard.calls('batchBannedWords');
    expect(call.postDataJSON).toEqual({ productIds: [E2E_PRODUCT_ID] });
    await expect(page.locator('.ant-drawer .ant-tag', { hasText: '禁止级命中' })).toBeVisible();
    await expect(page.locator('.ant-drawer').getByText('最佳')).toBeVisible();
  });
});
