import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const requestMock = vi.hoisted(() => vi.fn());
vi.mock('@umijs/max', () => ({
  request: requestMock,
}));

const messageWarningMock = vi.hoisted(() => vi.fn());
vi.mock('antd', () => ({
  message: { warning: messageWarningMock },
}));

import {
  AUTH_STATE_UNAVAILABLE,
  isAuthStateUnavailable,
  isAuthStateUnavailableResponse,
  resetAuthStateNoticeDedupe,
  retryFetchWhileAuthStateUnavailable,
  retryWhileAuthStateUnavailable,
} from '@/utils/authStateRetry';

function unavailableError(url = '/api/v1/products') {
  return {
    response: { status: 401, data: { code: 401, message: AUTH_STATE_UNAVAILABLE } },
    config: { url, method: 'GET' },
  };
}

function unavailableResponse(): Response {
  return {
    status: 401,
    clone() {
      return this;
    },
    json: async () => ({ code: 401, message: AUTH_STATE_UNAVAILABLE }),
  } as unknown as Response;
}

function okResponse(): Response {
  return {
    status: 200,
    clone() {
      return this;
    },
    json: async () => ({ code: 0, message: 'ok' }),
  } as unknown as Response;
}

describe('authStateRetry', () => {
  beforeEach(() => {
    resetAuthStateNoticeDedupe();
    requestMock.mockReset();
    messageWarningMock.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('isAuthStateUnavailable', () => {
    it('401 + AUTH_STATE_UNAVAILABLE 命中', () => {
      expect(isAuthStateUnavailable(unavailableError())).toBe(true);
    });

    it('普通 401（会话过期）不命中，仍走重登守卫', () => {
      expect(
        isAuthStateUnavailable({
          response: { status: 401, data: { code: 401, message: 'AUTH_SESSION_REVOKED' } },
        }),
      ).toBe(false);
      expect(
        isAuthStateUnavailable({
          response: { status: 401, data: { code: 401, message: 'token expired' } },
        }),
      ).toBe(false);
    });

    it('非 401 不命中', () => {
      expect(
        isAuthStateUnavailable({
          response: { status: 503, data: { message: AUTH_STATE_UNAVAILABLE } },
        }),
      ).toBe(false);
    });
  });

  describe('isAuthStateUnavailableResponse', () => {
    it('命中 401 + AUTH_STATE_UNAVAILABLE 响应体', async () => {
      expect(await isAuthStateUnavailableResponse(unavailableResponse())).toBe(true);
    });

    it('非 401 或普通 401 不命中', async () => {
      expect(await isAuthStateUnavailableResponse(okResponse())).toBe(false);
      const revoked = {
        status: 401,
        clone() {
          return this;
        },
        json: async () => ({ code: 401, message: 'AUTH_SESSION_REVOKED' }),
      } as unknown as Response;
      expect(await isAuthStateUnavailableResponse(revoked)).toBe(false);
    });
  });

  describe('retryWhileAuthStateUnavailable', () => {
    it('恢复后返回重放响应（无感续用），并提示「服务暂时不可用」', async () => {
      const recovered = { data: { code: 0, message: 'ok' } };
      requestMock
        .mockRejectedValueOnce(unavailableError())
        .mockResolvedValueOnce(recovered);

      const out = await retryWhileAuthStateUnavailable(unavailableError(), [1, 1, 1]);
      expect(out).toBe(recovered);
      expect(requestMock).toHaveBeenCalledTimes(2);
      // 重放带 sessionGuardRetry / skipErrorHandler，不再进入拦截器重试链
      expect(requestMock.mock.calls[0][1]).toMatchObject({
        sessionGuardRetry: true,
        skipErrorHandler: true,
      });
      expect(messageWarningMock).toHaveBeenCalledTimes(1);
      expect(String(messageWarningMock.mock.calls[0][0])).toContain('服务暂时不可用');
    });

    it('重试耗尽仍不可用时抛出最后错误（不清凭证、不跳登录页）', async () => {
      localStorage.setItem('trademind_admin_token', 't-keep');
      requestMock.mockRejectedValue(unavailableError());
      await expect(
        retryWhileAuthStateUnavailable(unavailableError(), [1, 1]),
      ).rejects.toMatchObject({
        response: { data: { message: AUTH_STATE_UNAVAILABLE } },
      });
      expect(requestMock).toHaveBeenCalledTimes(2);
      expect(localStorage.getItem('trademind_admin_token')).toBe('t-keep');
    });

    it('重试中变为普通 401 时立即抛出，交回上层守卫', async () => {
      const sessionExpired = {
        response: { status: 401, data: { code: 401, message: 'AUTH_SESSION_REVOKED' } },
        config: { url: '/api/v1/products' },
      };
      requestMock.mockRejectedValueOnce(sessionExpired);
      await expect(
        retryWhileAuthStateUnavailable(unavailableError(), [1, 1, 1]),
      ).rejects.toBe(sessionExpired);
      expect(requestMock).toHaveBeenCalledTimes(1);
    });

    it('提示在去重窗口内只出现一次', async () => {
      requestMock.mockRejectedValue(unavailableError());
      await expect(retryWhileAuthStateUnavailable(unavailableError(), [1])).rejects.toBeTruthy();
      await expect(retryWhileAuthStateUnavailable(unavailableError(), [1])).rejects.toBeTruthy();
      expect(messageWarningMock).toHaveBeenCalledTimes(1);
    });
  });

  describe('retryFetchWhileAuthStateUnavailable', () => {
    it('恢复后返回新响应', async () => {
      const ok = okResponse();
      const doFetch = vi.fn().mockResolvedValueOnce(unavailableResponse()).mockResolvedValueOnce(ok);
      const out = await retryFetchWhileAuthStateUnavailable(doFetch, unavailableResponse(), [1, 1, 1]);
      expect(out).toBe(ok);
      expect(doFetch).toHaveBeenCalledTimes(2);
    });

    it('重试耗尽仍不可用时返回最后一次响应，由调用方按失败处理', async () => {
      const last = unavailableResponse();
      const doFetch = vi.fn().mockResolvedValue(last);
      const out = await retryFetchWhileAuthStateUnavailable(doFetch, unavailableResponse(), [1, 1]);
      expect(out).toBe(last);
      expect(doFetch).toHaveBeenCalledTimes(2);
    });
  });
});
