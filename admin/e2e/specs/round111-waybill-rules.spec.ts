import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import {
  E2E_PRINT_ORDER_ID,
  E2E_RULE_ID,
  E2E_TEMPLATE_A4_ID,
  E2E_TEMPLATE_SMALL_ID,
  e2eShippingRules,
  e2eWaybillTemplates,
} from '../mocks/waybill';

test.describe('@round111 面单模板管理页', () => {
  test('展示预置/自定义模板、尺寸与显示字段', async ({ admin, page }) => {
    await admin.goto('/settings/waybill-templates');
    await expect(page.getByRole('cell', { name: /标准面单 100×180/ })).toBeVisible({ timeout: 20000 });
    await expect(page.getByRole('cell', { name: '100×180mm 标准面单' })).toBeVisible();
    await expect(page.getByRole('cell', { name: '100×150mm 小号面单' })).toBeVisible();
    await expect(page.getByRole('cell', { name: 'A4 一联单', exact: true }).first()).toBeVisible();
    await expect(page.getByText('默认', { exact: true })).toBeVisible();
    await expect(page.getByText('物流商 logo 位').first()).toBeVisible();
  });

  test('新增模板发起 POST 且携带字段勾选与页眉页脚', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'createTemplate',
      method: 'POST',
      path: /^\/api\/v1\/waybill-templates$/,
      response: ok({ ...e2eWaybillTemplates[2], id: 'e2e-template-new', name: '新模板' }),
    });
    await admin.goto('/settings/waybill-templates');
    await page.getByRole('button', { name: '新增模板' }).click();
    const dialog = page.getByRole('dialog', { name: '新增面单模板' });
    await dialog.getByLabel('模板名称').fill('新模板');
    await dialog.getByLabel('页眉文本（可选）').fill('测试页眉');
    await dialog.getByRole('button', { name: '确 定' }).click();
    await admin.writeGuard.expectRequestCount('createTemplate', 1);
    const [call] = admin.writeGuard.calls('createTemplate');
    expect(call.postDataJSON).toMatchObject({
      name: '新模板',
      sizeCode: '100x180',
      showRecipient: true,
      showCarrierLogo: false,
      headerText: '测试页眉',
    });
  });

  test('预置模板删除禁用，自定义模板可删除', async ({ admin, page }) => {
    await admin.goto('/settings/waybill-templates');
    const presetRow = page.getByRole('row', { name: /标准面单 100×180/ });
    await expect(presetRow.getByRole('button', { name: '删除' })).toBeDisabled();
    const customRow = page.getByRole('row', { name: /A4 一联单/ });
    await expect(customRow.getByRole('button', { name: '删除' })).toBeEnabled();
  });

  test('设为默认发起 PUT isDefault 请求', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'setDefault',
      method: 'PUT',
      path: new RegExp(`^/api/v1/waybill-templates/${E2E_TEMPLATE_SMALL_ID}$`),
      response: ok({ ...e2eWaybillTemplates[1], isDefault: true }),
    });
    await admin.goto('/settings/waybill-templates');
    const row = page.getByRole('row', { name: /小号面单 100×150/ });
    await row.getByRole('button', { name: '设为默认' }).click();
    await admin.writeGuard.expectRequestCount('setDefault', 1);
    const [call] = admin.writeGuard.calls('setDefault');
    expect(call.postDataJSON).toEqual({ isDefault: true });
  });

  test('readonly 角色新增/编辑禁用', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
      });
    });
    await admin.goto('/settings/waybill-templates');
    await expect(page.getByRole('button', { name: '新增模板' })).toBeDisabled();
  });
});

test.describe('@round111 发货规则管理页', () => {
  test('展示规则条件、优先级与推荐物流商', async ({ admin, page }) => {
    await admin.goto('/settings/shipping-rules');
    await expect(page.getByRole('cell', { name: '江浙沪标准件走中通' })).toBeVisible();
    await expect(page.getByText('省份：上海、江苏、浙江')).toBeVisible();
    await expect(page.getByText('重量：≤5kg')).toBeVisible();
    await expect(page.getByText('金额：≥500 元')).toBeVisible();
    await expect(page.getByRole('cell', { name: '中通快递' })).toBeVisible();
  });

  test('新增规则发起 POST 携带条件与物流商', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'createRule',
      method: 'POST',
      path: /^\/api\/v1\/shipping-rules$/,
      response: ok({ ...e2eShippingRules[0], id: 'e2e-rule-new', name: '新规则' }),
    });
    await admin.goto('/settings/shipping-rules');
    await page.getByRole('button', { name: '新增规则' }).click();
    const dialog = page.getByRole('dialog', { name: '新增发货规则' });
    await dialog.getByLabel('规则名称').fill('新规则');
    await dialog.getByLabel('推荐物流商').click();
    await page.locator('.ant-select-item-option', { hasText: '顺丰' }).first().click();
    await dialog.getByRole('button', { name: '确 定' }).click();
    await admin.writeGuard.expectRequestCount('createRule', 1);
    const [call] = admin.writeGuard.calls('createRule');
    expect(call.postDataJSON).toMatchObject({ name: '新规则', priority: 10, carrierCode: 'sf' });
  });

  test('启停规则发起 PUT enabled 请求', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'toggleRule',
      method: 'PUT',
      path: new RegExp(`^/api/v1/shipping-rules/${E2E_RULE_ID}$`),
      response: ok({ ...e2eShippingRules[0], enabled: false }),
    });
    await admin.goto('/settings/shipping-rules');
    const row = page.getByRole('row', { name: /江浙沪标准件走中通/ });
    await row.getByRole('switch').click();
    await admin.writeGuard.expectRequestCount('toggleRule', 1);
    const [call] = admin.writeGuard.calls('toggleRule');
    expect(call.postDataJSON).toEqual({ enabled: false });
  });
});

