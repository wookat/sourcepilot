import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';

const PENDING_TASK_1 = 'e2e-optask-pending-1';
const PENDING_TASK_2 = 'e2e-optask-pending-2';
const APPROVED_TASK = 'e2e-optask-approved-1';

function taskSummary(id: string, status: string, title: string) {
  return {
    id,
    sourceType: 'manual',
    taskType: 'title_optimize',
    platform: 'douyin',
    title,
    summary: 'e2e 摘要',
    status,
    priority: 'medium',
    revision: 3,
    latestDraftVersion: 2,
    createdBy: 'e2e-user',
    createdAt: '2026-08-01T02:03:04Z',
    updatedAt: '2026-08-01T02:03:04Z',
  };
}

function taskDetail(id: string, status: string, title: string) {
  const pending = status === 'pending_review';
  return {
    ...taskSummary(id, status, title),
    payload: {},
    latestDraft: {
      draftId: `${id}-draft`,
      draftVersion: 2,
      payloadHash: `hash-${id}`,
      status: pending ? 'pending_review' : 'approved',
      createdAt: '2026-08-01T02:03:04Z',
      updatedAt: '2026-08-01T02:03:04Z',
    },
    allowedActions: {
      canEditDraft: false,
      canApprove: pending,
      canReject: pending,
      canExecute: !pending,
      canRetry: false,
      canCancel: true,
    },
  };
}

async function mockOperationTasks(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/operation-tasks**', async (route) => {
    const request = route.request();
    if (request.method() !== 'GET') {
      await route.fallback();
      return;
    }
    const path = new URL(request.url()).pathname;
    let body: unknown = null;
    if (path === '/api/v1/operation-tasks') {
      body = ok({
        items: [
          taskSummary(PENDING_TASK_1, 'pending_review', 'e2e 待审核任务一'),
          taskSummary(PENDING_TASK_2, 'pending_review', 'e2e 待审核任务二'),
          taskSummary(APPROVED_TASK, 'approved', 'e2e 已批准任务'),
        ],
        hasMore: false,
        limit: 20,
      });
    } else if (path === `/api/v1/operation-tasks/${PENDING_TASK_1}`) {
      body = ok(taskDetail(PENDING_TASK_1, 'pending_review', 'e2e 待审核任务一'));
    } else if (path === `/api/v1/operation-tasks/${PENDING_TASK_2}`) {
      body = ok(taskDetail(PENDING_TASK_2, 'pending_review', 'e2e 待审核任务二'));
    } else if (path === `/api/v1/operation-tasks/${APPROVED_TASK}`) {
      body = ok(taskDetail(APPROVED_TASK, 'approved', 'e2e 已批准任务'));
    }
    if (!body) {
      await route.fallback();
      return;
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
  });
}

test.describe('@round63 刊登任务 tab 深链修复', () => {
  test('深链 ?tab=tasks 直接进入子任务 tab 且 URL 不被规范化回批次', async ({ admin, page }) => {
    await admin.goto('/product/publish-tasks?tab=tasks');
    await expect(page.getByRole('tab', { name: '子任务', selected: true })).toBeVisible();
    await expect(page.getByText('刊登记录')).toBeVisible();
    await expect.poll(() => new URL(page.url()).searchParams.get('tab')).toBe('tasks');
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('从默认批次 tab 切到子任务再切回，activeTab 与 URL 保持一致', async ({ admin, page }) => {
    await admin.goto('/product/publish-tasks');
    await expect(page.getByRole('tab', { name: '刊登批次', selected: true })).toBeVisible();

    await page.getByRole('tab', { name: '子任务' }).click();
    await expect(page.getByRole('tab', { name: '子任务', selected: true })).toBeVisible();
    await expect(page.getByText('刊登记录')).toBeVisible();
    await expect.poll(() => new URL(page.url()).searchParams.get('tab')).toBe('tasks');

    await page.getByRole('tab', { name: '刊登批次' }).click();
    await expect(page.getByRole('tab', { name: '刊登批次', selected: true })).toBeVisible();
    await expect.poll(() => new URL(page.url()).searchParams.get('tab')).toBeNull();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});

test.describe('@round63 运营任务批量批准/驳回', () => {
  test('批量批准仅对待审核任务逐个提交并汇总成功', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'batch-approve',
      method: 'POST',
      path: /\/api\/v1\/operation-tasks\/e2e-optask-pending-\d\/approve$/,
      response: ok(taskDetail(PENDING_TASK_1, 'approved', 'e2e 待审核任务一')),
    });
    await mockOperationTasks(page);
    await admin.goto('/ops/task-center/operation-tasks');
    await expect(page.getByText('e2e 待审核任务一')).toBeVisible();

    // 可写角色批量操作条常驻
    await expect(page.getByRole('button', { name: /批量批准（0）/ })).toBeVisible();
    await expect(page.getByRole('button', { name: /批量驳回（0）/ })).toBeVisible();

    // 全选（含已批准任务），计数只统计待审核
    await page.locator('.ant-table-thead .ant-checkbox-input').check();
    await expect(page.getByText(/已选 3 项，其中待审核 2 项/)).toBeVisible();
    const approveBtn = page.getByRole('button', { name: /批量批准（2）/ });
    await expect(approveBtn).toBeVisible();
    await approveBtn.click();

    const dialog = page.getByRole('dialog', { name: /批量批准（2 个任务）/ });
    await expect(dialog).toBeVisible();
    await dialog.getByLabel('批准说明').fill('e2e 批量批准');
    await dialog.getByRole('button', { name: '确认批量批准' }).click();

    await admin.writeGuard.expectRequestCount('batch-approve', 2);
    const calls = admin.writeGuard.calls('batch-approve');
    expect(calls.map((c) => c.path).sort()).toEqual([
      `/api/v1/operation-tasks/${PENDING_TASK_1}/approve`,
      `/api/v1/operation-tasks/${PENDING_TASK_2}/approve`,
    ]);
    for (const call of calls) {
      expect(call.postDataJSON).toMatchObject({
        draftVersion: 2,
        reason: 'e2e 批量批准',
        expectedTaskRevision: 3,
      });
    }
    await expect(page.getByText('已批准 2 个任务')).toBeVisible();
  });

  test('批量驳回取消时不发出任何写请求', async ({ admin, page }) => {
    await mockOperationTasks(page);
    await admin.goto('/ops/task-center/operation-tasks');
    await expect(page.getByText('e2e 待审核任务一')).toBeVisible();

    await page.locator('.ant-table-thead .ant-checkbox-input').check();
    await page.getByRole('button', { name: /批量驳回（2）/ }).click();
    const dialog = page.getByRole('dialog', { name: /批量驳回（2 个任务）/ });
    await expect(dialog).toBeVisible();
    await dialog.getByRole('button', { name: '取 消' }).click();
    await expect(dialog).toBeHidden();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('readonly 角色不显示批量操作入口与选择列', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({ id: 'e2e-readonly', username: 'e2e-readonly', role: 'readonly', status: 'active', permissions: [] }),
        ),
      });
    });
    await mockOperationTasks(page);
    await admin.goto('/ops/task-center/operation-tasks');
    await expect(page.getByText('e2e 待审核任务一')).toBeVisible();
    await expect(page.locator('.ant-table-thead .ant-checkbox-input')).toHaveCount(0);
    await expect(page.getByRole('button', { name: /批量批准/ })).toHaveCount(0);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
