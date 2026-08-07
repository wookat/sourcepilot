package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/health"
	"github.com/trademind-ai/trademind/backend/internal/logger"
	"github.com/trademind-ai/trademind/backend/internal/middleware"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/adminuser"
	"github.com/trademind-ai/trademind/backend/internal/modules/aioperationbatch"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiopsworkbench"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproductimage"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproducttext"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiprompt"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/alerting"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/modules/backup"
	"github.com/trademind-ai/trademind/backend/internal/modules/bannedwords"
	"github.com/trademind-ai/trademind/backend/internal/modules/carrier"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectbrowserprofile"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectrule"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectruleai"
	"github.com/trademind-ai/trademind/backend/internal/modules/configstatus"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/demoseed"
	"github.com/trademind-ai/trademind/backend/internal/modules/disasterrecovery"
	"github.com/trademind-ai/trademind/backend/internal/modules/douyinpreflight"
	"github.com/trademind-ai/trademind/backend/internal/modules/douyinruntime"
	"github.com/trademind-ai/trademind/backend/internal/modules/exportmod"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/finance"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventorysyncp9"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpaudit"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpserver"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"github.com/trademind-ai/trademind/backend/internal/modules/migrationimport"
	"github.com/trademind-ai/trademind/backend/internal/modules/observabilitymod"
	"github.com/trademind-ai/trademind/backend/internal/modules/openapi"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationdashboard"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/platformtenant"
	"github.com/trademind-ai/trademind/backend/internal/modules/pricing"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productcheck"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/release"
	"github.com/trademind-ai/trademind/backend/internal/modules/reports"
	"github.com/trademind-ai/trademind/backend/internal/modules/restore"
	"github.com/trademind-ai/trademind/backend/internal/modules/securitymod"
	"github.com/trademind-ai/trademind/backend/internal/modules/selection"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/skucandidate"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
	"github.com/trademind-ai/trademind/backend/internal/modules/storagepublic"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"github.com/trademind-ai/trademind/backend/internal/modules/waybill"
	"github.com/trademind-ai/trademind/backend/internal/modules/webhook"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	"github.com/trademind-ai/trademind/backend/internal/pkg/observability"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
	"github.com/trademind-ai/trademind/backend/internal/providers/marketprice"
	"github.com/trademind-ai/trademind/backend/internal/providers/markettrend"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	platformamazon "github.com/trademind-ai/trademind/backend/internal/providers/platform/amazon"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	platformgoofish "github.com/trademind-ai/trademind/backend/internal/providers/platform/goofish"
	platformlazada "github.com/trademind-ai/trademind/backend/internal/providers/platform/lazada"
	platformshopee "github.com/trademind-ai/trademind/backend/internal/providers/platform/shopee"
	platformtiktok "github.com/trademind-ai/trademind/backend/internal/providers/platform/tiktok"
	"github.com/trademind-ai/trademind/backend/internal/providers/sourceinfo"
	"github.com/trademind-ai/trademind/backend/internal/providers/sourcematch"
	"github.com/trademind-ai/trademind/backend/internal/providers/trade"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"gorm.io/gorm"
)

type collectRunnerAdapter struct {
	c *collect.CollectorClient
}

func (a collectRunnerAdapter) RunCollect(ctx context.Context, source, rawURL string, options map[string]any) (json.RawMessage, error) {
	out, err := a.c.Collect(ctx, source, rawURL, options)
	if err != nil {
		return nil, err
	}
	return out.ProductJSON, nil
}

// selection1688CollectorGW adapts the collector client to the sourcematch
// crawler provider's minimal gateway surface.
type selection1688CollectorGW struct {
	c *collect.CollectorClient
}

func (a selection1688CollectorGW) HasAuth(ctx context.Context) (bool, error) {
	if a.c == nil {
		return false, fmt.Errorf("collector unavailable")
	}
	st, err := a.c.Get1688AuthStatus(ctx)
	if err != nil {
		return false, err
	}
	return st != nil && st.LoggedIn, nil
}

func (a selection1688CollectorGW) CollectDetail(ctx context.Context, rawURL string) (json.RawMessage, error) {
	if a.c == nil {
		return nil, fmt.Errorf("collector unavailable")
	}
	out, err := a.c.Collect(ctx, "1688", rawURL, map[string]any{"useBrowserProfile": true})
	if err != nil {
		return nil, err
	}
	return out.ProductJSON, nil
}

type browserProfileCollectorGW struct {
	c *collect.CollectorClient
}

func (g browserProfileCollectorGW) OpenProfileLogin(ctx context.Context, profileKey, rawURL string) (string, error) {
	if g.c == nil {
		return "", fmt.Errorf("collector client unavailable")
	}
	return g.c.OpenBrowserProfileLogin(ctx, profileKey, rawURL)
}

func (g browserProfileCollectorGW) CheckProfileAccess(ctx context.Context, profileKey, rawURL string) (*collectbrowserprofile.CheckResultDTO, error) {
	if g.c == nil {
		return nil, fmt.Errorf("collector client unavailable")
	}
	out, err := g.c.CheckBrowserProfileAccess(ctx, profileKey, rawURL)
	if err != nil {
		return nil, err
	}
	return &collectbrowserprofile.CheckResultDTO{
		AccessStatus: out.AccessStatus,
		FinalURL:     out.FinalURL,
		ErrorCode:    out.ErrorCode,
		Message:      out.Message,
	}, nil
}

func (a collectRunnerAdapter) CustomRuleTest(ctx context.Context, rawURL string, options map[string]any) (*collectrule.RuleTestResultDTO, error) {
	raw, err := a.c.CustomRuleTest(ctx, rawURL, options)
	if err != nil {
		return nil, err
	}
	var extracted map[string]interface{}
	if len(raw.ExtractedFields) > 0 {
		_ = json.Unmarshal(raw.ExtractedFields, &extracted)
	}
	var qualityScore map[string]interface{}
	if len(raw.QualityScore) > 0 {
		_ = json.Unmarshal(raw.QualityScore, &qualityScore)
	}
	return &collectrule.RuleTestResultDTO{
		AccessStatus:    raw.AccessStatus,
		FinalURL:        raw.FinalURL,
		HTTPStatus:      raw.HTTPStatus,
		ExtractedFields: extracted,
		MissingFields:   raw.MissingFields,
		Warnings:        raw.Warnings,
		QualityScore:    qualityScore,
		ErrorCode:       raw.ErrorCode,
		Suggestion:      raw.Suggestion,
		Product:         raw.Product,
	}, nil
}

type collectRuleCreatorAdapter struct {
	svc *collectrule.Service
}

