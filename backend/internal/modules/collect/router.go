package collect

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// Register mounts authenticated collect routes on g (already under /api/v1).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	var db *gorm.DB
	if h.Svc != nil {
		db = h.Svc.DB
	}
	w := adminperm.RequireWritable(db)

	g.GET("/collect/providers", h.ListProviders)
	g.POST("/collect/tasks", w, h.Create)
	g.GET("/collect/tasks", h.List)
	g.GET("/collect/monitor", h.Monitor)
	g.GET("/collect/tasks/:id/events", h.ListTaskEvents)
	g.GET("/collect/tasks/:id", h.Get)
	g.POST("/collect/tasks/:id/retry", w, h.Retry)

	g.POST("/collect/batches", w, h.CreateBatch)
	g.GET("/collect/batches", h.ListBatches)
	g.GET("/collect/batches/:id/tasks", h.ListBatchTasks)
	g.GET("/collect/batches/:id", h.GetBatch)
	g.POST("/collect/batches/:id/retry-failed", w, h.RetryBatchFailed)

	g.GET("/collector/providers/1688/auth-status", h.Get1688AuthStatus)
	g.POST("/collector/providers/1688/open-login-browser", w, h.Open1688LoginBrowser)
	g.GET("/collector/providers/pinduoduo/auth-status", h.GetPinduoduoAuthStatus)
	g.POST("/collector/providers/pinduoduo/check-login", h.CheckPinduoduoLogin)
	g.POST("/collect/providers/pinduoduo/check-login", h.CheckPinduoduoLogin)
	g.POST("/collector/providers/pinduoduo/open-login-browser", w, h.OpenPinduoduoLoginBrowser)

	g.POST("/collector/providers/taobao_tmall/check-login", h.CheckTaobaoTmallLogin)
	g.POST("/collect/providers/taobao_tmall/check-login", h.CheckTaobaoTmallLogin)
	g.POST("/collector/providers/taobao_tmall/open-login-browser", w, h.OpenTaobaoTmallLoginBrowser)
	g.POST("/collect/providers/taobao_tmall/open-login-browser", w, h.OpenTaobaoTmallLoginBrowser)
}
