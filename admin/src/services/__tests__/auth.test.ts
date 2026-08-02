import { afterEach, describe, expect, it, vi } from 'vitest';
import { fetchProfileWithToken, resolveSessionUser, type LoginResult } from '../auth';

const loginResult: LoginResult = {
  token: 'test-token',
  expiresAt: Date.now() + 3600_000,
  user: { id: 'u1', username: 'u1@example.test', displayName: '用户一' },
};

const profileUser = {
  id: 'u1',
  username: 'u1@example.test',
  displayName: '用户一',
  role: 'readonly',
  permissions: ['product:view'],
  storePermissions: [],
};

function mockFetchOnce(status: number, body: unknown) {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok: status >= 200 && status < 300,
      status,
      json: async () => body,
    }),
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('fetchProfileWithToken', () => {
  it('returns the full profile with role and permissions', async () => {
    mockFetchOnce(200, { code: 0, data: profileUser });

    await expect(fetchProfileWithToken('test-token')).resolves.toEqual(profileUser);
    expect(fetch).toHaveBeenCalledWith('/api/v1/auth/profile', {
      headers: { Authorization: 'Bearer test-token' },
    });
  });

  it('returns undefined on non-zero envelope code', async () => {
    mockFetchOnce(200, { code: 401, message: 'unauthorized' });

    await expect(fetchProfileWithToken('test-token')).resolves.toBeUndefined();
  });

  it('returns undefined when the request throws', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('network down')));

    await expect(fetchProfileWithToken('test-token')).resolves.toBeUndefined();
  });
});

describe('resolveSessionUser', () => {
  it('prefers the full profile (with role) over the login user', async () => {
    mockFetchOnce(200, { code: 0, data: profileUser });

    const user = await resolveSessionUser(loginResult);
    expect(user.role).toBe('readonly');
    expect(user.permissions).toEqual(['product:view']);
  });

  it('falls back to the login user when the profile fetch fails', async () => {
    mockFetchOnce(500, { code: 500, message: 'boom' });

    await expect(resolveSessionUser(loginResult)).resolves.toEqual(loginResult.user);
  });
});
