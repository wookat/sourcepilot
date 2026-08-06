import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';

const backupItems = [
  {
    backupId: 'bk_e2e_uploaded_0001',
    environment: 'development',
    backupType: 'pg_dump_full',
    status: 'completed',
    verificationStatus: 'passed',
    encrypted: true,
    storageProvider: 'minio',
    uploadStatus: 'uploaded',
    uploadTarget: 's3://trademind-backups/backups/e2e (minio.local:9000)',
    uploadAttempts: 1,
    uploadedAt: '2026-08-06T08:00:00Z',
    artifactSize: 10240,
    createdAt: '2026-08-06T08:00:00Z',
  },
  {
    backupId: 'bk_e2e_failed_0002',
    environment: 'development',
    backupType: 'pg_dump_full',
    status: 'completed',
    verificationStatus: 'pending',
    encrypted: true,
    storageProvider: 'minio',
    uploadStatus: 'failed',
    uploadError: 'upload backups/e2e/x.dump: connection refused',
    uploadAttempts: 3,
    artifactSize: 10240,
    createdAt: '2026-08-06T07:00:00Z',
  },
  {
    backupId: 'bk_e2e_skipped_0003',
    environment: 'development',
    backupType: 'pg_dump_full',
    status: 'completed',
    verificationStatus: 'pending',
    encrypted: false,
    storageProvider: 'local',
    uploadStatus: 'skipped',
    artifactSize: 10240,
    createdAt: '2026-08-06T06:00:00Z',
  },
];

async function mockBackupList(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/ops/backups?*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ items: backupItems, total: backupItems.length })),
    });
  });
}

async function mockProfile(
  page: import('@playwright/test').Page,
  overrides: Record<string, unknown>,
) {
  await page.route('**/api/v1/auth/profile', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ ...e2eUser, ...overrides })),
    });
  });
}

test.describe('@ops R138 备份对象存储上传状态', () => {
  test('平台管理员展示上传状态/目标列，failed 行有「重试上传」', async ({ page, admin }) => {
    await mockProfile(page, { tenantId: 0 });
    await mockBackupList(page);
    await admin.goto('/ops/backups');
    await expect(page.getByText('已上传').first()).toBeVisible();
    await expect(page.getByText('上传失败').first()).toBeVisible();
    await expect(page.getByText('仅本地').first()).toBeVisible();
    await expect(page.getByText('s3://trademind-backups/backups/e2e (minio.local:9000)')).toBeVisible();
    const retryBtn = page.getByRole('button', { name: '重试上传' });
    await expect(retryBtn).toHaveCount(1);
    await expect(retryBtn).toBeEnabled();
    await expect(page.getByRole('button', { name: '创建备份' })).toBeEnabled();
  });

  test('readonly 角色被路由门控拦截，无任何写入口', async ({ page, admin }) => {
    await mockProfile(page, { role: 'readonly', tenantId: 0, permissions: [] });
    await mockBackupList(page);
    await admin.goto('/ops/backups');
    await expect(page.getByText('暂无访问权限').first()).toBeVisible();
    await expect(page.getByRole('button', { name: '创建备份' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: '重试上传' })).toHaveCount(0);
  });
});
