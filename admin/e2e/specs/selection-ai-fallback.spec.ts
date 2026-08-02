import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import {
  E2E_SELECTION_TASK_ID,
  integrationsOverviewAIUnconfigured,
} from '../mocks/selection';

test.describe('@selection AI 降级提示', () => {
  test('create modal warns when AI provider is not configured', async ({ admin, page }) => {
    await page.route('**/api/v1/settings/integrations/overview', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok(integrationsOverviewAIUnconfigured)),
      });
    });
    await admin.goto('/selection/tasks');
    await page.getByRole('button', { name: '新建选品任务' }).click();
    await expect(page.getByText('AI 服务商未配置，任务将使用规则兜底评分')).toBeVisible();
  });

  test('detail page marks rule-fallback scores', async ({ admin, page }) => {
    await admin.goto(`/selection/tasks/${E2E_SELECTION_TASK_ID}`);
    await expect(page.getByText('本任务存在规则兜底评分')).toBeVisible();
    await expect(page.getByText('规则兜底', { exact: true }).first()).toBeVisible();
  });
});
