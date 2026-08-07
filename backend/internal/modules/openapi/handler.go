package openapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/readonlyquery"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// pageParams validates page/pageSize query values: absent values take the
// defaults, but a non-integer or non-positive value answers 400, matching the
// date-parameter policy (no silent normalization). Values above the page-size
// cap stay clamped by the query layer, which the API contract documents.
func pageParams(c *gin.Context) (page, pageSize int, ok bool) {
	page, ok = positiveIntQ(c, "page", 1)
	if !ok {
		return 0, 0, false
	}
	pageSize, ok = positiveIntQ(c, "pageSize", 20)
	if !ok {
		return 0, 0, false
	}
	return page, pageSize, true
}

func positiveIntQ(c *gin.Context, key string, def int) (int, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return def, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest,
			"invalid "+key+" (want a positive integer)")
		return 0, false
	}
	return n, true
}

// tenantID resolves the authenticated tenant; the middleware guarantees the
// token is present, so a miss is a server-side wiring error.
func tenantID(c *gin.Context) (int64, bool) {
	tok := tokenOf(c)
	if tok == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "open api entry unavailable")
		return 0, false
	}
	return tok.TenantID, true
}

// GET /api/open/v1/orders
func (h *handlers) ordersList(c *gin.Context) {
	tid, ok := tenantID(c)
	if !ok {
		return
	}
	page, pageSize, ok := pageParams(c)
	if !ok {
		return
	}
	in := readonlyquery.OrdersQueryIn{
		Status:        c.Query("status"),
		PaymentStatus: c.Query("paymentStatus"),
		Platform:      c.Query("platform"),
		Keyword:       c.Query("keyword"),
		StartDate:     c.Query("startDate"),
		EndDate:       c.Query("endDate"),
		Page:          page,
		PageSize:      pageSize,
	}
	out, err := h.d.queries().OrdersQuery(c.Request.Context(), tid, in)
	if err != nil {
		failQuery(c, err)
		return
	}
	response.OK(c, gin.H{"list": out.Items, "total": out.Total})
}

// GET /api/open/v1/orders/:orderNo
func (h *handlers) orderDetail(c *gin.Context) {
	tid, ok := tenantID(c)
	if !ok {
		return
	}
	out, err := h.d.queries().OrderDetail(c.Request.Context(), tid, c.Param("orderNo"))
	if err != nil {
		if errors.Is(err, readonlyquery.ErrOrderNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "order not found")
			return
		}
		failQuery(c, err)
		return
	}
	response.OK(c, out)
}

// GET /api/open/v1/inventory
func (h *handlers) inventoryList(c *gin.Context) {
	tid, ok := tenantID(c)
	if !ok {
		return
	}
	page, pageSize, ok := pageParams(c)
	if !ok {
		return
	}
	raw := strings.TrimSpace(c.Query("lowStockOnly"))
	in := readonlyquery.InventoryQueryIn{
		Keyword:      c.Query("keyword"),
		LowStockOnly: strings.EqualFold(raw, "true") || raw == "1",
		Page:         page,
		PageSize:     pageSize,
	}
	out, err := h.d.queries().InventoryQuery(c.Request.Context(), tid, in)
	if err != nil {
		failQuery(c, err)
		return
	}
	response.OK(c, gin.H{"list": out.Items, "total": out.Total})
}

// GET /api/open/v1/reports/summary
func (h *handlers) reportsSummary(c *gin.Context) {
	tid, ok := tenantID(c)
	if !ok {
		return
	}
	in := readonlyquery.ReportSummaryIn{
		StartDate: c.Query("startDate"),
		EndDate:   c.Query("endDate"),
	}
	out, err := h.d.queries().ReportSummary(c.Request.Context(), tid, in)
	if err != nil {
		failQuery(c, err)
		return
	}
	response.OK(c, out)
}

// GET /api/open/v1/exceptions
func (h *handlers) exceptionsList(c *gin.Context) {
	tid, ok := tenantID(c)
	if !ok {
		return
	}
	page, pageSize, ok := pageParams(c)
	if !ok {
		return
	}
	in := readonlyquery.ExceptionsPendingIn{
		ExceptionType: c.Query("exceptionType"),
		Severity:      c.Query("severity"),
		Page:          page,
		PageSize:      pageSize,
	}
	out, err := h.d.queries().ExceptionsPending(c.Request.Context(), tid, in)
	if err != nil {
		failQuery(c, err)
		return
	}
	response.OK(c, gin.H{"list": out.Items, "total": out.Total, "totalOpen": out.TotalOpen})
}

// failQuery maps query errors: bad user input answers 400, anything else is
// an internal error with no detail leaked.
func failQuery(c *gin.Context, err error) {
	if errors.Is(err, readonlyquery.ErrBadInput) {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "query failed")
}
