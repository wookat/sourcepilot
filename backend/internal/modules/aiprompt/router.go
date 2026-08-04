package aiprompt

import "github.com/gin-gonic/gin"

// Register mounts /api/v1/ai/prompts routes on an authenticated group.
// ai_prompts is a deployment-wide catalog keyed by code that may carry
// platform-customized prompt content, so reads and writes are both guarded
// (platform tenant only). Business tenants' AI features consume the catalog
// server-side via GetEnabledByCode and are unaffected.
func Register(g *gin.RouterGroup, h *Handler, guards ...gin.HandlerFunc) {
	if g == nil || h == nil {
		return
	}
	rg := g.Group("/ai/prompts", guards...)
	rg.GET("", h.List)
	rg.GET("/:id", h.Get)
	rg.POST("", h.Create)
	rg.PUT("/:id", h.Put)
	rg.DELETE("/:id", h.Delete)
	rg.POST("/:id/enable", h.Enable)
	rg.POST("/:id/disable", h.Disable)
}
