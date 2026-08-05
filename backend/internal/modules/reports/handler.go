package reports

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler serves the deep report read APIs. All endpoints are GET-only and
// readable by every role (readonly included); scope is enforced in queries.
type Handler struct {
	Svc *Service
}

func (h *Handler) ok() bool { return h != nil && h.Svc != nil && h.Svc.DB != nil }

func parseDays(c *gin.Context) (int, bool) {
	raw := strings.TrimSpace(c.Query("days"))
	if raw == "" {
		return 0, true
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

func (h *Handler) resolveRange(c *gin.Context) (DateRange, bool) {
	days, ok := parseDays(c)
	if !ok {
		response.Fail(c, 400, response.CodeBadRequest, "days 参数无效")
		return DateRange{}, false
	}
	r, err := ResolveRange(days, c.Query("start"), c.Query("end"))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return DateRange{}, false
	}
	return r, true
}

// GetProfit GET /reports/profit?dimension=order|product|shop&days=30 |
// &start=YYYY-MM-DD&end=YYYY-MM-DD
func (h *Handler) GetProfit(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "reports unavailable")
		return
	}
	dimension := strings.TrimSpace(c.DefaultQuery("dimension", DimensionOrder))
	r, ok := h.resolveRange(c)
	if !ok {
		return
	}
	res, err := h.Svc.ProfitReport(c, dimension, r)
	if err != nil {
		if strings.Contains(err.Error(), "dimension") {
			response.Fail(c, 400, response.CodeBadRequest, err.Error())
			return
		}
		response.Fail(c, 500, response.CodeInternalError, err.Error())
		return
	}
	response.OK(c, res)
}

// ExportProfitCSV GET /reports/profit/export.csv (same params as GetProfit).
func (h *Handler) ExportProfitCSV(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "reports unavailable")
		return
	}
	dimension := strings.TrimSpace(c.DefaultQuery("dimension", DimensionOrder))
	r, ok := h.resolveRange(c)
	if !ok {
		return
	}
	data, name, err := h.Svc.ExportProfitCSV(c, dimension, r)
	if err != nil {
		if strings.Contains(err.Error(), "dimension") {
			response.Fail(c, 400, response.CodeBadRequest, err.Error())
			return
		}
		response.Fail(c, 500, response.CodeInternalError, err.Error())
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	c.Data(200, "text/csv; charset=utf-8", data)
}

// GetProcurement GET /reports/procurement?days=30 | &start=&end=
func (h *Handler) GetProcurement(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "reports unavailable")
		return
	}
	r, ok := h.resolveRange(c)
	if !ok {
		return
	}
	res, err := h.Svc.ProcurementReport(c, r)
	if err != nil {
		response.Fail(c, 500, response.CodeInternalError, err.Error())
		return
	}
	response.OK(c, res)
}

// GetInventory GET /reports/inventory?slowDays=30&warehouseId=<uuid>
func (h *Handler) GetInventory(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "reports unavailable")
		return
	}
	slowDays := 0
	if raw := strings.TrimSpace(c.Query("slowDays")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			response.Fail(c, 400, response.CodeBadRequest, "slowDays 参数无效")
			return
		}
		slowDays = v
	}
	var warehouseID *uuid.UUID
	if raw := strings.TrimSpace(c.Query("warehouseId")); raw != "" {
		u, err := uuid.Parse(raw)
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "warehouseId 参数无效")
			return
		}
		warehouseID = &u
	}
	res, err := h.Svc.InventoryReport(c, slowDays, warehouseID)
	if err != nil {
		if errors.Is(err, inventory.ErrWarehouseNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "仓库不存在")
			return
		}
		response.Fail(c, 500, response.CodeInternalError, err.Error())
		return
	}
	response.OK(c, res)
}
