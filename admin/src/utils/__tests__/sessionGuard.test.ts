import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  AUTH_REFRESH_TOKEN_KEY,
  AUTH_SESSION_MODE_KEY,
  AUTH_TOKEN_EXPIRES_KEY,
  AUTH_TOKEN_KEY,
} from '@/constants/auth';
import {
  canAttemptRefresh,
  clearSessionCredentials,
  isAuthUrl,
  resetRefreshFailureCooldown,
  refreshAccessToken,
  registerReloginHandler,
  requireRelogin,
  saveSessionCredentials,
  shouldRefreshSoon,
  TOKEN_REFRESH_THRESHOLD_SECONDS,
} from '@/utils/sessionGuard';

function okRefreshResponse(token = 'new-token', expiresAt = 9999999999) {
  return {
    ok: true,
    json: async () => ({ code: 0, message: 'ok', data: { token, expiresAt, refreshToken: 'rt-2' } }),
  } as Response;
}

describe('sessionGuard', () => {
  beforeEach(() => {
    localStorage.clear();
    registerReloginHandler(null);
    resetRefreshFailureCooldown();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('saveSessionCredentials / clearSessionCredentials', () => {
    it('保存 token、expiresAt、refreshToken 和 sessionMode', () => {
      saveSessionCredentials({
        token: 't1',
        expiresAt: 1234567890,
        refreshToken: 'rt-1',
        sessionMode: 'legacy_local_storage',
      });
      expect(localStorage.getItem(AUTH_TOKEN_KEY)).toBe('t1');
      expect(localStorage.getItem(AUTH_TOKEN_EXPIRES_KEY)).toBe('1234567890');
      expect(localStorage.getItem(AUTH_REFRESH_TOKEN_KEY)).toBe('rt-1');
      expect(localStorage.getItem(AUTH_SESSION_MODE_KEY)).toBe('legacy_local_storage');
    });

    it('secure 模式无 refreshToken 时保留已有 cookie 语义，不覆盖 expiresAt 缺失', () => {
      saveSessionCredentials({ token: 't2' });
      expect(localStorage.getItem(AUTH_TOKEN_KEY)).toBe('t2');
      expect(localStorage.getItem(AUTH_TOKEN_EXPIRES_KEY)).toBeNull();
      expect(localStorage.getItem(AUTH_REFRESH_TOKEN_KEY)).toBeNull();
    });

    it('clearSessionCredentials 清除 token / expiresAt / refreshToken', () => {
      saveSessionCredentials({ token: 't3', expiresAt: 1, refreshToken: 'rt-3' });
      clearSessionCredentials();
      expect(localStorage.getItem(AUTH_TOKEN_KEY)).toBeNull();
      expect(localStorage.getItem(AUTH_TOKEN_EXPIRES_KEY)).toBeNull();
      expect(localStorage.getItem(AUTH_REFRESH_TOKEN_KEY)).toBeNull();
    });
  });

  describe('shouldRefreshSoon', () => {
    const nowMs = 1_700_000_000_000;

    it('未登录时不续期', () => {
      expect(shouldRefreshSoon(nowMs)).toBe(false);
    });

    it('剩余有效期大于阈值时不续期', () => {
      const exp = nowMs / 1000 + TOKEN_REFRESH_THRESHOLD_SECONDS + 60;
      saveSessionCredentials({ token: 't', expiresAt: exp });
      expect(shouldRefreshSoon(nowMs)).toBe(false);
    });

    it('剩余有效期小于阈值时续期', () => {
      const exp = nowMs / 1000 + TOKEN_REFRESH_THRESHOLD_SECONDS - 60;
      saveSessionCredentials({ token: 't', expiresAt: exp });
      expect(shouldRefreshSoon(nowMs)).toBe(true);
    });

    it('已过期时也返回 true', () => {
      saveSessionCredentials({ token: 't', expiresAt: nowMs / 1000 - 10 });
      expect(shouldRefreshSoon(nowMs)).toBe(true);
    });

    it('expiresAt 非法时不续期', () => {
      localStorage.setItem(AUTH_TOKEN_KEY, 't');
      localStorage.setItem(AUTH_TOKEN_EXPIRES_KEY, 'not-a-number');
      expect(shouldRefreshSoon(nowMs)).toBe(false);
    });
  });

  describe('isAuthUrl', () => {
    it('auth 链路 URL 不参与 401 重试', () => {
      expect(isAuthUrl('/api/v1/auth/login')).toBe(true);
      expect(isAuthUrl('/api/v1/auth/refresh')).toBe(true);
      expect(isAuthUrl('/api/v1/auth/register')).toBe(true);
      expect(isAuthUrl('/api/v1/orders')).toBe(false);
      expect(isAuthUrl('/api/v1/settings')).toBe(false);
    });
  });

  describe('refreshAccessToken', () => {
    it('续期成功时保存新 token / expiresAt / refreshToken', async () => {
      saveSessionCredentials({ token: 'old', expiresAt: 1, refreshToken: 'rt-old' });
      const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(okRefreshResponse());
      await expect(refreshAccessToken()).resolves.toBe(true);
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/v1/auth/refresh',
        expect.objectContaining({ method: 'POST', credentials: 'same-origin' }),
      );
      const body = JSON.parse(String(fetchMock.mock.calls[0][1]?.body));
      expect(body).toEqual({ refreshToken: 'rt-old' });
      expect(localStorage.getItem(AUTH_TOKEN_KEY)).toBe('new-token');
      expect(localStorage.getItem(AUTH_TOKEN_EXPIRES_KEY)).toBe('9999999999');
      expect(localStorage.getItem(AUTH_REFRESH_TOKEN_KEY)).toBe('rt-2');
    });

    it('并发调用共享同一次续期请求（single-flight）', async () => {
      const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(okRefreshResponse());
      const [a, b, c] = await Promise.all([
        refreshAccessToken(),
        refreshAccessToken(),
        refreshAccessToken(),
      ]);
      expect([a, b, c]).toEqual([true, true, true]);
      expect(fetchMock).toHaveBeenCalledTimes(1);
    });

    it('后端返回 401 时续期失败且不改写本地 token', async () => {
      saveSessionCredentials({ token: 'old', expiresAt: 1 });
      vi.spyOn(globalThis, 'fetch').mockResolvedValue({
        ok: false,
        json: async () => ({ code: 401, message: 'refresh token revoked' }),
      } as Response);
      await expect(refreshAccessToken()).resolves.toBe(false);
      expect(localStorage.getItem(AUTH_TOKEN_KEY)).toBe('old');
    });

    it('网络异常时续期失败', async () => {
      vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('network down'));
      await expect(refreshAccessToken()).resolves.toBe(false);
    });
  });

  describe('canAttemptRefresh', () => {
    it('legacy 模式且无 refreshToken 时不尝试续期', () => {
      localStorage.setItem('trademind_auth_session_mode', 'legacy_local_storage');
      expect(canAttemptRefresh()).toBe(false);
    });

    it('legacy 模式但有 refreshToken 时可续期', () => {
      saveSessionCredentials({
        token: 't',
        refreshToken: 'rt',
        sessionMode: 'legacy_local_storage',
      });
      expect(canAttemptRefresh()).toBe(true);
    });

    it('续期失败后进入冷却期，不重复敲 /auth/refresh', async () => {
      const fetchMock = vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('down'));
      await expect(refreshAccessToken()).resolves.toBe(false);
      await expect(refreshAccessToken()).resolves.toBe(false);
      expect(fetchMock).toHaveBeenCalledTimes(1);
    });
  });

  describe('requireRelogin', () => {
    it('未注册弹窗 handler 时直接返回 false', async () => {
      await expect(requireRelogin()).resolves.toBe(false);
    });

    it('并发 401 共享同一个重登弹窗（single-flight）', async () => {
      const handler = vi.fn(
        () => new Promise<boolean>((resolve) => setTimeout(() => resolve(true), 0)),
      );
      registerReloginHandler(handler);
      const [a, b] = await Promise.all([requireRelogin(), requireRelogin()]);
      expect([a, b]).toEqual([true, true]);
      expect(handler).toHaveBeenCalledTimes(1);
    });

    it('用户放弃重登时返回 false', async () => {
      registerReloginHandler(() => Promise.resolve(false));
      await expect(requireRelogin()).resolves.toBe(false);
    });
  });
});
