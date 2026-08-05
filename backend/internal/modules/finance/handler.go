package finance

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/reports"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler exposes the finance HTTP API.
type Handler struct {
	Svc *Service
}

func (h *Handler) ok() bool { return h != nil && h.Svc != nil && h.Svc.DB != nil }

// requireWrite is the route-level guard for finance write endpoints (readonly
// principals answer 403; per-shop operate is re-checked in the service).
func (h *Handler) requireWrite() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.ok() && !adminperm.CanWriteOrders(c, h.Svc.DB) {
			response.Fail(c, 403, response.CodeForbidden, "当前账号为只读权限，无法执行此操作")
			c.Abort()
			return
		}
		c.Next()
	}
}

func adminUUID(c *gin.Context) *uuid.UUID {
	raw, ok := c.Get(ctxkey.AdminID)
	if !ok {
		return nil
	}
	s, _ := raw.(string)
	u, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &u
}

func handleFinanceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrBadRequest):
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrNotFound):
		response.Fail(c, 404, response.CodeNotFound, "资源不存在")
	case errors.Is(err, ErrForbidden):
		response.Fail(c, 403, response.CodeForbidden, "当前账号无该店铺的操作权限")
	default:
		response.Fail(c, 500, response.CodeInternalError, "服务器内部错误")
	}
}

func parsePathID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil || id == uuid.Nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func queryInt(c *gin.Context, key string, def int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}

func resolveRange(c *gin.Context) (reports.DateRange, error) {
	days := queryInt(c, "days", 0)
	return reports.ResolveRange(days, c.Query("start"), c.Query("end"))
}

// GetExpenseTypes GET /finance/expense-types
func (h *Handler) GetExpenseTypes(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "finance unavailable")
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, 500, response.CodeInternalError, "服务器内部错误")
		return
	}
	response.OK(c, gin.H{"items": h.Svc.ExpenseTypes(c.Request.Context(), tid)})
}

// ListPayments GET /finance/payments
func (h *Handler) ListPayments(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "finance unavailable")
		return
	}
	q := ListPaymentsQuery{
		OrderID:  c.Query("orderId"),
		ShopID:   c.Query("shopId"),
		Status:   c.Query("status"),
		Page:     queryInt(c, "page", 1),
		PageSize: queryInt(c, "pageSize", 20),
	}
	items, total, err := h.Svc.ListPayments(c, q)
	if err != nil {
		handleFinanceError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items, "total": total})
}

// CreatePayment POST /finance/payments
func (h *Handler) CreatePayment(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "finance unavailable")
		return
	}
	var body PaymentBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	rec, err := h.Svc.CreatePayment(c, body, adminUUID(c))
	if err != nil {
		handleFinanceError(c, err)
		return
	}
	h.Svc.logOp(c, adminUUID(c), "finance.payment.create", rec.ID.String(),
		fmt.Sprintf("order=%s amount=%.2f %s", rec.OrderID, rec.Amount, rec.Currency))
	response.OK(c, rec)
}

// DeletePayment DELETE /finance/payments/:id
func (h *Handler) DeletePayment(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "finance unavailable")
		return
	}
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeletePayment(c, id, adminUUID(c)); err != nil {
		handleFinanceError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// CreateOrderExpense POST /finance/order-expenses
func (h *Handler) CreateOrderExpense(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "finance unavailable")
		return
	}
	var body OrderExpenseBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	exp, err := h.Svc.CreateOrderExpense(c, body, adminUUID(c))
	if err != nil {
		handleFinanceError(c, err)
		return
	}
	response.OK(c, exp)
}

// DeleteOrderExpense DELETE /finance/order-expenses/:id
func (h *Handler) DeleteOrderExpense(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "finance unavailable")
		return
	}
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteOrderExpense(c, id, adminUUID(c)); err != nil {
		handleFinanceError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// ListShopExpenses GET /finance/shop-expenses
func (h *Handler) ListShopExpenses(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "finance unavailable")
		return
	}
	items, total, err := h.Svc.ListShopExpenses(c, c.Query("shopId"), c.Query("month"),
		queryInt(c, "page", 1), queryInt(c, "pageSize", 20))
	if err != nil {
		handleFinanceError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items, "total": total})
}

// CreateShopExpense POST /finance/shop-expenses
func (h *Handler) CreateShopExpense(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "finance unavailable")
		return
	}
	var body ShopExpenseBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	exp, err := h.Svc.CreateShopExpense(c, body, adminUUID(c))
	if err != nil {
		handleFinanceError(c, err)
		return
	}
	response.OK(c, exp)
}

// DeleteShopExpense DELETE /finance/shop-expenses/:id
func (h *Handler) DeleteShopExpense(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "finance unavailable")
		return
	}
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteShopExpense(c, id, adminUUID(c)); err != nil {
		handleFinanceError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// GetOrderSummary GET /finance/orders/:id/summary
func (h *Handler) GetOrderSummary(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "finance unavailable")
		return
	}
	id, ok := parsePathID(c)
	if !ok {
		return
	}
	out, err := h.Svc.OrderSummary(c, id)
	if err != nil {
		handleFinanceError(c, err)
		return
	}
	response.OK(c, out)
}

// GetReconciliation GET /finance/reconciliation
func (h *Handler) GetReconciliation(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "finance unavailable")
		return
	}
	r, err := resolveRange(c)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	out, err := h.Svc.Reconciliation(c, r, c.Query("status"))
	if err != nil {
		handleFinanceError(c, err)
		return
	}
	response.OK(c, out)
}

// ExportReconciliationCSV GET /finance/reconciliation/export.csv
func (h *Handler) ExportReconciliationCSV(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "finance unavailable")
		return
	}
	r, err := resolveRange(c)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	data, name, err := h.Svc.ExportReconciliationCSV(c, r, c.Query("status"))
	if err != nil {
		handleFinanceError(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	c.Data(200, "text/csv; charset=utf-8", data)
}

// GetReport GET /finance/report
func (h *Handler) GetReport(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "finance unavailable")
		return
	}
	r, err := resolveRange(c)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	out, err := h.Svc.Report(c, r)
	if err != nil {
		handleFinanceError(c, err)
		return
	}
	response.OK(c, out)
}

// ExportReportCSV GET /finance/report/export.csv
func (h *Handler) ExportReportCSV(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "finance unavailable")
		return
	}
	r, err := resolveRange(c)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	data, name, err := h.Svc.ExportReportCSV(c, r)
	if err != nil {
		handleFinanceError(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	c.Data(200, "text/csv; charset=utf-8", data)
}
