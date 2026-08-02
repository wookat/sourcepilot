package shop_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func openShopGuardTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:shopguard_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &shop.Shop{}, &shop.ShopAuthToken{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedGuardUser(t *testing.T, db *gorm.DB, role string) uuid.UUID {
	t.Helper()
	uid := uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uid},
		Username:     "u-" + uid.String()[:12],
		Email:        "u-" + uid.String()[:12] + "@example.com",
		PasswordHash: "x",
		Role:         role,
		Status:       "active",
	}).Error; err != nil {
		t.Fatalf("seed %s user: %v", role, err)
	}
	return uid
}

func newShopGuardRouter(db *gorm.DB, actorID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TenantID, int64(0))
		c.Next()
	})
	shop.Register(r.Group("/api/v1"), &shop.Handler{Svc: &shop.Service{DB: db}})
	return r
}

func doGuardReq(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	r.ServeHTTP(w, req)
	return w
}

// readonly must be rejected with 403 on every settings-center / shop write route
// before any business validation runs.
func TestShopWriteRoutesRejectReadonly(t *testing.T) {
	db := openShopGuardTestDB(t)
	actor := seedGuardUser(t, db, "readonly")
	r := newShopGuardRouter(db, actor)
	sid := uuid.New().String()

	cases := []struct{ method, path, body string }{
		{http.MethodPut, "/api/v1/platform/settings/douyin_shop", `{}`},
		{http.MethodPost, "/api/v1/platform/settings/douyin_shop/test-connection", `{}`},
		{http.MethodPut, "/api/v1/platform/publish-settings/douyin_shop", `{}`},
		{http.MethodPost, "/api/v1/shops", `{"platform":"douyin_shop","shopName":"x"}`},
		{http.MethodPut, "/api/v1/shops/" + sid, `{"shopName":"x"}`},
		{http.MethodDelete, "/api/v1/shops/" + sid, ""},
		{http.MethodPut, "/api/v1/shops/" + sid + "/auth", `{}`},
		{http.MethodPost, "/api/v1/shops/" + sid + "/test-connection", `{}`},
	}
	for _, tc := range cases {
		if w := doGuardReq(r, tc.method, tc.path, tc.body); w.Code != http.StatusForbidden {
			t.Fatalf("readonly %s %s: got %d body=%s, want 403", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}

// platform app/publish settings belong to the settings center and require
// settings.manage: operator is rejected there but keeps store operations.
func TestPlatformSettingsRoutesRequireSettingsManage(t *testing.T) {
	db := openShopGuardTestDB(t)
	actor := seedGuardUser(t, db, "operator")
	r := newShopGuardRouter(db, actor)

	for _, tc := range []struct{ method, path string }{
		{http.MethodPut, "/api/v1/platform/settings/douyin_shop"},
		{http.MethodPut, "/api/v1/platform/publish-settings/douyin_shop"},
	} {
		if w := doGuardReq(r, tc.method, tc.path, `{}`); w.Code != http.StatusForbidden {
			t.Fatalf("operator %s %s: got %d body=%s, want 403", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	// operator retains store.operate: guard passes, request reaches the handler.
	if w := doGuardReq(r, http.MethodDelete, "/api/v1/shops/"+uuid.New().String(), ""); w.Code == http.StatusForbidden {
		t.Fatalf("operator shop delete must pass permission guard, got 403 body=%s", w.Body.String())
	}
}

// admin passes the guards; unknown shop id surfaces as business 404, not 403.
func TestShopWriteRoutesAllowAdmin(t *testing.T) {
	db := openShopGuardTestDB(t)
	actor := seedGuardUser(t, db, "admin")
	r := newShopGuardRouter(db, actor)
	if w := doGuardReq(r, http.MethodDelete, "/api/v1/shops/"+uuid.New().String(), ""); w.Code == http.StatusForbidden {
		t.Fatalf("admin shop delete must pass permission guard, got 403 body=%s", w.Body.String())
	}
}
