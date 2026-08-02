import { afterEach, describe, expect, it } from 'vitest';
import { getCollectorProxyOption, resolveCollectorUserAgent } from '../launch-options.js';

const KEYS = [
  'COLLECTOR_PROXY_SERVER',
  'COLLECTOR_PROXY_USERNAME',
  'COLLECTOR_PROXY_PASSWORD',
  'COLLECTOR_PROXY_BYPASS',
  'COLLECTOR_USER_AGENT',
] as const;
const original = Object.fromEntries(KEYS.map((key) => [key, process.env[key]]));

afterEach(() => {
  for (const key of KEYS) {
    const value = original[key];
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
});

describe('getCollectorProxyOption', () => {
  it('returns undefined without COLLECTOR_PROXY_SERVER', () => {
    delete process.env.COLLECTOR_PROXY_SERVER;
    expect(getCollectorProxyOption()).toBeUndefined();

    process.env.COLLECTOR_PROXY_SERVER = '   ';
    expect(getCollectorProxyOption()).toBeUndefined();
  });

  it('builds proxy option from env', () => {
    process.env.COLLECTOR_PROXY_SERVER = 'http://proxy.local:8888';
    process.env.COLLECTOR_PROXY_USERNAME = 'u';
    process.env.COLLECTOR_PROXY_PASSWORD = 'p';
    process.env.COLLECTOR_PROXY_BYPASS = 'localhost,127.0.0.1';
    expect(getCollectorProxyOption()).toEqual({
      server: 'http://proxy.local:8888',
      username: 'u',
      password: 'p',
      bypass: 'localhost,127.0.0.1',
    });
  });

  it('omits optional fields when unset', () => {
    process.env.COLLECTOR_PROXY_SERVER = 'socks5://proxy.local:1080';
    delete process.env.COLLECTOR_PROXY_USERNAME;
    delete process.env.COLLECTOR_PROXY_PASSWORD;
    delete process.env.COLLECTOR_PROXY_BYPASS;
    expect(getCollectorProxyOption()).toEqual({ server: 'socks5://proxy.local:1080' });
  });
});

describe('resolveCollectorUserAgent', () => {
  it('prefers COLLECTOR_USER_AGENT override without probing', async () => {
    process.env.COLLECTOR_USER_AGENT = 'CustomUA/1.0';
    await expect(resolveCollectorUserAgent()).resolves.toBe('CustomUA/1.0');
  });
});
