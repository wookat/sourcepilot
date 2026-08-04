import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';

const TASK_ID = 'e2e-optask-round98-1';

function taskSummary() {
  return {
    id: TASK_ID,
    sourceType: 'manual',
    taskType: 'product_content',
    platform: 'douyin',
    title: 'e2e 运营任务标题',
    summary: 'e2e 摘要',
    status: 'pending_review',
    priority: 'high',
    revision: 3,
    latestDraftVersion: 2,
    latestExecutionStatus: 'succeeded',
    createdBy: 'e2e-user-round98',
    createdAt: '2026-08-01T02:03:04Z',
    updatedAt: '2026-08-02T05:06:07Z',
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
    if (path !== '/api/v1/operation-tasks') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ items: [taskSummary()], hasMore: false, limit: 20 })),
    });
  });
}

test.describe('@round98 运营任务列表列渲染口径', () => {
  test('各列渲染中文语义值，不出现 [object Object]', async ({ admin, page }) => {
    await mockOperationTasks(page);
    await admin.goto('/ops/task-center/operation-tasks');
    await expect(page.getByText('e2e 运营任务标题')).toBeVisible();

    const row = page.locator('.ant-table-tbody tr', { hasText: 'e2e 运营任务标题' });
    await expect(row.getByText('商品内容')).toBeVisible();
    await expect(row.getByText('抖音')).toBeVisible();
    await expect(row.getByText('待审核')).toBeVisible();
    await expect(row.getByText('高', { exact: true })).toBeVisible();
    await expect(row.getByText('v2', { exact: true })).toBeVisible();
    await expect(row.getByText('已完成')).toBeVisible();
    await expect(row.getByText('e2e-user-r…')).toBeVisible();

    await expect(page.locator('body')).not.toContainText('[object Object]');
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
