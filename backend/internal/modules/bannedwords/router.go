package bannedwords

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// Register mounts /banned-words and product scan routes (parent already /api/v1 + JWT).
func Register(parent *gin.RouterGroup, h *Handler) {
	if parent == nil || h == nil {
		return
	}
	w := adminperm.RequireWritable(h.Svc.DB)
	g := parent.Group("/banned-words")
	g.GET("", h.List)
	g.POST("", w, h.Create)
	g.GET("/categories", h.ListCategories)
	g.PUT("/categories/:category", w, h.ToggleCategory)
	g.PUT("/:id", w, h.Update)
	g.DELETE("/:id", w, h.Delete)

	parent.GET("/products/:id/banned-words/check", h.CheckProduct)
	parent.POST("/products/banned-words/check-batch", h.BatchCheck)
}
