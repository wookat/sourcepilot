package platformtenant

import (
	"github.com/gin-gonic/gin"
)

// Register mounts platform tenant management routes.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	rg := g.Group("/platform/tenants")
	rg.GET("", h.List)
	rg.POST("", h.Create)
	rg.PUT("/:id", h.Rename)
	rg.POST("/:id/disable", h.Disable)
	rg.POST("/:id/enable", h.Enable)
}
