package ordersync

import (
	"errors"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// Regression (R71): POST /shops/:id/sync-orders must verify the shop's
// tenant + store scope before creating a task, returning 404 for
// cross-tenant / unauthorized shops without side effects.
func TestCreateShopSyncScopedByTenantAndStore(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "scope.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &shop.Shop{}, &OrderSyncTask{}); err != nil {
		t.Fatal(err)
	}
	sh := &shop.Shop{TenantID: 1, ShopName: "s1", Platform: "douyin_shop", Status: shop.StatusActive}
	if err := db.Create(sh).Error; err != nil {
		t.Fatal(err)
	}
	opID := uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: opID},
		TenantID:     1,
		Username:     "op-" + opID.String()[:8],
		Email:        "op-" + opID.String()[:8] + "@example.com",
		PasswordHash: "x",
		Role:         "operator",
		Status:       "active",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&admin.UserStorePermission{UserID: opID, StoreID: uuid.New(), PermissionScope: "operate"}).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	svc := &Service{DB: db}
	ginCtx := func(tid int64, adminID string) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/", nil)
		c.Set(ctxkey.TenantID, tid)
		if adminID != "" {
			c.Set(ctxkey.AdminID, adminID)
		}
		return c
	}

	// 角色1：同租户 admin —— 通过 scope 校验（后续业务校验不得是 404）
	if _, err := svc.CreateShopSync(ginCtx(1, ""), sh.ID, SyncOrdersBody{}, nil); errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("same-tenant create must pass scope check, got %v", err)
	}
	// 角色2：跨租户 admin —— 404，不泄露存在性
	if _, err := svc.CreateShopSync(ginCtx(2, ""), sh.ID, SyncOrdersBody{}, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant create: want ErrRecordNotFound, got %v", err)
	}
	// 角色3：operator 只授权其他店铺 —— 404
	if _, err := svc.CreateShopSync(ginCtx(1, opID.String()), sh.ID, SyncOrdersBody{}, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("foreign-shop operator create: want ErrRecordNotFound, got %v", err)
	}
	// 越权请求不得落库
	var cnt int64
	if err := db.Model(&OrderSyncTask{}).Where("tenant_id <> ?", 1).Count(&cnt).Error; err != nil || cnt != 0 {
		t.Fatalf("out-of-scope create must not persist tasks, cnt=%d err=%v", cnt, err)
	}
}
