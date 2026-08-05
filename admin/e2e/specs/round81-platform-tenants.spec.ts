import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';

const tenantList = {
  list: [
    { id: 0, name: '平台租户（默认）', adminCount: 3 },
    { id: 2, name: '华东运营中心', adminCount: 1, createdAt: '2026-08-01T00:00:00Z' },
  ],
};

function mockProfile(role: string, tenantId: number) {
  return { ...e2eUser, role, permissions: [], tenantId };
}

test.describe('@settings 平台租户页（round81）', () => {
  test('平台管理员可见平台租户页并可创建租户（mock 写请求）', async ({ page, admin }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok(mockProfile('admin', 0))),
      });
    });
    await page.route('**/api/v1/platform/tenants', async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok(tenantList)) });
        return;
      }
      await route.fallback();
    });
    admin.writeGuard.allow({
      operation: 'createTenant',
      method: 'POST',
      path: /^\/api\/v1\/platform\/tenants$/,
      response: ok({
        tenant: { id: 3, name: 'e2e-新租户', adminCount: 1, createdAt: '2026-08-03T00:00:00Z' },
        adminId: 'new-admin-id',
        adminEmail: 'tenant3-admin@example.test',
      }),
    });

    await admin.goto('/settings/platform-tenants');
    await expect(page.getByText('平台租户').first()).toBeVisible();
    await expect(page.getByText('华东运营中心')).toBeVisible();

    await page.getByRole('button', { name: '新建租户' }).first().click();
    await page.getByLabel('租户名称').fill('e2e-新租户');
    await page.getByLabel('初始管理员邮箱').fill('tenant3-admin@example.test');
    await page.getByLabel('初始管理员密码').fill('secret123');
    await page.getByRole('button', { name: '确 定' }).click();

    await admin.writeGuard.expectRequestCount('createTenant', 1);
    const [call] = admin.writeGuard.calls('createTenant');
    expect(call.postDataJSON).toEqual({
      name: 'e2e-新租户',
      adminEmail: 'tenant3-admin@example.test',
      adminPassword: 'secret123',
    });
  });

  for (const persona of [
    { name: '非 tenant0 admin', role: 'admin', tenantId: 2 },
    { name: 'tenant0 operator', role: 'operator', tenantId: 0 },
    { name: 'tenant0 readonly', role: 'readonly', tenantId: 0 },
  ]) {
    test(`${persona.name} 不可见平台租户菜单，直达 URL 显示统一语义页`, async ({ page, admin }) => {
      await page.route('**/api/v1/auth/profile', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(ok(mockProfile(persona.role, persona.tenantId))),
        });
      });
      await admin.goto('/settings/platform-tenants');
      await expect(page.getByText('暂无访问权限')).toBeVisible();
      await expect(page.getByRole('button', { name: '返回工作台' })).toBeVisible();
      await expect(page.getByRole('button', { name: '新建租户' })).toHaveCount(0);
      await expect(page.getByRole('menuitem', { name: '平台租户' })).toHaveCount(0);
    });
  }
});
