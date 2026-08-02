package sourcing

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// Register mounts sourcing routes on g (already under /api/v1, authenticated).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	w := adminperm.RequireWritable(h.Svc.DB)

	g.GET("/suppliers", h.ListSuppliers)
	g.POST("/suppliers", w, h.CreateSupplier)
	g.PUT("/suppliers/:id", w, h.UpdateSupplier)
	g.DELETE("/suppliers/:id", w, h.DeleteSupplier)

	g.GET("/products/:id/sources", h.ListProductSources)
	g.POST("/products/:id/sources", w, h.BindSource)
	g.POST("/products/:id/sources/refresh", w, h.Refresh)
	g.GET("/product-sources/orphans", h.ListOrphanSources)
	g.PUT("/product-sources/:id", w, h.UpdateSource)
	g.DELETE("/product-sources/:id", w, h.DeleteSource)
	g.POST("/product-sources/:id/set-primary", w, h.SetPrimary)
	g.POST("/product-sources/:id/sku-mappings", w, h.SaveSKUMappings)
	g.GET("/product-source-skus/:id/price-history", h.PriceHistory)
	g.DELETE("/product-source-skus/:id", w, h.DeleteSKUMapping)
	g.GET("/source-switch-events", h.ListSwitchEvents)
	g.POST("/source-switch-events/:id/adopt", w, h.AdoptSwitchSuggestion)
	g.POST("/source-switch-events/:id/ignore", w, h.IgnoreSwitchSuggestion)
	g.GET("/product-source-alerts", h.ListSourceAlerts)
}
