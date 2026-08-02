package selection

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// Register mounts selection routes on the authenticated /api/v1 group.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	var db *gorm.DB
	if h.Svc != nil {
		db = h.Svc.DB
	}
	w := adminperm.RequireWritable(db)
	g.POST("/selection/tasks", w, h.CreateTask)
	g.GET("/selection/tasks", h.ListTasks)
	g.GET("/selection/tasks/:id", h.GetTask)
	g.GET("/selection/tasks/:id/candidates", h.ListCandidates)
	g.POST("/selection/tasks/:id/retry", w, h.Retry)
	g.POST("/selection/candidates/:id/decision", w, h.Decide)
	g.POST("/selection/candidates/:id/to-draft", w, h.ToDraft)
}
