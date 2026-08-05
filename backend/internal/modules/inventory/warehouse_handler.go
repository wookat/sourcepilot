package inventory

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

func (h *Handler) tenantID(c *gin.Context) (int64, bool) {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, 401, response.CodeUnauthorized, "租户信息缺失")
		return 0, false
	}
	return tid, true
}

func warehouseFail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrWarehouseNotFound):
		response.Fail(c, 404, response.CodeNotFound, "仓库不存在")
	case errors.Is(err, ErrWarehouseCodeConflict):
		response.Fail(c, 400, response.CodeBadRequest, "仓库编码已存在")
	case errors.Is(err, ErrDefaultWarehouseLocked):
		response.Fail(c, 400, response.CodeBadRequest, "默认仓不可删除或停用")
	case errors.Is(err, ErrWarehouseHasStock):
		response.Fail(c, 400, response.CodeBadRequest, "仓库仍有库存，请先调拨至其他仓库")
	case errors.Is(err, ErrWarehouseDisabled):
		response.Fail(c, 400, response.CodeBadRequest, "目标仓库已停用")
	case errors.Is(err, ErrInsufficientWarehouse):
		response.Fail(c, 400, response.CodeBadRequest, "源仓库库存不足")
	case errors.Is(err, ErrTransferSameWarehouse):
		response.Fail(c, 400, response.CodeBadRequest, "源仓库与目标仓库不能相同")
	case errors.Is(err, ErrTransferInvalidQuantity):
		response.Fail(c, 400, response.CodeBadRequest, "调拨数量必须大于 0")
	case errors.Is(err, ErrDefaultMustBeEnabled):
		response.Fail(c, 400, response.CodeBadRequest, "已停用仓库不可设为默认仓，请先启用")
	default:
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
	}
}

// ListWarehouses GET /inventory/warehouses
func (h *Handler) ListWarehouses(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	tid, ok := h.tenantID(c)
	if !ok {
		return
	}
	rows, err := h.Svc.ListWarehouses(c.Request.Context(), tid)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": rows, "total": len(rows)})
}

// GetWarehouseSummary GET /inventory/warehouses/summary
func (h *Handler) GetWarehouseSummary(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	tid, ok := h.tenantID(c)
	if !ok {
		return
	}
	rows, err := h.Svc.WarehouseSummary(c.Request.Context(), tid)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": rows})
}

// GetWarehouseMigrationPreview GET /inventory/warehouses/migration-preview
func (h *Handler) GetWarehouseMigrationPreview(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	tid, ok := h.tenantID(c)
	if !ok {
		return
	}
	out, err := h.Svc.PreviewWarehouseMigration(c.Request.Context(), tid)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

// CreateWarehouse POST /inventory/warehouses
func (h *Handler) CreateWarehouse(c *gin.Context) {
	if !h.requireInventoryWrite(c) {
		return
	}
	tid, ok := h.tenantID(c)
	if !ok {
		return
	}
	var body CreateWarehouseBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	w, err := h.Svc.CreateWarehouse(c.Request.Context(), tid, body)
	if err != nil {
		warehouseFail(c, err)
		return
	}
	response.OK(c, w)
}

// UpdateWarehouse PUT /inventory/warehouses/:id
func (h *Handler) UpdateWarehouse(c *gin.Context) {
	if !h.requireInventoryWrite(c) {
		return
	}
	tid, ok := h.tenantID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body UpdateWarehouseBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	w, err := h.Svc.UpdateWarehouse(c.Request.Context(), tid, id, body)
	if err != nil {
		warehouseFail(c, err)
		return
	}
	response.OK(c, w)
}

// DeleteWarehouse DELETE /inventory/warehouses/:id
func (h *Handler) DeleteWarehouse(c *gin.Context) {
	if !h.requireInventoryWrite(c) {
		return
	}
	tid, ok := h.tenantID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.Svc.DeleteWarehouse(c.Request.Context(), tid, id); err != nil {
		warehouseFail(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// SetDefaultWarehouse POST /inventory/warehouses/:id/set-default
func (h *Handler) SetDefaultWarehouse(c *gin.Context) {
	if !h.requireInventoryWrite(c) {
		return
	}
	tid, ok := h.tenantID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	w, err := h.Svc.SetDefaultWarehouse(c.Request.Context(), tid, id)
	if err != nil {
		warehouseFail(c, err)
		return
	}
	response.OK(c, w)
}

// TransferStock POST /inventory/transfers
func (h *Handler) TransferStock(c *gin.Context) {
	if !h.requireInventoryWrite(c) {
		return
	}
	tid, ok := h.tenantID(c)
	if !ok {
		return
	}
	var body TransferStockBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	res, err := h.Svc.TransferStock(c.Request.Context(), tid, body, adminUUID(c))
	if err != nil {
		warehouseFail(c, err)
		return
	}
	response.OK(c, res)
}

// GetSKUWarehouseStocks GET /inventory/sku-warehouse-stocks?productSkuId=...
func (h *Handler) GetSKUWarehouseStocks(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "inventory unavailable")
		return
	}
	tid, ok := h.tenantID(c)
	if !ok {
		return
	}
	skuID, err := uuid.Parse(strings.TrimSpace(c.Query("productSkuId")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid productSkuId")
		return
	}
	total, err := h.Svc.skuTotalStockScoped(c.Request.Context(), tid, skuID)
	if err != nil {
		warehouseFail(c, err)
		return
	}
	rows, err := h.Svc.WarehouseStocksForSKU(c.Request.Context(), tid, skuID, total)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": rows, "totalStock": total})
}
