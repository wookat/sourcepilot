import { chromium, type FullConfig } from '@playwright/test';

// dev webServer（max dev）按需编译前端 bundle：冷启动后的首次导航可能耗时
// 30s+，导致最先执行的 spec 偶发超时。这里在所有用例前先真实打开一次首页，
// 等 #root 渲染完成再开始跑用例，让首批用例面对的是已编译完成的应用。
export default async function globalSetup(config: FullConfig) {
  const baseURL = (config.projects[0]?.use?.baseURL as string) || 'http://127.0.0.1:8001';
  const browser = await chromium.launch();
  const page = await browser.newPage();
  try {
    await page.goto(baseURL, { waitUntil: 'domcontentloaded', timeout: 120_000 });
    await page.locator('#root').waitFor({ state: 'visible', timeout: 120_000 });
  } catch (e) {
    // 预热失败不阻断用例：用例本身仍会以真实结果为准。
    console.warn(`[e2e warmup] failed: ${(e as Error).message}`);
  } finally {
    await browser.close();
  }
}
