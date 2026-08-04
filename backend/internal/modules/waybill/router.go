package waybill

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// Register mounts /waybill-templates and /shipping-rules routes (parent group
// already covers /api/v1 + JWT).
func Register(parent *gin.RouterGroup, h *Handler) {
	if parent == nil || h == nil {
		return
	}
	w := adminperm.RequireWritable(h.Svc.DB)

	t := parent.Group("/waybill-templates")
	t.GET("", h.ListTemplates)
	t.POST("", w, h.CreateTemplate)
	t.PUT("/:id", w, h.UpdateTemplate)
	t.DELETE("/:id", w, h.DeleteTemplate)

	r := parent.Group("/shipping-rules")
	r.GET("", h.ListRules)
	r.POST("", w, h.CreateRule)
	r.POST("/recommend", h.Recommend)
	r.PUT("/:id", w, h.UpdateRule)
	r.DELETE("/:id", w, h.DeleteRule)
}
