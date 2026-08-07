package order

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

func parseOrderTagID(c *gin.Context, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param(param)))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid tag id")
		return uuid.Nil, false
	}
	return id, true
}

func failOrderTagError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrOrderTagNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "标签不存在")
	case errors.Is(err, ErrNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "订单不存在")
	case errors.Is(err, adminperm.ErrStoreNotOperable):
		response.Fail(c, http.StatusForbidden, response.CodeStorePermissionDenied, "店铺无操作权限")
	default:
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	}
}

// GetOrderTags GET /order-tags
func (h *Handler) GetOrderTags(c *gin.Context) {
	rows, err := h.Svc.ListOrderTags(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": rows})
}

// PostOrderTag POST /order-tags
func (h *Handler) PostOrderTag(c *gin.Context) {
	var body OrderTagBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.CreateOrderTag(c, body, adminUUID(c))
	if err != nil {
		failOrderTagError(c, err)
		return
	}
	response.OK(c, row)
}

// PutOrderTag PUT /order-tags/:tagId
func (h *Handler) PutOrderTag(c *gin.Context) {
	id, ok := parseOrderTagID(c, "tagId")
	if !ok {
		return
	}
	var body OrderTagBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.UpdateOrderTag(c, id, body, adminUUID(c))
	if err != nil {
		failOrderTagError(c, err)
		return
	}
	response.OK(c, row)
}

// DeleteOrderTag DELETE /order-tags/:tagId
func (h *Handler) DeleteOrderTag(c *gin.Context) {
	id, ok := parseOrderTagID(c, "tagId")
	if !ok {
		return
	}
	if err := h.Svc.DeleteOrderTag(c, id, adminUUID(c)); err != nil {
		failOrderTagError(c, err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// PostOrderTagsAttach POST /orders/:id/tags
func (h *Handler) PostOrderTagsAttach(c *gin.Context) {
	orderID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid order id")
		return
	}
	var body OrderTagOpBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	tags, err := h.Svc.AddOrderTags(c, orderID, body.TagIDs, adminUUID(c))
	if err != nil {
		failOrderTagError(c, err)
		return
	}
	response.OK(c, gin.H{"tags": tags})
}

// DeleteOrderTagLink DELETE /orders/:id/tags/:tagId
func (h *Handler) DeleteOrderTagLink(c *gin.Context) {
	orderID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid order id")
		return
	}
	tagID, ok := parseOrderTagID(c, "tagId")
	if !ok {
		return
	}
	tags, err := h.Svc.RemoveOrderTag(c, orderID, tagID, adminUUID(c))
	if err != nil {
		failOrderTagError(c, err)
		return
	}
	response.OK(c, gin.H{"tags": tags})
}

// PostBatchOrderTags POST /orders/batch-tags
func (h *Handler) PostBatchOrderTags(c *gin.Context) {
	var body BatchOrderTagBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	res, err := h.Svc.BatchTagOrders(c, body, adminUUID(c))
	if err != nil {
		failOrderTagError(c, err)
		return
	}
	response.OK(c, res)
}
