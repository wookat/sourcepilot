import { test, expect } from '../fixtures/admin.fixture';
import { E2E_SELECTION_TASK_ID } from '../mocks/selection';

const VIEWPORTS = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
  { width: 375, height: 812 },
];

test.describe('@selection Round120 选品数据面', () => {
  test('数据面板 drawer shows collected facts, benchmark, AI breakdown and degraded external sources', async ({
    admin,
    page,
  }) => {
    await admin.goto(`/selection/tasks/${E2E_SELECTION_TASK_ID}`);
    await page.getByLabel('查看数据面板：E2E 已采集候选').click();

    await expect(page.getByText('数据面板：E2E 已采集候选')).toBeVisible();
    // collected facts
    await expect(page.getByText('19.90 USD', { exact: true })).toBeVisible();
    await expect(page.getByText('货源采集价')).toBeVisible();
    await expect(page.getByText('21.20 CNY', { exact: true })).toBeVisible();
    // benchmark from internal orders
    await expect(page.getByText('站内同类目经营（e2e-家居，近 90 天）')).toBeVisible();
    await expect(page.getByText('41.5%')).toBeVisible();
    // AI score decomposition
    await expect(page.getByText('81 分').first()).toBeVisible();
    await expect(page.getByText('E2E 数据面板评分摘要')).toBeVisible();
    // degraded external sources: declared but not configured
    await expect(page.getByText('TikTok 热销榜')).toBeVisible();
    await expect(page.getByText('未配置平台开放接口凭证').first()).toBeVisible();
    await expect(page.getByText('未接入').first()).toBeVisible();
  });

  test('数据面板 shows 未采集 and trend empty-state guidance without collect history', async ({
    admin,
    page,
  }) => {
    await admin.goto(`/selection/tasks/${E2E_SELECTION_TASK_ID}`);
    await page.getByLabel('查看数据面板：E2E 规则兜底候选').click();

    await expect(page.getByText('数据面板：E2E 规则兜底候选')).toBeVisible();
    await expect(page.getByText('未采集').first()).toBeVisible();
    await expect(page.getByText('暂无价格走势')).toBeVisible();
    await expect(page.getByText('同一来源链接需要至少 2 次成功采集才能绘制价格走势', { exact: false })).toBeVisible();
    await expect(page.getByRole('button', { name: '前往采集任务' })).toBeVisible();
  });

  test('对比所选 opens side-by-side comparison with supply readiness and banned risk', async ({
    admin,
    page,
  }) => {
    await admin.goto(`/selection/tasks/${E2E_SELECTION_TASK_ID}`);

    const compareBtn = page.getByRole('button', { name: /对比所选/ });
    await expect(compareBtn).toBeDisabled();

    await page.getByLabel('选择候选：E2E 规则兜底候选').check();
    await expect(page.getByText('再勾选至少 1 个候选即可发起对比（最多 5 个）。')).toBeVisible();
    await page.getByLabel('选择候选：E2E 已采集候选').check();
    await expect(compareBtn).toBeEnabled();
    await compareBtn.click();

    await expect(page.getByText('选品对比（2 个候选）')).toBeVisible();
    await expect(page.getByText('供应链就绪度')).toBeVisible();
    await expect(page.getByText('已就绪（e2e-义乌供应商）')).toBeVisible();
    await expect(page.getByText('未匹配货源档案')).toBeVisible();
    await expect(page.getByText('违禁词风险')).toBeVisible();
    await expect(page.getByText('禁用 1：最强')).toBeVisible();

    // CSV export triggers a download without any write request
    const downloadPromise = page.waitForEvent('download');
    await page.getByRole('button', { name: '导出 CSV' }).click();
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toMatch(/^选品对比-\d{4}-\d{2}-\d{2}\.csv$/);
  });

  test('selection detail with drawers keeps no root horizontal overflow across viewports', async ({
    admin,
    page,
  }) => {
    await admin.goto(`/selection/tasks/${E2E_SELECTION_TASK_ID}`);
    for (const viewport of VIEWPORTS) {
      await page.setViewportSize(viewport);
      await expect
        .poll(async () =>
          page.evaluate(() => ({
            doc: document.documentElement.scrollWidth <= document.documentElement.clientWidth,
            body: document.body.scrollWidth <= document.body.clientWidth,
          })),
        )
        .toEqual({ doc: true, body: true });
    }
  });
});
