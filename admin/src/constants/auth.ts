/** localStorage key for JWT (Authorization: Bearer) — legacy mode only */
export const AUTH_TOKEN_KEY = 'trademind_admin_token';
/** Session mode from login response */
export const AUTH_SESSION_MODE_KEY = 'trademind_auth_session_mode';
export const AUTH_SESSION_SECURE = 'secure_session';
export const AUTH_SESSION_LEGACY = 'legacy_local_storage';
/** Access token expiry (unix seconds), used for proactive silent refresh */
export const AUTH_TOKEN_EXPIRES_KEY = 'trademind_admin_token_expires_at';
/** Refresh token — legacy mode only (secure mode uses HttpOnly cookie) */
export const AUTH_REFRESH_TOKEN_KEY = 'trademind_admin_refresh_token';
