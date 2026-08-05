import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import {
  E2E_REVIEW_ORDER_HELD_ID,
  E2E_REVIEW_ORDER_PENDING_ID,
  E2E_REVIEW_RULE_ID,
  e2eOrderReviewRules,
} from '../mocks/order-review';

test.describe('@round114 审单规则管理页', () => {
  test('展示规则条件、动作与启停状态', async ({ admin, page }) => {
    await admin.goto('/settings/order-review-rules');
    await expect(page.getByRole('cell', { name: '大额订单人工审核' })).toBeVisible({ timeout: 20000 });
    await expect(page.getByText('金额：≥500 元')).toBeVisible();
    await expect(page.getByText('备注关键词：加急')).toBeVisible();
    await expect(page.getByText('地址关键词：某某区')).toBeVisible();
    await expect(page.getByText('待人工审核', { exact: true })).toBeVisible();
    await expect(page.getByText('挂起拦截', { exact: true })).toBeVisible();
  });

  test('新增规则发起 POST 携带动作与条件', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'createRule',
      method: 'POST',
      path: /^\/api\/v1\/order-review-rules$/,
      response: ok({ ...e2eOrderReviewRules[0], id: 'e2e-review-rule-new', name: '新审单规则' }),
    });
    await admin.goto('/settings/order-review-rules');
    await page.getByRole('button', { name: '新增规则' }).click();
    const dialog = page.getByRole('dialog', { name: '新增审单规则' });
    await dialog.getByLabel('规则名称').fill('新审单规则');
    await dialog.locator('input[id="minAmount"]').fill('300');
    await dialog.getByRole('button', { name: '保 存' }).click();
    await admin.writeGuard.expectRequestCount('createRule', 1);
    const [call] = admin.writeGuard.calls('createRule');
    expect(call.postDataJSON).toMatchObject({
      name: '新审单规则',
      priority: 10,
      action: 'review',
      minAmount: 300,
    });
  });

  test('测试跑发起 dry-run 并展示命中数与样本', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'dryRun',
      method: 'POST',
      path: /^\/api\/v1\/order-review-rules\/dry-run$/,
      response: ok({
        scanned: 120,
        matched: 3,
        samples: [
          { orderId: 'o1', orderNo: 'SO-DR-1', amount: 800, reason: '订单金额 800.00 落入阈值区间' },
        ],
      }),
    });
    await admin.goto('/settings/order-review-rules');
    await page.getByRole('button', { name: '新增规则' }).click();
    const dialog = page.getByRole('dialog', { name: '新增审单规则' });
    await dialog.getByLabel('规则名称').fill('临时规则');
    await dialog.locator('input[id="minAmount"]').fill('500');
    await dialog.getByRole('button', { name: '测试跑（dry-run）' }).click();
    await admin.writeGuard.expectRequestCount('dryRun', 1);
    await expect(dialog.getByText('测试跑结果：扫描最近 120 单，命中 3 单')).toBeVisible();
    await expect(dialog.getByText('SO-DR-1（800）：订单金额 800.00 落入阈值区间')).toBeVisible();
  });

  test('启停规则发起 PUT enabled 请求', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'toggleRule',
      method: 'PUT',
      path: new RegExp(`^/api/v1/order-review-rules/${E2E_REVIEW_RULE_ID}$`),
      response: ok({ ...e2eOrderReviewRules[0], enabled: false }),
    });
    await admin.goto('/settings/order-review-rules');
    const row = page.getByRole('row', { name: /大额订单人工审核/ });
    await row.getByRole('switch').click();
    await admin.writeGuard.expectRequestCount('toggleRule', 1);
    const [call] = admin.writeGuard.calls('toggleRule');
    expect(call.postDataJSON).toEqual({ enabled: false });
  });

  test('readonly 角色新增/编辑禁用', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
      });
    });
    await admin.goto('/settings/order-review-rules');
    await expect(page.getByRole('button', { name: '新增规则' })).toBeDisabled();
  });
});

