import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';

const tenantList = {
  list: [
    { id: 0, name: '平台租户（默认）', status: 'active', adminCount: 3 },
    {
      id: 2,
      name: '华东运营中心',
      status: 'active',
      adminCount: 1,
      createdAt: '2026-08-01T00:00:00Z',
    },
    {
      id: 3,
      name: '华南运营中心',
      status: 'disabled',
      adminCount: 2,
      createdAt: '2026-08-02T00:00:00Z',
    },
  ],
};

function mockProfile(role: string, tenantId: number) {
  return { ...e2eUser, role, permissions: [], tenantId };
}

async function setupPlatformAdmin(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/auth/profile', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok(mockProfile('admin', 0))),
    });
  });
  await page.route('**/api/v1/platform/tenants', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok(tenantList)),
      });
      return;
    }
    await route.fallback();
  });
}

test.describe('@settings 平台租户治理（round82）', () => {
  test('平台管理员可改名租户（二次确认 Modal，mock 写请求）', async ({ page, admin }) => {
    await setupPlatformAdmin(page);
    admin.writeGuard.allow({
      operation: 'renameTenant',
      method: 'PUT',
      path: /^\/api\/v1\/platform\/tenants\/2$/,
      response: ok({ id: 2, name: 'e2e-改名后', status: 'active', adminCount: 1 }),
    });

    await admin.goto('/settings/platform-tenants');
    await expect(page.getByText('华东运营中心')).toBeVisible();
    // 平台租户行（id 0）无操作入口
    const platformRow = page.getByRole('row', { name: /平台租户（默认）/ });
    await expect(platformRow.getByRole('button', { name: '改名' })).toHaveCount(0);

    const row = page.getByRole('row', { name: /华东运营中心/ });
    await row.getByRole('button', { name: '改名' }).click();
    await expect(page.getByText('租户改名')).toBeVisible();
    await page.getByLabel('租户名称').fill('e2e-改名后');
    await page.getByRole('button', { name: '确 定' }).click();

    await admin.writeGuard.expectRequestCount('renameTenant', 1);
    const [call] = admin.writeGuard.calls('renameTenant');
    expect(call.postDataJSON).toEqual({ name: 'e2e-改名后' });
  });

  test('平台管理员停用租户需二次确认，取消不发请求（mock 写请求）', async ({ page, admin }) => {
    await setupPlatformAdmin(page);
    admin.writeGuard.allow({
      operation: 'disableTenant',
      method: 'POST',
      path: /^\/api\/v1\/platform\/tenants\/2\/disable$/,
      response: ok({ id: 2, name: '华东运营中心', status: 'disabled', adminCount: 1 }),
    });

    await admin.goto('/settings/platform-tenants');
    const row = page.getByRole('row', { name: /华东运营中心/ });
    await row.getByRole('button', { name: '停用' }).click();
    await expect(page.locator('.ant-modal-confirm-title')).toHaveText('停用租户「华东运营中心」？');
    await expect(
      page.getByText('停用后该租户所有账号将无法登录，已登录会话将在下次请求时失效。'),
    ).toBeVisible();
    // 先取消：不应发出写请求
    await page.getByRole('button', { name: '取 消' }).click();
    await admin.writeGuard.expectRequestCount('disableTenant', 0);

    // 再确认：恰好一次写请求
    await row.getByRole('button', { name: '停用' }).click();
    await page.getByRole('button', { name: '停 用' }).click();
    await admin.writeGuard.expectRequestCount('disableTenant', 1);
  });

  test('平台管理员可启用已停用租户（二次确认，mock 写请求）', async ({ page, admin }) => {
    await setupPlatformAdmin(page);
    admin.writeGuard.allow({
      operation: 'enableTenant',
      method: 'POST',
      path: /^\/api\/v1\/platform\/tenants\/3\/enable$/,
      response: ok({ id: 3, name: '华南运营中心', status: 'active', adminCount: 2 }),
    });

    await admin.goto('/settings/platform-tenants');
    const row = page.getByRole('row', { name: /华南运营中心/ });
    await expect(row.getByText('已停用')).toBeVisible();
    await row.getByRole('button', { name: '启用' }).click();
    await expect(page.locator('.ant-modal-confirm-title')).toHaveText('启用租户「华南运营中心」？');
    await page.getByRole('button', { name: '启 用' }).click();
    await admin.writeGuard.expectRequestCount('enableTenant', 1);
  });

  for (const persona of [
    { name: '非 tenant0 admin', role: 'admin', tenantId: 2 },
    { name: 'tenant0 operator', role: 'operator', tenantId: 0 },
  ]) {
    test(`${persona.name} 不可见治理入口，直达 URL 显示 403`, async ({ page, admin }) => {
      await page.route('**/api/v1/auth/profile', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(ok(mockProfile(persona.role, persona.tenantId))),
        });
      });
      await admin.goto('/settings/platform-tenants');
      await expect(page.getByText('仅平台管理员可管理平台租户')).toBeVisible();
      await expect(page.getByRole('button', { name: '改名' })).toHaveCount(0);
      await expect(page.getByRole('button', { name: '停用' })).toHaveCount(0);
    });
  }
});
