import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { expectNoRootOverflow } from '../utils/assertions';

const ISO_CREATED_AT = '2026-08-01T02:03:04Z';

function dashboardPayload() {
  return {
    summary: {
      orderExceptions: 0,
      collectFailedCount: 0,
      aiTitleCompletedCount: 0,
      aiDescriptionCompletedCount: 0,
      collectedProductsCount: 100,
      aiTextCompletedCount: 113,
      readinessPassedCount: 40,
    },
    todos: [
      {
        id: 'todo-1',
        key: 'collect_review',
        title: '待检查采集结果',
        count: 3,
        severity: 'high',
        level: 'high',
        description: '',
        link: '/product/drafts',
      },
    ],
    funnel: [
      { key: 'collected', title: '已采集商品', count: 100, link: '/product/drafts' },
      { key: 'ai_text', title: 'AI 文案完成', count: 113, link: '/ai/batches' },
      { key: 'readiness_pass', title: '发布检查通过', count: 40, link: '/product/drafts' },
      { key: 'published', title: '已发布', count: 10, link: '/product/publish-tasks' },
    ],
    exceptions: [],
    charts: {},
    quickLinks: [],
    recent: {},
  };
}

async function mockDashboard(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/dashboard/product-operations**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok(dashboardPayload())),
    });
  });
  await page.route('**/api/v1/orders/stats/sales**', async (route) => {
    if (route.request().method() !== 'GET') {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(ok({ windows: [] })),
    });
  });
}

test.describe('@round63-semantic-visual R63 语义组件与首页信息设计', () => {
  test('首页漏斗转化超过 100% 时封顶展示并另行标注超额', async ({ admin, page }) => {
    await mockDashboard(page);
    await admin.goto('/dashboard');
    await expect(page.getByText('AI 商品运营进度')).toBeVisible();

    // 113/100 → 封顶 100% 并标注超额 +13%，不出现 113% 溢出
    await expect(page.getByText('100% · 超额 +13%')).toBeVisible();
    await expect(page.getByText(/^113%$/)).toHaveCount(0);

    // 进度条宽度封顶 ≤100%
    const overWidth = await page.evaluate(
      () =>
        Array.from(document.querySelectorAll<HTMLElement>('div'))
          .filter((el) => el.style.borderRadius === '999px' && el.style.width.endsWith('%'))
          .map((el) => parseFloat(el.style.width))
          .filter((w) => w > 100).length,
    );
    expect(overWidth).toBe(0);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('首页 375 小屏无根节点横向溢出', async ({ admin, page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await mockDashboard(page);
    await admin.goto('/dashboard');
    await expect(page.getByText('AI 商品运营进度')).toBeVisible();
    await expectNoRootOverflow(page);
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('供应商列表平台字段展示语义 Tag 而非内部枚举', async ({ admin, page }) => {
    await page.route('**/api/v1/suppliers?**', async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            items: [
              {
                id: 'e2e-supplier-r63',
                platform: 'douyin_shop',
                name: 'e2e 抖店供应商',
                status: 'active',
                createdAt: ISO_CREATED_AT,
              },
            ],
            total: 1,
            page: 1,
            pageSize: 20,
          }),
        ),
      });
    });
    await admin.goto('/sourcing/suppliers');
    await expect(page.getByText('e2e 抖店供应商')).toBeVisible();
    await expect(page.locator('.ant-table-tbody').getByText('抖店', { exact: true })).toBeVisible();
    await expect(page.locator('.ant-table-tbody')).not.toContainText('douyin_shop');
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });

  test('任务失败中心平台列展示语义 Tag', async ({ admin, page }) => {
    await page.route('**/api/v1/task-center/failures**', async (route) => {
      if (route.request().method() !== 'GET') {
        await route.fallback();
        return;
      }
      const url = route.request().url();
      if (url.includes('/categories') || url.includes('/summary')) {
        await route.fallback();
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(
          ok({
            list: [
              {
                id: 'e2e-failure-r63',
                taskType: 'publish',
                platform: 'tiktok',
                title: 'e2e 失败任务',
                normalizedStatus: 'failed',
                createdAt: ISO_CREATED_AT,
              },
            ],
            total: 1,
            summary: {},
          }),
        ),
      });
    });
    await admin.goto('/ops/task-center/failures');
    await expect(page.locator('.ant-table-tbody')).toContainText('e2e 失败任务');
    await expect(page.locator('.ant-table-tbody').getByText('TikTok Shop', { exact: true }).first()).toBeVisible();
    await expect(page.locator('.ant-table-tbody')).not.toContainText('tiktok');
    await admin.writeGuard.expectRequestCount('unexpected', 0);
  });
});