test.describe('@round114 审单工作台', () => {
  test('展示待审/挂起订单、命中规则与原因', async ({ admin, page }) => {
    await admin.goto('/orders/review');
    await expect(page.getByRole('cell', { name: 'SO-E2E-HELD-1' })).toBeVisible({ timeout: 20000 });
    await expect(page.getByText('已挂起', { exact: true })).toBeVisible();
    await expect(page.getByText('待人工审核', { exact: true })).toBeVisible();
    await expect(page.getByText('黑名单地区挂起（生效）')).toBeVisible();
    await expect(page.getByText('挂起拦截：收货地址含关键词「某某区」')).toBeVisible();
    await expect(page.getByText('待处理共 2 单')).toBeVisible();
  });

  test('单个放行发起 POST approve', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'approve',
      method: 'POST',
      path: /^\/api\/v1\/order-review\/approve$/,
      response: ok({
        total: 1,
        done: 1,
        failed: 0,
        results: [{ orderId: E2E_REVIEW_ORDER_HELD_ID, orderNo: 'SO-E2E-HELD-1', ok: true }],
      }),
    });
    await admin.goto('/orders/review');
    const row = page.getByRole('row', { name: /SO-E2E-HELD-1/ });
    await row.getByRole('button', { name: '放行' }).click();
    await page.getByRole('button', { name: '放 行' }).click();
    await admin.writeGuard.expectRequestCount('approve', 1);
    const [call] = admin.writeGuard.calls('approve');
    expect(call.postDataJSON).toEqual({ orderIds: [E2E_REVIEW_ORDER_HELD_ID] });
    await expect(page.getByText('放行成功 1 单')).toBeVisible();
  });

  test('批量放行发起 POST approve 携带全部选中订单', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'approveBatch',
      method: 'POST',
      path: /^\/api\/v1\/order-review\/approve$/,
      response: ok({
        total: 2,
        done: 2,
        failed: 0,
        results: [
          { orderId: E2E_REVIEW_ORDER_HELD_ID, ok: true },
          { orderId: E2E_REVIEW_ORDER_PENDING_ID, ok: true },
        ],
      }),
    });
    await admin.goto('/orders/review');
    await expect(page.getByRole('cell', { name: 'SO-E2E-HELD-1' })).toBeVisible({ timeout: 20000 });
    await page.getByRole('checkbox', { name: 'Select all' }).check();
    await page.getByRole('button', { name: /批量放行（2）/ }).click();
    await admin.writeGuard.expectRequestCount('approveBatch', 1);
    const [call] = admin.writeGuard.calls('approveBatch');
    expect(call.postDataJSON).toEqual({
      orderIds: [E2E_REVIEW_ORDER_HELD_ID, E2E_REVIEW_ORDER_PENDING_ID],
    });
    await expect(page.getByText('放行成功 2 单')).toBeVisible();
  });

  test('拒绝需确认并发起 POST reject（入取消动线）', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'reject',
      method: 'POST',
      path: /^\/api\/v1\/order-review\/reject$/,
      response: ok({
        total: 1,
        done: 1,
        failed: 0,
        results: [{ orderId: E2E_REVIEW_ORDER_PENDING_ID, orderNo: 'SO-E2E-PEND-1', ok: true }],
      }),
    });
    await admin.goto('/orders/review');
    const row = page.getByRole('row', { name: /SO-E2E-PEND-1/ });
    await row.getByRole('button', { name: '拒绝' }).click();
    await expect(
      page.getByText('拒绝后订单将进入取消动线（订单状态置为已取消），不可继续采购和发货。'),
    ).toBeVisible();
    await page.getByRole('button', { name: '拒绝并取消订单' }).click();
    await admin.writeGuard.expectRequestCount('reject', 1);
    const [call] = admin.writeGuard.calls('reject');
    expect(call.postDataJSON).toEqual({ orderIds: [E2E_REVIEW_ORDER_PENDING_ID] });
    await expect(page.getByText('拒绝成功 1 单')).toBeVisible();
  });

  test('readonly 角色批量操作与行内操作禁用', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
      });
    });
    await admin.goto('/orders/review');
    await expect(page.getByRole('cell', { name: 'SO-E2E-HELD-1' })).toBeVisible({ timeout: 20000 });
    await expect(page.getByRole('button', { name: /批量放行/ })).toBeDisabled();
    const row = page.getByRole('row', { name: /SO-E2E-HELD-1/ });
    await expect(row.getByRole('button', { name: '放行' })).toBeDisabled();
  });
});
