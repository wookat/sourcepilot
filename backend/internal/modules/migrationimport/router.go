package migrationimport

import "github.com/gin-gonic/gin"

// Register mounts authenticated routes (already under Bearer /api/v1).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	w := h.requireWrite()

	imp := g.Group("/imports")
	imp.POST("/parse", w, h.Parse)
	imp.POST("/validate", w, h.Validate)
	imp.POST("/commit", w, h.Commit)
	imp.GET("", h.List)
	imp.GET("/:id", h.Get)
	imp.GET("/:id/errors.csv", h.ErrorsCSV)
}
