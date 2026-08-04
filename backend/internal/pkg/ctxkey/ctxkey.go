package ctxkey

// TraceID is the *gin.Context key for the request correlation id (see middleware.RequestID).
const TraceID = "trace_id"

// AdminID holds the authenticated admin UUID string (*gin.Context key).
const AdminID = "admin_id"

// AdminUsername holds the JWT username claim (*gin.Context key).
const AdminUsername = "admin_username"

// TenantID holds the authenticated tenant id (*gin.Context key).
const TenantID = "tenant_id"

// SessionID holds the authenticated session id (*gin.Context key).
const SessionID = "session_id"

// AuthStateBridged marks a request whose authentication passed through the
// last-known-good snapshot because the database was unreachable
// (*gin.Context key, bool). Handler-level 5xx failures on such requests are
// rewritten to the AUTH_STATE_UNAVAILABLE/503 retryable contract.
const AuthStateBridged = "auth_state_bridged"
