package procurement

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler exposes procurement HTTP API.
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

func handleProcurementError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		response.Fail(c, 404, response.CodeNotFound, err.Error())
	case errors.Is(err, ErrBadRequest):
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrConflict):
		response.Fail(c, 409, response.CodeBadRequest, err.Error())
	default:
		response.HandleError(c, err)
	}
}

func (h *Handler) ok() bool { return h != nil && h.Svc != nil }

func atoiQ(c *gin.Context, key string, def int) int {
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

func parseID(c *gin.Context) (uuid.UUID, bool) {
	u, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return uuid.Nil, false
	}
	return u, true
}

// Generate POST /procurement/orders/generate
func (h *Handler) Generate(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "procurement unavailable")
		return
	}
	var body GenerateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	if body.IdempotencyKey == "" {
		body.IdempotencyKey = strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	}
	out, err := h.Svc.Generate(c.Request.Context(), body, adminUUID(c))
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	response.OK(c, out)
}

// List GET /procurement/orders
func (h *Handler) List(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "procurement unavailable")
		return
	}
	out, err := h.Svc.List(c.Request.Context(), ListQuery{
		Page:       atoiQ(c, "page", 1),
		PageSize:   atoiQ(c, "pageSize", 20),
		Status:     c.Query("status"),
		SupplierID: c.Query("supplierId"),
		Keyword:    c.Query("keyword"),
	})
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	response.OK(c, out)
}

// Detail GET /procurement/orders/:id
func (h *Handler) Detail(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "procurement unavailable")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	out, err := h.Svc.Detail(c.Request.Context(), id)
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) simpleTransition(c *gin.Context, fn func(ctx context.Context, id uuid.UUID, operator *uuid.UUID) (*PurchaseOrder, error)) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "procurement unavailable")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	out, err := fn(c.Request.Context(), id, adminUUID(c))
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	response.OK(c, out)
}

// Submit POST /procurement/orders/:id/submit
func (h *Handler) Submit(c *gin.Context) { h.simpleTransition(c, h.Svc.Submit) }

// Confirm POST /procurement/orders/:id/confirm
func (h *Handler) Confirm(c *gin.Context) { h.simpleTransition(c, h.Svc.Confirm) }

// Retry POST /procurement/orders/:id/retry
func (h *Handler) Retry(c *gin.Context) { h.simpleTransition(c, h.Svc.Retry) }

// MarkDelivered POST /procurement/orders/:id/mark-delivered
func (h *Handler) MarkDelivered(c *gin.Context) { h.simpleTransition(c, h.Svc.MarkDelivered) }

// MarkPlaced POST /procurement/orders/:id/mark-placed
func (h *Handler) MarkPlaced(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "procurement unavailable")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body MarkPlacedBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	out, err := h.Svc.MarkPlaced(c.Request.Context(), id, body, adminUUID(c))
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	response.OK(c, out)
}

// MarkPaid POST /procurement/orders/:id/mark-paid
func (h *Handler) MarkPaid(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "procurement unavailable")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body MarkPaidBody
	_ = c.ShouldBindJSON(&body)
	out, err := h.Svc.MarkPaid(c.Request.Context(), id, body, adminUUID(c))
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	response.OK(c, out)
}

// FillLogistics POST /procurement/orders/:id/logistics
func (h *Handler) FillLogistics(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "procurement unavailable")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body LogisticsBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	out, err := h.Svc.FillLogistics(c.Request.Context(), id, body, adminUUID(c))
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	response.OK(c, out)
}

// Cancel POST /procurement/orders/:id/cancel
func (h *Handler) Cancel(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "procurement unavailable")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	out, err := h.Svc.Cancel(c.Request.Context(), id, body.Reason, adminUUID(c))
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	response.OK(c, out)
}

// BatchMarkPlaced POST /procurement/orders/batch-mark-placed
func (h *Handler) BatchMarkPlaced(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "procurement unavailable")
		return
	}
	var body BatchMarkPlacedBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	out, err := h.Svc.BatchMarkPlaced(c.Request.Context(), body, adminUUID(c))
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	response.OK(c, out)
}

// BatchLogistics POST /procurement/orders/batch-logistics
func (h *Handler) BatchLogistics(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "procurement unavailable")
		return
	}
	var body BatchLogisticsBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	out, err := h.Svc.BatchFillLogistics(c.Request.Context(), body, adminUUID(c))
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	response.OK(c, out)
}

// ExportCSV GET /procurement/orders/:id/export.csv
func (h *Handler) ExportCSV(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "procurement unavailable")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	data, name, err := h.Svc.ExportCSV(c.Request.Context(), id)
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	c.Data(200, "text/csv; charset=utf-8", data)
}
