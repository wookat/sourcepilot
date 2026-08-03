import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { routeInventoryCenterData } from '../mocks/inventory-center.fixture';

// R77：TmProTable 按可见列总宽兜底 scroll.x —— 声明列宽总和超过页面
// scroll.x 时表格不再被压缩，固定操作列不挤压相邻数据列（1440 视口复现）。
test.describe('@round77 表格列宽兜底', () => {
  test('1440px 库存中心：表格宽度不低于列宽总和，固定列不挤压数据列', async ({ admin, page }) => {
    await routeInventoryCenterData(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    await admin.goto('/inventory');
    await expect(page.getByText('库存中心').first()).toBeVisible({ timeout: 20_000 });
    await expect(page.getByRole('link', { name: 'E2E 保温杯 500ml' }).first()).toBeVisible();
    await expect(page.locator('td.ant-table-cell-fix-right').first()).toBeVisible();
    // 库存中心可见列声明宽度总和为 1640，页面原 scroll.x=1500；
    // 兜底后表格实际渲染宽度不得低于列宽总和，列不再被按比例压缩。
    const tableWidth = await page
      .locator('.ant-pro-table .ant-table-content table, .ant-pro-table .ant-table-body table')
      .first()
      .evaluate((el) => el.getBoundingClientRect().width);
    expect(tableWidth).toBeGreaterThanOrEqual(1640);
    // 与固定操作列相邻的数据列（异常）表头完整可读，无被盖住/截断
    const exceptionHeader = page.locator('th', { hasText: '异常' }).first();
    await expect(exceptionHeader).toBeVisible();
    const headerBox = await exceptionHeader.evaluate((el) => ({
      scrollWidth: el.scrollWidth,
      clientWidth: el.clientWidth,
    }));
    expect(headerBox.scrollWidth).toBeLessThanOrEqual(headerBox.clientWidth);
  });

  test('1440px 失败任务中心：超长内容被裁切，不溢出盖住相邻列', async ({ admin, page }) => {
    await page.route('**/api/v1/task-center/failures**', async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      const url = route.request().url();
      if (url.includes('/categories') || url.includes('/summary')) {
        await route.fallback();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            list: [
              {
                id: 'e2e-failure-r77',
                taskType: 'publish',
                platform: 'tiktok',
                title: 'e2e 超长失败任务标题用于验证列内容截断兜底——不得溢出盖住相邻列或固定操作列，应以省略号收尾',
                normalizedStatus: 'failed',
                createdAt: '2026-08-01T08:00:00Z',
              },
            ],
            total: 1,
            summary: {},
            page: 1,
            pageSize: 20,
          }),
        ),
      });
    });
    await page.setViewportSize({ width: 1440, height: 900 });
    await admin.goto('/ops/task-center/failures');
    await expect(page.getByText('失败任务中心').first()).toBeVisible({ timeout: 20_000 });
    await expect(page.getByText('e2e 超长失败任务标题', { exact: false }).first()).toBeVisible();
    // 数据列缺省 ellipsis 兜底：内容超宽的单元格必须裁切（overflow 不为 visible），
    // 不得溢出盖到相邻列
    const leaking = await page
      .locator('.ant-pro-table td.ant-table-cell:not(.ant-table-cell-fix-right)')
      .evaluateAll((cells) =>
        cells.filter((el) => {
          if (el.scrollWidth <= el.clientWidth + 1) return false;
          return window.getComputedStyle(el).overflow === 'visible';
        }).length,
      );
    expect(leaking).toBe(0);
  });
});
