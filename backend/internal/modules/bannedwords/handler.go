package bannedwords

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler serves /banned-words and product scan routes.
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

// List GET /banned-words?category=&level=&keyword=&enabled=1
func (h *Handler) List(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "banned words unavailable")
		return
	}
	q := ListQuery{
		Category: c.Query("category"),
		Level:    c.Query("level"),
		Keyword:  c.Query("keyword"),
	}
	if raw := strings.TrimSpace(c.Query("enabled")); raw == "1" || strings.EqualFold(raw, "true") {
		q.EnabledOnly = true
	}
	rows, err := h.Svc.List(c, q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": rows})
}

// Create POST /banned-words
func (h *Handler) Create(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "banned words unavailable")
		return
	}
	var body CreateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.Create(c, body, adminUUID(c))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

// Update PUT /banned-words/:id
func (h *Handler) Update(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "banned words unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
		return
	}
	var body UpdateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.Update(c, id, body, adminUUID(c))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "违禁词不存在")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

// Delete DELETE /banned-words/:id
func (h *Handler) Delete(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "banned words unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.Svc.Delete(c, id, adminUUID(c)); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "违禁词不存在")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// ListCategories GET /banned-words/categories
func (h *Handler) ListCategories(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "banned words unavailable")
		return
	}
	items, err := h.Svc.ListCategories(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items})
}

// ToggleCategory PUT /banned-words/categories/:category
func (h *Handler) ToggleCategory(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "banned words unavailable")
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Enabled == nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body: enabled required")
		return
	}
	info, err := h.Svc.ToggleCategory(c, c.Param("category"), *body.Enabled, adminUUID(c))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "违禁词分类不存在")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, info)
}

// CheckProduct GET /products/:id/banned-words/check
func (h *Handler) CheckProduct(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "banned words unavailable")
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
		return
	}
	prod, err := h.Svc.FindProductScoped(c, pid)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "商品不存在")
			return
		}
		response.HandleError(c, err)
		return
	}
	res, err := h.Svc.ScanProduct(c.Request.Context(), *prod)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	h.logCheck(c, pid.String(), fmt.Sprintf("forbidden=%d warn=%d", res.ForbiddenCount, res.WarningCount))
	response.OK(c, res)
}

const maxBatchCheck = 100

// BatchCheck POST /products/banned-words/check-batch
func (h *Handler) BatchCheck(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "banned words unavailable")
		return
	}
	var body struct {
		ProductIDs []string `json:"productIds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	if len(body.ProductIDs) == 0 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "productIds required")
		return
	}
	if len(body.ProductIDs) > maxBatchCheck {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "at most 100 productIds per request")
		return
	}
	list := make([]*ScanResult, 0, len(body.ProductIDs))
	var sumF, sumW int
	for _, raw := range body.ProductIDs {
		pid, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid product id in list")
			return
		}
		prod, err := h.Svc.FindProductScoped(c, pid)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				response.Fail(c, http.StatusNotFound, response.CodeNotFound, "商品不存在")
				return
			}
			response.HandleError(c, err)
			return
		}
		res, err := h.Svc.ScanProduct(c.Request.Context(), *prod)
		if err != nil {
			response.HandleError(c, err)
			return
		}
		// Batch rows do not need full field texts; keep payload small.
		res.Fields = nil
		sumF += res.ForbiddenCount
		sumW += res.WarningCount
		list = append(list, res)
	}
	h.logCheck(c, "batch", fmt.Sprintf("size=%d forbidden=%d warn=%d", len(list), sumF, sumW))
	response.OK(c, gin.H{"list": list})
}

func (h *Handler) logCheck(c *gin.Context, resourceID, msg string) {
	if h == nil || h.OpLog == nil {
		return
	}
	_ = h.OpLog.Write(c, operationlog.WriteOpts{
		AdminUserID: adminUUID(c),
		Action:      "banned_word.check",
		Resource:    "product",
		ResourceID:  resourceID,
		Status:      "success",
		Message:     msg,
	})
}
