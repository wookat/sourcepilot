import { test, expect } from '../fixtures/admin.fixture';
import { ok, fail } from '../mocks/envelope';
import { expectNoRootOverflow } from '../utils/assertions';

const CONV_ID = 'e2e-conv-fallback-1';

async function mockConversationFailure(
  page: import('@playwright/test').Page,
  status: number,
  message: string,
) {
  await page.route(`**/api/v1/customer/conversations/${CONV_ID}**`, async (route) => {
    const request = route.request();
    if (request.method() !== 'GET') {
      await route.fallback();
      return;
    }
    const path = new URL(request.url()).pathname;
    if (path.endsWith('/messages') || path.endsWith('/ai-suggestions')) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ list: [] })),
      });
      return;
    }
    await route.fulfill({
      status,
      contentType: 'application/json',
      body: JSON.stringify(fail(message, status)),
    });
  });
}

test.describe('@regression 客服会话详情加载兜底', () => {
  test('越权访问（403）显示无权访问语义页且无红屏', async ({ admin, page }) => {
    admin.consoleGuard.allowForTest(/status of 403/);
    await mockConversationFailure(page, 403, '无权访问该资源');
    await admin.goto(`/customer/conversations/${CONV_ID}`);
    await expect(page.getByText('无权访问该会话')).toBeVisible();
    await expect(page.getByRole('button', { name: '返回会话列表' })).toBeVisible();
    await expect(page.getByRole('button', { name: /重\s*试/ })).toHaveCount(0);
    await expectNoRootOverflow(page);
  });

  test('会话不存在（404）显示不存在语义页', async ({ admin, page }) => {
    admin.consoleGuard.allowForTest(/status of 404/);
    await mockConversationFailure(page, 404, '资源不存在');
    await admin.goto(`/customer/conversations/${CONV_ID}`);
    await expect(page.getByText('会话不存在或已被删除')).toBeVisible();
    await expect(page.getByRole('button', { name: '返回会话列表' })).toBeVisible();
  });

  test('服务异常（500）显示加载失败并提供重试', async ({ admin, page }) => {
    admin.consoleGuard.allowForTest(/status of 500/);
    await mockConversationFailure(page, 500, '服务器繁忙');
    await admin.goto(`/customer/conversations/${CONV_ID}`);
    await expect(page.getByText('会话加载失败')).toBeVisible();
    await expect(page.getByRole('button', { name: /重\s*试/ })).toBeVisible();
  });

  test('返回会话列表按钮跳转列表页', async ({ admin, page }) => {
    admin.consoleGuard.allowForTest(/status of 404/);
    await mockConversationFailure(page, 404, '资源不存在');
    await page.route('**/api/v1/customer/conversations?**', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(ok({ list: [], pagination: { page: 1, pageSize: 20, total: 0, totalPages: 1 } })),
      });
    });
    await admin.goto(`/customer/conversations/${CONV_ID}`);
    await page.getByRole('button', { name: '返回会话列表' }).click();
    await expect(page).toHaveURL(/\/customer\/conversations$/);
  });
});
