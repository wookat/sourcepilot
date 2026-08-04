package order_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

// GET /orders/:id/sku-matches must return 404 (not 500) when the order does
// not exist or is out of the caller's tenant/store scope, matching the
// sibling /orders/:id endpoint.
func TestGetOrderSKUMatchesNotFoundReturns404(t *testing.T) {
	db := openImportTestDB(t)
	if err := db.AutoMigrate(&order.OrderItemSKUMatch{}); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, uuid.New().String())
		c.Set(ctxkey.TenantID, int64(0))
		c.Next()
	})
	order.Register(r.Group("/api/v1"), &order.Handler{Svc: &order.Service{DB: db}})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+uuid.New().String()+"/sku-matches", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404 (body: %s)", w.Code, w.Body.String())
	}
}
