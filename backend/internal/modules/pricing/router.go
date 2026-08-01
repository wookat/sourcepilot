package pricing

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// Register mounts pricing routes on g (already under /api/v1, authenticated).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	w := adminperm.RequireWritable(h.Svc.DB)
	g.POST("/pricing/calculate", h.Calculate)
	g.POST("/products/:id/pricing/apply", w, h.ApplyProduct)
	g.POST("/products/pricing/batch-apply", w, h.BatchApply)
}
