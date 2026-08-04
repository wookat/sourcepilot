package order_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// Order numbers are unique per tenant only. A globally unique index turned a
// duplicate-key error into an existence oracle for other tenants' order
// numbers (and let a tenant squat them), so the same number must be storable
// in two tenants while staying rejected inside one tenant.
func TestOrderNoUniquePerTenantNotGlobally(t *testing.T) {
	db := openImportTestDB(t)

	a := order.Order{
		Base: model.Base{ID: uuid.New()}, TenantID: 1, OrderNo: "SHARED-0001",
		Platform: "douyin", Status: "paid", Currency: "CNY", TotalAmount: 10, CustomerName: "A",
	}
	if err := db.Create(&a).Error; err != nil {
		t.Fatalf("tenant 1 order: %v", err)
	}
	b := order.Order{
		Base: model.Base{ID: uuid.New()}, TenantID: 2, OrderNo: "SHARED-0001",
		Platform: "douyin", Status: "paid", Currency: "CNY", TotalAmount: 10, CustomerName: "B",
	}
	if err := db.Create(&b).Error; err != nil {
		t.Fatalf("tenant 2 must be able to reuse another tenant's order number: %v", err)
	}
	dup := order.Order{
		Base: model.Base{ID: uuid.New()}, TenantID: 2, OrderNo: "SHARED-0001",
		Platform: "douyin", Status: "paid", Currency: "CNY", TotalAmount: 10, CustomerName: "B2",
	}
	if err := db.Create(&dup).Error; err == nil {
		t.Fatal("order number must stay unique inside a single tenant")
	}
}

// Manual import duplicate detection is tenant-scoped: another tenant's order
// number must not be reported as an existing duplicate.
func TestManualImportDuplicateIsTenantScoped(t *testing.T) {
	db := openImportTestDB(t)
	svc := &order.Service{DB: db}

	if err := db.Create(&order.Order{
		Base: model.Base{ID: uuid.New()}, TenantID: 1, OrderNo: "IMP-0001",
		Platform: "douyin", Status: "paid", Currency: "CNY", TotalAmount: 10, CustomerName: "A",
	}).Error; err != nil {
		t.Fatal(err)
	}

	sum, err := svc.ImportOrders(importTestCtx(2), order.ImportBody{Orders: []order.CreateBody{importOrderBody("IMP-0001")}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Duplicate != 0 || sum.Created != 1 {
		t.Fatalf("tenant 2 import of another tenant's order number: created=%d duplicate=%d", sum.Created, sum.Duplicate)
	}
	sum, err = svc.ImportOrders(importTestCtx(2), order.ImportBody{Orders: []order.CreateBody{importOrderBody("IMP-0001")}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Duplicate != 1 {
		t.Fatalf("same-tenant duplicate must be skipped: %+v", sum)
	}
}
