import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';

const userList = {
  list: [
    {
      id: 'e2e-user-target',
      username: 'e2e-op',
      email: 'e2e-op@example.test',
      displayName: 'E2E 运营',
      role: 'operator',
      status: 'active',
      storePermissions: [],
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:00Z',
    },
  ],
  pagination: { page: 1, pageSize: 20, total: 1, totalPages: 1 },
};

async function setupUsersPage(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/admin/users?*', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok(userList)),
      });
      return;
    }
    await route.fallback();
  });
}

test.describe('@settings 用户管理改密码（r83）', () => {
  test('改密码弹窗：取消不发请求，确认恰好一次且 payload 正确', async ({ page, admin }) => {
    await setupUsersPage(page);
    admin.writeGuard.allow({
      operation: 'resetPassword',
      method: 'POST',
      path: /^\/api\/v1\/admin\/users\/e2e-user-target\/reset-password$/,
      response: ok({ ok: true }),
    });

    await admin.goto('/settings/users');
    const row = page.getByRole('row', { name: /E2E 运营/ });
    await row.getByRole('button', { name: '改密码' }).click();
    await expect(page.getByText('修改密码 — E2E 运营')).toBeVisible();

    // 先取消：不发写请求
    await page.getByRole('button', { name: '取 消' }).click();
    await admin.writeGuard.expectRequestCount('resetPassword', 0);

    // 再确认：校验最少 6 位后恰好一次写请求
    await row.getByRole('button', { name: '改密码' }).click();
    const dialog = page.getByRole('dialog').filter({ hasText: '修改密码' });
    await dialog.getByPlaceholder('至少 6 位').fill('123');
    await dialog.getByRole('button', { name: '确认修改' }).click();
    await expect(page.getByText('密码至少 6 位')).toBeVisible();
    await admin.writeGuard.expectRequestCount('resetPassword', 0);

    await dialog.getByPlaceholder('至少 6 位').fill('e2e-new-secret');
    await dialog.getByRole('button', { name: '确认修改' }).click();
    await admin.writeGuard.expectRequestCount('resetPassword', 1);
    const [call] = admin.writeGuard.calls('resetPassword');
    expect(call.postDataJSON).toEqual({ password: 'e2e-new-secret' });
  });
});
