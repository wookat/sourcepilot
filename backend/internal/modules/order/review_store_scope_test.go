package order_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/waybill"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

type storeScopeEnv struct {
	router          *gin.Engine
	db              *gorm.DB
	grantedOrderID  uuid.UUID
	ungrantedOrder  uuid.UUID
	ungrantedNo     string
	ungrantedShopID uuid.UUID
}

// setupOperatorStoreScopeEnv builds one tenant with two shops and an operator
// granted on the first shop only.
func setupOperatorStoreScopeEnv(t *testing.T) *storeScopeEnv {
	t.Helper()
	db := openReviewTestDB(t)
	if err := db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &waybill.ShippingRule{}); err != nil {
		t.Fatal(err)
	}
	granted := shop.Shop{Base: model.Base{ID: uuid.New()}, TenantID: 1, Platform: "douyin", ShopName: "授权店铺", Status: "active"}
	ungranted := shop.Shop{Base: model.Base{ID: uuid.New()}, TenantID: 1, Platform: "douyin", ShopName: "未授权店铺", Status: "active"}
	if err := db.Create(&granted).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ungranted).Error; err != nil {
		t.Fatal(err)
	}
	mkOrder := func(no string, shopID uuid.UUID) order.Order {
		o := order.Order{
			Base: model.Base{ID: uuid.New()}, TenantID: 1, ShopID: &shopID, OrderNo: no,
			Platform: "douyin", Status: order.StatusPending, ReviewStatus: order.ReviewStatusHeld,
			Currency: "CNY", TotalAmount: 100, CustomerName: "客户",
		}
		if err := db.Create(&o).Error; err != nil {
			t.Fatal(err)
		}
		return o
	}
	og := mkOrder("SCOPE-GRANTED", granted.ID)
	ou := mkOrder("SCOPE-UNGRANTED", ungranted.ID)

	operator := admin.AdminUser{Base: model.Base{ID: uuid.New()}, TenantID: 1, Username: "operator", Role: "operator", Status: admin.StatusActive}
	if err := db.Create(&operator).Error; err != nil {
		t.Fatal(err)
	}
	perm := admin.UserStorePermission{UserID: operator.ID, StoreID: granted.ID, Platform: "douyin", PermissionScope: admin.StorePermScopeOperate}
	if err := db.Create(&perm).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, operator.ID.String())
		c.Set(ctxkey.TenantID, int64(1))
		c.Next()
	})
	order.Register(r.Group("/api/v1"), &order.Handler{Svc: &order.Service{DB: db, Waybill: &waybill.Service{DB: db}}})
	return &storeScopeEnv{router: r, db: db, grantedOrderID: og.ID, ungrantedOrder: ou.ID, ungrantedNo: ou.OrderNo, ungrantedShopID: ungranted.ID}
}

func (e *storeScopeEnv) post(t *testing.T, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

func (e *storeScopeEnv) reviewStatus(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var o order.Order
	if err := e.db.First(&o, "id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	return o.ReviewStatus
}

// An operator may only approve / reject orders of the stores granted to the
// account: ids of an ungranted store must fail and leave the order untouched.
func TestReviewDecisionRespectsStoreScope(t *testing.T) {
	env := setupOperatorStoreScopeEnv(t)

	for _, action := range []string{"approve", "reject"} {
		w := env.post(t, "/api/v1/order-review/"+action, `{"orderIds":["`+env.ungrantedOrder.String()+`"]}`)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: got %d, want 200 envelope (body %s)", action, w.Code, w.Body.String())
		}
		var resp struct {
			Data struct {
				Done    int `json:"done"`
				Failed  int `json:"failed"`
				Results []struct {
					OK      bool   `json:"ok"`
					OrderNo string `json:"orderNo"`
				} `json:"results"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp.Data.Done != 0 || resp.Data.Failed != 1 {
			t.Fatalf("%s: ungranted store order was decided: %s", action, w.Body.String())
		}
		if len(resp.Data.Results) == 1 && resp.Data.Results[0].OrderNo != "" {
			t.Fatalf("%s: leaked order number of ungranted store: %s", action, w.Body.String())
		}
		if got := env.reviewStatus(t, env.ungrantedOrder); got != order.ReviewStatusHeld {
			t.Fatalf("%s: review status changed to %q", action, got)
		}
	}

	w := env.post(t, "/api/v1/order-review/approve", `{"orderIds":["`+env.grantedOrderID.String()+`"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("granted store approve: got %d (%s)", w.Code, w.Body.String())
	}
	if got := env.reviewStatus(t, env.grantedOrderID); got != order.ReviewStatusApproved {
		t.Fatalf("granted store approve did not apply, status=%q", got)
	}
}

// The workbench header count (pendingTotal) must follow the same store scope
// as the list items: an operator granted one shop only counts that shop's
// pending / held orders, not the whole tenant.
func TestReviewWorkbenchPendingTotalRespectsStoreScope(t *testing.T) {
	env := setupOperatorStoreScopeEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/order-review", nil)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d (%s)", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Items []struct {
				OrderNo string `json:"orderNo"`
			} `json:"items"`
			Total        int64 `json:"total"`
			PendingTotal int64 `json:"pendingTotal"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data.Items) != 1 || resp.Data.Items[0].OrderNo != "SCOPE-GRANTED" {
		t.Fatalf("list should only contain granted-store order: %s", w.Body.String())
	}
	if resp.Data.Total != 1 || resp.Data.PendingTotal != 1 {
		t.Fatalf("pendingTotal must follow the store scope, got total=%d pendingTotal=%d (%s)",
			resp.Data.Total, resp.Data.PendingTotal, w.Body.String())
	}
}

// Shipping recommendations must not resolve orders outside the operator's
// store scope, by id or by order number.
func TestShippingRecommendationsRespectStoreScope(t *testing.T) {
	env := setupOperatorStoreScopeEnv(t)

	cases := map[string]string{
		"by id": `{"items":[{"orderId":"` + env.ungrantedOrder.String() + `"}]}`,
		"by no": `{"items":[{"orderNo":"` + env.ungrantedNo + `"}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			w := env.post(t, "/api/v1/orders/shipping-recommendations", body)
			if w.Code != http.StatusOK {
				t.Fatalf("got %d (%s)", w.Code, w.Body.String())
			}
			var resp struct {
				Data struct {
					Items []struct {
						OrderID string `json:"orderId"`
						Message string `json:"message"`
					} `json:"items"`
				} `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatal(err)
			}
			if len(resp.Data.Items) != 1 {
				t.Fatalf("unexpected items: %s", w.Body.String())
			}
			if resp.Data.Items[0].OrderID != "" {
				t.Fatalf("resolved order outside store scope: %s", w.Body.String())
			}
			if resp.Data.Items[0].Message == "" {
				t.Fatalf("expected not-found message: %s", w.Body.String())
			}
		})
	}
}
