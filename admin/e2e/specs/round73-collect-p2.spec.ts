import { test, expect } from '../fixtures/admin.fixture';
import { ok, paged } from '../mocks/envelope';

const ISO_AT = '2026-08-02T10:00:00Z';

const BATCH_SAME_URL_HINT =
  '该链接单独采集成功，批量失败可能由并发、访问频率或目标站点风控导致。建议降低批量并发或稍后重试。';

async function mockCollectProviders(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/collect/providers', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        ok([
          {
            source: '1688',
            name: '1688 采集器',
            status: 'available',
            batchSupported: true,
            features: ['title', 'price', 'mainImages'],
          },
        ]),
      ),
    });
  });
}

async function mockCollectTasks(
  page: import('@playwright/test').Page,
  tasks: Array<Record<string, unknown>>,
) {
  await page.route('**/api/v1/collect/tasks?*', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    const url = new URL(route.request().url());
    const status = url.searchParams.get('status');
    const list = status ? tasks.filter((t) => t.status === status) : tasks;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok(paged(list))),
    });
  });
}

test.describe('R73 采集侧 P2 修复', () => {
  test('采集中心「最近任务」retrying 状态展示共享中文映射，不出现裸枚举', async ({ admin }) => {
    const { page } = admin;
    await mockCollectProviders(page);
    await mockCollectTasks(page, [
      {
        id: 'e2e-collect-task-retrying',
        source: '1688',
        sourceUrl: 'https://detail.1688.com/offer/e2e-hub-recent.html',
        status: 'retrying',
        createdAt: ISO_AT,
        updatedAt: ISO_AT,
      },
    ]);

    await admin.goto('/collect/hub');

    const recentTag = page.locator('.tm-collect-hub-task-list .ant-tag');
    await expect(recentTag).toHaveText('等待重试');
    await expect(page.locator('.tm-collect-hub-task-list')).not.toContainText('retrying');
  });

  test('采集规则编辑器「使用简单模板」直接填充规则 JSON', async ({ admin }) => {
    const { page } = admin;
    await admin.goto('/collect/rules');

    await page.getByRole('button', { name: '手动新建规则' }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByText('手动新建采集规则')).toBeVisible();

    await dialog.getByRole('button', { name: '使用简单模板' }).click();
    await expect(page.getByText('已将简单模板填入下方「规则 JSON」', { exact: false })).toBeVisible();

    await dialog.getByText('采集规则内容（高级）').click();
    const ruleJson = dialog.locator('textarea#ruleJson');
    await expect(ruleJson).toBeVisible();
    await expect(ruleJson).toHaveValue(/"selector": "h1"/);

    await dialog.getByRole('button', { name: '使用高级模板' }).click();
    await expect(ruleJson).toHaveValue(/"selectors": \[/);
  });

  test('失败任务「处理建议」按单条 / 批量场景区分话术', async ({ admin }) => {
    const { page } = admin;
    await mockCollectProviders(page);
    await mockCollectTasks(page, [
      {
        id: 'e2e-collect-task-single-fail',
        source: '1688',
        sourceUrl: 'https://detail.1688.com/offer/e2e-single.html',
        status: 'failed',
        failureHint: BATCH_SAME_URL_HINT,
        sameUrlSucceededElsewhere: true,
        createdAt: ISO_AT,
        updatedAt: ISO_AT,
      },
      {
        id: 'e2e-collect-task-batch-fail',
        batchId: 'e2e-collect-batch-1',
        source: '1688',
        sourceUrl: 'https://detail.1688.com/offer/e2e-batch.html',
        status: 'failed',
        failureHint: BATCH_SAME_URL_HINT,
        sameUrlSucceededElsewhere: true,
        createdAt: ISO_AT,
        updatedAt: ISO_AT,
      },
    ]);

    await page.setViewportSize({ width: 1600, height: 900 });
    await admin.goto('/collect/tasks');

    const singleRow = page.locator('tr', { hasText: 'e2e-single.html' });
    await expect(singleRow).toContainText('该链接此前已采集成功');
    await expect(singleRow).not.toContainText('批量失败');

    const batchRow = page.locator('tr', { hasText: 'e2e-batch.html' });
    await expect(batchRow).toContainText('批量失败可能由并发、访问频率或目标站点风控导致');
  });
});
