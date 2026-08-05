import type { Page } from '@playwright/test';
import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';

async function useReadonlyProfile(page: Page) {
  await page.route('**/api/v1/auth/profile', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
    });
  });
}

const failureRow = {
  id: 'e2e-failure-1',
  taskType: 'collect',
  sourceTable: 'collect_tasks',
  sourceId: 'e2e-collect-1',
  title: 'e2e 采集失败任务',
  platform: '1688',
  status: 'failed',
  normalizedStatus: 'failed',
  retryable: true,
  ignored: false,
  handled: false,
  errorMessage: 'url is not supported by source "1688"',
  errorCode: 'UNSUPPORTED_URL',
  retryCount: 1,
  createdAt: '2026-08-01T10:00:00Z',
  updatedAt: '2026-08-01T10:01:00Z',
};

const failuresSummary = {
  total: 1,
  retryable: 1,
  deadLetter: 0,
  byTaskType: { collect: 1 },
  byCategory: {},
};

async function routeFailures(page: Page) {
  await page.route('**/api/v1/task-center/failures?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ list: [failureRow], total: 1, summary: failuresSummary })),
    });
  });
}

const aiBatchRow = {
  id: 'e2e-ai-batch-1',
  batchNo: 'B-e2e-1',
  operationType: 'title_optimize',
  status: 'partial_success',
  productCount: 2,
  taskCount: 2,
  successCount: 1,
  failedCount: 1,
  skippedCount: 0,
  createdAt: '2026-08-01T10:00:00Z',
};

async function routeAiBatches(page: Page) {
  await page.route('**/api/v1/ai/batches?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ list: [aiBatchRow], pagination: { page: 1, pageSize: 20, total: 1, totalPages: 1 } })),
    });
  });
  await page.route('**/api/v1/ai/batches/e2e-ai-batch-1/tasks?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        ok({
          kind: 'ai_tasks',
          list: [
            {
              id: 'e2e-ai-task-1',
              taskType: 'title_optimize',
              status: 'failed',
              productId: 'e2e-product-1',
              errorMessage: 'AI_INVALID_KEY: 401 unauthorized from upstream',
            },
          ],
          pagination: { page: 1, pageSize: 20, total: 1, totalPages: 1 },
        }),
      ),
    });
  });
}

test.describe('R86 P2 readonly 写入口 UI 收口', () => {
  test('失败中心 readonly 直达 URL 显示统一语义页，无写操作入口', async ({ page, admin }) => {
    await useReadonlyProfile(page);
    await routeFailures(page);
    await admin.goto('/ops/task-center/failures');
    await expect(page.getByText('暂无访问权限')).toBeVisible();
    await expect(page.getByRole('button', { name: '返回工作台' })).toBeVisible();
    await expect(page.getByRole('button', { name: '批量重试' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: '批量忽略' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: '生成告警' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: '重试', exact: true })).toHaveCount(0);
    await expect(page.getByRole('button', { name: '更多' })).toHaveCount(0);
  });

  test('失败中心 admin 仍显示批量与行内写操作', async ({ page, admin }) => {
    await routeFailures(page);
    await admin.goto('/ops/task-center/failures');
    await expect(page.getByText('e2e 采集失败任务')).toBeVisible();
    await expect(page.getByRole('button', { name: '批量重试' })).toBeVisible();
    await expect(page.getByRole('button', { name: '重试', exact: true }).first()).toBeVisible();
  });

  test('AI 批次 readonly 隐藏重试失败/应用结果，仍可看详情与子任务', async ({ page, admin }) => {
    await useReadonlyProfile(page);
    await routeAiBatches(page);
    await admin.goto('/ai/batches');
    await expect(page.getByText('B-e2e-1')).toBeVisible();
    await expect(page.getByText('重试失败', { exact: true })).toHaveCount(0);
    await expect(page.getByText('应用结果', { exact: true })).toHaveCount(0);
    await expect(page.getByText('详情', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('子任务', { exact: true }).first()).toBeVisible();
  });

  test('AI 批次子任务抽屉：失败原因中文化且不崩溃', async ({ page, admin }) => {
    await routeAiBatches(page);
    await admin.goto('/ai/batches');
    await expect(page.getByText('B-e2e-1')).toBeVisible();
    await page.getByText('子任务', { exact: true }).first().click();
    await expect(page.getByText('title_optimize')).toBeVisible();
    await expect(page.getByText('API Key 无效或未授权', { exact: false }).first()).toBeVisible();
    await expect(page.getByText('Something went wrong')).toHaveCount(0);
  });

  test('AI 图片任务 readonly 隐藏新建任务与快捷模板', async ({ page, admin }) => {
    await useReadonlyProfile(page);
    await admin.goto('/ai/image-tasks');
    await expect(page.getByText('AI 图片任务').first()).toBeVisible();
    await expect(page.getByRole('button', { name: '新建任务' })).toHaveCount(0);
    await expect(page.getByText('快捷模板')).toHaveCount(0);
  });
});
