import { test, expect } from '../fixtures/admin.fixture';
import { dailyStatsResponse } from '../mocks/reports';

/**
 * R108 P2：
 * 1. 切换账号后旧 token 的迟到 401 不再弹「登录已过期」重登弹窗，直接用当前凭证重放；
 * 2. 停库快照放行后业务查询失败的 503 + AUTH_STATE_UNAVAILABLE 与 401 形态同口径退避重试。
 */
test.describe('@round108-auth-p2 多账号切换与 503 瞬断口径', () => {
  test('it should replay with current token instead of relogin modal on stale-token 401', async ({
    admin,
    page,
  }) => {
    admin.consoleGuard.allowForTest(/status of 401/);
    let staleCalls = 0;
    let restoreToken: string | null = null;
    await page.route('**/api/v1/orders/stats/daily?*', async (route) => {
      const url = new URL(route.request().url());
      const days = Number(url.searchParams.get('days') ?? '30');
      const authz = route.request().headers().authorization ?? '';
      // 模拟后端：旧账号 token 一律 401（会话已随切换失效）；
      // 401 返回前把本地凭证恢复为当前账号 token，模拟切换动线中
      // 旧 token 请求在飞行、401 到达时本地凭证已是新会话
      if (days === 7 && authz.includes('stale-account-token')) {
        staleCalls += 1;
        await page.evaluate((t) => {
          localStorage.setItem('trademind_admin_token', String(t));
        }, restoreToken);
        await route.fulfill({
          status: 401,
          contentType: 'application/json',
          body: JSON.stringify({ code: 40101, message: 'AUTH_SESSION_REVOKED', data: null }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(dailyStatsResponse(days)),
      });
    });

    await admin.goto('/orders/reports');
    await expect(page.getByText('近 30 天合计')).toBeVisible();

    restoreToken = await page.evaluate(() => localStorage.getItem('trademind_admin_token'));
    await page.evaluate(() => {
      localStorage.setItem('trademind_admin_token', 'stale-account-token');
    });
    await page.getByText('近 7 天', { exact: true }).click();

    // 旧 token 401 后用当前凭证重放成功：无重登弹窗、不跳登录页
    await expect(page.getByText('近 7 天合计')).toBeVisible({ timeout: 15_000 });
    await expect(page.locator('.ant-modal').filter({ hasText: '登录已过期' })).not.toBeVisible();
    await expect(page).not.toHaveURL(/\/user\/login/);
    expect(staleCalls).toBeGreaterThanOrEqual(1);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('it should auto retry and resume on 503 AUTH_STATE_UNAVAILABLE', async ({
    admin,
    page,
  }) => {
    admin.consoleGuard.allowForTest(/status of 503/);
    let unavailableCalls = 0;
    await page.route('**/api/v1/orders/stats/daily?*', async (route) => {
      const url = new URL(route.request().url());
      const days = Number(url.searchParams.get('days') ?? '30');
      // days=7 首次返回 503 瞬断（认证经快照放行、业务查询失败的统一改写口径）
      if (days === 7 && unavailableCalls === 0) {
        unavailableCalls += 1;
        await route.fulfill({
          status: 503,
          contentType: 'application/json',
          body: JSON.stringify({ code: 50301, message: 'AUTH_STATE_UNAVAILABLE', data: null }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(dailyStatsResponse(days)),
      });
    });

    await admin.goto('/orders/reports');
    await expect(page.getByText('近 30 天合计')).toBeVisible();

    await page.getByText('近 7 天', { exact: true }).click();

    await expect(page.getByText('服务暂时不可用，正在自动重试')).toBeVisible();
    await expect(page.locator('.ant-modal').filter({ hasText: '登录已过期' })).not.toBeVisible();
    await expect(page).not.toHaveURL(/\/user\/login/);

    await expect(page.getByText('近 7 天合计')).toBeVisible({ timeout: 15_000 });
    expect(unavailableCalls).toBe(1);

    const token = await page.evaluate(() => localStorage.getItem('trademind_admin_token'));
    expect(token).toBeTruthy();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
