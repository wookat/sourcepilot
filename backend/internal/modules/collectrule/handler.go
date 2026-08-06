package collectrule

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

type Handler struct {
	Svc *Service
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

func atoiQP(c *gin.Context, key string, def int) int {
	s := strings.TrimSpace(c.Query(key))
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}

func (h *Handler) List(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "采集规则服务不可用")
		return
	}
	q := ListQuery{
		Page:     atoiQP(c, "page", 1),
		PageSize: atoiQP(c, "pageSize", 20),
		Name:     c.Query("name"),
		Domain:   c.Query("domain"),
		Status:   c.Query("status"),
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	res, err := h.Svc.List(c.Request.Context(), tenantID, q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"list": res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

func (h *Handler) Create(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "采集规则服务不可用")
		return
	}
	var body CreateRuleBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "请求体不是有效 JSON")
		return
	}
	out, err := h.Svc.Create(c, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, localizeRuleError(err))
		return
	}
	response.OK(c, out)
}

func (h *Handler) Get(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "采集规则服务不可用")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "规则 ID 无效")
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	out, err := h.Svc.GetDetail(c.Request.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "采集规则不存在")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) Update(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "采集规则服务不可用")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "规则 ID 无效")
		return
	}
	var body UpdateRuleBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "请求体不是有效 JSON")
		return
	}
	out, err := h.Svc.Update(c, id, body, adminUUID(c))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "采集规则不存在")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, localizeRuleError(err))
		return
	}
	response.OK(c, out)
}

func (h *Handler) Delete(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "采集规则服务不可用")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "规则 ID 无效")
		return
	}
	if err := h.Svc.Delete(c, id, adminUUID(c)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "采集规则不存在")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) Enable(c *gin.Context) {
	h.setStatus(c, StatusEnabled)
}

func (h *Handler) Disable(c *gin.Context) {
	h.setStatus(c, StatusDisabled)
}

func (h *Handler) setStatus(c *gin.Context, status string) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "采集规则服务不可用")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "规则 ID 无效")
		return
	}
	out, err := h.Svc.SetStatus(c, id, status, adminUUID(c))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "采集规则不存在")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, localizeRuleError(err))
		return
	}
	response.OK(c, out)
}

func (h *Handler) Test(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "采集规则服务不可用")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "规则 ID 无效")
		return
	}
	var body TestRuleBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "请求体不是有效 JSON")
		return
	}
	out, err := h.Svc.TestPreview(c, id, body, adminUUID(c))
	if err != nil {
		reason := localizeRuleError(err)
		if strings.Contains(strings.ToLower(err.Error()), "collector rejected") {
			response.Fail(c, 422, response.CodeBadRequest, reason)
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, reason)
		return
	}
	response.OK(c, out)
}
