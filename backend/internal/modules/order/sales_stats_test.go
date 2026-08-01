package order_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

func TestSalesStats(t *testing.T) {
	db := openImportTestDB(t)
	svc := &order.Service{DB: db}
	paid := createFlowTestOrder(t, svc, "SO-ST-PAID")
	if err := db.Model(&order.Order{}).Where("id = ?", paid.ID).
		Updates(map[string]any{"payment_status": order.PaymentPaid, "total_amount": 12.5, "currency": "USD"}).Error; err != nil {
		t.Fatal(err)
	}
	shipped := createFlowTestOrder(t, svc, "SO-ST-SHIPPED")
	if err := db.Model(&order.Order{}).Where("id = ?", shipped.ID).
		Updates(map[string]any{"payment_status": order.PaymentPaid, "status": order.StatusShipped, "total_amount": 7.5, "currency": "USD"}).Error; err != nil {
		t.Fatal(err)
	}
	createFlowTestOrder(t, svc, "SO-ST-UNPAID")

	c := importTestCtx(1)
	res, err := svc.SalesStats(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Windows) != 3 {
		t.Fatalf("expected 3 windows, got %d", len(res.Windows))
	}
	for _, w := range res.Windows {
		if w.OrderCount != 3 || w.PaidCount != 2 || w.ShippedCount != 1 {
			t.Fatalf("window %s expected 3/2/1, got %d/%d/%d", w.Key, w.OrderCount, w.PaidCount, w.ShippedCount)
		}
		if len(w.PaidAmounts) != 1 || w.PaidAmounts[0].Currency != "USD" || w.PaidAmounts[0].Amount != 20 || w.PaidAmounts[0].Orders != 2 {
			t.Fatalf("window %s unexpected paid amounts: %+v", w.Key, w.PaidAmounts)
		}
	}
}

// Non-admin principals must only see sales stats for their granted shops,
// matching the order-list and daily-stats store scope.
func TestSalesStatsStoreScope(t *testing.T) {
	db := openImportTestDB(t)
	if err := db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}); err != nil {
		t.Fatal(err)
	}
	svc := &order.Service{DB: db}
	shopA := uuid.New()
	shopB := uuid.New()
	granted := createFlowTestOrder(t, svc, "SO-ST-SCOPE-A")
	if err := db.Model(&order.Order{}).Where("id = ?", granted.ID).
		Updates(map[string]any{"shop_id": shopA, "payment_status": order.PaymentPaid, "total_amount": 20, "currency": "CNY"}).Error; err != nil {
		t.Fatal(err)
	}
	other := createFlowTestOrder(t, svc, "SO-ST-SCOPE-B")
	if err := db.Model(&order.Order{}).Where("id = ?", other.ID).
		Updates(map[string]any{"shop_id": shopB, "payment_status": order.PaymentPaid, "total_amount": 100, "currency": "CNY"}).Error; err != nil {
		t.Fatal(err)
	}

	uid := uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uid},
		Username:     "op-" + uid.String()[:8],
		Email:        "op-" + uid.String()[:8] + "@example.com",
		PasswordHash: "x",
		Role:         "operator",
		Status:       "active",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&admin.UserStorePermission{
		ID:              uuid.New(),
		UserID:          uid,
		StoreID:         shopA,
		PermissionScope: "operate",
	}).Error; err != nil {
		t.Fatal(err)
	}

	c := importTestCtx(1)
	c.Set(ctxkey.AdminID, uid.String())
	res, err := svc.SalesStats(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range res.Windows {
		if w.OrderCount != 1 || w.PaidCount != 1 {
			t.Fatalf("window %s expected 1/1 scoped orders, got %d/%d", w.Key, w.OrderCount, w.PaidCount)
		}
		if len(w.PaidAmounts) != 1 || w.PaidAmounts[0].Currency != "CNY" || w.PaidAmounts[0].Amount != 20 || w.PaidAmounts[0].Orders != 1 {
			t.Fatalf("window %s unexpected paid amounts: %+v", w.Key, w.PaidAmounts)
		}
	}
}
