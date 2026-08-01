package procurement

import "github.com/gin-gonic/gin"

// Register mounts procurement routes on g (already under /api/v1, authed).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	w := h.requireWrite()

	g.POST("/procurement/orders/generate", w, h.Generate)
	g.POST("/procurement/orders/batch-mark-placed", w, h.BatchMarkPlaced)
	g.POST("/procurement/orders/batch-logistics", w, h.BatchLogistics)
	g.GET("/procurement/orders", h.List)
	g.GET("/procurement/cost-estimates/:id", h.CostEstimate)
	g.POST("/procurement/cost-estimates/batch", h.CostEstimateBatch)
	g.GET("/procurement/orders/:id", h.Detail)
	g.GET("/procurement/orders/:id/export.csv", h.ExportCSV)
	g.GET("/procurement/purchase-lists/export.csv", h.ExportBatchCSV)
	g.POST("/procurement/orders/:id/submit", w, h.Submit)
	g.POST("/procurement/orders/:id/confirm", w, h.Confirm)
	g.POST("/procurement/orders/:id/retry", w, h.Retry)
	g.POST("/procurement/orders/:id/cancel", w, h.Cancel)
	g.PUT("/procurement/orders/:id/items/:itemId/price", w, h.UpdateItemPrice)
	g.POST("/procurement/orders/:id/mark-placed", w, h.MarkPlaced)
	g.POST("/procurement/orders/:id/mark-paid", w, h.MarkPaid)
	g.POST("/procurement/orders/:id/logistics", w, h.FillLogistics)
	g.POST("/procurement/orders/:id/mark-delivered", w, h.MarkDelivered)
}
