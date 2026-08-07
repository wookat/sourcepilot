import type { Page } from '@playwright/test';
import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import { expectNoRootOverflow } from '../utils/assertions';

const ACTIVE_TOKEN = {
  id: 'e2e-mcp-token-1',
  name: 'claude-desktop',
  maskedToken: 'sp_mcp_ro_1a2b…9f0e',
  scope: 'readonly',
  purpose: 'mcp',
  revoked: false,
  createdAt: '2026-08-01 10:00:00',
  lastUsedAt: '2026-08-05 09:30:00',
};

const REVOKED_TOKEN = {
  id: 'e2e-mcp-token-2',
  name: 'mcp-inspector',
  maskedToken: 'sp_mcp_ro_3c4d…77aa',
  scope: 'readonly',
  purpose: 'both',
  revoked: true,
  createdAt: '2026-07-20 08:00:00',
  revokedAt: '2026-07-30 12:00:00',
};

const PLAINTEXT = `sp_mcp_ro_${'e2'.repeat(32)}`;

async function routeTokenList(page: Page, rows = [ACTIVE_TOKEN, REVOKED_TOKEN]) {
  await page.route('**/api/v1/mcp/tokens', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ items: rows })),
    });
  });
}

test.describe('@round144 MCP 只读接入 token 管理', () => {
  test('it should list masked tokens with scope and status', async ({ admin, page }) => {
    await routeTokenList(page);
    await admin.goto('/settings/mcp-tokens');
    await expect(page.getByText('只读 API 接入（MCP / 开放 API）').first()).toBeVisible();

    const activeRow = page.locator('.ant-table-tbody tr', { hasText: 'claude-desktop' });
    await expect(activeRow.getByText('sp_mcp_ro_1a2b…9f0e')).toBeVisible();
    await expect(activeRow.getByText('只读', { exact: true })).toBeVisible();
    await expect(activeRow.getByText('MCP 只读', { exact: true })).toBeVisible();
    await expect(activeRow.getByText('有效')).toBeVisible();
    await expect(activeRow.getByRole('button', { name: /吊\s*销/ })).toBeEnabled();

    const revokedRow = page.locator('.ant-table-tbody tr', { hasText: 'mcp-inspector' });
    await expect(revokedRow.getByText('MCP + 开放 API')).toBeVisible();
    await expect(revokedRow.getByText('已吊销')).toBeVisible();
    await expect(revokedRow.getByRole('button', { name: /吊\s*销/ })).toHaveCount(0);

    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should create a token and show plaintext exactly once', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'create-mcp-token',
      method: 'POST',
      path: /\/api\/v1\/mcp\/tokens$/,
      response: ok({ token: ACTIVE_TOKEN, plaintext: PLAINTEXT }),
    });
    await routeTokenList(page);
    await admin.goto('/settings/mcp-tokens');

    await page.getByRole('button', { name: '创建只读 token' }).click();
    const dialog = page.getByRole('dialog', { name: '创建只读 token' });
    await dialog.getByPlaceholder('如 claude-desktop').fill('claude-desktop');
    await dialog.getByRole('button', { name: /确\s*定/ }).click();

    await admin.writeGuard.expectRequestCount('create-mcp-token', 1);
    const [call] = admin.writeGuard.calls('create-mcp-token');
    expect(call.postDataJSON).toEqual({ name: 'claude-desktop', purpose: 'mcp' });

    const result = page.getByRole('dialog', { name: 'Token 创建成功' });
    await expect(result.getByText('明文 token 仅展示这一次')).toBeVisible();
    await expect(result.getByText(PLAINTEXT)).toBeVisible();
    await result.getByRole('button', { name: '我已保存' }).click();
    await expect(page.getByText(PLAINTEXT)).toHaveCount(0);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should revoke a token after confirmation', async ({ admin, page }) => {
    admin.writeGuard.allow({
      operation: 'revoke-mcp-token',
      method: 'POST',
      path: /\/api\/v1\/mcp\/tokens\/e2e-mcp-token-1\/revoke$/,
      response: ok({ token: { ...ACTIVE_TOKEN, revoked: true, revokedAt: '2026-08-06 12:00:00' } }),
    });
    await routeTokenList(page);
    await admin.goto('/settings/mcp-tokens');

    const activeRow = page.locator('.ant-table-tbody tr', { hasText: 'claude-desktop' });
    await activeRow.getByRole('button', { name: /吊\s*销/ }).click();
    await expect(page.getByText(/确认吊销 claude-desktop？/)).toBeVisible();
    await page.getByRole('button', { name: /确\s*定/ }).click();

    await admin.writeGuard.expectRequestCount('revoke-mcp-token', 1);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should format ISO datetimes in expiry and audit time columns', async ({
    admin,
    page,
  }) => {
    const expiringToken = {
      ...ACTIVE_TOKEN,
      id: 'e2e-mcp-token-3',
      name: 'iso-expiry-token',
      maskedToken: 'sp_mcp_ro_5e6f…11bb',
      expiresAt: '2030-01-02T03:04:05Z',
      expired: false,
    };
    await routeTokenList(page, [expiringToken]);
    await page.route('**/api/v1/mcp/audit-logs**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            items: [
              {
                id: 'e2e-mcp-audit-1',
                tokenName: 'iso-expiry-token',
                tokenMasked: 'sp_mcp_ro_5e6f…11bb',
                tool: 'orders_query',
                status: 'success',
                durationMs: 12,
                createdAt: '2026-08-06T01:02:03Z',
              },
            ],
            total: 1,
          }),
        ),
      });
    });
    await admin.goto('/settings/mcp-tokens');

    const tokenRow = page.locator('.ant-table-tbody tr', { hasText: 'iso-expiry-token' });
    await expect(tokenRow.getByText(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/).first()).toBeVisible();
    await expect(tokenRow.getByText('2030-01-02T03:04:05Z')).toHaveCount(0);

    const auditRow = page.locator('.ant-table-tbody tr', { hasText: 'orders_query' });
    await expect(auditRow.getByText(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/).first()).toBeVisible();
    await expect(auditRow.getByText('2026-08-06T01:02:03Z')).toHaveCount(0);

    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should disable write entries for readonly role', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'readonly', permissions: [] })),
      });
    });
    await routeTokenList(page);
    await admin.goto('/settings/mcp-tokens');

    await expect(page.getByText('claude-desktop')).toBeVisible({ timeout: 20000 });
    await expect(page.getByRole('button', { name: '创建只读 token' })).toBeDisabled();
    const activeRow = page.locator('.ant-table-tbody tr', { hasText: 'claude-desktop' });
    await expect(activeRow.getByRole('button', { name: /吊\s*销/ })).toBeDisabled();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
