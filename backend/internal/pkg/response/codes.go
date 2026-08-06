package response

// Business-layer result codes (HTTP status may differ).
const (
	CodeOK                            = 0
	CodeBadRequest                    = 40001
	CodeCustomCollectProviderConflict = 40002
	CodeAIRuleInvalid                 = 40003
	CodePublishConfigInvalid          = 40004
	CodeUnauthorized                  = 40101
	CodeForbidden                     = 40301
	CodePermissionDenied              = 40302
	CodeStorePermissionDenied         = 40303
	CodeReadonlyForbidden             = 40304
	CodeSettingsPermissionRequired    = 40305
	CodeUserManagePermissionRequired  = 40306
	CodeNotFound                      = 40401
	CodeTooManyRequests               = 42901
	CodeInternalError                 = 50000
	// CodeServiceUnavailable indicates dependency unavailable (e.g. Redis queue).
	CodeServiceUnavailable = 50301
	// CodeCollectorUnreachable indicates the collector service is not running
	// or cannot be reached (environment issue, not a business error).
	CodeCollectorUnreachable = 50302
)

// MsgAuthStateUnavailable mirrors auth.ErrAuthStateUnavailable (kept local to
// avoid an import cycle): the database is temporarily unreachable and the
// request is safe to retry with backoff. Also used for handler-level 5xx on
// snapshot-bridged requests (503).
const MsgAuthStateUnavailable = "AUTH_STATE_UNAVAILABLE"
