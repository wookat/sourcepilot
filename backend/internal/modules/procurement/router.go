package procurement

import "github.com/gin-gonic/gin"

// Register mounts procurement routes on g (already under /api/v1, authed).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	w := h.requireWrite()
	sg := h.scopePO()

	g.POST("/procurement/orders/generate", w, h.Generate)
	g.POST("/procurement/orders/batch-mark-placed", w, h.BatchMarkPlaced)
	g.POST("/procurement/orders/batch-logistics", w, h.BatchLogistics)
	g.GET("/procurement/orders", h.List)
	g.GET("/procurement/cost-estimates/:id", h.CostEstimate)
	g.POST("/procurement/cost-estimates/batch", h.CostEstimateBatch)
	g.GET("/procurement/orders/:id", sg, h.Detail)
	g.GET("/procurement/orders/:id/export.csv", sg, h.ExportCSV)
	g.GET("/procurement/purchase-lists/export.csv", h.ExportBatchCSV)
	g.POST("/procurement/orders/:id/submit", w, sg, h.Submit)
	g.POST("/procurement/orders/:id/confirm", w, sg, h.Confirm)
	g.POST("/procurement/orders/:id/retry", w, sg, h.Retry)
	g.POST("/procurement/orders/:id/cancel", w, sg, h.Cancel)
	g.POST("/procurement/orders/:id/void", w, sg, h.Void)
	g.PUT("/procurement/orders/:id/items/:itemId/price", w, sg, h.UpdateItemPrice)
	g.PUT("/procurement/orders/:id/items/:itemId/actual-price", w, sg, h.UpdateItemActualPrice)
	g.POST("/procurement/orders/:id/mark-placed", w, sg, h.MarkPlaced)
	g.POST("/procurement/orders/:id/mark-paid", w, sg, h.MarkPaid)
	g.POST("/procurement/orders/:id/logistics", w, sg, h.FillLogistics)
	g.POST("/procurement/orders/:id/mark-delivered", w, sg, h.MarkDelivered)
}
