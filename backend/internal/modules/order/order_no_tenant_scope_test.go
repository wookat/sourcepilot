package order_test

import (
	"strings"
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

// Creating an order whose number already exists in the tenant must return a
// readable business message instead of the raw SQL unique-violation error.
func TestCreateDuplicateOrderNoReturnsReadableError(t *testing.T) {
	db := openImportTestDB(t)
	svc := &order.Service{DB: db}

	if _, err := svc.Create(importTestCtx(1), importOrderBody("DUP-0001"), nil); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create(importTestCtx(1), importOrderBody("DUP-0001"), nil)
	if err == nil {
		t.Fatal("duplicate order number in one tenant must be rejected")
	}
	msg := err.Error()
	if !strings.Contains(msg, "订单号「DUP-0001」已存在") {
		t.Fatalf("error must carry a readable business message, got: %v", err)
	}
	if strings.Contains(strings.ToLower(msg), "sqlstate") || strings.Contains(strings.ToLower(msg), "constraint") {
		t.Fatalf("raw SQL error must not leak to the caller: %v", err)
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
