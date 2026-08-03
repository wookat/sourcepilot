package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authcookie"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

// BearerAuth validates JWT and populates admin + tenant context.
func BearerAuth(cfg *config.Config) gin.HandlerFunc {
	return BearerAuthWithDB(cfg, nil, nil)
}

// BearerAuthWithDB validates JWT and optionally checks session state.
func BearerAuthWithDB(cfg *config.Config, db *gorm.DB, sessions *auth.SessionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg == nil {
			response.Fail(c, 500, response.CodeInternalError, "auth misconfigured")
			c.Abort()
			return
		}
		h := c.GetHeader("Authorization")
		if h == "" {
			response.Fail(c, 401, response.CodeUnauthorized, auth.ErrAuthenticationRequired)
			c.Abort()
			return
		}
		parts := strings.SplitN(h, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Fail(c, 401, response.CodeUnauthorized, "invalid authorization header")
			c.Abort()
			return
		}
		raw := strings.TrimSpace(parts[1])
		claims, err := auth.ParseAccessToken(cfg, nil, raw)
		if err != nil {
			code := auth.ErrAuthenticationRequired
			if strings.Contains(strings.ToLower(err.Error()), "expired") {
				code = auth.ErrAccessTokenExpired
			}
			response.Fail(c, 401, response.CodeUnauthorized, code)
			c.Abort()
			return
		}
		uid, err := uuid.Parse(strings.TrimSpace(claims.Subject))
		if err != nil || uid == uuid.Nil {
			response.Fail(c, 401, response.CodeUnauthorized, "invalid token subject")
			c.Abort()
			return
		}
		sessID := uuid.Nil
		if s := strings.TrimSpace(claims.SessionID); s != "" {
			sessID, _ = uuid.Parse(s)
		}
		if cfg.UsesSecureSession() && sessID == uuid.Nil {
			response.Fail(c, 401, response.CodeUnauthorized, auth.ErrSessionBindingRequired)
			c.Abort()
			return
		}
		if sessions != nil && sessID != uuid.Nil {
			if err := sessions.ValidateSessionAccess(c.Request.Context(), sessID, uid, claims.TokenVersion); err != nil {
				response.Fail(c, 401, response.CodeUnauthorized, err.Error())
				c.Abort()
				return
			}
		} else if db != nil {
			if err := auth.EnsureTenantActive(c.Request.Context(), db, claims.TenantID); err != nil {
				response.Fail(c, 401, response.CodeUnauthorized, err.Error())
				c.Abort()
				return
			}
		}
		c.Set(ctxkey.AdminID, claims.Subject)
		c.Set(ctxkey.AdminUsername, claims.Username)
		tenantID := claims.TenantID
		authSource := security.AuthSourceAccessToken
		if cfg != nil {
			resolved, src, err := cfg.ResolveRequestTenantID(claims.TenantID)
			if err != nil && IsProductionLike(cfg, claims.TenantID) {
				// Tenant 0 is the platform tenant: its active admins operate
				// platform governance APIs and must not be treated as a
				// legacy tenant fallback. Business tenant scoping still
				// rejects tenant 0 via RequireTenantID.
				if claims.TenantID == 0 && isActivePlatformTenantUser(c, db, uid) {
					err = nil
					authSource = security.AuthSourcePlatformTenant
				} else {
					response.Fail(c, 403, response.CodeForbidden, err.Error())
					c.Abort()
					return
				}
			}
			if resolved > 0 {
				tenantID = resolved
				if src != "" {
					authSource = src
				}
			}
		}
		c.Set(ctxkey.TenantID, tenantID)
		if sessID != uuid.Nil {
			c.Set(ctxkey.SessionID, sessID.String())
		}
		tc := security.BuildTenantContext(c, tenantID, uid, sessID, "", nil, nil)
		tc.AuthSource = authSource
		security.SetGin(c, tc)
		c.Next()
	}
}

// isActivePlatformTenantUser reports whether uid is an active admin_users row
// that belongs to the platform tenant (tenant 0).
func isActivePlatformTenantUser(c *gin.Context, db *gorm.DB, uid uuid.UUID) bool {
	if db == nil || uid == uuid.Nil {
		return false
	}
	var row struct {
		TenantID int64
		Status   string
	}
	if err := db.WithContext(c.Request.Context()).Table("admin_users").
		Select("tenant_id", "status").Where("id = ?", uid).Take(&row).Error; err != nil {
		return false
	}
	return row.TenantID == 0 && strings.EqualFold(strings.TrimSpace(row.Status), "active")
}

func IsProductionLike(cfg *config.Config, rawTenant int64) bool {
	if cfg == nil {
		return false
	}
	return config.IsStagingOrProduction(cfg.AppEnv)
}

// ReadRefreshCookie extracts refresh token from cookie.
func ReadRefreshCookie(c *gin.Context) string {
	return authcookie.ReadRefresh(c)
}

// SetRefreshCookie writes HttpOnly refresh token cookie.
func SetRefreshCookie(c *gin.Context, cfg *config.Config, token string, expires interface{}) {
	// deprecated: use authcookie.SetRefresh
}

// ClearRefreshCookie removes refresh token cookie.
func ClearRefreshCookie(c *gin.Context, cfg *config.Config) {
	authcookie.Clear(c, cfg)
}
