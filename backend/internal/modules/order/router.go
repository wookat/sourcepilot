package order

import "github.com/gin-gonic/gin"

// Register mounts authenticated routes (already under Bearer /api/v1).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	w := h.requireWrite()

	g.GET("/order-item-sku-matches", h.ListGlobalSKUMatches)
	g.POST("/order-items/:itemId/bind-sku", w, h.PostBindOrderItemSKU)

	o := g.Group("/orders")
	o.GET("", h.List)
	o.POST("", w, h.Create)
	o.POST("/import", w, h.Import)

	o.POST("/:id/items", w, h.PostItem)
	o.PUT("/:id/items/:itemId", w, h.PutItem)
	o.DELETE("/:id/items/:itemId", w, h.DeleteItem)

	o.POST("/:id/deduct-inventory", w, h.PostDeductInventory)
	o.POST("/:id/restore-inventory", w, h.PostRestoreInventory)
	o.GET("/:id/inventory-effects", h.GetOrderInventoryEffects)
	o.GET("/:id/sku-matches", h.GetOrderSKUMatches)
	o.POST("/:id/match-skus", w, h.PostMatchOrderSKUs)

	o.GET("/stats/sales", h.GetSalesStats)
	o.GET("/stats/daily", h.GetDailyStats)
	o.GET("/stats/daily/export.csv", h.ExportDailyStatsCSV)
	o.GET("/shipping-list/export.csv", h.ExportShippingListCSV)
	o.POST("/shipments/batch", w, h.PostBatchShipments)
	o.GET("/print/sheets", h.GetPrintSheets)
	o.POST("/:id/shipments", w, h.PostShipment)
	o.PUT("/:id/shipments/:shipmentId", w, h.PutShipment)
	o.DELETE("/:id/shipments/:shipmentId", w, h.DeleteShipment)
	o.POST("/:id/shipments/:shipmentId/refresh-tracking", w, h.PostRefreshShipmentTracking)

	o.GET("/:id", h.Get)
	o.PUT("/:id", w, h.Update)
	o.DELETE("/:id", w, h.Delete)
}
