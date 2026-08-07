package customerchat

import "github.com/gin-gonic/gin"

// Register mounts authenticated routes on g (already under /api/v1).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	registerCustomerRoutes(g.Group("/customer"), h)
	registerCustomerRoutes(g.Group("/customer-service"), h)
}

func registerCustomerRoutes(c *gin.RouterGroup, h *Handler) {
	if c == nil || h == nil {
		return
	}
	c.GET("/dashboard", h.GetDashboard)
	c.GET("/conversations", h.ListConversations)
	c.POST("/conversations", h.CreateConversation)

	c.GET("/conversations/:id/messages", h.ListMessages)
	c.POST("/conversations/:id/messages", h.CreateMessage)
	c.POST("/conversations/:id/mark-replied", h.MarkReplied)
	c.POST("/conversations/:id/ai/generate-reply", h.GenerateReply)
	c.POST("/conversations/:id/ai-suggestions", h.GenerateAISuggestion)
	c.GET("/conversations/:id/ai-suggestions", h.ListSuggestions)
	c.POST("/conversations/:id/send-platform-message", h.SendPlatformMessage)

	c.GET("/conversations/:id", h.GetConversation)
	c.PUT("/conversations/:id", h.UpdateConversation)
	c.DELETE("/conversations/:id", h.DeleteConversation)

	c.GET("/buyer-message-rules", h.ListBuyerMsgRules)
	c.GET("/buyer-message-rules/backfill-estimate", h.EstimateBuyerMsgBackfill)
	c.POST("/buyer-message-rules", h.CreateBuyerMsgRule)
	c.PUT("/buyer-message-rules/:id", h.UpdateBuyerMsgRule)
	c.DELETE("/buyer-message-rules/:id", h.DeleteBuyerMsgRule)

	c.GET("/buyer-messages/drafts", h.ListBuyerMsgDrafts)
	c.POST("/buyer-messages/generate", h.GenerateBuyerMsgDrafts)
	c.PUT("/buyer-messages/drafts/:id", h.UpdateBuyerMsgDraft)
	c.POST("/buyer-messages/drafts/:id/regenerate", h.RegenerateBuyerMsgDraft)
	c.POST("/buyer-messages/drafts/:id/mark-sent", h.MarkBuyerMsgDraftSent)
	c.POST("/buyer-messages/drafts/:id/ignore", h.IgnoreBuyerMsgDraft)
	c.POST("/buyer-messages/drafts/batch-mark-sent", h.BatchMarkBuyerMsgDraftsSent)

	c.GET("/reply-templates", h.ListTemplates)
	c.POST("/reply-templates", h.CreateTemplate)
	c.POST("/reply-templates/reorder", h.ReorderTemplates)
	c.PUT("/reply-templates/:id", h.UpdateTemplate)
	c.DELETE("/reply-templates/:id", h.DeleteTemplate)

	c.PUT("/reply-suggestions/:id", h.UpdateSuggestion)
	c.POST("/reply-suggestions/:id/accept", h.AcceptSuggestion)
	c.POST("/reply-suggestions/:id/discard", h.DiscardSuggestion)
	c.POST("/ai-suggestions/:id/apply", h.ApplySuggestion)
	c.POST("/ai-suggestions/:id/reject", h.RejectSuggestion)
}
