package reports

import "github.com/gin-gonic/gin"

// Register mounts the deep report read routes (already under /api/v1,
// authed). All routes are GET-only: readable by every role including
// readonly; tenant / store scope is applied inside each query.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	r := g.Group("/reports")
	r.GET("/profit", h.GetProfit)
	r.GET("/profit/export.csv", h.ExportProfitCSV)
	r.GET("/procurement", h.GetProcurement)
	r.GET("/inventory", h.GetInventory)
}
