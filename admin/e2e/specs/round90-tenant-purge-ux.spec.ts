import { test, expect } from '../fixtures/admin.fixture';
import { expectNoRootOverflow } from '../utils/assertions';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';

const disabledTenant = {
  id: 3,
  name: '华南运营中心',
  status: 'disabled',
  adminCount: 2,
  createdAt: '2026-08-02T00:00:00Z',
};

function tenantList(includeDisabled: boolean) {
  return {
    list: [
      { id: 0, name: '平台租户（默认）', status: 'active', adminCount: 3 },
      ...(includeDisabled ? [disabledTenant] : []),
    ],
  };
}

async function setupPlatformAdmin(
  page: import('@playwright/test').Page,
  state: { purged: boolean },
) {
  await page.route('**/api/v1/auth/profile', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ ...e2eUser, role: 'admin', permissions: [], tenantId: 0 })),
    });
  });
  await page.route('**/api/v1/platform/tenants', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok(tenantList(!state.purged))),
      });
      return;
    }
    await route.fallback();
  });
}

function purgeTask(status: 'pending' | 'running' | 'succeeded' | 'failed', error?: string) {
  return {
    id: 'task-1',
    tenantId: 3,
    tenantName: disabledTenant.name,
    status,
    error,
    createdAt: '2026-08-03T00:00:00Z',
  };
}

test.describe('@round90-tenant-purge-ux 清退中状态与轮询（round90）', () => {
  test('清退提交后列表行显示「清退中」，任务完成后行消失并提示', async ({ page, admin }) => {
    const state = { purged: false };
    await setupPlatformAdmin(page, state);

    let statusCalls = 0;
    await page.route('**/api/v1/platform/tenants/3/purge', async (route) => {
      if (route.request().method() === 'GET') {
        statusCalls += 1;
        const status = statusCalls >= 2 ? 'succeeded' : 'running';
        if (status === 'succeeded') state.purged = true;
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(ok(purgeTask(status))),
        });
        return;
      }
      await route.fallback();
    });
    admin.writeGuard.allow({
      operation: 'purgeTenant',
      method: 'POST',
      path: /^\/api\/v1\/platform\/tenants\/3\/purge$/,
      response: ok(purgeTask('pending')),
    });

    await admin.goto('/settings/platform-tenants');
    const row = page.getByRole('row', { name: /华南运营中心/ });
    await expect(row.getByText('已停用')).toBeVisible();

    await row.getByRole('button', { name: '清退删除' }).click();
    await page.getByLabel(/请输入租户名称/).fill(disabledTenant.name);
    await page.getByRole('button', { name: '下一步' }).click();
    await page.getByRole('button', { name: '确认清退' }).click();
    await expect(page.getByText('清退任务已提交，将在后台执行')).toBeVisible();
    await admin.writeGuard.expectRequestCount('purgeTenant', 1);

    // 竞态窗口内：行不再显示「已停用」，而是「清退中」，且操作入口隐藏
    await expect(row.getByText('清退中')).toBeVisible();
    await expect(row.getByRole('button', { name: '清退删除' })).toHaveCount(0);
    await expect(row.getByRole('button', { name: '启用' })).toHaveCount(0);

    // 轮询到 succeeded 后：提示完成并刷新列表，行消失
    await expect(page.getByText(`租户「${disabledTenant.name}」清退完成`)).toBeVisible({
      timeout: 15_000,
    });
    await expect(page.getByRole('row', { name: /华南运营中心/ })).toHaveCount(0);
  });

  for (const viewport of [
    { width: 1440, height: 900 },
    { width: 375, height: 812 },
  ]) {
    test(`it should have no root overflow at ${viewport.width}x${viewport.height}`, async ({ page, admin }) => {
      const state = { purged: false };
      await setupPlatformAdmin(page, state);
      await page.setViewportSize(viewport);
      await admin.goto('/settings/platform-tenants');
      // 375 下名称列在表格横向滚动区内，断言行已加载即可
      await expect(page.getByRole('row', { name: /华南运营中心/ })).toBeAttached();
      await expectNoRootOverflow(page);
    });
  }

  test('清退任务失败时提示失败原因并恢复行操作', async ({ page, admin }) => {
    const state = { purged: false };
    await setupPlatformAdmin(page, state);

    await page.route('**/api/v1/platform/tenants/3/purge', async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(ok(purgeTask('failed', '库存表残留 3 行'))),
        });
        return;
      }
      await route.fallback();
    });
    admin.writeGuard.allow({
      operation: 'purgeTenant',
      method: 'POST',
      path: /^\/api\/v1\/platform\/tenants\/3\/purge$/,
      response: ok(purgeTask('pending')),
    });

    await admin.goto('/settings/platform-tenants');
    const row = page.getByRole('row', { name: /华南运营中心/ });
    await row.getByRole('button', { name: '清退删除' }).click();
    await page.getByLabel(/请输入租户名称/).fill(disabledTenant.name);
    await page.getByRole('button', { name: '下一步' }).click();
    await page.getByRole('button', { name: '确认清退' }).click();

    await expect(row.getByText('清退中')).toBeVisible();
    await expect(
      page.getByText(`租户「${disabledTenant.name}」清退失败：库存表残留 3 行`),
    ).toBeVisible({ timeout: 15_000 });
    // 失败后恢复「已停用」状态与操作入口，可重试
    await expect(row.getByText('已停用')).toBeVisible();
    await expect(row.getByRole('button', { name: '清退删除' })).toBeVisible();
  });
});
