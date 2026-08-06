package mcptoken

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// Register mounts /mcp/tokens management routes (parent already covers /api/v1).
func Register(parent *gin.RouterGroup, h *Handler) {
	if parent == nil || h == nil {
		return
	}
	w := adminperm.RequireWritable(h.Svc.DB)
	g := parent.Group("/mcp/tokens")
	g.GET("", h.List)
	g.POST("", w, h.Create)
	g.POST("/:id/revoke", w, h.Revoke)
}
