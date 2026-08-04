package auth

import "errors"

// auditTenantError wraps a login failure for a known account so the audit
// layer can attribute the operation log row to the account's tenant.
type auditTenantError struct {
	tenantID int64
	err      error
}

func (e *auditTenantError) Error() string { return e.err.Error() }
func (e *auditTenantError) Unwrap() error { return e.err }

func withAuditTenant(tenantID int64, err error) error {
	if err == nil {
		return nil
	}
	return &auditTenantError{tenantID: tenantID, err: err}
}

// LoginAuditTenant returns the tenant of the account behind a failed login,
// or 0 when the account is unknown (platform-level security audit rows).
func LoginAuditTenant(err error) int64 {
	var ae *auditTenantError
	if errors.As(err, &ae) {
		return ae.tenantID
	}
	return 0
}

// Auth error codes for API responses.
const (
	ErrAccessTokenExpired       = "AUTH_ACCESS_TOKEN_EXPIRED"
	ErrRefreshTokenExpired      = "AUTH_REFRESH_TOKEN_EXPIRED"
	ErrRefreshTokenRevoked      = "AUTH_REFRESH_TOKEN_REVOKED"
	ErrRefreshTokenReused       = "AUTH_REFRESH_TOKEN_REUSED"
	ErrSessionRevoked           = "AUTH_SESSION_REVOKED"
	ErrUserDisabled             = "AUTH_USER_DISABLED"
	ErrTenantDisabled           = "AUTH_TENANT_DISABLED"
	ErrReauthenticationRequired = "AUTH_REAUTHENTICATION_REQUIRED"
	ErrSessionBindingRequired   = "AUTH_SESSION_BINDING_REQUIRED"
	ErrInvalidCredentials       = "AUTH_INVALID_CREDENTIALS"
	ErrAccountTemporarilyLocked = "AUTH_ACCOUNT_TEMPORARILY_LOCKED"
	ErrTooManyAttempts          = "AUTH_TOO_MANY_ATTEMPTS"
	ErrPasswordChangeRequired   = "AUTH_PASSWORD_CHANGE_REQUIRED"
	ErrAuthenticationRequired   = "AUTHENTICATION_REQUIRED"
	ErrAuthStateUnavailable     = "AUTH_STATE_UNAVAILABLE"
	ErrPermissionDenied         = "PERMISSION_DENIED"
	ErrTenantAccessDenied       = "TENANT_ACCESS_DENIED"
	ErrShopAccessDenied         = "SHOP_ACCESS_DENIED"
	ErrSensitiveOperationDenied = "SENSITIVE_OPERATION_DENIED"
	ErrCSRFTokenMissing         = "CSRF_TOKEN_MISSING"
	ErrCSRFTokenInvalid         = "CSRF_TOKEN_INVALID"
	ErrOriginNotAllowed         = "ORIGIN_NOT_ALLOWED"
)
