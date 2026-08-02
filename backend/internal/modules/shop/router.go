package shop

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// Register mounts shop routes under authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	var db *gorm.DB
	if h.Svc != nil {
		db = h.Svc.DB
	}
	// Platform open-app / publish presets belong to the settings center.
	settingsRead := adminperm.RequirePermissionMW(db, adminperm.PermSettingsManage)
	settingsWrite := adminperm.RequireWriteMW(db, adminperm.PermSettingsManage)
	// Shop records and their authorizations are store-scoped operations.
	storeWrite := adminperm.RequireWriteMW(db, adminperm.PermStoreOperate)

	g.GET("/platform/providers", h.ListProviders)
	g.GET("/platform/settings/:platform", settingsRead, h.GetPlatformAppSettings)
	g.PUT("/platform/settings/:platform", settingsWrite, h.PutPlatformAppSettings)
	g.POST("/platform/settings/:platform/test-connection", settingsWrite, h.TestPlatformAppSettings)
	g.GET("/platform/publish-settings/:platform", settingsRead, h.GetPlatformPublishSettings)
	g.PUT("/platform/publish-settings/:platform", settingsWrite, h.PutPlatformPublishSettings)
	g.GET("/platform/douyin/categories", h.ListDouyinCategories)
	g.POST("/platform/douyin/categories/sync", settingsWrite, h.SyncDouyinCategories)
	g.GET("/platform/douyin/categories/stats", h.DouyinCategoryStats)
	g.GET("/platform/douyin/categories/:categoryId/attributes", h.ListDouyinCategoryAttributes)
	g.POST("/platform/douyin/categories/:categoryId/attributes/sync", settingsWrite, h.SyncDouyinCategoryAttributes)

	s := g.Group("/shops")
	s.GET("", h.List)
	s.POST("", storeWrite, h.Create)
	s.GET("/:id", h.Get)
	s.PUT("/:id", storeWrite, h.Update)
	s.DELETE("/:id", storeWrite, h.Delete)
	s.PUT("/:id/auth", storeWrite, h.PutAuth)
	s.POST("/:id/test-connection", storeWrite, h.TestConnection)
	s.GET("/oauth/douyin/start", h.DouyinOAuthStart)
	s.GET("/:id/oauth/douyin/authorize-url", h.DouyinOAuthAuthorizeURL)
	s.POST("/:id/oauth/douyin/refresh", storeWrite, h.DouyinOAuthRefresh)
	s.POST("/:id/oauth/douyin/revoke", storeWrite, h.DouyinOAuthRevoke)
	s.POST("/:id/oauth/douyin/test", storeWrite, h.DouyinOAuthTest)
	s.POST("/:id/oauth/douyin/sync-shop-info", storeWrite, h.DouyinSyncShopInfo)
	s.GET("/:id/oauth/tiktok/authorize-url", h.TikTokOAuthAuthorizeURL)
	s.POST("/:id/oauth/tiktok/callback", storeWrite, h.TikTokOAuthCallback)
	s.GET("/:id/oauth/shopee/authorize-url", h.ShopeeOAuthAuthorizeURL)
	s.POST("/:id/oauth/shopee/callback", storeWrite, h.ShopeeOAuthCallback)
	s.GET("/:id/oauth/lazada/authorize-url", h.LazadaOAuthAuthorizeURL)
	s.POST("/:id/oauth/lazada/callback", storeWrite, h.LazadaOAuthCallback)
	s.GET("/:id/oauth/amazon/authorize-url", h.AmazonOAuthAuthorizeURL)
	s.POST("/:id/oauth/amazon/callback", storeWrite, h.AmazonOAuthCallback)
}

// RegisterPublic mounts OAuth callbacks that external platforms call directly.
func RegisterPublic(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/shops/oauth/douyin/callback", h.DouyinOAuthCallback)
}
