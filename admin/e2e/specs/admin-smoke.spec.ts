import { test, expect } from '../fixtures/admin.fixture';
import { expectNoRootOverflow } from '../utils/assertions';

const smokeRoutes = [
  { path: '/dashboard/product-operations', name: /运营总览|工作台/ },
  { path: '/collect/hub', name: /采集中心/ },
  { path: '/ai/operation-workbench', name: /商品运营工作台/ },
  { path: '/product/drafts', name: /商品草稿|E2E 商品草稿/ },
  { path: '/ops/task-center/alerts', name: /告警中心/ },
  { path: '/files', name: /文件管理/ },
];

test.describe('@smoke Admin route smoke', () => {
  // 冒烟批常为整套 E2E 的首批用例：dev webServer 冷启动按需编译叠加高
  // CPU 竞争时，首个页面首屏可远超默认 8s expect 超时（实测可达 30s+），
  // 放宽首屏等待与用例总超时；通过路径耗时不变，不掩盖真实失败。
  test.describe.configure({ timeout: 120_000 });
  for (const route of smokeRoutes) {
    test(`renders ${route.path} without login, fatal error, or writes`, async ({ admin, page }) => {
      await admin.goto(route.path);
      await expect(page.locator('#root')).toBeVisible({ timeout: 60_000 });
      await expect(page.getByText(route.name).first()).toBeVisible();
      await expect(page).not.toHaveURL(/\/user\/login/);
      await expectNoRootOverflow(page);
      await admin.writeGuard.expectRequestCount('unexpected', 0);
    });
  }
});
