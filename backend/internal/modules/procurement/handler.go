package procurement

import (
	"context"
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

// Handler exposes procurement HTTP API.
type Handler struct {
	Svc *Service
}

// requireWrite is the route-level guard for procurement write endpoints.
func (h *Handler) requireWrite() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h != nil && h.Svc != nil && h.Svc.DB != nil && !adminperm.CanWriteOrders(c, h.Svc.DB) {
			response.Fail(c, 403, response.CodeForbidden, "当前账号为只读权限，无法执行此操作")
			c.Abort()
			return
		}
		c.Next()
	}
}

// requestScope resolves the trusted tenant/store scope from the authenticated
// principal. Missing tenant context falls back to nil (legacy/unit-test paths).
func (h *Handler) requestScope(c *gin.Context) Scope {
	sc := Scope{}
	if tid, err := adminperm.TenantIDFromGin(c); err == nil {
		sc.TenantID = &tid
	}
	var db *gorm.DB
	if h != nil && h.Svc != nil {
		db = h.Svc.DB
	}
	if p, _ := adminperm.LoadPrincipal(c, db); p != nil {
		sc.AllowedShopIDs = p.AllowedStoreIDs()
	}
	return sc
}

// scopePO is the route-level guard for /procurement/orders/:id endpoints:
// purchase orders outside the caller's tenant/store scope answer 404 so
// foreign IDs leak no existence information.
func (h *Handler) scopePO() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !h.ok() {
			c.Next()
			return
		}
		id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
		if err != nil {
			c.Next() // the handler reports invalid id as 400
			return
		}
		visible, err := h.Svc.POInScope(c.Request.Context(), id, h.requestScope(c))
		if err != nil {
			handleProcurementError(c, err)
			c.Abort()
			return
		}
		if !visible {
			response.Fail(c, 404, response.CodeNotFound, "采购单不存在")
			c.Abort()
			return
		}
		c.Next()
	}
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
	sc := h.requestScope(c)
	for _, raw := range body.OrderIDs {
		oid, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			continue // service reports invalid ids
		}
		visible, err := h.Svc.SalesOrderInScope(c.Request.Context(), oid, sc)
		if err != nil {
			handleProcurementError(c, err)
			return
		}
		if !visible {
			response.Fail(c, 404, response.CodeNotFound, "订单不存在")
			return
		}
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
		Page:         atoiQ(c, "page", 1),
		PageSize:     atoiQ(c, "pageSize", 20),
		Status:       c.Query("status"),
		SupplierID:   c.Query("supplierId"),
		Keyword:      c.Query("keyword"),
		SalesOrderID: c.Query("salesOrderId"),
		Scope:        h.requestScope(c),
	})
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	response.OK(c, out)
}

// CostEstimate GET /procurement/cost-estimates/:id (id = sales order id)
func (h *Handler) CostEstimate(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "procurement unavailable")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	visible, err := h.Svc.SalesOrderInScope(c.Request.Context(), id, h.requestScope(c))
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	if !visible {
		response.Fail(c, 404, response.CodeNotFound, "订单不存在")
		return
	}
	out, err := h.Svc.EstimateOrderCost(c.Request.Context(), id)
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	response.OK(c, out)
}

// CostEstimateBatch POST /procurement/cost-estimates/batch
func (h *Handler) CostEstimateBatch(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "procurement unavailable")
		return
	}
	var body struct {
		OrderIDs []string `json:"orderIds"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.OrderIDs) == 0 {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	if len(body.OrderIDs) > MaxBatchEstimateOrders {
		response.Fail(c, 400, response.CodeBadRequest, "too many orderIds")
		return
	}
	sc := h.requestScope(c)
	ids := make([]uuid.UUID, 0, len(body.OrderIDs))
	for _, raw := range body.OrderIDs {
		u, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid order id")
			return
		}
		visible, err := h.Svc.SalesOrderInScope(c.Request.Context(), u, sc)
		if err != nil {
			handleProcurementError(c, err)
			return
		}
		if !visible {
			continue // out-of-scope orders are omitted like missing ones
		}
		ids = append(ids, u)
	}
	out, err := h.Svc.EstimateOrderCostBatch(c.Request.Context(), ids)
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	response.OK(c, gin.H{"items": out})
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
	out, err := h.Svc.BatchMarkPlaced(c.Request.Context(), body, h.requestScope(c), adminUUID(c))
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
	out, err := h.Svc.BatchFillLogistics(c.Request.Context(), body, h.requestScope(c), adminUUID(c))
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	response.OK(c, out)
}

// UpdateItemPrice PUT /procurement/orders/:id/items/:itemId/price
func (h *Handler) UpdateItemPrice(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "procurement unavailable")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	itemID, err := uuid.Parse(strings.TrimSpace(c.Param("itemId")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid item id")
		return
	}
	var body struct {
		ExpectedPrice float64 `json:"expectedPrice"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	out, err := h.Svc.UpdateItemPrice(c.Request.Context(), id, itemID, body.ExpectedPrice, adminUUID(c))
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

// MaxBatchExportOrders caps how many purchase orders one merged CSV can cover.
const MaxBatchExportOrders = 50

// ExportBatchCSV GET /procurement/purchase-lists/export.csv?ids=id1,id2
func (h *Handler) ExportBatchCSV(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "procurement unavailable")
		return
	}
	raw := strings.Split(c.Query("ids"), ",")
	ids := make([]uuid.UUID, 0, len(raw))
	seen := map[uuid.UUID]bool{}
	for _, r := range raw {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		u, err := uuid.Parse(r)
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid order id")
			return
		}
		if seen[u] {
			continue
		}
		seen[u] = true
		ids = append(ids, u)
	}
	if len(ids) == 0 {
		response.Fail(c, 400, response.CodeBadRequest, "ids required")
		return
	}
	if len(ids) > MaxBatchExportOrders {
		response.Fail(c, 400, response.CodeBadRequest, "too many ids")
		return
	}
	sc := h.requestScope(c)
	for _, id := range ids {
		visible, err := h.Svc.POInScope(c.Request.Context(), id, sc)
		if err != nil {
			handleProcurementError(c, err)
			return
		}
		if !visible {
			response.Fail(c, 404, response.CodeNotFound, "采购单不存在")
			return
		}
	}
	data, name, err := h.Svc.ExportBatchCSV(c.Request.Context(), ids)
	if err != nil {
		handleProcurementError(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	c.Data(200, "text/csv; charset=utf-8", data)
}
