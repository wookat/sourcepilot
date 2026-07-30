package selection

import "github.com/gin-gonic/gin"

// Register mounts selection routes on the authenticated /api/v1 group.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.POST("/selection/tasks", h.CreateTask)
	g.GET("/selection/tasks", h.ListTasks)
	g.GET("/selection/tasks/:id", h.GetTask)
	g.GET("/selection/tasks/:id/candidates", h.ListCandidates)
	g.POST("/selection/tasks/:id/retry", h.Retry)
	g.POST("/selection/candidates/:id/decision", h.Decide)
	g.POST("/selection/candidates/:id/to-draft", h.ToDraft)
}
