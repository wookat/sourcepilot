package platformtenant

import (
	"errors"
	"strconv"
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

func tenantIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id < 0 {
		response.Fail(c, 400, response.CodeBadRequest, "租户编号无效")
		return 0, false
	}
	return id, true
}

func handleTenantError(c *gin.Context, err error) {
	if errors.Is(err, ErrTenantNotFound) {
		response.Fail(c, 404, response.CodeNotFound, err.Error())
		return
	}
	response.Fail(c, 400, response.CodeBadRequest, err.Error())
}

// RenameBody renames a tenant.
type RenameBody struct {
	Name string `json:"name"`
}

// Rename PUT /api/v1/platform/tenants/:id
func (h *Handler) Rename(c *gin.Context) {
	if !h.requirePlatformAdmin(c) {
		return
	}
	id, ok := tenantIDParam(c)
	if !ok {
		return
	}
	var body RenameBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "请求参数无效")
		return
	}
	row, err := h.Svc.Rename(c, id, body.Name, actorUUID(c))
	if err != nil {
		handleTenantError(c, err)
		return
	}
	response.OK(c, row)
}

// Disable POST /api/v1/platform/tenants/:id/disable
func (h *Handler) Disable(c *gin.Context) {
	h.setStatus(c, StatusDisabled)
}

// Enable POST /api/v1/platform/tenants/:id/enable
func (h *Handler) Enable(c *gin.Context) {
	h.setStatus(c, StatusActive)
}

// PurgeBody confirms a tenant purge by exact tenant name.
type PurgeBody struct {
	ConfirmName string `json:"confirmName"`
}

// Purge POST /api/v1/platform/tenants/:id/purge
func (h *Handler) Purge(c *gin.Context) {
	if !h.requirePlatformAdmin(c) {
		return
	}
	id, ok := tenantIDParam(c)
	if !ok {
		return
	}
	var body PurgeBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "请求参数无效")
		return
	}
	dto, err := h.Svc.StartPurge(c, id, body.ConfirmName, actorUUID(c))
	if err != nil {
		handleTenantError(c, err)
		return
	}
	response.OK(c, dto)
}

// PurgeStatus GET /api/v1/platform/tenants/:id/purge
func (h *Handler) PurgeStatus(c *gin.Context) {
	if !h.requirePlatformAdmin(c) {
		return
	}
	id, ok := tenantIDParam(c)
	if !ok {
		return
	}
	dto, err := h.Svc.LatestPurge(c, id)
	if err != nil {
		handleTenantError(c, err)
		return
	}
	response.OK(c, dto)
}

func (h *Handler) setStatus(c *gin.Context, status string) {
	if !h.requirePlatformAdmin(c) {
		return
	}
	id, ok := tenantIDParam(c)
	if !ok {
		return
	}
	row, err := h.Svc.SetStatus(c, id, status, actorUUID(c))
	if err != nil {
		handleTenantError(c, err)
		return
	}
	response.OK(c, row)
}
