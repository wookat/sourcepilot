import { test, expect } from '../fixtures/admin.fixture';
import { dailyStatsResponse } from '../mocks/reports';

/**
 * R107：AUTH_STATE_UNAVAILABLE（后端 fail-closed 数据库瞬断）前端专门处理——
 * 不强制登出、提示「服务暂时不可用」并自动重试，恢复后无感续用；与 401 重登守卫区分。
 */
test.describe('@round107-auth-state AUTH_STATE_UNAVAILABLE 瞬断重试', () => {
  test('it should auto retry with notice and resume without logout on AUTH_STATE_UNAVAILABLE', async ({
    admin,
    page,
  }) => {
    admin.consoleGuard.allowForTest(/status of 401/);
    let unavailableCalls = 0;
    await page.route('**/api/v1/orders/stats/daily?*', async (route) => {
      const url = new URL(route.request().url());
      const days = Number(url.searchParams.get('days') ?? '30');
      // days=7 首次返回瞬断，模拟数据库不可用后恢复
      if (days === 7 && unavailableCalls === 0) {
        unavailableCalls += 1;
        await route.fulfill({
          status: 401,
          contentType: 'application/json',
          body: JSON.stringify({ code: 40100, message: 'AUTH_STATE_UNAVAILABLE', data: null }),
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

    // 中文瞬断提示出现，且不弹「登录已过期」重登弹窗、不跳登录页
    await expect(page.getByText('服务暂时不可用，正在自动重试')).toBeVisible();
    // 重登弹窗 forceRender 常驻 DOM，断言不可见即未被唤起
    await expect(page.locator('.ant-modal').filter({ hasText: '登录已过期' })).not.toBeVisible();
    await expect(page).not.toHaveURL(/\/user\/login/);

    // 退避重试后恢复，数据无感续用
    await expect(page.getByText('近 7 天合计')).toBeVisible({ timeout: 15_000 });
    expect(unavailableCalls).toBe(1);

    // 凭证未被清除
    const token = await page.evaluate(() => localStorage.getItem('trademind_admin_token'));
    expect(token).toBeTruthy();
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
