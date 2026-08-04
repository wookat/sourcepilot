package aiprompt

import "github.com/gin-gonic/gin"

// Register mounts /api/v1/ai/prompts routes on an authenticated group.
// ai_prompts is a deployment-wide catalog keyed by code, so every write is
// guarded by writeGuards (platform tenant only); reads stay open to all
// tenants because prompts drive their AI features.
func Register(g *gin.RouterGroup, h *Handler, writeGuards ...gin.HandlerFunc) {
	if g == nil || h == nil {
		return
	}
	rg := g.Group("/ai/prompts")
	rg.GET("", h.List)
	rg.GET("/:id", h.Get)

	wg := rg.Group("", writeGuards...)
	wg.POST("", h.Create)
	wg.PUT("/:id", h.Put)
	wg.DELETE("/:id", h.Delete)
	wg.POST("/:id/enable", h.Enable)
	wg.POST("/:id/disable", h.Disable)
}
