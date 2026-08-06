package mcptoken

import (
	"errors"
	"net/http"
	"strings"

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
	Revoked     bool      `json:"revoked"`
	CreatedAt   string    `json:"createdAt"`
	LastUsedAt  string    `json:"lastUsedAt,omitempty"`
	RevokedAt   string    `json:"revokedAt,omitempty"`
}

func toView(t Token) tokenView {
	v := tokenView{
		ID:          t.ID,
		Name:        t.Name,
		MaskedToken: t.Masked(),
		Scope:       t.Scope,
		Revoked:     t.RevokedAt != nil,
		CreatedAt:   t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
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
	items := make([]tokenView, 0, len(rows))
	for _, r := range rows {
		items = append(items, toView(r))
	}
	response.OK(c, gin.H{"items": items})
}

// CreateBody names a new token.
type CreateBody struct {
	Name string `json:"name"`
}

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
	res, err := h.Svc.Create(c.Request.Context(), tid, body.Name, adminUUID(c))
	if err != nil {
		if errors.Is(err, ErrTooManyTokens) {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest,
				"活跃 token 数量已达上限（20 个），请先吊销不再使用的 token")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	h.log(c, "mcp_token_create", res.Token.ID.String(), "创建 MCP 只读 token："+res.Token.Name)
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
	row, err := h.Svc.Revoke(c.Request.Context(), tid, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "token not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	h.log(c, "mcp_token_revoke", row.ID.String(), "吊销 MCP 只读 token："+row.Name)
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
