package product_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

type createScopeEnv struct {
	db       *gorm.DB
	mkRouter func(role string, grants ...admin.UserStorePermission) *gin.Engine
	shopID   uuid.UUID
}

func newCreateScopeEnv(t *testing.T) *createScopeEnv {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(
		&admin.AdminUser{}, &admin.UserStorePermission{},
		&product.Product{}, &product.ProductImage{}, &product.ProductSKU{},
		&product.ProductPlatformPublishConfig{}, &productpublish.ProductPublication{}, &shop.Shop{}, &imagetask.ImageTask{},
	); err != nil {
		t.Fatal(err)
	}
	sh := shop.Shop{TenantID: 0, Platform: "douyin_shop", ShopName: "e2e-shop", Status: "active", AuthStatus: "authorized"}
	if err := db.Create(&sh).Error; err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	return &createScopeEnv{
		db:     db,
		shopID: sh.ID,
		mkRouter: func(role string, grants ...admin.UserStorePermission) *gin.Engine {
			uid := uuid.New()
			if err := db.Create(&admin.AdminUser{
				Base:         model.Base{ID: uid},
				Username:     role + "-" + uid.String()[:8],
				Email:        role + "-" + uid.String()[:8] + "@example.com",
				PasswordHash: "x",
				Role:         role,
				Status:       "active",
			}).Error; err != nil {
				t.Fatalf("seed %s user: %v", role, err)
			}
			for _, g := range grants {
				g.ID = uuid.New()
				g.UserID = uid
				if err := db.Create(&g).Error; err != nil {
					t.Fatalf("seed grant: %v", err)
				}
			}
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set(ctxkey.AdminID, uid.String())
				c.Set(ctxkey.TenantID, int64(0))
				c.Next()
			})
			product.Register(r.Group("/api/v1"), &product.Handler{Svc: &product.Service{DB: db}})
			return r
		},
	}
}

func postDraft(r *gin.Engine, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func listDrafts(r *gin.Engine) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	r.ServeHTTP(w, req)
	return w
}

