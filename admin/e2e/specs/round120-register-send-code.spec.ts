import { test, expect } from '../fixtures/admin.fixture';

/**
 * R120 QA v16：注册页「获取验证码」回归
 * 1. 注册表单邮箱值必须随请求体发送（登录/注册 Form 切换不得复用同一实例导致取值为空）；
 * 2. 后端 503 结构化中文指引（如 SMTP 未配置）必须透传给用户，而非通用「发送失败」。
 */
test.describe('@round120-register-send-code 注册获取验证码', () => {
  test('it should send the register email and surface backend 503 guidance', async ({
    admin,
    page,
  }) => {
    admin.consoleGuard.allowForTest(/status of 503/);
    const guidance =
      '邮件服务未配置，无法发送注册验证码。请管理员在「设置 → 邮件设置」完成 SMTP 配置。';
    let sentBody = '';
    await page.route('**/api/v1/auth/send-email-code', async (route) => {
      sentBody = route.request().postData() ?? '';
      await route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: JSON.stringify({ code: 50301, message: guidance, data: null }),
      });
    });

    await page.addInitScript(() => window.localStorage.removeItem('trademind_admin_token'));
    await page.goto('/user/login');
    await page.getByRole('tab', { name: '注册' }).click();
    await page.getByPlaceholder('请输入邮箱').fill('qa-regression@example.com');
    await page.getByRole('button', { name: '获取验证码' }).click();

    await expect(page.getByText(guidance)).toBeVisible();
    expect(JSON.parse(sentBody)).toMatchObject({
      email: 'qa-regression@example.com',
      scene: 'register',
    });
  });
});
