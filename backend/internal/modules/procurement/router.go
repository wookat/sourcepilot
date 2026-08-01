package procurement

import "github.com/gin-gonic/gin"

// Register mounts procurement routes on g (already under /api/v1, authed).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.POST("/procurement/orders/generate", h.Generate)
	g.POST("/procurement/orders/batch-mark-placed", h.BatchMarkPlaced)
	g.POST("/procurement/orders/batch-logistics", h.BatchLogistics)
	g.GET("/procurement/orders", h.List)
	g.GET("/procurement/orders/:id", h.Detail)
	g.GET("/procurement/orders/:id/export.csv", h.ExportCSV)
	g.POST("/procurement/orders/:id/submit", h.Submit)
	g.POST("/procurement/orders/:id/confirm", h.Confirm)
	g.POST("/procurement/orders/:id/retry", h.Retry)
	g.POST("/procurement/orders/:id/cancel", h.Cancel)
	g.POST("/procurement/orders/:id/mark-placed", h.MarkPlaced)
	g.POST("/procurement/orders/:id/mark-paid", h.MarkPaid)
	g.POST("/procurement/orders/:id/logistics", h.FillLogistics)
	g.POST("/procurement/orders/:id/mark-delivered", h.MarkDelivered)
}
