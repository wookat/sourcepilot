import path from 'node:path';
import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';

const inventoryParseResult = {
  kind: 'inventory',
  fileName: 'generic-inventory.csv',
  fileHash: 'e2e-hash-inventory-history',
  sourceFormat: 'custom',
  columns: ['SKU编码', '仓库编码', '期初数量', '参考进价'],
  rows: [
    ['DEMO-SKU-1-1', 'default', '120', '45.00'],
    ['DEMO-SKU-1-2', 'DEMO-WH-2', '80', '45.00'],
  ],
  totalRows: 2,
  mapping: { skuCode: 0, warehouseCode: 1, quantity: 2, costPrice: 3 },
  fields: [
    { key: 'skuCode', label: 'SKU编码', required: true },
    { key: 'warehouseCode', label: '仓库编码', required: false },
    { key: 'quantity', label: '期初数量', required: true },
    { key: 'costPrice', label: '参考进价', required: false },
  ],
};

const committedJob = {
  id: 'e2e-job-history-1',
  kind: 'inventory',
  fileName: 'generic-inventory.csv',
  sourceFormat: 'custom',
  status: 'success',
  totalRows: 2,
  successRows: 2,
  failedRows: 0,
  duplicateRows: 0,
  errorRowCount: 0,
  createdAt: '2026-08-05T10:00:00Z',
};

test.describe('R118 数据搬家：导入历史自动刷新', () => {
  test('提交导入后切到导入历史 Tab，新 job 无需刷新页面即可见', async ({ admin }) => {
    const { page, writeGuard } = admin;
    const jobs: (typeof committedJob)[] = [];

    void page.route('**/api/v1/imports/mappings?*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ list: [] })),
      });
    });
    void page.route('**/api/v1/imports?*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ list: jobs, total: jobs.length, page: 1, pageSize: 20 })),
      });
    });
    writeGuard.allow({
      operation: 'import-parse',
      method: 'POST',
      path: /\/api\/v1\/imports\/parse$/,
      response: ok(inventoryParseResult),
    });
    writeGuard.allow({
      operation: 'import-validate',
      method: 'POST',
      path: /\/api\/v1\/imports\/validate$/,
      response: ok({ totalRows: 2, validRows: 2, errorRows: 0, groupCount: 2, errors: [] }),
    });
    writeGuard.allow({
      operation: 'import-commit',
      method: 'POST',
      path: /\/api\/v1\/imports\/commit$/,
      response: ok({
        jobId: committedJob.id,
        status: 'success',
        totalRows: 2,
        successRows: 2,
        failedRows: 0,
        duplicateRows: 0,
        replayed: false,
      }),
    });

    await admin.goto('/settings/migration?kind=inventory');

    // 先访问一次导入历史（此时无记录），使其组件保持挂载态
    await page.getByRole('tab', { name: '导入历史' }).click();
    await expect(page.getByText('暂无导入记录', { exact: false })).toBeVisible();

    // 回到向导完成一次导入
    await page.getByRole('tab', { name: '导入向导' }).click();
    await page
      .locator('input[type="file"]')
      .setInputFiles(path.join(__dirname, '../fixtures/files/generic-inventory.csv'));
    await expect(page.getByText('数据预览（前 5 行）')).toBeVisible();
    await page.getByRole('button', { name: '下一步：校验' }).click();
    await page.getByRole('button', { name: '确认导入 2 行' }).click();
    await expect(page.getByText('导入成功：共 2 行')).toBeVisible();
    jobs.unshift(committedJob);

    // 切回导入历史：应自动重新拉取列表，新 job 立即可见
    await page.getByRole('tab', { name: '导入历史' }).click();
    await expect(
      page.getByRole('row').filter({ hasText: 'generic-inventory.csv' }).first(),
    ).toBeVisible();
    await writeGuard.expectRequestCount('import-commit', 1);
  });
});