test.describe('@round111 打印页按模板渲染', () => {
  test('默认模板渲染页眉页脚与收发件人', async ({ admin, page }) => {
    await admin.goto(`/orders/print?ids=${E2E_PRINT_ORDER_ID}`);
    await expect(page.getByRole('heading', { name: /拣货 \/ 发货单/ })).toBeVisible();
    await expect(page.getByText('E2E 页眉文本')).toBeVisible();
    await expect(page.getByText('E2E 页脚文本')).toBeVisible();
    await expect(page.getByText('发件人', { exact: true })).toBeVisible();
    await expect(page.getByText('收件人', { exact: true })).toBeVisible();
    await expect(page.getByText('非电子面单').first()).toBeVisible();
  });

  test('切换 100×150 模板后隐藏发件人/备注并显示 logo 位', async ({ admin, page }) => {
    await admin.goto(`/orders/print?ids=${E2E_PRINT_ORDER_ID}&templateId=${E2E_TEMPLATE_SMALL_ID}`);
    await expect(page.getByText('物流商 logo 位（接入电子面单后展示）')).toBeVisible();
    await expect(page.getByText('发件人', { exact: true })).toBeHidden();
    await expect(page.getByText('备注', { exact: true })).toBeHidden();
    await expect(page.getByText('收件人', { exact: true })).toBeVisible();
  });

  test('切换 A4 一联单模板正常渲染', async ({ admin, page }) => {
    await admin.goto(`/orders/print?ids=${E2E_PRINT_ORDER_ID}&templateId=${E2E_TEMPLATE_A4_ID}`);
    await expect(page.getByText('商品明细（拣货）')).toBeVisible();
    await expect(page.getByText('E2E 测试商品')).toBeVisible();
  });

  test('标记已打单发起 POST /orders/print/mark，不改发货状态', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'markPrinted',
      method: 'POST',
      path: /^\/api\/v1\/orders\/print\/mark$/,
      response: ok({ marked: 1 }),
    });
    await admin.goto(`/orders/print?ids=${E2E_PRINT_ORDER_ID}`);
    await page.getByRole('button', { name: '标记已打单' }).click();
    await admin.writeGuard.expectRequestCount('markPrinted', 1);
    const [call] = admin.writeGuard.calls('markPrinted');
    expect(call.postDataJSON).toEqual({ ids: [E2E_PRINT_ORDER_ID] });
    await expect(page.getByText('已标记 1 单为已打单（不影响发货状态）')).toBeVisible();
  });
});

test.describe('@round111 批量发货按规则推荐', () => {
  test('按规则推荐展示命中结果并可填入缺失行（可手动覆盖）', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'recommend',
      method: 'POST',
      path: /^\/api\/v1\/orders\/shipping-recommendations$/,
      response: ok({
        items: [
          {
            key: 'SO-E2E-0001',
            matched: true,
            ruleId: E2E_RULE_ID,
            ruleName: '江浙沪标准件走中通',
            carrierCode: 'zto',
            carrierName: '中通快递',
          },
          {
            key: 'SO-E2E-0002',
            matched: false,
            message: '没有命中任何发货规则，可手动选择物流商',
          },
        ],
      }),
    });
    await admin.goto('/orders/list');
    await page.getByRole('button', { name: '批量发货' }).click();
    const dialog = page.getByRole('dialog', { name: '批量发货' });
    await dialog
      .getByRole('textbox')
      .last()
      .fill('SO-E2E-0001 SF000111\nSO-E2E-0002 SF000222');
    await dialog.getByRole('button', { name: '按规则推荐物流商' }).click();
    await admin.writeGuard.expectRequestCount('recommend', 1);
    await expect(dialog.getByText('命中规则：江浙沪标准件走中通')).toBeVisible();
    await expect(dialog.getByText('没有命中任何发货规则，可手动选择物流商')).toBeVisible();
    await dialog.getByRole('button', { name: '将推荐填入缺失行' }).click();
    await expect(dialog.getByRole('textbox').last()).toHaveValue(
      'SO-E2E-0001 SF000111 zto\nSO-E2E-0002 SF000222',
    );
  });
});
