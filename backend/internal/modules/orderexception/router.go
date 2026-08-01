package orderexception

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// Register mounts routes under /orders/exceptions (parent group already covers /api/v1).
func Register(parent *gin.RouterGroup, h *Handler) {
	if parent == nil || h == nil {
		return
	}
	w := adminperm.RequireWritable(h.Svc.DB)
	g := parent.Group("/orders/exceptions")
	g.GET("", h.List)
	g.GET("/:sourceType/:sourceId", h.Detail)
	g.POST("/:sourceType/:sourceId/handle", w, h.Handle)
	g.POST("/:sourceType/:sourceId/ignore", w, h.Ignore)
	g.DELETE("/:sourceType/:sourceId/mark", w, h.Unmark)
	g.POST("/:sourceType/:sourceId/bind-sku", w, h.BindSKU)
	g.POST("/:sourceType/:sourceId/retry-deduct", w, h.RetryDeduct)
	g.POST("/:sourceType/:sourceId/retry-inventory-sync", w, h.RetryInventorySync)
}
