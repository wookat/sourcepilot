package customerchat

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// ListTemplates GET /api/v1/customer/reply-templates
func (h *Handler) ListTemplates(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	q := TemplateListQuery{
		GroupKey: c.Query("group"),
		Keyword:  c.Query("keyword"),
		Enabled:  parseTriBoolQuery(c.Query("enabled")),
	}
	rows, err := h.Svc.ListTemplates(c, q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"list":     rows,
		"canWrite": adminperm.CanWriteCustomer(c, h.Svc.DB),
	})
}

// CreateTemplate POST /api/v1/customer/reply-templates
func (h *Handler) CreateTemplate(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	if !adminperm.CanWriteCustomer(c, h.Svc.DB) {
		response.Fail(c, 403, response.CodeForbidden, "readonly 账号不可创建话术模板")
		return
	}
	var body TemplateUpsertBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.CreateTemplate(c, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// UpdateTemplate PUT /api/v1/customer/reply-templates/:id
func (h *Handler) UpdateTemplate(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	if !adminperm.CanWriteCustomer(c, h.Svc.DB) {
		response.Fail(c, 403, response.CodeForbidden, "readonly 账号不可编辑话术模板")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body TemplateUpsertBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.UpdateTemplate(c, id, body, adminUUID(c))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// DeleteTemplate DELETE /api/v1/customer/reply-templates/:id
func (h *Handler) DeleteTemplate(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	if !adminperm.CanWriteCustomer(c, h.Svc.DB) {
		response.Fail(c, 403, response.CodeForbidden, "readonly 账号不可删除话术模板")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.Svc.DeleteTemplate(c, id, adminUUID(c)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// ReorderTemplates POST /api/v1/customer/reply-templates/reorder
func (h *Handler) ReorderTemplates(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return
	}
	if !adminperm.CanWriteCustomer(c, h.Svc.DB) {
		response.Fail(c, 403, response.CodeForbidden, "readonly 账号不可调整话术模板排序")
		return
	}
	var body ReorderTemplatesBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	if err := h.Svc.ReorderTemplates(c, body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}
