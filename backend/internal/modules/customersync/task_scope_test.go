package customersync

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

// Regression (R70): message sync task detail/retry must apply tenant + store
// scope and return 404 for out-of-scope tasks instead of leaking them.
func TestSyncTaskScopedByTenantAndStore(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "scope.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &shop.Shop{}, &CustomerMessageSyncTask{}); err != nil {
		t.Fatal(err)
	}
	sh := &shop.Shop{TenantID: 1, ShopName: "s1", Platform: "douyin_shop", Status: shop.StatusActive}
	if err := db.Create(sh).Error; err != nil {
		t.Fatal(err)
	}
	task := &CustomerMessageSyncTask{TenantID: 1, ShopID: sh.ID, Platform: "douyin_shop", TaskType: TaskTypeCustomerMessageSync, Status: StatusFailed, Mode: ModeManual}
	if err := db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	opID := uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: opID},
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
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Set(ctxkey.TenantID, tid)
		if adminID != "" {
			c.Set(ctxkey.AdminID, adminID)
		}
		return c
	}

	if _, err := svc.GetDTO(ginCtx(1, ""), task.ID); err != nil {
		t.Fatalf("same-tenant admin get: %v", err)
	}
	if _, err := svc.GetDTO(ginCtx(2, ""), task.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant get: want ErrRecordNotFound, got %v", err)
	}
	if _, err := svc.GetDTO(ginCtx(1, opID.String()), task.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("foreign-shop operator get: want ErrRecordNotFound, got %v", err)
	}
	if _, err := svc.RetryFailed(ginCtx(2, ""), task.ID, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant retry: want ErrRecordNotFound, got %v", err)
	}
	if _, err := svc.CreateShopSync(ginCtx(2, ""), sh.ID, SyncCustomerMessagesBody{}, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant create: want ErrRecordNotFound, got %v", err)
	}
}
