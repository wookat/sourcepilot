import type { Page } from '@playwright/test';
import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { e2eUser } from '../mocks/auth';
import { expectNoRootOverflow } from '../utils/assertions';

// R180 W2: MCP 写白名单后台治理 — 租户 write_enabled 开关（admin-only，默认关，
// 风险确认）、写 token 创建/吊销（operator 不可见）、写审计列表 mode /
// paramsSummary / confirmHash 展示。

const WRITE_TOKEN = {
  id: 'e2e-mcp-write-1',
  name: 'claude-write',
  maskedToken: 'sp_mcp_ro_9a8b…22cc',
  scope: 'readonly,write:ops',
  purpose: 'mcp',
  revoked: false,
  expired: false,
  createdAt: '2026-08-01 10:00:00',
  expiresAt: '2026-08-31 10:00:00',
};

const READ_TOKEN = {
  id: 'e2e-mcp-read-1',
  name: 'claude-desktop',
  maskedToken: 'sp_mcp_ro_1a2b…9f0e',
  scope: 'readonly',
  purpose: 'mcp',
  revoked: false,
  expired: false,
  createdAt: '2026-08-01 10:00:00',
};

const PLAINTEXT = `sp_mcp_ro_${'a1'.repeat(32)}`;

async function routeTokenList(page: Page, rows = [READ_TOKEN, WRITE_TOKEN]) {
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

async function routeSettings(page: Page, writeEnabled: boolean) {
  await page.route('**/api/v1/settings', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(
        ok({
          items: writeEnabled
            ? [{ groupKey: 'mcp', itemKey: 'write_enabled', itemValue: 'true', isEncrypted: false }]
            : [],
        }),
      ),
    });
  });
}

test.describe('@r180 MCP 写白名单治理', () => {
  test('it should show admin-only write card with default-off gate and write token list', async ({
    admin,
    page,
  }) => {
    await routeTokenList(page);
    await routeSettings(page, false);
    await admin.goto('/settings/mcp-tokens');

    const writeCard = page.locator('.ant-card', {
      hasText: 'MCP 写白名单（write:ops，仅管理员）',
    });
    await expect(writeCard.getByText('高风险能力：三层闸门默认全关')).toBeVisible();
    await expect(writeCard.getByRole('switch')).not.toBeChecked();
    // 写 token 只在写卡片列出，只读表格不混排
    await expect(writeCard.getByText('claude-write')).toBeVisible();
    const readCard = page.locator('.ant-card', { hasText: '只读 API 接入（MCP / 开放 API）' });
    await expect(readCard.getByText('claude-write')).toHaveCount(0);

    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should require risk confirmation before enabling the tenant gate', async ({
    admin,
    page,
  }) => {
    admin.writeGuard.allow({
      operation: 'save-write-gate',
      method: 'PUT',
      path: /\/api\/v1\/settings$/,
      response: ok({ items: [] }),
    });
    await routeTokenList(page, []);
    await routeSettings(page, false);
    await admin.goto('/settings/mcp-tokens');

    const writeCard = page.locator('.ant-card', {
      hasText: 'MCP 写白名单（write:ops，仅管理员）',
    });
    await writeCard.getByRole('switch').click();
    // 未确认前不落盘
    await admin.writeGuard.expectRequestCount('save-write-gate', 0);
    const dialog = page.getByRole('dialog', { name: '确认开启本租户的 MCP 写白名单？' });
    await expect(dialog.getByText(/MCP_WRITE_ENABLED/)).toBeVisible();
    await dialog.getByRole('button', { name: '确认开启' }).click();

    await admin.writeGuard.expectRequestCount('save-write-gate', 1);
    const [call] = admin.writeGuard.calls('save-write-gate');
    expect(call.postDataJSON).toEqual({
      items: [
        expect.objectContaining({ groupKey: 'mcp', itemKey: 'write_enabled', itemValue: 'true' }),
      ],
    });
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should create a write token with scopes and show plaintext once', async ({
    admin,
    page,
  }) => {
    admin.writeGuard.allow({
      operation: 'create-write-token',
      method: 'POST',
      path: /\/api\/v1\/mcp\/tokens$/,
      response: ok({ token: WRITE_TOKEN, plaintext: PLAINTEXT }),
    });
    await routeTokenList(page, []);
    await routeSettings(page, false);
    await admin.goto('/settings/mcp-tokens');

    await page.getByRole('button', { name: '创建写 token' }).click();
    const dialog = page.getByRole('dialog', { name: '创建写 token（write:ops）' });
    await expect(dialog.getByText('写 token 可对本租户执行白名单写操作')).toBeVisible();
    await dialog.getByPlaceholder('如 claude-write').fill('claude-write');
    await dialog.getByRole('button', { name: '确认创建' }).click();

    await admin.writeGuard.expectRequestCount('create-write-token', 1);
    const [call] = admin.writeGuard.calls('create-write-token');
    expect(call.postDataJSON).toEqual({ name: 'claude-write', scopes: ['readonly', 'write:ops'] });

    const result = page.getByRole('dialog', { name: 'Token 创建成功' });
    await expect(result.getByText(PLAINTEXT)).toBeVisible();
    await result.getByRole('button', { name: '我已保存' }).click();
    await expect(page.getByText(PLAINTEXT)).toHaveCount(0);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should hide the write card entirely for operator role', async ({ admin, page }) => {
    await page.route('**/api/v1/auth/profile', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ ...e2eUser, role: 'operator', permissions: [] })),
      });
    });
    await routeTokenList(page, [READ_TOKEN]);
    await admin.goto('/settings/mcp-tokens');

    await expect(page.getByText('claude-desktop')).toBeVisible({ timeout: 20000 });
    await expect(page.getByText('MCP 写白名单（write:ops，仅管理员）')).toHaveCount(0);
    await expect(page.getByRole('button', { name: '创建写 token' })).toHaveCount(0);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should render mode, paramsSummary and confirmHash in the audit table', async ({
    admin,
    page,
  }) => {
    await routeTokenList(page, []);
    await routeSettings(page, false);
    await page.route('**/api/v1/mcp/audit-logs**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            items: [
              {
                id: 'e2e-mcp-audit-w1',
                tokenName: 'claude-write',
                tokenMasked: 'sp_mcp_ro_9a8b…22cc',
                tool: 'procurement_mark_placed',
                status: 'success',
                durationMs: 21,
                createdAt: '2026-08-07T01:02:03Z',
                mode: 'execute',
                paramsSummary: 'purchaseOrderId=po-1 externalOrderId=1688-X1',
                confirmHash: 'abcdef0123456789abcdef0123456789',
              },
            ],
            total: 1,
          }),
        ),
      });
    });
    await admin.goto('/settings/mcp-tokens');

    const auditRow = page.locator('.ant-table-tbody tr', { hasText: 'procurement_mark_placed' });
    await expect(auditRow.getByText('execute 执行')).toBeVisible();
    await expect(auditRow.getByText('purchaseOrderId=po-1 externalOrderId=1688-X1')).toBeVisible();
    await expect(auditRow.getByText('abcdef012345…')).toBeVisible();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
