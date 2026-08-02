import { test, expect } from '../fixtures/admin.fixture';
import { ok, fail } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import {
  E2E_SELECTION_FAILED_TASK_ID,
  E2E_SELECTION_CANDIDATE_ERROR,
  E2E_SELECTION_TASK_ERROR,
} from '../mocks/selection';

const READONLY_403_MESSAGE = '当前账号为只读权限，无法执行此操作';

async function useReadonlyProfile(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/auth/profile', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
    });
  });
}

test.describe('@selection R57 选品 P2 修复', () => {
  test('task list supports status filter and passes status to API', async ({ admin, page }) => {
    await admin.goto('/selection/tasks');
    await expect(page.getByText('E2E 选品任务')).toBeVisible();
    await expect(page.getByText('E2E 失败任务')).toBeVisible();

    const statusRequest = page.waitForRequest(
      (req) => req.url().includes('/api/v1/selection/tasks') && req.url().includes('status=failed'),
    );
    await page.getByRole('combobox', { name: '状态' }).click();
    await page.locator('.ant-select-item-option', { hasText: 'failed（失败）' }).click();
    await page.getByRole('button', { name: /查 询|查询/ }).click();
    await statusRequest;
    await expect(page.getByText('E2E 失败任务')).toBeVisible();
    await expect(page.getByText('E2E 选品任务')).toBeHidden();
  });

  test('failed task detail shows task-level and candidate-level error reasons', async ({
    admin,
    page,
  }) => {
    await admin.goto(`/selection/tasks/${E2E_SELECTION_FAILED_TASK_ID}`);
    await expect(page.getByText('任务失败')).toBeVisible();
    await expect(page.getByText(`失败原因：${E2E_SELECTION_TASK_ERROR}`)).toBeVisible();
    await expect(page.getByText(E2E_SELECTION_CANDIDATE_ERROR)).toBeVisible();
  });

  test('readonly account sees no write actions on selection pages', async ({ admin, page }) => {
    // 预存问题（非本次改动引入）：readonly 菜单过滤后 /inventory 父子同 key 触发 antd Menu 警告
    admin.consoleGuard.allowForTest(/Duplicated key '\/inventory' used in Menu/);
    await useReadonlyProfile(page);
    await admin.goto('/selection/tasks');
    await expect(page.getByText('E2E 失败任务')).toBeVisible();
    await expect(page.getByRole('button', { name: '新建选品任务' })).toHaveCount(0);
    await expect(page.getByRole('link', { name: '重试' })).toHaveCount(0);
    await expect(page.getByText('重试', { exact: true })).toHaveCount(0);

    await admin.goto(`/selection/tasks/${E2E_SELECTION_FAILED_TASK_ID}`);
    await expect(page.getByText('任务失败')).toBeVisible();
    await expect(page.getByText('通过', { exact: true })).toHaveCount(0);
    await expect(page.getByText('转草稿', { exact: true })).toHaveCount(0);
  });

  test('403 envelope message is surfaced to the user on retry', async ({ admin, page }) => {
    // 本测试故意 mock 403 响应，浏览器会输出资源加载错误，属预期行为
    admin.consoleGuard.allowForTest(/Failed to load resource: .*403/);
    admin.writeGuard.allow({
      operation: 'retry-selection-task',
      method: 'POST',
      path: new RegExp(`/api/v1/selection/tasks/${E2E_SELECTION_FAILED_TASK_ID}/retry$`),
      status: 403,
      response: fail(READONLY_403_MESSAGE, 40301),
    });
    await admin.goto('/selection/tasks');
    await page.getByText('重试', { exact: true }).first().click();
    await expect(page.getByText(READONLY_403_MESSAGE)).toBeVisible();
  });

  test('selection detail header has no horizontal overflow at 375px', async ({ admin, page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await admin.goto(`/selection/tasks/${E2E_SELECTION_FAILED_TASK_ID}`);
    await expect(page.getByText('任务失败')).toBeVisible();
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    );
    expect(overflow).toBeLessThanOrEqual(1);
  });
});
