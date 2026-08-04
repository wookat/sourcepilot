import path from 'node:path';
import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';

const SHOP_ID = 'e2e-shop-0001';

const productParseResult = {
  kind: 'product',
  fileName: 'dianxiaomi-products.csv',
  fileHash: 'e2e-hash-product',
  sourceFormat: 'dianxiaomi',
  columns: ['商品名称', 'SKU', '规格名称', '售价', '采购参考价', '库存数量', '币种', '图片链接'],
  rows: [
    ['无线蓝牙耳机 X100', 'DXM-X100-BLK', '黑色', '89.00', '45.00', '120', 'CNY', ''],
    ['无线蓝牙耳机 X100', 'DXM-X100-WHT', '白色', '89.00', '45.00', '80', 'CNY', ''],
    ['桌面手机支架', 'DXM-STAND-01', '', '19.90', 'abc', '50', 'CNY', ''],
  ],
  totalRows: 3,
  mapping: {
    title: 0,
    skuCode: 1,
    skuName: 2,
    price: 3,
    costPrice: 4,
    stock: 5,
    currency: 6,
    imageUrl: 7,
    description: -1,
    sourceUrl: -1,
  },
  fields: [
    { key: 'title', label: '商品名称', required: true },
    { key: 'skuCode', label: 'SKU编码', required: false },
    { key: 'skuName', label: '规格名称', required: false },
    { key: 'price', label: '售价', required: false },
    { key: 'costPrice', label: '成本价', required: false },
    { key: 'stock', label: '库存数量', required: false },
    { key: 'currency', label: '币种', required: false },
    { key: 'imageUrl', label: '图片链接', required: false },
    { key: 'description', label: '商品描述', required: false },
    { key: 'sourceUrl', label: '来源链接', required: false },
  ],
};

const validateResult = {
  totalRows: 3,
  validRows: 2,
  errorRows: 1,
  groupCount: 1,
  errors: [{ rowNumber: 3, field: 'costPrice', message: '成本价需为非负数字' }],
};

const commitResult = {
  jobId: 'e2e-job-0001',
  status: 'partial_success',
  totalRows: 3,
  successRows: 2,
  failedRows: 1,
  duplicateRows: 0,
  replayed: false,
};

const historyJob = {
  id: 'e2e-job-0001',
  kind: 'product',
  batchKey: 'e2e-hash-product',
  shopId: SHOP_ID,
  sourceFormat: 'dianxiaomi',
  fileName: 'dianxiaomi-products.csv',
  status: 'partial_success',
  totalRows: 3,
  successRows: 2,
  failedRows: 1,
  duplicateRows: 0,
  errorRowCount: 1,
  createdAt: '2026-08-01T10:00:00Z',
  updatedAt: '2026-08-01T10:00:00Z',
};

function routeReadApis(page: import('@playwright/test').Page) {
  void page.route('**/api/v1/shops?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        ok({
          list: [
            {
              id: SHOP_ID,
              platform: 'tiktok',
              shopName: 'E2E 店铺',
              status: 'active',
              authStatus: 'authorized',
            },
          ],
          pagination: { page: 1, pageSize: 500, total: 1, totalPages: 1 },
        }),
      ),
    });
  });
  void page.route('**/api/v1/imports?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ list: [historyJob], total: 1, page: 1, pageSize: 20 })),
    });
  });
  void page.route('**/api/v1/imports/e2e-job-0001', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        ok({
          job: historyJob,
          errorRows: [
            {
              id: 'row-1',
              jobId: 'e2e-job-0001',
              rowNumber: 3,
              status: 'failed',
              field: 'costPrice',
              message: '成本价需为非负数字',
              rawValues: { 商品名称: '桌面手机支架' },
            },
          ],
        }),
      ),
    });
  });
}

test.describe('R92 迁移导入向导', () => {
  test('商品导入：上传 → 映射 → 校验 → 导入', async ({ admin }) => {
    const { page, writeGuard } = admin;
    routeReadApis(page);
    writeGuard.allow({
      operation: 'import-parse',
      method: 'POST',
      path: /\/api\/v1\/imports\/parse$/,
      response: ok(productParseResult),
    });
    writeGuard.allow({
      operation: 'import-validate',
      method: 'POST',
      path: /\/api\/v1\/imports\/validate$/,
      response: ok(validateResult),
    });
    writeGuard.allow({
      operation: 'import-commit',
      method: 'POST',
      path: /\/api\/v1\/imports\/commit$/,
      response: ok(commitResult),
    });

    await admin.goto('/settings/migration?kind=product');
    await expect(page.getByText('点击或拖拽上传 CSV / XLSX 文件')).toBeVisible();

    await page
      .locator('input[type="file"]')
      .setInputFiles(path.join(__dirname, '../fixtures/files/dianxiaomi-products.csv'));

    // 列映射步骤：识别为店小秘格式并展示映射表
    await expect(page.getByText('店小秘格式')).toBeVisible();
    await expect(page.getByText('数据预览（前 5 行）')).toBeVisible();

    // 选择归属店铺后进入校验
    await page.getByRole('combobox').first().click();
    await page.getByText('E2E 店铺').click();
    await page.getByRole('button', { name: '下一步：校验' }).click();

    // 校验报告：2 行可导入、1 行错误
    await expect(page.getByText('共 3 行：可导入 2 行，存在问题 1 行')).toBeVisible();
    await expect(page.getByText('成本价需为非负数字')).toBeVisible();

    // 确认导入
    await page.getByRole('button', { name: '确认导入 2 行' }).click();
    await expect(page.getByText('部分成功：成功 2 行，失败 1 行')).toBeVisible();
    await expect(page.getByRole('button', { name: '下载错误行报告' })).toBeVisible();

    await writeGuard.expectRequestCount('import-parse', 1);
    await writeGuard.expectRequestCount('import-validate', 1);
    await writeGuard.expectRequestCount('import-commit', 1);
  });

  test('导入历史：批次结果与错误行查看', async ({ admin }) => {
    const { page } = admin;
    routeReadApis(page);

    await admin.goto('/settings/migration');
    await page.getByRole('tab', { name: '导入历史' }).click();

    await expect(page.getByText('dianxiaomi-products.csv').first()).toBeVisible();
    await expect(page.getByText('部分成功').first()).toBeVisible();

    await page.getByRole('button', { name: /查\s*看/ }).first().click();
    await expect(page.getByText('成本价需为非负数字')).toBeVisible();
    await expect(page.getByRole('button', { name: '错误行下载' }).first()).toBeVisible();
  });
});
