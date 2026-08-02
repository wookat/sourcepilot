import { chromium } from 'playwright';

export type CollectorProxyOption = {
  server: string;
  username?: string;
  password?: string;
  bypass?: string;
};

/**
 * 采集出口代理（可选，仅读取配置，不内置任何第三方代理服务）。
 * COLLECTOR_PROXY_SERVER 形如 http://host:port 或 socks5://host:port。
 */
export function getCollectorProxyOption(): CollectorProxyOption | undefined {
  const server = process.env.COLLECTOR_PROXY_SERVER?.trim();
  if (!server) return undefined;
  const username = process.env.COLLECTOR_PROXY_USERNAME?.trim();
  const password = process.env.COLLECTOR_PROXY_PASSWORD;
  const bypass = process.env.COLLECTOR_PROXY_BYPASS?.trim();
  return {
    server,
    ...(username ? { username } : {}),
    ...(password ? { password } : {}),
    ...(bypass ? { bypass } : {}),
  };
}

const FALLBACK_UA =
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36';

let cachedUserAgent: string | null = null;
let probePromise: Promise<string> | null = null;

async function probeBundledChromeUserAgent(): Promise<string> {
  const browser = await chromium.launch({ headless: true });
  try {
    // browser.version() 形如 "151.0.7922.34"；UA 与实际内核主版本保持一致，
    // 避免 UA 声称的版本与 Sec-CH-UA / JS 指纹暴露的真实内核不符。
    const major = browser.version().split('.')[0];
    if (!major || !/^\d+$/.test(major)) return FALLBACK_UA;
    return `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/${major}.0.0.0 Safari/537.36`;
  } finally {
    await browser.close().catch(() => undefined);
  }
}

/**
 * 采集上下文 UA：优先 COLLECTOR_USER_AGENT；否则按 bundled Chromium
 * 实际主版本生成（一次探测后缓存），探测失败回退到静态 UA。
 */
export async function resolveCollectorUserAgent(): Promise<string> {
  const override = process.env.COLLECTOR_USER_AGENT?.trim();
  if (override) return override;
  if (cachedUserAgent) return cachedUserAgent;
  if (!probePromise) {
    probePromise = probeBundledChromeUserAgent().catch(() => FALLBACK_UA);
  }
  cachedUserAgent = await probePromise;
  return cachedUserAgent;
}
