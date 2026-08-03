package productpublish

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
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// Regression (R71): product publications / SKU bindings must verify the
// parent product's tenant and the shop's store scope, returning 404 for
// cross-tenant / unauthorized access.
func TestPublicationSubresourcesScopedByTenantAndStore(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "scope.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &product.Product{}, &shop.Shop{}, &ProductPublication{}, &ProductPublicationSKU{}); err != nil {
		t.Fatal(err)
	}

	prod := &product.Product{TenantID: 1, Source: "manual", Title: "p1", Status: "draft"}
	if err := db.Create(prod).Error; err != nil {
		t.Fatal(err)
	}
	sh := &shop.Shop{TenantID: 1, ShopName: "s1", Platform: "douyin_shop", Status: shop.StatusActive}
	if err := db.Create(sh).Error; err != nil {
		t.Fatal(err)
	}
	pub := &ProductPublication{ProductID: prod.ID, ShopID: sh.ID, Platform: "douyin_shop", Status: StatusDraft, PublishStatus: StatusDraftCreated}
	if err := db.Create(pub).Error; err != nil {
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
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Set(ctxkey.TenantID, tid)
		if adminID != "" {
			c.Set(ctxkey.AdminID, adminID)
		}
		return c
	}

	// 角色1：同租户 admin —— 正常读取
	if list, err := svc.ListPublicationsByProduct(ginCtx(1, ""), prod.ID); err != nil || len(list) != 1 {
		t.Fatalf("same-tenant list publications: n=%d err=%v", len(list), err)
	}
	if _, err := svc.loadDouyinPublicationScoped(ginCtx(1, ""), pub.ID); err != nil {
		t.Fatalf("same-tenant sku-bindings parent: %v", err)
	}
	// 角色2：跨租户 admin —— 404，不泄露存在性
	if _, err := svc.ListPublicationsByProduct(ginCtx(2, ""), prod.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant list publications: want ErrRecordNotFound, got %v", err)
	}
	if _, err := svc.GetDouyinSKUBindings(ginCtx(2, ""), pub.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant sku-bindings: want ErrRecordNotFound, got %v", err)
	}
	// 角色3：operator 只授权其他店铺 —— 发布记录被店铺 scope 过滤 / SKU 绑定 404
	if list, err := svc.ListPublicationsByProduct(ginCtx(1, opID.String()), prod.ID); err != nil || len(list) != 0 {
		t.Fatalf("foreign-shop operator list publications: n=%d err=%v", len(list), err)
	}
	if _, err := svc.GetDouyinSKUBindings(ginCtx(1, opID.String()), pub.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("foreign-shop operator sku-bindings: want ErrRecordNotFound, got %v", err)
	}
}