func (a collectRuleCreatorAdapter) CreateFromAI(c *gin.Context, body collectrule.CreateRuleBody, adminID *uuid.UUID) (*collectrule.RuleDetailDTO, error) {
	if a.svc == nil {
		return nil, fmt.Errorf("collect rules unavailable")
	}
	return a.svc.Create(c, body, adminID)
}

// Deps holds process-wide dependencies for HTTP handlers.
type Deps struct {
	Config          *config.Config
	DB              *gorm.DB
	Redis           *rdb.Client
	Encrypter       *encrypt.Service
	MigrationsReady bool
	// OpLog optional; when nil Register creates a default operation log service from DB.
	OpLog *operationlog.Service
	// Obs optional P5 observability facade.
	Obs *observability.Observability
}

// RegisterNoRoute installs the unified JSON 404 envelope for unknown routes,
// replacing Gin's default plain-text "404 page not found".
func RegisterNoRoute(e *gin.Engine) {
	if e == nil {
		return
	}
	e.NoRoute(func(c *gin.Context) {
		response.Fail(c, 404, response.CodeNotFound, "接口不存在，请检查请求路径")
	})
}

// Register mounts routes on the engine and returns services for optional async workers.
func Register(r gin.IRouter, dep *Deps) (*collect.Service, *imagetask.Service, *ordersync.Service, *customersync.Service, *productpublish.Service, *inventory.Service, *taskcenter.Service, *douyinruntime.Service, *webhook.Service, *files.Service, *securitymod.Service, *selection.Service, *backup.Service) {
	if dep == nil {
		dep = &Deps{}
	}
	platformp.Bootstrap()
	h := healthHandler(dep)
	r.GET("/health", h)
	// k8s/监控探针惯用别名，与 /health 同语义
	r.GET("/healthz", h)
	r.GET("/api/v1/health", h)
	health.Register(r, &health.Deps{
		Config:          dep.Config,
		DB:              dep.DB,
		Redis:           dep.Redis,
		MigrationsReady: dep.MigrationsReady,
	})

	if dep.Obs != nil && dep.Config != nil && dep.Config.Observability.MetricsEnabled {
		metricsPath := strings.TrimSpace(dep.Config.Observability.MetricsPath)
		if metricsPath == "" {
			metricsPath = "/internal/metrics"
		}
		internal := r.Group(metricsPath)
		internal.Use(middleware.MetricsGuard(dep.Config.Observability.MetricsInternalOnly, nil))
		internal.GET("", observabilitymod.MetricsEndpoint(dep.Obs))
	}

	alertCooldown := 5 * time.Minute
	alertRecovery := true
	if dep.Config != nil {
		if dep.Config.Observability.AlertDefaultCooldownSecs > 0 {
			alertCooldown = time.Duration(dep.Config.Observability.AlertDefaultCooldownSecs) * time.Second
		}
		alertRecovery = dep.Config.Observability.AlertRecoveryEnabled
	}

	adminStore := &admin.Store{DB: dep.DB}
	var metricCatalog *metrics.Catalog
	if dep.Obs != nil {
		metricCatalog = dep.Obs.Catalog
	}
	sessionSvc := &auth.SessionService{Cfg: dep.Config, DB: dep.DB, Admins: adminStore, Metrics: metricCatalog}
	loginSvc := &auth.LoginService{Cfg: dep.Config, Admins: adminStore, Sessions: sessionSvc, Metrics: metricCatalog}
	settingsSvc := &settings.Service{DB: dep.DB, Encrypter: dep.Encrypter}
	opLogSvc := dep.OpLog
	if opLogSvc == nil {
		opLogSvc = &operationlog.Service{DB: dep.DB}
	}
	idempotencySvc := &idempotency.Service{DB: dep.DB}

	aiGateway := &aigate.Gateway{Settings: settingsSvc}

	authH := &auth.Handler{LoginSvc: loginSvc, Sessions: sessionSvc, Admins: adminStore, OpLog: opLogSvc, Redis: dep.Redis, Settings: settingsSvc, DB: dep.DB, Cfg: dep.Config}
	sessionH := &auth.SessionHandler{Cfg: dep.Config, Sessions: sessionSvc, OpLog: opLogSvc, DB: dep.DB}
	setH := &settings.Handler{Svc: settingsSvc, OpLog: opLogSvc, AIGateway: aiGateway, DB: dep.DB}
	opLogH := &operationlog.Handler{Svc: opLogSvc, DB: dep.DB}

	maxUp := int64(10 << 20)
	if dep.Config != nil {
		maxUp = dep.Config.MaxUploadBytes()
	}
	fileSvc := &files.Service{DB: dep.DB, Redis: dep.Redis, Settings: settingsSvc, MaxBytes: maxUp, Metrics: metricCatalog}
	fileH := &files.Handler{Svc: fileSvc}
	staticH := &files.StaticHandler{Settings: settingsSvc}

	collectorTimeout := 120 * time.Second
	if dep.Config != nil && dep.Config.CollectorTimeoutSeconds > 0 {
		collectorTimeout = time.Duration(dep.Config.CollectorTimeoutSeconds) * time.Second
	}
	collectorBase := "http://127.0.0.1:3100"
	if dep.Config != nil && dep.Config.CollectorBaseURL != "" {
		collectorBase = dep.Config.CollectorBaseURL
	}
	collectorClient := collect.NewCollectorClient(collectorBase, collectorTimeout)

	profileSvc := &collectbrowserprofile.Service{
		DB:        dep.DB,
		Collector: browserProfileCollectorGW{c: collectorClient},
		OpLog:     opLogSvc,
		Timeout:   collectorTimeout,
	}
	profileH := &collectbrowserprofile.Handler{Svc: profileSvc}

	collectRuleSvc := &collectrule.Service{
		DB:          dep.DB,
		OpLog:       opLogSvc,
		Runner:      collectRunnerAdapter{c: collectorClient},
		Profiles:    profileSvc,
		TestTimeout: collectorTimeout,
	}

	promptSvc := &aiprompt.Service{DB: dep.DB}
	aiTaskSvc := &aitask.Service{DB: dep.DB}
	imageTaskSvc := &imagetask.Service{
		DB:        dep.DB,
		OpLog:     opLogSvc,
		Settings:  settingsSvc,
		Files:     fileSvc,
		Redis:     dep.Redis,
		AIGateway: aiGateway,
	}
	if dep.Config != nil {
		imageTaskSvc.QueueEnabled = dep.Config.ImageQueueEnabled
		imageTaskSvc.AutoRetryEnabled = dep.Config.ImageAutoRetryEnabled
		imageTaskSvc.MaxAutoRetries = dep.Config.ImageMaxRetries
		imageTaskSvc.RetryBaseDelaySec = dep.Config.ImageRetryBaseDelaySeconds
		imageTaskSvc.RetryMaxDelaySec = dep.Config.ImageRetryMaxDelaySeconds
		if strings.TrimSpace(dep.Config.ImageQueueName) != "" {
			imageTaskSvc.QueueName = strings.TrimSpace(dep.Config.ImageQueueName)
		} else {
			imageTaskSvc.QueueName = "image:tasks"
		}
		if dep.Config.ImageTaskTimeoutSeconds > 0 {
			imageTaskSvc.TaskTimeoutMax = time.Duration(dep.Config.ImageTaskTimeoutSeconds) * time.Second
		}
	}
	imageTaskH := &imagetask.Handler{Svc: imageTaskSvc}

	productSvc := &product.Service{
		DB:          dep.DB,
		OpLog:       opLogSvc,
		Settings:    settingsSvc,
		Prompts:     promptSvc,
		AITasks:     aiTaskSvc,
		AIGateway:   aiGateway,
		Idempotency: idempotencySvc,
	}
	productH := &product.Handler{Svc: productSvc, Files: fileSvc}

	aiBatchSvc := &aioperationbatch.Service{
		DB:       dep.DB,
		Settings: settingsSvc,
		Products: productSvc,
		Image:    imageTaskSvc,
		OpLog:    opLogSvc,
	}
	aiBatchH := &aioperationbatch.Handler{Svc: aiBatchSvc}

	aiProductTextSvc := &aiproducttext.Service{
		DB:          dep.DB,
		Settings:    settingsSvc,
		Products:    productSvc,
		OpLog:       opLogSvc,
		Idempotency: idempotencySvc,
		Metrics:     metricCatalog,
	}
	aiProductTextH := &aiproducttext.Handler{Svc: aiProductTextSvc}

	aiProductImageSvc := &aiproductimage.Service{
		DB:          dep.DB,
		Settings:    settingsSvc,
		Products:    productSvc,
		Image:       imageTaskSvc,
		OpLog:       opLogSvc,
		Idempotency: idempotencySvc,
		Metrics:     metricCatalog,
	}
	aiProductImageH := &aiproductimage.Handler{Svc: aiProductImageSvc}

	promptH := &aiprompt.Handler{Svc: promptSvc}
	aiTaskH := &aitask.Handler{Svc: aiTaskSvc}

	collectSvc := &collect.Service{
		DB:       dep.DB,
		Products: productSvc,
		Rules:    collectRuleSvc,
		Profiles: profileSvc,
		OpLog:    opLogSvc,
		Client:   collectorClient,
		Redis:    dep.Redis,
	}
	if dep.Config != nil {
		collectSvc.QueueName = dep.Config.CollectQueueName
		collectSvc.QueueEnabled = dep.Config.CollectQueueEnabled
		collectSvc.BatchMaxURLs = dep.Config.CollectBatchMaxURLs
		collectSvc.CollectorTimeoutSeconds = dep.Config.CollectorTimeoutSeconds
		collectSvc.AutoRetryEnabled = dep.Config.CollectAutoRetryEnabled
		collectSvc.MaxAutoRetries = dep.Config.CollectMaxRetries
		collectSvc.RetryBaseDelaySec = dep.Config.CollectRetryBaseDelaySeconds
		collectSvc.RetryMaxDelaySec = dep.Config.CollectRetryMaxDelaySeconds
		collectSvc.TaskLeaseTimeoutSeconds = dep.Config.CollectTaskTimeoutSeconds
		collectSvc.Batch1688Concurrency = dep.Config.CollectBatchConcurrency1688
		collectSvc.Batch1688DelayMinMs = dep.Config.CollectBatchDelayMinMs1688
		collectSvc.Batch1688DelayMaxMs = dep.Config.CollectBatchDelayMaxMs1688
		collectSvc.BatchRetryOnBlocked = dep.Config.CollectBatchRetryOnBlocked
		collectSvc.BatchRetryOnTimeout = dep.Config.CollectBatchRetryOnTimeout
		collectSvc.Batch1688MaxRetries = dep.Config.CollectBatchMaxRetries1688
		collectSvc.Settings = settingsSvc
	}
	collectH := &collect.Handler{Svc: collectSvc}
	collectRuleH := &collectrule.Handler{Svc: collectRuleSvc}

	shopSvc := &shop.Service{
		DB:        dep.DB,
		Encrypter: dep.Encrypter,
		OpLog:     opLogSvc,
		Redis:     dep.Redis,
		Settings:  settingsSvc,
	}
	productSvc.Shops = shopSvc
	platformtiktok.BindShops(shopSvc.TikTokShopsBridge())
	platformtiktok.BindPublishImages(newTikTokListingImageFetcher(settingsSvc))
	platformdouyin.BindShops(shopSvc.DouyinShopsBridge())
	platformdouyin.RegisterProvider()
	platformtiktok.RegisterProvider()
	platformshopee.BindShops(shopSvc.ShopeeShopsBridge())
	platformshopee.BindPublishImages(newTikTokListingImageFetcher(settingsSvc))
	platformshopee.RegisterProvider()
	platformlazada.BindShops(shopSvc.LazadaShopsBridge())
	platformlazada.BindPublishImages(newTikTokListingImageFetcher(settingsSvc))
	platformlazada.RegisterProvider()
	platformamazon.BindShops(shopSvc.AmazonShopsBridge())
	platformamazon.RegisterProvider()
	platformgoofish.RegisterProvider()
	shopH := &shop.Handler{Svc: shopSvc}

	storagePublicSvc := &storagepublic.Service{Settings: settingsSvc, OpLog: opLogSvc}
	storagePublicH := &storagepublic.Handler{Svc: storagePublicSvc, OpLog: opLogSvc, DB: dep.DB}

	douyinPreflightSvc := &douyinpreflight.Service{
		DB:       dep.DB,
		Settings: settingsSvc,
		Shops:    shopSvc,
		Storage:  storagePublicSvc,
	}
	douyinPreflightH := &douyinpreflight.Handler{Svc: douyinPreflightSvc, OpLog: opLogSvc}
	douyinRuntimeSvc := &douyinruntime.Service{
		DB:        dep.DB,
		Settings:  settingsSvc,
		Preflight: douyinPreflightSvc,
		OpLog:     opLogSvc,
	}
	douyinRuntimeH := &douyinruntime.Handler{Svc: douyinRuntimeSvc}

	inventorySvc := &inventory.Service{
		DB:          dep.DB,
		Redis:       dep.Redis,
		Shops:       shopSvc,
		Settings:    settingsSvc,
		OpLog:       opLogSvc,
		Idempotency: idempotencySvc,
		Metrics:     metricCatalog,
	}
	if dep.Config != nil {
		inventorySvc.QueueEnabled = dep.Config.InventorySyncQueueEnabled
		if strings.TrimSpace(dep.Config.InventorySyncQueueName) != "" {
			inventorySvc.QueueName = strings.TrimSpace(dep.Config.InventorySyncQueueName)
		} else {
			inventorySvc.QueueName = "inventory:sync:tasks"
		}
		if dep.Config.InventorySyncTaskTimeoutSeconds > 0 {
			inventorySvc.TaskTimeout = time.Duration(dep.Config.InventorySyncTaskTimeoutSeconds) * time.Second
		}
	}
	inventoryH := &inventory.Handler{Svc: inventorySvc}

	carrierSvc := &carrier.Service{DB: dep.DB, OpLog: opLogSvc}
	carrierH := &carrier.Handler{Svc: carrierSvc}

	waybillSvc := &waybill.Service{DB: dep.DB, OpLog: opLogSvc, Carriers: carrierSvc}
	waybillH := &waybill.Handler{Svc: waybillSvc}

	orderSvc := &order.Service{DB: dep.DB, OpLog: opLogSvc, Shops: shopSvc, Settings: settingsSvc, Idempotency: idempotencySvc, Carriers: carrierSvc, Waybill: waybillSvc}
	orderH := &order.Handler{Svc: orderSvc, Inv: inventorySvc}

	orderSyncSvc := &ordersync.Service{
		DB:          dep.DB,
		Redis:       dep.Redis,
		Shops:       shopSvc,
		Orders:      orderSvc,
		Inventory:   inventorySvc,
		OpLog:       opLogSvc,
		Idempotency: idempotencySvc,
		Metrics:     metricCatalog,
	}
	if dep.Config != nil {
		orderSyncSvc.QueueEnabled = dep.Config.OrderSyncQueueEnabled
		if strings.TrimSpace(dep.Config.OrderSyncQueueName) != "" {
			orderSyncSvc.QueueName = strings.TrimSpace(dep.Config.OrderSyncQueueName)
		} else {
			orderSyncSvc.QueueName = "order:sync:tasks"
		}
		if dep.Config.OrderSyncTaskTimeoutSeconds > 0 {
			orderSyncSvc.TaskTimeout = time.Duration(dep.Config.OrderSyncTaskTimeoutSeconds) * time.Second
		}
	}
	orderSyncH := &ordersync.Handler{Svc: orderSyncSvc}

	excSvc := &orderexception.Service{
		DB:     dep.DB,
		Orders: orderSvc,
		Inv:    inventorySvc,
	}
	excCmd := &orderexception.Commands{
		Svc:    excSvc,
		Orders: orderSvc,
		Inv:    inventorySvc,
	}
	excH := &orderexception.Handler{
		Svc:   excSvc,
		Cmds:  excCmd,
		OpLog: opLogSvc,
	}

	customerChatSvc := &customerchat.Service{
		DB:          dep.DB,
		Settings:    settingsSvc,
		Prompts:     promptSvc,
		AITasks:     aiTaskSvc,
		AIGateway:   aiGateway,
		OpLog:       opLogSvc,
		Orders:      orderSvc,
		Shops:       shopSvc,
		Idempotency: idempotencySvc,
	}
	customerChatH := &customerchat.Handler{Svc: customerChatSvc}

	customerSyncSvc := &customersync.Service{
		DB:           dep.DB,
		Redis:        dep.Redis,
		Shops:        shopSvc,
		Settings:     settingsSvc,
		CustomerChat: customerChatSvc,
		OpLog:        opLogSvc,
	}
	if dep.Config != nil {
		customerSyncSvc.QueueEnabled = dep.Config.CustomerMessageSyncQueueEnabled
		if strings.TrimSpace(dep.Config.CustomerMessageSyncQueueName) != "" {
			customerSyncSvc.QueueName = strings.TrimSpace(dep.Config.CustomerMessageSyncQueueName)
		} else {
			customerSyncSvc.QueueName = "customer:message:sync:tasks"
		}
		if dep.Config.CustomerMessageSyncTaskTimeoutSeconds > 0 {
			customerSyncSvc.TaskTimeout = time.Duration(dep.Config.CustomerMessageSyncTaskTimeoutSeconds) * time.Second
		}
	}
	customerSyncH := &customersync.Handler{Svc: customerSyncSvc}

	bannedWordsSvc := &bannedwords.Service{DB: dep.DB, OpLog: opLogSvc}
	bannedWordsH := &bannedwords.Handler{Svc: bannedWordsSvc, OpLog: opLogSvc}
	productSvc.Compliance = &bannedwords.AIComplianceAdvisor{Svc: bannedWordsSvc}

	readinessSvc := &productcheck.Service{
		DB:       dep.DB,
		Settings: settingsSvc,
		Shops:    shopSvc,
		Banned:   bannedWordsSvc,
	}
	productSvc.Readiness = func(ctx context.Context, req product.OperationReadinessRequest) (*product.OperationReadinessResult, error) {
		res, err := readinessSvc.CheckProductReadiness(ctx, productcheck.CheckProductReadinessRequest{
			ProductID: req.ProductID,
			Platform:  req.Platform,
			Mode:      req.Mode,
		})
		if err != nil {
			return nil, err
		}
		out := &product.OperationReadinessResult{
			Status:       res.Status,
			Result:       res.Result,
			CanPublish:   res.CanPublish,
			ErrorCount:   res.ErrorCount,
			WarningCount: res.WarningCount,
			Checks:       make([]product.OperationReadinessCheck, 0, len(res.Checks)),
		}
		for _, c := range res.Checks {
			out.Checks = append(out.Checks, product.OperationReadinessCheck{
				Group:      c.Group,
				Code:       c.Code,
				Level:      c.Level,
				Message:    c.Message,
				Suggestion: c.Suggestion,
			})
		}
		return out, nil
	}

	productPublishSvc := &productpublish.Service{
		DB:          dep.DB,
		Redis:       dep.Redis,
		Shops:       shopSvc,
		Settings:    settingsSvc,
		OpLog:       opLogSvc,
		Readiness:   readinessSvc,
		Idempotency: idempotencySvc,
	}
	if dep.Config != nil {
		productPublishSvc.AllowTenantZeroTasks = dep.Config.EnableDemoSeed && !config.IsProduction(dep.Config.AppEnv)
		productPublishSvc.QueueEnabled = dep.Config.ProductPublishQueueEnabled
		if strings.TrimSpace(dep.Config.ProductPublishQueueName) != "" {
			productPublishSvc.QueueName = strings.TrimSpace(dep.Config.ProductPublishQueueName)
		} else {
			productPublishSvc.QueueName = "product:publish:tasks"
		}
		if dep.Config.ProductPublishTaskTimeoutSeconds > 0 {
			productPublishSvc.TaskTimeout = time.Duration(dep.Config.ProductPublishTaskTimeoutSeconds) * time.Second
		}
		productPublishSvc.BatchMaxProducts = dep.Config.PublishBatchMaxProducts
		productPublishSvc.BatchMaxTargets = dep.Config.PublishBatchMaxTargets
		productPublishSvc.BatchMaxTasks = dep.Config.PublishBatchMaxTasks
	}
	productPublishH := &productpublish.Handler{Svc: productPublishSvc}
	readinessH := &productcheck.Handler{
		Svc:   readinessSvc,
		OpLog: opLogSvc,
	}

	r.GET("/static/*filepath", staticH.Serve)

	v1 := r.Group("/api/v1")
	v1.POST("/auth/login", authH.Login)
	v1.POST("/auth/register", authH.Register)
	v1.GET("/auth/register-config", authH.RegisterConfig)
	v1.POST("/auth/send-email-code", authH.SendEmailCode)
	v1.POST("/auth/refresh", sessionH.Refresh)

	authed := v1.Group("")
	authed.Use(middleware.BearerAuthWithDB(dep.Config, dep.DB, sessionSvc))
	authed.Use(adminperm.ReadonlyWriteGuard(dep.DB))
	authed.Use(adminperm.ProductRouteTenantGuard(dep.DB))
	authed.GET("/auth/profile", authH.Profile)
	authed.POST("/auth/logout", authH.Logout)
	authed.GET("/auth/sessions", sessionH.ListSessions)
	authed.DELETE("/auth/sessions/:id", sessionH.DeleteSession)
	authed.POST("/auth/sessions/revoke-others", sessionH.RevokeOthers)
	authed.POST("/auth/logout-all", sessionH.LogoutAll)
	authed.GET("/settings", setH.List)
	authed.PUT("/settings", setH.Put)
	authed.GET("/settings/report-currency", setH.GetReportCurrency)
	authed.PUT("/settings/report-currency", setH.PutReportCurrency)
	authed.GET("/settings/integration-schemas", setH.IntegrationSchemas)
	authed.GET("/settings/integrations/overview", setH.IntegrationOverview)
	authed.POST("/settings/test-ai", setH.TestAI)
	authed.POST("/settings/test-image", adminperm.RequireWriteMW(dep.DB, adminperm.PermSettingsManage), imageTaskH.TestImage)
	authed.POST("/settings/test-ocr", adminperm.RequireWriteMW(dep.DB, adminperm.PermSettingsManage), imageTaskH.TestOCR)
	authed.POST("/settings/test-storage", setH.TestStorage)
	authed.POST("/settings/test-platform-tiktok", setH.TestPlatformTikTok)
	authed.POST("/settings/test-email", setH.TestEmail)

	authed.GET("/operation-logs", opLogH.List)
	authed.POST("/files/upload", fileH.Upload)
	authed.GET("/files", fileH.List)
	authed.DELETE("/files/:id", fileH.Delete)

	aiprompt.Register(authed, promptH, adminperm.RequirePlatformAdminMW(dep.DB))
	aioperationbatch.Register(authed, aiBatchH)
	aitask.Register(authed, aiTaskH)
	imagetask.Register(authed, imageTaskH)
	product.Register(authed, productH)
	aiproducttext.Register(authed, aiProductTextH)
	aiproductimage.Register(authed, aiProductImageH)
	pricingSvc := &pricing.Service{DB: dep.DB, Settings: settingsSvc, OpLog: opLogSvc}
	pricingH := &pricing.Handler{Svc: pricingSvc}
	pricing.Register(authed, pricingH)
	collect.Register(authed, collectH)
	selectionSvc := &selection.Service{
		DB:          dep.DB,
		Redis:       dep.Redis,
		Products:    productSvc,
		Settings:    settingsSvc,
		OpLog:       opLogSvc,
		AIGateway:   aiGateway,
		Prompts:     promptSvc,
		MarketMock:  &marketprice.MockProvider{},
		SourceMock:  &sourcematch.MockProvider{},
		SourceCrawl: &sourcematch.CrawlerProvider{Collector: selection1688CollectorGW{c: collectorClient}},
		SourceOpen:  &sourcematch.Open1688Provider{},
		Banned:      bannedWordsSvc,
		Trend:       markettrend.NewRegistry(),
	}
	if dep.Config != nil {
		selectionSvc.QueueName = dep.Config.SelectionQueueName
		selectionSvc.TaskLeaseTimeoutSeconds = dep.Config.SelectionTaskTimeoutSeconds
	}
	selectionH := &selection.Handler{Svc: selectionSvc}
	selection.Register(authed, selectionH)
	collectRuleAISvc := &collectruleai.Service{
		Settings:    settingsSvc,
		Prompts:     promptSvc,
		AIGateway:   aiGateway,
		Analyzer:    collectruleai.NewPageAnalyzer(collectorClient),
		Runner:      collectRunnerAdapter{c: collectorClient},
		Profiles:    profileSvc,
		Providers:   collectruleai.NewProviderResolver(collectSvc),
		Rules:       collectRuleCreatorAdapter{svc: collectRuleSvc},
		OpLog:       opLogSvc,
		TestTimeout: collectorTimeout,
	}
	collectRuleAIH := &collectruleai.Handler{Svc: collectRuleAISvc}
	collectruleai.Register(authed, collectRuleAIH)
	collectrule.Register(authed, collectRuleH)
	collectbrowserprofile.Register(authed, profileH)

	// 1688 采集浏览器登录态（与 /api/v1/collector/... 等价，便于前端与文档引用）
	collectorAlias := r.Group("/api/collector")
	collectorAlias.Use(middleware.BearerAuthWithDB(dep.Config, dep.DB, sessionSvc))
	collectorAlias.Use(adminperm.ReadonlyWriteGuard(dep.DB))
	collectorAlias.GET("/providers/1688/auth-status", collectH.Get1688AuthStatus)
	collectorAlias.POST("/providers/1688/open-login-browser", collectH.Open1688LoginBrowser)
	collectorAlias.GET("/providers/pinduoduo/auth-status", collectH.GetPinduoduoAuthStatus)
	collectorAlias.POST("/providers/pinduoduo/check-login", collectH.CheckPinduoduoLogin)
	collectorAlias.POST("/providers/pinduoduo/open-login-browser", collectH.OpenPinduoduoLoginBrowser)
	collectorAlias.POST("/providers/taobao_tmall/check-login", collectH.CheckTaobaoTmallLogin)
	collectorAlias.POST("/providers/taobao_tmall/open-login-browser", collectH.OpenTaobaoTmallLoginBrowser)
	productcheck.Register(authed, readinessH)
	order.Register(authed, orderH)
	carrier.Register(authed, carrierH)
	waybill.Register(authed, waybillH)
	bannedwords.Register(authed, bannedWordsH)
	sourcingSvc := &sourcing.Service{DB: dep.DB, Settings: settingsSvc, OpLog: opLogSvc, Provider: &sourceinfo.Mock{}}
	migrationImportSvc := &migrationimport.Service{DB: dep.DB, Products: productSvc, Orders: orderSvc, Sourcing: sourcingSvc, OpLog: opLogSvc}
	migrationimport.Register(authed, &migrationimport.Handler{Svc: migrationImportSvc})
	sourcingH := &sourcing.Handler{Svc: sourcingSvc}
	sourcing.Register(authed, sourcingH)
	procurementSvc := &procurement.Service{DB: dep.DB, OpLog: opLogSvc, Provider: trade.NewMock1688(), Settings: settingsSvc, OrderEvents: orderSvc}
	excSvc.Cost = procurementSvc
	orderSvc.Automation = &order.AutomationHooks{
		GenerateProcurement: func(ctx context.Context, tenantID int64, orderID uuid.UUID, idempotencyKey string) (string, error) {
			res, err := procurementSvc.Generate(ctx, procurement.GenerateBody{
				OrderIDs:       []string{orderID.String()},
				IdempotencyKey: idempotencyKey,
			}, nil)
			if err != nil {
				return "", err
			}
			if len(res.Blockers) > 0 {
				b := res.Blockers[0]
				if b.Code == "order.empty" {
					// 前置条件重试也无法满足：记「跳过」而非可重试失败。
					return "", &order.AutomationSkip{Reason: fmt.Sprintf("跳过生成采购单：%s", b.Message)}
				}
				return "", fmt.Errorf("生成采购单被阻断：%s", b.Message)
			}
			if len(res.Orders) == 0 {
				return "订单已有有效采购单，无需重复生成", nil
			}
			return fmt.Sprintf("已自动生成 %d 张采购单", len(res.Orders)), nil
		},
		PlanWarehouse: func(ctx context.Context, tenantID int64, strategy string, demands []order.AutomationWarehouseDemand) (*order.AutomationWarehousePlan, error) {
			lines := make([]inventory.WarehouseDemand, 0, len(demands))
			for _, d := range demands {
				lines = append(lines, inventory.WarehouseDemand{
					ProductSKUID: d.ProductSKUID,
					SKUCode:      d.SKUCode,
					Quantity:     d.Quantity,
				})
			}
			plan, err := inventorySvc.PlanOrderWarehouse(ctx, tenantID, strategy, lines)
			if err != nil {
				return nil, err
			}
			return &order.AutomationWarehousePlan{
				WarehouseID:   plan.WarehouseID,
				WarehouseName: plan.WarehouseName,
			}, nil
		},
	}
	procurementH := &procurement.Handler{Svc: procurementSvc}
	procurement.Register(authed, procurementH)
	reportsSvc := &reports.Service{DB: dep.DB, Settings: settingsSvc, Proc: procurementSvc}
	reportsH := &reports.Handler{Svc: reportsSvc}
	reports.Register(authed, reportsH)

	financeSvc := &finance.Service{DB: dep.DB, Settings: settingsSvc, Proc: procurementSvc, OpLog: opLogSvc}
	financeH := &finance.Handler{Svc: financeSvc}
	finance.Register(authed, financeH)
	// The payment import kind commits through the finance service (created
	// after the import service because it needs the procurement service).
	migrationImportSvc.Finance = financeSvc
	skuCandH := &skucandidate.Handler{Svc: &skucandidate.Service{DB: dep.DB}}
	skucandidate.Register(authed, skuCandH)
	orderexception.Register(authed, excH)

	// MCP read-only entry: tenant API token management + tool-call audit log
	// + POST /api/mcp endpoint.
	mcpTokenSvc := &mcptoken.Service{DB: dep.DB}
	mcptoken.Register(authed, &mcptoken.Handler{Svc: mcpTokenSvc, OpLog: opLogSvc})
	mcpAuditSvc := &mcpaudit.Service{DB: dep.DB}
	mcpaudit.Register(authed, &mcpaudit.Handler{Svc: mcpAuditSvc})
	if dep.Config == nil || dep.Config.MCPEnabled {
		mcpDeps := &mcpserver.Deps{DB: dep.DB, Tokens: mcpTokenSvc, Exceptions: excSvc, Audits: mcpAuditSvc}
		if dep.Redis != nil && dep.Redis.Client != nil {
			mcpDeps.Redis = dep.Redis.Client
		}
		if dep.Config != nil {
			mcpDeps.RateRPS = float64(dep.Config.MCPRateRPS)
			mcpDeps.RateBurst = dep.Config.MCPRateBurst
			mcpDeps.Version = dep.Config.AppVersion
		}
		r.POST("/api/mcp", mcpserver.GinHandler(mcpDeps))
	}
	// Open API read-only entry: GET /api/open/v1/* authenticated by the same
	// tenant token system (purpose openapi/both).
	if dep.Config == nil || dep.Config.OpenAPIEnabled {
		openDeps := &openapi.Deps{DB: dep.DB, Tokens: mcpTokenSvc, Exceptions: excSvc, Audits: mcpAuditSvc}
		if dep.Redis != nil && dep.Redis.Client != nil {
			openDeps.Redis = dep.Redis.Client
		}
		if dep.Config != nil {
			openDeps.RateRPS = float64(dep.Config.OpenAPIRateRPS)
			openDeps.RateBurst = dep.Config.OpenAPIRateBurst
		}
		openapi.Register(r, openDeps)
	}

	ordersync.Register(authed, orderSyncH)
	customersync.Register(authed, customerSyncH)
	customerchat.Register(authed, customerChatH)
	shop.RegisterPublic(v1, shopH)
	webhookRegistry := webhook.NewRegistry(dep.Config)
	// Register Douyin webhook signature verifier — loads app_secret from settings
	// (platform_douyin_shop group). If secret is missing the verifier is still
	// registered but Verify returns CodeVerifierNotConfigured.
	if settingsSvc != nil {
		appEnv := ""
		if dep.Config != nil {
			appEnv = dep.Config.AppEnv
		}
		if plain, err := settingsSvc.PlainByGroup(context.Background(), 0, "platform_douyin_shop"); err == nil {
			appSecret := plain["app_secret"]
			webhookRegistry.Register("douyin_shop", webhook.NewDouyinVerifierWithEnv(appSecret, appEnv))
			webhookRegistry.Register("douyin", webhook.NewDouyinVerifierWithEnv(appSecret, appEnv))
		} else {
			webhookRegistry.Register("douyin_shop", webhook.NewDouyinVerifierWithEnv("", appEnv))
			webhookRegistry.Register("douyin", webhook.NewDouyinVerifierWithEnv("", appEnv))
		}
	}
	webhookSvc := &webhook.Service{
		DB:          dep.DB,
		Idempotency: idempotencySvc,
		Verifiers:   webhookRegistry,
		Metrics:     metricCatalog,
		ShopResolver: &webhook.DBWebhookShopResolver{
			DB: dep.DB,
			AppEnv: func() string {
				if dep.Config != nil {
					return dep.Config.AppEnv
				}
				return ""
			}(),
		},
		OrderHandler: &ordersync.DouyinOrderWebhookHandler{
			DB:     dep.DB,
			Shops:  shopSvc,
			Orders: orderSvc,
		},
		AppEnv: "",
	}
	if dep.Config != nil {
		webhookSvc.MaxPayloadBytes = dep.Config.WebhookMaxBodyBytes()
		webhookSvc.MaxClockSkew = dep.Config.WebhookMaxClockSkew()
		webhookSvc.AppEnv = dep.Config.AppEnv
	}
	webhookH := &webhook.Handler{Svc: webhookSvc}
	webhook.Register(authed, webhookH)
	webhook.RegisterPublic(v1, webhookH)
	shop.Register(authed, shopH)
	storagepublic.Register(authed, storagePublicH)
	douyinpreflight.Register(authed, douyinPreflightH)
	douyinruntime.Register(authed, douyinRuntimeH)
	productpublish.Register(authed, productPublishH)
	inventory.Register(authed, inventoryH)
	workerH := &worker.Handler{DB: dep.DB, Cfg: dep.Config}
	worker.Register(authed, workerH)

	operationTaskH := &operationtask.Handler{
		Svc:                   operationtask.NewAPIService(dep.DB),
		AllowTenantZeroWrites: dep.Config != nil && dep.Config.EnableDemoSeed && !config.IsProduction(dep.Config.AppEnv),
	}
	operationtask.Register(authed, operationTaskH)
	inventorySyncP9H := &inventorysyncp9.Handler{Svc: inventorysyncp9.NewAPIService(dep.DB)}
	inventorysyncp9.Register(authed, inventorySyncP9H)

	tcSvc := &taskcenter.Service{
		DB:             dep.DB,
		Cfg:            dep.Config,
		OpLog:          opLogSvc,
		Settings:       settingsSvc,
		Collect:        collectSvc,
		Image:          imageTaskSvc,
		OrderSync:      orderSyncSvc,
		CustomerSync:   customerSyncSvc,
		ProductPublish: productPublishSvc,
		Inventory:      inventorySvc,
		AIProductText:  aiProductTextSvc,
		CustomerChat:   customerChatSvc,
	}
	tcH := &taskcenter.Handler{Svc: tcSvc}
	taskcenter.Register(authed, tcH)

	configStatusSvc := &configstatus.Service{
		DB:       dep.DB,
		Settings: settingsSvc,
		Redis:    dep.Redis,
		Config:   dep.Config,
		Shops:    shopSvc,
	}
	configStatusH := &configstatus.Handler{Svc: configStatusSvc}
	configstatus.Register(authed, configStatusH)

	dashSvc := &operationdashboard.Service{
		DB:              dep.DB,
		Inventory:       inventorySvc,
		TaskCenter:      tcSvc,
		OrderExceptions: excSvc,
		ConfigStatus:    configStatusSvc,
	}
	dashH := &operationdashboard.Handler{Svc: dashSvc, Reports: reportsSvc, Settings: settingsSvc, OpLog: opLogSvc}
	operationdashboard.Register(authed, dashH)

	aiOpsWorkbenchSvc := &aiopsworkbench.Service{
		DB:           dep.DB,
		ProductCheck: readinessSvc,
		TaskCenter:   tcSvc,
	}
	aiOpsWorkbenchH := &aiopsworkbench.Handler{Svc: aiOpsWorkbenchSvc}
	aiopsworkbench.Register(authed, aiOpsWorkbenchH)

	adminUserSvc := &adminuser.Service{DB: dep.DB, OpLog: opLogSvc, Sessions: sessionSvc}
	adminUserH := &adminuser.Handler{Svc: adminUserSvc}
	adminuser.Register(authed, adminUserH)

	platformTenantSvc := &platformtenant.Service{DB: dep.DB, OpLog: opLogSvc}
	platformTenantH := &platformtenant.Handler{Svc: platformTenantSvc}
	platformtenant.Register(authed, platformTenantH)

	secSvc := &securitymod.Service{DB: dep.DB, Cfg: dep.Config, OpLogs: opLogSvc, Metrics: metricCatalog}
	secH := &securitymod.Handler{Svc: secSvc, DB: dep.DB}
	securitymod.RegisterRoutes(authed, secH)

	exportSvc := &exportmod.Service{DB: dep.DB}
	exportH := &exportmod.Handler{Svc: exportSvc}
	exportmod.RegisterRoutes(authed, exportH)

	backupStore, backupStoreErr := backup.NewStore(dep.Config)
	if backupStoreErr != nil {
		logger.L().Warn("backup object storage disabled", "error", backupStoreErr)
	} else if backupStore != nil {
		logger.L().Info("backup object storage enabled", "target", backupStore.Target())
	}
	backupSvc := &backup.Service{DB: dep.DB, Cfg: dep.Config, Enc: dep.Encrypter, OpLog: opLogSvc, Metrics: metricCatalog, Store: backupStore}
	backupH := &backup.Handler{Svc: backupSvc}
	// /ops/* operates on the whole deployment (database backups, restores,
	// releases, DR drills), so it is platform-tenant only.
	opsPlatform := authed.Group("", adminperm.RequirePlatformAdminMW(dep.DB))
	backup.Register(opsPlatform, backupH)
	restoreSvc := &restore.Service{DB: dep.DB, Cfg: dep.Config, Enc: dep.Encrypter, Backup: backupSvc, OpLog: opLogSvc}
	restoreH := &restore.Handler{Svc: restoreSvc}
	restore.Register(opsPlatform, restoreH)
	releaseSvc := &release.Service{DB: dep.DB, Cfg: dep.Config, Backup: backupSvc, OpLog: opLogSvc}
	releaseH := &release.Handler{Svc: releaseSvc}
	release.Register(opsPlatform, releaseH)
	drSvc := &disasterrecovery.Service{DB: dep.DB, Cfg: dep.Config, Backup: backupSvc}
	drH := &disasterrecovery.Handler{Svc: drSvc}
	disasterrecovery.Register(opsPlatform, drH)

	alertSvc := alerting.NewService(dep.DB, alertCooldown, alertRecovery)
	obsH := &observabilitymod.Handler{DB: dep.DB, Cfg: dep.Config, Obs: dep.Obs, Alert: alertSvc}
	observabilitymod.Register(authed, obsH)

	if dep.Config != nil && dep.Config.EnableDemoSeed && !config.IsProduction(dep.Config.AppEnv) {
		demoSeedSvc := &demoseed.Service{DB: dep.DB, OpLog: opLogSvc, AppEnv: dep.Config.AppEnv}
		demoSeedH := &demoseed.Handler{Svc: demoSeedSvc}
		demoseed.Register(authed, demoSeedH)
	}

	return collectSvc, imageTaskSvc, orderSyncSvc, customerSyncSvc, productPublishSvc, inventorySvc, tcSvc, douyinRuntimeSvc, webhookSvc, fileSvc, secSvc, selectionSvc, backupSvc
}

