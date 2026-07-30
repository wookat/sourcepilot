package sourcing

import "github.com/gin-gonic/gin"

// Register mounts sourcing routes on g (already under /api/v1, authenticated).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/suppliers", h.ListSuppliers)
	g.POST("/suppliers", h.CreateSupplier)
	g.PUT("/suppliers/:id", h.UpdateSupplier)
	g.DELETE("/suppliers/:id", h.DeleteSupplier)

	g.GET("/products/:id/sources", h.ListProductSources)
	g.POST("/products/:id/sources", h.BindSource)
	g.POST("/products/:id/sources/refresh", h.Refresh)
	g.PUT("/product-sources/:id", h.UpdateSource)
	g.POST("/product-sources/:id/set-primary", h.SetPrimary)
	g.POST("/product-sources/:id/sku-mappings", h.SaveSKUMappings)
	g.GET("/product-source-skus/:id/price-history", h.PriceHistory)
	g.GET("/source-switch-events", h.ListSwitchEvents)
}
