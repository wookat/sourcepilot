import { getJSON, postJSON } from '@/services/request';
import {
  isAuthStateUnavailableResponse,
  retryFetchWhileAuthStateUnavailable,
} from '@/utils/authStateRetry';

export type LoginUser = {
  id: string;
  username: string; // login identity (email or phone)
  email?: string;
  phone?: string;
  displayName: string;
};

export type LoginResult = {
  token: string;
  expiresAt: number;
  user: LoginUser;
  sessionMode?: string;
  refreshToken?: string;
  deprecatedSessionMode?: boolean;
};

/** POST /api/v1/auth/refresh */
export async function refreshSession() {
  return postJSON<{ token: string; expiresAt: number; refreshToken?: string }>(
    '/api/v1/auth/refresh',
    {},
  );
}

/** POST /api/v1/auth/login */
export async function login(account: string, password: string) {
  return postJSON<LoginResult>('/api/v1/auth/login', {
    account,
    password,
  });
}

/** POST /api/v1/auth/send-email-code */
export async function sendEmailCode(email: string, scene: 'register' = 'register') {
  return postJSON<{ ok: boolean }>('/api/v1/auth/send-email-code', {
    email,
    scene,
  });
}

/** POST /api/v1/auth/register（emailVerifyRequired=false 时 code 可为空） */
export async function register(params: { email: string; code?: string; password: string; confirmPassword: string }) {
  return postJSON<LoginResult>('/api/v1/auth/register', params);
}

/** GET /api/v1/auth/register-config */
export async function getRegisterConfig() {
  return getJSON<{ emailVerifyRequired: boolean }>('/api/v1/auth/register-config');
}

export type ProfileUser = LoginUser & {
  createdAt?: string;
  updatedAt?: string;
};

/** GET /api/v1/auth/profile 的启动期结果：authStateUnavailable 表示后端 fail-closed 瞬断，凭证仍有效 */
export type BootstrapProfileResult = {
  user?: API.CurrentUser;
  authStateUnavailable?: boolean;
};

/** GET /api/v1/auth/profile，使用显式 token（登录/续期时机早于请求拦截器注入的 token）；
 * 命中 AUTH_STATE_UNAVAILABLE 时退避重试，耗尽仍不可用则标记 authStateUnavailable，不当作会话失效。 */
export async function fetchProfileWithTokenDetailed(
  token: string,
): Promise<BootstrapProfileResult> {
  try {
    const doFetch = () =>
      fetch('/api/v1/auth/profile', {
        headers: { Authorization: `Bearer ${token}` },
      });
    let res = await doFetch();
    if (await isAuthStateUnavailableResponse(res)) {
      res = await retryFetchWhileAuthStateUnavailable(doFetch, res);
      if (await isAuthStateUnavailableResponse(res)) return { authStateUnavailable: true };
    }
    const json = (await res.json()) as { code: number; data?: API.CurrentUser };
    if (!res.ok || json.code !== 0 || !json.data) return {};
    return { user: json.data };
  } catch {
    return {};
  }
}

/** 兼容入口：只关心能否拿到用户时使用（登录/重登链路） */
export async function fetchProfileWithToken(token: string): Promise<API.CurrentUser | undefined> {
  return (await fetchProfileWithTokenDetailed(token)).user;
}

/**
 * 登录/重登返回的 user 不含 role/permissions/storePermissions，直接写入 initialState
 * 会让 usePermission 在整个 SPA 会话内按默认角色误判（readonly 也会看到写入口）。
 * 写入 initialState 前必须先拉取完整 profile；拉取失败时退回登录返回的用户信息。
 */
export async function resolveSessionUser(data: LoginResult): Promise<API.CurrentUser> {
  const profile = await fetchProfileWithToken(data.token);
  return profile ?? data.user;
}