func countRows(t *testing.T, db *gorm.DB, m any) int64 {
	t.Helper()
	var n int64
	if err := db.Model(m).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func assertZeroDirty(t *testing.T, db *gorm.DB) {
	t.Helper()
	if n := countRows(t, db, &product.Product{}); n != 0 {
		t.Fatalf("left %d dirty product row(s), want 0", n)
	}
	if n := countRows(t, db, &product.ProductPlatformPublishConfig{}); n != 0 {
		t.Fatalf("left %d dirty publish config row(s), want 0", n)
	}
}

// Operator without a shop selection is rejected up front (400) and leaves no rows.
func TestCreateDraftOperatorWithoutShopRejected(t *testing.T) {
	env := newCreateScopeEnv(t)
	w := postDraft(env.mkRouter("operator", admin.UserStorePermission{StoreID: env.shopID, PermissionScope: "operate"}), `{"title":"QA-Create-Scope"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("operator create without shop: got %d, want 400 (body=%s)", w.Code, w.Body.String())
	}
	assertZeroDirty(t, env.db)
}

// Operator with a shop outside their grants gets 404 (no existence leak) and zero dirty rows.
func TestCreateDraftOperatorUnauthorizedShopRejected(t *testing.T) {
	env := newCreateScopeEnv(t)
	other := uuid.New()
	w := postDraft(env.mkRouter("operator", admin.UserStorePermission{StoreID: env.shopID, PermissionScope: "operate"}),
		fmt.Sprintf(`{"title":"QA-Create-Scope","shopId":%q}`, other))
	if w.Code != http.StatusNotFound {
		t.Fatalf("operator create with unauthorized shop: got %d, want 404 (body=%s)", w.Code, w.Body.String())
	}
	assertZeroDirty(t, env.db)
}

// Operator with only a view grant on the shop gets 403 and zero dirty rows.
func TestCreateDraftOperatorViewOnlyGrantRejected(t *testing.T) {
	env := newCreateScopeEnv(t)
	w := postDraft(env.mkRouter("operator", admin.UserStorePermission{StoreID: env.shopID, PermissionScope: "view"}),
		fmt.Sprintf(`{"title":"QA-Create-Scope","shopId":%q}`, env.shopID))
	if w.Code != http.StatusForbidden {
		t.Fatalf("operator create with view-only grant: got %d, want 403 (body=%s)", w.Code, w.Body.String())
	}
	assertZeroDirty(t, env.db)
}

// Operator with an operate grant creates a draft bound to the shop and can see it in their own list.
func TestCreateDraftOperatorAuthorizedShopAllowedAndVisible(t *testing.T) {
	env := newCreateScopeEnv(t)
	r := env.mkRouter("operator", admin.UserStorePermission{StoreID: env.shopID, PermissionScope: "operate"})
	w := postDraft(r, fmt.Sprintf(`{"title":"QA-Create-Scope","shopId":%q}`, env.shopID))
	if w.Code != http.StatusOK {
		t.Fatalf("operator create with authorized shop: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	if n := countRows(t, env.db, &product.Product{}); n != 1 {
		t.Fatalf("persisted %d product row(s), want 1", n)
	}
	var cfg product.ProductPlatformPublishConfig
	if err := env.db.First(&cfg).Error; err != nil {
		t.Fatalf("publish config not persisted: %v", err)
	}
	if cfg.ShopID == nil || *cfg.ShopID != env.shopID {
		t.Fatalf("publish config shop = %v, want %s", cfg.ShopID, env.shopID)
	}
	if cfg.Platform != "douyin_shop" {
		t.Fatalf("publish config platform = %q, want douyin_shop", cfg.Platform)
	}
	lw := listDrafts(r)
	if lw.Code != http.StatusOK || !strings.Contains(lw.Body.String(), "QA-Create-Scope") {
		t.Fatalf("operator list should include own draft (code=%d body=%s)", lw.Code, lw.Body.String())
	}
}

// Readonly write-guard path (403) must not persist a row either.
func TestCreateDraftReadonlyRejectedWithoutDirtyRow(t *testing.T) {
	env := newCreateScopeEnv(t)
	w := postDraft(env.mkRouter("readonly"), `{"title":"QA-Create-Scope"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("readonly create: got %d, want 403 (body=%s)", w.Code, w.Body.String())
	}
	assertZeroDirty(t, env.db)
}

// Admin keeps the existing behavior: shop selection is optional.
func TestCreateDraftAdminWithoutShopStillAllowed(t *testing.T) {
	env := newCreateScopeEnv(t)
	w := postDraft(env.mkRouter("admin"), `{"title":"QA-Create-Scope"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("admin create: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	if n := countRows(t, env.db, &product.Product{}); n != 1 {
		t.Fatalf("admin create persisted %d row(s), want 1", n)
	}
	if n := countRows(t, env.db, &product.ProductPlatformPublishConfig{}); n != 0 {
		t.Fatalf("admin create without shop persisted %d config row(s), want 0", n)
	}
}

// Admin may optionally bind a shop at creation time.
func TestCreateDraftAdminWithShopBindsConfig(t *testing.T) {
	env := newCreateScopeEnv(t)
	w := postDraft(env.mkRouter("admin"), fmt.Sprintf(`{"title":"QA-Create-Scope","shopId":%q}`, env.shopID))
	if w.Code != http.StatusOK {
		t.Fatalf("admin create with shop: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	var cfg product.ProductPlatformPublishConfig
	if err := env.db.First(&cfg).Error; err != nil {
		t.Fatalf("publish config not persisted: %v", err)
	}
	if cfg.ShopID == nil || *cfg.ShopID != env.shopID {
		t.Fatalf("publish config shop = %v, want %s", cfg.ShopID, env.shopID)
	}
}

// Admin with a nonexistent shop id gets 404 and zero dirty rows.
func TestCreateDraftAdminUnknownShopRejected(t *testing.T) {
	env := newCreateScopeEnv(t)
	w := postDraft(env.mkRouter("admin"), fmt.Sprintf(`{"title":"QA-Create-Scope","shopId":%q}`, uuid.New()))
	if w.Code != http.StatusNotFound {
		t.Fatalf("admin create with unknown shop: got %d, want 404 (body=%s)", w.Code, w.Body.String())
	}
	assertZeroDirty(t, env.db)
}
