package carrier

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// Register mounts /carriers routes (parent group already covers /api/v1).
func Register(parent *gin.RouterGroup, h *Handler) {
	if parent == nil || h == nil {
		return
	}
	w := adminperm.RequireWritable(h.Svc.DB)
	g := parent.Group("/carriers")
	g.GET("", h.List)
	g.POST("", w, h.Create)
	g.PUT("/:id", w, h.Update)
	g.DELETE("/:id", w, h.Delete)
}
