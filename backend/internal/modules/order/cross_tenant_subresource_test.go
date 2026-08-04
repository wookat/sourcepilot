package order_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// crossTenantEnv builds tenant A's order (with items + shipment) and a tenant B
// admin router, so every order sub-resource route can be probed cross-tenant.
type crossTenantEnv struct {
	router     *gin.Engine
	orderID    uuid.UUID
	itemID     uuid.UUID
	shipmentID uuid.UUID
	db         *gorm.DB
}

func setupCrossTenantOrderEnv(t *testing.T) *crossTenantEnv {
	t.Helper()
	db := openImportTestDB(t)
	if err := db.AutoMigrate(&admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}

	shopA := shop.Shop{Base: model.Base{ID: uuid.New()}, TenantID: 1, Platform: "douyin", ShopName: "租户A店铺", Status: "active"}
	if err := db.Create(&shopA).Error; err != nil {
		t.Fatal(err)
	}
	o := order.Order{
		Base: model.Base{ID: uuid.New()}, TenantID: 1, ShopID: &shopA.ID, OrderNo: "A-1001",
		Platform: "douyin", Status: "paid", Currency: "CNY", TotalAmount: 100,
		CustomerName: "租户A客户", CustomerPhone: "13800000000",
	}
	if err := db.Create(&o).Error; err != nil {
		t.Fatal(err)
	}
	item := order.OrderItem{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, OrderID: o.ID, ProductTitle: "商品A", SKUCode: "SKU-A", Quantity: 1, UnitPrice: 100, TotalPrice: 100}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ship := order.OrderShipment{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, OrderID: o.ID, Carrier: "顺丰速运", CarrierCode: "sf", TrackingNo: "SF123456", Status: "shipped", ShippedAt: &now}
	if err := db.Create(&ship).Error; err != nil {
		t.Fatal(err)
	}

	tenantBAdmin := admin.AdminUser{Base: model.Base{ID: uuid.New()}, TenantID: 2, Username: "tenantb-admin", Role: "admin", Status: admin.StatusActive}
	if err := db.Create(&tenantBAdmin).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, tenantBAdmin.ID.String())
		c.Set(ctxkey.TenantID, int64(2))
		c.Next()
	})
	order.Register(r.Group("/api/v1"), &order.Handler{Svc: &order.Service{DB: db}})

	return &crossTenantEnv{router: r, orderID: o.ID, itemID: item.ID, shipmentID: ship.ID, db: db}
}

// Tenant B admin must not reach tenant A order items / shipments through any
// sub-resource route: every probe returns 404 (no existence leak) and leaves
// tenant A data untouched.
func TestOrderSubresourceCrossTenant404(t *testing.T) {
	env := setupCrossTenantOrderEnv(t)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"append item", http.MethodPost, "/api/v1/orders/" + env.orderID.String() + "/items", `{"productTitle":"越权商品","quantity":1,"unitPrice":1,"totalPrice":1}`},
		{"patch item", http.MethodPut, "/api/v1/orders/" + env.orderID.String() + "/items/" + env.itemID.String(), `{"productTitle":"被改标题","quantity":9,"unitPrice":1,"totalPrice":9}`},
		{"delete item", http.MethodDelete, "/api/v1/orders/" + env.orderID.String() + "/items/" + env.itemID.String(), ""},
		{"append shipment", http.MethodPost, "/api/v1/orders/" + env.orderID.String() + "/shipments", `{"carrier":"顺丰速运","trackingNo":"SF999999","status":"shipped"}`},
		{"patch shipment", http.MethodPut, "/api/v1/orders/" + env.orderID.String() + "/shipments/" + env.shipmentID.String(), `{"carrier":"顺丰速运","trackingNo":"SF000000","status":"delivered"}`},
		{"refresh tracking", http.MethodPost, "/api/v1/orders/" + env.orderID.String() + "/shipments/" + env.shipmentID.String() + "/refresh-tracking", ""},
		{"delete shipment", http.MethodDelete, "/api/v1/orders/" + env.orderID.String() + "/shipments/" + env.shipmentID.String(), ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			if tc.body == "" {
				req = httptest.NewRequest(tc.method, tc.path, nil)
			} else {
				req = httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			env.router.ServeHTTP(w, req)
			if w.Code != http.StatusNotFound {
				t.Fatalf("%s %s: got %d, want 404 (body: %s)", tc.method, tc.path, w.Code, w.Body.String())
			}
		})
	}

	var itemCount, shipCount int64
	env.db.Model(&order.OrderItem{}).Where("order_id = ?", env.orderID).Count(&itemCount)
	env.db.Model(&order.OrderShipment{}).Where("order_id = ?", env.orderID).Count(&shipCount)
	if itemCount != 1 || shipCount != 1 {
		t.Fatalf("cross-tenant probes mutated tenant A data: items=%d shipments=%d", itemCount, shipCount)
	}
	var item order.OrderItem
	if err := env.db.First(&item, "id = ?", env.itemID).Error; err != nil {
		t.Fatal(err)
	}
	if item.ProductTitle != "商品A" || item.Quantity != 1 {
		t.Fatalf("tenant A item mutated: %+v", item)
	}
	var ship order.OrderShipment
	if err := env.db.First(&ship, "id = ?", env.shipmentID).Error; err != nil {
		t.Fatal(err)
	}
	if ship.TrackingNo != "SF123456" || ship.Status != "shipped" {
		t.Fatalf("tenant A shipment mutated: %+v", ship)
	}
}
