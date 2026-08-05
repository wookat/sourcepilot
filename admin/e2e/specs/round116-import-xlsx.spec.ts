import path from 'node:path';
import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';

const inventoryXlsxParseResult = {
  kind: 'inventory',
  fileName: 'generic-inventory.xlsx',
  fileHash: 'e2e-hash-inventory-xlsx',
  sourceFormat: 'custom',
  columns: ['SKU编码', '仓库编码', '期初数量', '参考进价'],
  rows: [
    ['SKU-X100-BLK', 'WH-MAIN', '120', '45.00'],
    ['SKU-X100-WHT', '', '80', ''],
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

const inventoryValidateResult = {
  totalRows: 2,
  validRows: 2,
  errorRows: 0,
  groupCount: 2,
  errors: [],
};

const inventoryCommitResult = {
  jobId: 'e2e-job-inv-xlsx-1',
  status: 'success',
  totalRows: 2,
  successRows: 2,
  failedRows: 0,
  duplicateRows: 0,
  replayed: false,
};

function routeMappings(page: import('@playwright/test').Page) {
  void page.route('**/api/v1/imports/mappings?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ list: [] })),
    });
  });
}

test.describe('R116 数据搬家：XLSX 导入与进度反馈', () => {
  test('XLSX 上传：解析预览，空单元格以占位符对齐显示', async ({ admin }) => {
    const { page, writeGuard } = admin;
    routeMappings(page);
    writeGuard.allow({
      operation: 'import-parse',
      method: 'POST',
      path: /\/api\/v1\/imports\/parse$/,
      response: ok(inventoryXlsxParseResult),
    });

    await admin.goto('/settings/migration?kind=inventory');
    await page
      .locator('input[type="file"]')
      .setInputFiles(path.join(__dirname, '../fixtures/files/generic-inventory.xlsx'));

    await expect(page.getByText('数据预览（前 5 行）')).toBeVisible();
    await expect(page.getByText('generic-inventory.xlsx')).toBeVisible();
    // 空单元格渲染“—”占位符，避免视觉上列错位
    const previewRow = page.getByRole('row').filter({ hasText: 'SKU-X100-WHT' });
    await expect(previewRow.getByText('—').first()).toBeVisible();
    await writeGuard.expectRequestCount('import-parse', 1);
  });

  test('店小秘 XLSX：自动识别来源格式', async ({ admin }) => {
    const { page, writeGuard } = admin;
    routeMappings(page);
    writeGuard.allow({
      operation: 'import-parse',
      method: 'POST',
      path: /\/api\/v1\/imports\/parse$/,
      response: ok({
        ...inventoryXlsxParseResult,
        kind: 'product',
        fileName: 'dianxiaomi-products.xlsx',
        sourceFormat: 'dianxiaomi',
      }),
    });

    await admin.goto('/settings/migration?kind=product');
    await page
      .locator('input[type="file"]')
      .setInputFiles(path.join(__dirname, '../fixtures/files/dianxiaomi-products.xlsx'));

    await expect(page.getByText('店小秘格式')).toBeVisible();
  });

  test('模板下载：CSV 与 Excel 双入口', async ({ admin }) => {
    const { page } = admin;
    routeMappings(page);
    await admin.goto('/settings/migration?kind=order');
    await expect(page.getByRole('button', { name: '下载订单模板（CSV）' })).toBeVisible();
    await expect(page.getByRole('button', { name: '下载订单模板（Excel）' })).toBeVisible();
  });

  test('导入进行中：展示进度反馈并防止重复提交', async ({ admin }) => {
    const { page, writeGuard } = admin;
    routeMappings(page);
    writeGuard.allow({
      operation: 'import-parse',
      method: 'POST',
      path: /\/api\/v1\/imports\/parse$/,
      response: ok(inventoryXlsxParseResult),
    });
    writeGuard.allow({
      operation: 'import-validate',
      method: 'POST',
      path: /\/api\/v1\/imports\/validate$/,
      response: ok(inventoryValidateResult),
    });
    void page.route('**/api/v1/imports/progress?*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ active: true, processed: 1, total: 2 })),
      });
    });
    // commit 延迟返回，模拟大批量导入进行中
    writeGuard.allow({
      operation: 'import-commit',
      method: 'POST',
      path: /\/api\/v1\/imports\/commit$/,
      response: ok(inventoryCommitResult),
      delayMs: 2500,
    });

    await admin.goto('/settings/migration?kind=inventory');
    await page
      .locator('input[type="file"]')
      .setInputFiles(path.join(__dirname, '../fixtures/files/generic-inventory.xlsx'));
    await expect(page.getByText('数据预览（前 5 行）')).toBeVisible();
    await page.getByRole('button', { name: '下一步：校验' }).click();
    await expect(page.getByText('共 2 行：可导入 2 行，存在问题 0 行')).toBeVisible();

    await page.getByRole('button', { name: '确认导入 2 行' }).click();
    // 导入中：按钮进入 loading/禁用态，展示进度条与提示
    await expect(page.getByTestId('import-commit-progress')).toBeVisible();
    await expect(page.getByText('请勿关闭或重复提交')).toBeVisible();
    await expect(page.getByRole('button', { name: '导入中…' })).toBeDisabled();
    await expect(page.getByRole('button', { name: '返回调整映射' })).toBeDisabled();

    await expect(page.getByText('导入成功：共 2 行')).toBeVisible({ timeout: 10_000 });
    await writeGuard.expectRequestCount('import-commit', 1);
  });
});
