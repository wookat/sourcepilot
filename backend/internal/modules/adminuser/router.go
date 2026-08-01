package adminuser

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// Register mounts admin user management routes.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	var db *gorm.DB
	if h.Svc != nil {
		db = h.Svc.DB
	}
	rg := g.Group("/admin/users")
	rg.GET("", h.List)
	rg.POST("", h.Create)
	rg.GET("/:id", h.Get)
	rg.PATCH("/:id", h.Update)
	rg.PUT("/:id/store-permissions", h.SetStorePermissions)
	rg.DELETE("/:id", adminperm.RequireWritable(db), h.Delete)
}
