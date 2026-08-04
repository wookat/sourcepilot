package shop_test

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

func newTenantShopRouter(db *gorm.DB, actorID uuid.UUID, tenantID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TenantID, tenantID)
		c.Next()
	})
	shop.Register(r.Group("/api/v1"), &shop.Handler{Svc: &shop.Service{DB: db}})
	return r
}

// Credential-carrying shop routes must be tenant scoped: tenant B may neither
// overwrite tenant A's platform credentials nor learn that the shop exists.
func TestShopAuthRoutesAreTenantScoped(t *testing.T) {
	db := openShopGuardTestDB(t)

	shopA := shop.Shop{Base: model.Base{ID: uuid.New()}, TenantID: 1, Platform: "douyin_shop", ShopName: "租户A店铺", Status: "active"}
	if err := db.Create(&shopA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&shop.ShopAuthToken{ShopID: shopA.ID, Platform: "douyin_shop", AppKey: "a-app-key"}).Error; err != nil {
		t.Fatal(err)
	}
	adminB := uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base: model.Base{ID: adminB}, TenantID: 2, Username: "tenantb", Email: "tenantb@example.com",
		PasswordHash: "x", Role: "admin", Status: "active",
	}).Error; err != nil {
		t.Fatal(err)
	}
	r := newTenantShopRouter(db, adminB, 2)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"put auth", http.MethodPut, "/api/v1/shops/" + shopA.ID.String() + "/auth", `{"appKey":"hijacked","appSecret":"hijacked","accessToken":"hijacked"}`},
		{"test connection", http.MethodPost, "/api/v1/shops/" + shopA.ID.String() + "/test-connection", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doGuardReq(r, tc.method, tc.path, tc.body)
			if w.Code == http.StatusOK {
				t.Fatalf("%s %s: cross-tenant call succeeded (body: %s)", tc.method, tc.path, w.Body.String())
			}
		})
	}

	var tok shop.ShopAuthToken
	if err := db.Where("shop_id = ?", shopA.ID).First(&tok).Error; err != nil {
		t.Fatal(err)
	}
	if tok.AppKey != "a-app-key" {
		t.Fatalf("tenant A credentials were overwritten cross-tenant: %+v", tok)
	}
}
