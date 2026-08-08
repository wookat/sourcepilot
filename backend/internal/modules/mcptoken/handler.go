package mcptoken

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler serves /mcp/tokens management routes.
type Handler struct {
	Svc   *Service
	OpLog *operationlog.Service
}

func adminUUID(c *gin.Context) *uuid.UUID {
	if v, ok := c.Get(ctxkey.AdminID); ok {
		if s, ok := v.(string); ok {
			if u, err := uuid.Parse(strings.TrimSpace(s)); err == nil {
				return &u
			}
		}
	}
	return nil
}

// tokenView is the masked API shape; hash and plaintext never appear.
type tokenView struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	MaskedToken string    `json:"maskedToken"`
	Scope       string    `json:"scope"`
	Purpose     string    `json:"purpose"`
	Revoked     bool      `json:"revoked"`
	Expired     bool      `json:"expired"`
	CreatedAt   string    `json:"createdAt"`
	ExpiresAt   string    `json:"expiresAt,omitempty"`
	LastUsedAt  string    `json:"lastUsedAt,omitempty"`
	RevokedAt   string    `json:"revokedAt,omitempty"`
}

func toView(t Token) tokenView {
	v := tokenView{
		ID:          t.ID,
		Name:        t.Name,
		MaskedToken: t.Masked(),
		Scope:       t.Scope,
		Purpose:     normPurpose(t.Purpose),
		Revoked:     t.RevokedAt != nil,
		Expired:     t.Expired(time.Now().UTC()),
		CreatedAt:   t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if t.ExpiresAt != nil {
		v.ExpiresAt = t.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	if t.LastUsedAt != nil {
		v.LastUsedAt = t.LastUsedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	if t.RevokedAt != nil {
		v.RevokedAt = t.RevokedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	return v
}

// List GET /mcp/tokens
func (h *Handler) List(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "mcp tokens unavailable")
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant scope required")
		return
	}
	rows, err := h.Svc.List(c.Request.Context(), tid)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	// write:ops tokens are admin-governed: they never appear to operator /
	// readonly accounts (fail closed when the principal cannot be resolved).
	admin := h.callerIsAdmin(c)
	items := make([]tokenView, 0, len(rows))
	for _, r := range rows {
		if !admin && r.HasScope(ScopeWriteOps) {
			continue
		}
		items = append(items, toView(r))
	}
	response.OK(c, gin.H{"items": items})
}

// normPurpose maps legacy empty purposes onto mcp for display.
func normPurpose(p string) string {
	if p == "" {
		return PurposeMCP
	}
	return p
}

// CreateBody names a new token; ExpiresInDays is optional (0 = never
// expires for readonly, default 30 days for write:ops); Purpose is optional
// (mcp/openapi/both, default mcp). Scopes is optional (default readonly);
// write:ops is admin-only and only grantable at creation time.
type CreateBody struct {
	Name          string   `json:"name"`
	Purpose       string   `json:"purpose,omitempty"`
	Scopes        []string `json:"scopes,omitempty"`
	ExpiresInDays int      `json:"expiresInDays,omitempty"`
}

// callerIsAdmin resolves the caller's role; any resolution failure counts as
// non-admin (fail closed).
func (h *Handler) callerIsAdmin(c *gin.Context) bool {
	p, err := adminperm.LoadPrincipal(c, h.Svc.DB)
	return err == nil && p.IsAdmin()
}

func scopesInclude(scopes []string, want string) bool {
	for _, s := range scopes {
		if strings.TrimSpace(s) == want {
			return true
		}
	}
	return false
}

// maxExpiresInDays bounds the optional token lifetime (2 years).
const maxExpiresInDays = 730

// Create POST /mcp/tokens — returns the plaintext token exactly once.
func (h *Handler) Create(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "mcp tokens unavailable")
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant scope required")
		return
	}
	var body CreateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	if body.ExpiresInDays < 0 || body.ExpiresInDays > maxExpiresInDays {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest,
			"有效期天数非法：可选 0（不过期）或 1-730 天")
		return
	}
	wantsWrite := scopesInclude(body.Scopes, ScopeWriteOps)
	if wantsWrite {
		// D4: only admin may govern write tokens — operator/readonly get 403
		// even though they can still create plain readonly tokens.
		p, perr := adminperm.LoadPrincipal(c, h.Svc.DB)
		if perr != nil || !p.IsAdmin() {
			response.Fail(c, http.StatusForbidden, response.CodeForbidden,
				"仅管理员可创建带 write:ops 作用域的 token")
			return
		}
		if body.ExpiresInDays > WriteTokenMaxExpiryDays {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest,
				"write:ops token 有效期非法：可选 0（默认 30 天）或 1-90 天，不支持不过期")
			return
		}
	}
	if p := strings.TrimSpace(body.Purpose); p != "" && !ValidPurpose(p) {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest,
			"token 用途非法：可选 mcp（MCP 只读）/ openapi（开放 API）/ both（两者）")
		return
	}
	var expiresAt *time.Time
	if body.ExpiresInDays > 0 {
		t := time.Now().UTC().Add(time.Duration(body.ExpiresInDays) * 24 * time.Hour)
		expiresAt = &t
	}
	res, err := h.Svc.CreateScoped(c.Request.Context(), tid, body.Name, body.Purpose, body.Scopes, expiresAt, adminUUID(c))
	if err != nil {
		if errors.Is(err, ErrTooManyTokens) {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest,
				"活跃 token 数量已达上限（20 个），请先吊销不再使用的 token")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	h.log(c, "mcp_token_create", res.Token.ID.String(),
		"创建 token（作用域 "+res.Token.Scope+"，用途 "+normPurpose(res.Token.Purpose)+"）："+res.Token.Name)
	response.OK(c, gin.H{
		"token": toView(res.Token),
		// plaintext is shown once; clients must store it themselves.
		"plaintext": res.Plaintext,
	})
}

// Revoke POST /mcp/tokens/:id/revoke
func (h *Handler) Revoke(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "mcp tokens unavailable")
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant scope required")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
		return
	}
	// D4 (R180): write:ops tokens are admin-governed — operator / readonly
	// accounts can neither see nor revoke them (404 keeps them invisible).
	if !h.callerIsAdmin(c) {
		var existing Token
		if ferr := h.Svc.DB.WithContext(c.Request.Context()).
			First(&existing, "id = ? AND tenant_id = ?", id, tid).Error; ferr == nil && existing.HasScope(ScopeWriteOps) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "token not found")
			return
		}
	}
	row, err := h.Svc.Revoke(c.Request.Context(), tid, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "token not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	kind := "MCP 只读"
	if row.HasScope(ScopeWriteOps) {
		kind = "MCP 写（write:ops）"
	}
	h.log(c, "mcp_token_revoke", row.ID.String(), "吊销 "+kind+" token："+row.Name)
	response.OK(c, gin.H{"token": toView(*row)})
}

func (h *Handler) log(c *gin.Context, action, resourceID, msg string) {
	if h == nil || h.OpLog == nil {
		return
	}
	_ = h.OpLog.Write(c, operationlog.WriteOpts{
		AdminUserID: adminUUID(c),
		Action:      action,
		Resource:    "mcp_token",
		ResourceID:  resourceID,
		Status:      "success",
		Message:     msg,
	})
}
