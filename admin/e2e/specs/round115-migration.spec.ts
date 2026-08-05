import path from 'node:path';
import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';

const inventoryParseResult = {
  kind: 'inventory',
  fileName: 'generic-inventory.csv',
  fileHash: 'e2e-hash-inventory',
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

const inventoryValidateResult = {
  totalRows: 2,
  validRows: 2,
  errorRows: 0,
  groupCount: 2,
  errors: [],
};

const inventoryCommitResult = {
  jobId: 'e2e-job-inv-1',
  status: 'success',
  totalRows: 2,
  successRows: 2,
  failedRows: 0,
  duplicateRows: 0,
  replayed: false,
};

const mappingPreset = {
  id: 'e2e-preset-1',
  kind: 'inventory',
  name: '通用库存模板',
  columns: ['SKU编码', '仓库编码', '期初数量', '参考进价'],
  mapping: { skuCode: 0, warehouseCode: 1, quantity: 2, costPrice: 3 },
  createdAt: '2026-08-01T10:00:00Z',
  updatedAt: '2026-08-01T10:00:00Z',
};

function routeMappings(page: import('@playwright/test').Page, list: unknown[]) {
  void page.route('**/api/v1/imports/mappings?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ list })),
    });
  });
}

test.describe('R115 数据搬家：库存期初导入与映射方案', () => {
  test('库存期初导入：无需店铺，上传 → 映射 → 校验 → 导入', async ({ admin }) => {
    const { page, writeGuard } = admin;
    routeMappings(page, []);
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
      response: ok(inventoryValidateResult),
    });
    writeGuard.allow({
      operation: 'import-commit',
      method: 'POST',
      path: /\/api\/v1\/imports\/commit$/,
      response: ok(inventoryCommitResult),
    });

    await admin.goto('/settings/migration?kind=inventory');
    await expect(page.locator('.ant-radio-button-wrapper-checked')).toContainText('库存期初');
    await expect(page.getByRole('button', { name: '下载库存期初模板（CSV）' })).toBeVisible();
    await expect(page.getByRole('button', { name: '下载库存期初模板（Excel）' })).toBeVisible();

    await page
      .locator('input[type="file"]')
      .setInputFiles(path.join(__dirname, '../fixtures/files/generic-inventory.csv'));

    // 库存导入为租户级：不展示归属店铺选择
    await expect(page.getByText('数据预览（前 5 行）')).toBeVisible();
    await expect(page.getByText('归属店铺')).toHaveCount(0);

    await page.getByRole('button', { name: '下一步：校验' }).click();
    await expect(page.getByText('共 2 行：可导入 2 行，存在问题 0 行')).toBeVisible();
    await expect(page.getByText('将写入 2 条期初库存（含库存流水）；有问题的行不会入库')).toBeVisible();

    await page.getByRole('button', { name: '确认导入 2 行' }).click();
    await expect(page.getByText('导入成功：共 2 行')).toBeVisible();

    await writeGuard.expectRequestCount('import-parse', 1);
    await writeGuard.expectRequestCount('import-validate', 1);
    await writeGuard.expectRequestCount('import-commit', 1);
  });

  test('列映射：保存与套用映射方案', async ({ admin }) => {
    const { page, writeGuard } = admin;
    routeMappings(page, [mappingPreset]);
    writeGuard.allow({
      operation: 'import-parse',
      method: 'POST',
      path: /\/api\/v1\/imports\/parse$/,
      response: ok({ ...inventoryParseResult, mapping: { skuCode: -1, warehouseCode: -1, quantity: -1, costPrice: -1 } }),
    });
    writeGuard.allow({
      operation: 'save-mapping',
      method: 'POST',
      path: /\/api\/v1\/imports\/mappings$/,
      response: ok(mappingPreset),
    });

    await admin.goto('/settings/migration?kind=inventory');
    await page
      .locator('input[type="file"]')
      .setInputFiles(path.join(__dirname, '../fixtures/files/generic-inventory.csv'));
    await expect(page.getByText('数据预览（前 5 行）')).toBeVisible();

    // 套用已保存的方案
    await page.getByRole('combobox').first().click();
    await page.getByText('通用库存模板').click();
    await expect(page.getByText('已套用映射方案「通用库存模板」')).toBeVisible();

    // 保存当前映射为新方案
    await page.getByPlaceholder('方案名称（如：店小秘库存）').fill('我的库存方案');
    await page.getByRole('button', { name: '保存当前映射' }).click();
    await expect(page.getByText('映射方案已保存，下次导入可直接套用')).toBeVisible();
    await writeGuard.expectRequestCount('save-mapping', 1);
  });

  test('数据导出：四类全量 CSV 导出入口', async ({ admin }) => {
    const { page } = admin;
    routeMappings(page, []);

    await admin.goto('/settings/migration');
    await page.getByRole('tab', { name: '数据导出' }).click();

    await expect(page.getByText('四类数据均可全量导出为 CSV', { exact: false })).toBeVisible();
    for (const label of ['商品', '订单', '库存期初', '货源档案']) {
      await expect(
        page.getByRole('row').filter({ hasText: label }).getByRole('button', { name: '导出 CSV' }).first(),
      ).toBeVisible();
    }
  });

  test('三视口：向导页无横向溢出', async ({ admin }) => {
    const { page } = admin;
    routeMappings(page, []);
    for (const vp of [
      { width: 1440, height: 900 },
      { width: 768, height: 900 },
      { width: 375, height: 812 },
    ]) {
      await page.setViewportSize(vp);
      await admin.goto('/settings/migration?kind=source');
      await expect(page.getByRole('button', { name: '下载货源档案模板（CSV）' })).toBeVisible();
      const rootOverflow = await page.evaluate(
        () => document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
      );
      expect(rootOverflow, `viewport ${vp.width} 出现横向溢出`).toBe(false);
    }
  });
});
