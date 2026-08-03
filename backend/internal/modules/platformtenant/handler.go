package platformtenant

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler serves the platform tenant management HTTP API.
type Handler struct {
	Svc *Service
}

func actorUUID(c *gin.Context) *uuid.UUID {
	if v, ok := c.Get(ctxkey.AdminID); ok {
		if s, ok := v.(string); ok {
			if u, err := uuid.Parse(strings.TrimSpace(s)); err == nil {
				return &u
			}
		}
	}
	return nil
}

// requirePlatformAdmin allows only active admin accounts of the platform
// tenant (tenant 0). Everyone else is rejected with the unified 403 envelope.
func (h *Handler) requirePlatformAdmin(c *gin.Context) bool {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, 500, response.CodeInternalError, "租户管理不可用")
		return false
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil || tid != PlatformTenantID {
		adminperm.DenyPermission(c)
		return false
	}
	p, err := adminperm.LoadPrincipal(c, h.Svc.DB)
	if err != nil || p == nil || !p.IsAdmin() {
		adminperm.DenyPermission(c)
		return false
	}
	return true
}

// List GET /api/v1/platform/tenants
func (h *Handler) List(c *gin.Context) {
	if !h.requirePlatformAdmin(c) {
		return
	}
	rows, err := h.Svc.List(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": rows})
}

// Create POST /api/v1/platform/tenants
func (h *Handler) Create(c *gin.Context) {
	if !h.requirePlatformAdmin(c) {
		return
	}
	var body CreateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "请求参数无效")
		return
	}
	res, err := h.Svc.Create(c, body, actorUUID(c))
	if err != nil {
		if errors.Is(err, ErrDuplicateTenantName) || errors.Is(err, ErrDuplicateAdminEmail) {
			response.Fail(c, 400, response.CodeBadRequest, err.Error())
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, res)
}