func healthHandler(dep *Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		checks := gin.H{
			"database": "unknown",
			"redis":    "unknown",
		}

		if dep.DB != nil {
			sqlDB, err := dep.DB.DB()
			if err != nil || sqlDB.PingContext(ctx) != nil {
				checks["database"] = "down"
			} else {
				checks["database"] = "ok"
			}
		} else {
			checks["database"] = "down"
		}

		switch {
		case dep.Redis == nil:
			checks["redis"] = "skipped"
		default:
			if err := dep.Redis.Ping(ctx).Err(); err != nil {
				checks["redis"] = "down"
			} else {
				checks["redis"] = "ok"
			}
		}

		status := "up"
		if checks["database"] != "ok" {
			response.Fail(c, 503, response.CodeInternalError, "database unavailable")
			return
		}
		if checks["redis"] == "down" {
			status = "degraded"
		}

		appEnv := ""
		queueEnabled := false
		queueName := "collect:tasks"
		workerConc := 2
		if dep.Config != nil {
			appEnv = dep.Config.AppEnv
			queueEnabled = dep.Config.CollectQueueEnabled
			if strings.TrimSpace(dep.Config.CollectQueueName) != "" {
				queueName = strings.TrimSpace(dep.Config.CollectQueueName)
			}
			workerConc = dep.Config.CollectWorkerConcurrency
			if workerConc < 1 {
				workerConc = 2
			}
		}
		cq := collect.BuildCollectQueueHealthBlock(ctx, dep.Redis, queueEnabled, queueName, workerConc)
		if queueEnabled && !cq.RedisAvailable && checks["redis"] == "ok" {
			status = "degraded"
		}

		imgQEnabled := false
		imgQName := "image:tasks"
		imgWConc := 2
		if dep.Config != nil {
			imgQEnabled = dep.Config.ImageQueueEnabled
			if strings.TrimSpace(dep.Config.ImageQueueName) != "" {
				imgQName = strings.TrimSpace(dep.Config.ImageQueueName)
			}
			imgWConc = dep.Config.ImageWorkerConcurrency
			if imgWConc < 1 {
				imgWConc = 2
			}
		}
		iq := imagetask.BuildImageQueueHealthBlock(ctx, dep.Redis, imgQEnabled, imgQName, imgWConc)
		if imgQEnabled && !iq.RedisAvailable && checks["redis"] == "ok" {
			status = "degraded"
		}

		osQEnabled := false
		osQName := "order:sync:tasks"
		osWConc := 1
		if dep.Config != nil {
			osQEnabled = dep.Config.OrderSyncQueueEnabled
			if strings.TrimSpace(dep.Config.OrderSyncQueueName) != "" {
				osQName = strings.TrimSpace(dep.Config.OrderSyncQueueName)
			}
			osWConc = dep.Config.OrderSyncWorkerConcurrency
			if osWConc < 1 {
				osWConc = 1
			}
		}
		osq := ordersync.BuildOrderSyncQueueHealthBlock(ctx, dep.Redis, osQEnabled, osQName, osWConc)
		if osQEnabled && !osq.RedisAvailable && checks["redis"] == "ok" {
			status = "degraded"
		}

		cmQEnabled := false
		cmQName := "customer:message:sync:tasks"
		cmWConc := 1
		if dep.Config != nil {
			cmQEnabled = dep.Config.CustomerMessageSyncQueueEnabled
			if strings.TrimSpace(dep.Config.CustomerMessageSyncQueueName) != "" {
				cmQName = strings.TrimSpace(dep.Config.CustomerMessageSyncQueueName)
			}
			cmWConc = dep.Config.CustomerMessageSyncWorkerConcurrency
			if cmWConc < 1 {
				cmWConc = 1
			}
		}
		cmq := customersync.BuildCustomerMessageSyncQueueHealthBlock(ctx, dep.Redis, cmQEnabled, cmQName, cmWConc)
		if cmQEnabled && !cmq.RedisAvailable && checks["redis"] == "ok" {
			status = "degraded"
		}

		ppQEnabled := false
		ppQName := "product:publish:tasks"
		ppWConc := 1
		if dep.Config != nil {
			ppQEnabled = dep.Config.ProductPublishQueueEnabled
			if strings.TrimSpace(dep.Config.ProductPublishQueueName) != "" {
				ppQName = strings.TrimSpace(dep.Config.ProductPublishQueueName)
			}
			ppWConc = dep.Config.ProductPublishWorkerConcurrency
			if ppWConc < 1 {
				ppWConc = 1
			}
		}
		ppq := productpublish.BuildProductPublishQueueHealthBlock(ctx, dep.Redis, ppQEnabled, ppQName, ppWConc)
		if ppQEnabled && !ppq.RedisAvailable && checks["redis"] == "ok" {
			status = "degraded"
		}

		invQEnabled := false
		invQName := "inventory:sync:tasks"
		invWConc := 1
		if dep.Config != nil {
			invQEnabled = dep.Config.InventorySyncQueueEnabled
			if strings.TrimSpace(dep.Config.InventorySyncQueueName) != "" {
				invQName = strings.TrimSpace(dep.Config.InventorySyncQueueName)
			}
			invWConc = dep.Config.InventorySyncWorkerConcurrency
			if invWConc < 1 {
				invWConc = 1
			}
		}
		invq := inventory.BuildInventorySyncQueueHealthBlock(ctx, dep.Redis, invQEnabled, invQName, invWConc)
		if invQEnabled && !invq.RedisAvailable && checks["redis"] == "ok" {
			status = "degraded"
		}

		workers := worker.BuildHealthWorkersBlock(ctx, dep.DB, dep.Config)
		if workers.Degraded {
			status = "degraded"
		}

		response.OK(c, gin.H{
			"status":                   status,
			"appEnv":                   appEnv,
			"checks":                   checks,
			"collectQueue":             cq,
			"imageQueue":               iq,
			"orderSyncQueue":           osq,
			"customerMessageSyncQueue": cmq,
			"productPublishQueue":      ppq,
			"inventorySyncQueue":       invq,
			"workers":                  workers,
			"timestamp":                time.Now().UTC().Format(time.RFC3339),
		})
	}
}
