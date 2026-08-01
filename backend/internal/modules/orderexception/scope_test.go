package orderexception

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"gorm.io/gorm"
)

// seedScopedBlockedOrders seeds two procurement-blocked orders bound to
// different tenants and shops so scope filters can be asserted.
func seedScopedBlockedOrders(t *testing.T, db *gorm.DB) (fxA, fxB procBlockedFixture, shopA, shopB uuid.UUID) {
	t.Helper()
	fxA = seedPaidOrderWithSKU(t, db)
	fxB = seedPaidOrderWithSKU(t, db)
	shopA = uuid.New()
	shopB = uuid.New()
	if err := db.Model(&order.Order{}).Where("id = ?", fxA.orderID).
		Updates(map[string]any{"tenant_id": 1, "shop_id": shopA}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&order.Order{}).Where("id = ?", fxB.orderID).
		Updates(map[string]any{"tenant_id": 2, "shop_id": shopB}).Error; err != nil {
		t.Fatal(err)
	}
	return fxA, fxB, shopA, shopB
}

func listBlockedScoped(t *testing.T, svc *Service, tenantID *int64, shops []uuid.UUID) *ListOrderExceptionsResult {
	t.Helper()
	res, err := svc.ListOrderExceptions(context.Background(), ListOrderExceptionsRequest{
		ExceptionType:  TypeProcurementBlocked,
		Page:           1,
		PageSize:       50,
		TenantID:       tenantID,
		AllowedShopIDs: shops,
	})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestListOrderExceptionsTenantScope(t *testing.T) {
	db := openProcBlockedTestDB(t)
	svc := &Service{DB: db}
	fxA, fxB, _, _ := seedScopedBlockedOrders(t, db)

	all := listBlockedScoped(t, svc, nil, nil)
	if len(all.List) != 2 {
		t.Fatalf("unrestricted: expected 2 rows, got %d", len(all.List))
	}

	t1 := int64(1)
	res := listBlockedScoped(t, svc, &t1, nil)
	if len(res.List) != 1 || res.List[0].OrderID != fxA.orderID.String() {
		t.Fatalf("tenant 1: expected only order A, got %+v", res.List)
	}
	if res.Summary.TotalOpen != 1 {
		t.Fatalf("tenant 1 summary: expected totalOpen 1, got %d", res.Summary.TotalOpen)
	}

	t2 := int64(2)
	res = listBlockedScoped(t, svc, &t2, nil)
	if len(res.List) != 1 || res.List[0].OrderID != fxB.orderID.String() {
		t.Fatalf("tenant 2: expected only order B, got %+v", res.List)
	}
}

func TestListOrderExceptionsStoreScope(t *testing.T) {
	db := openProcBlockedTestDB(t)
	svc := &Service{DB: db}
	fxA, _, shopA, _ := seedScopedBlockedOrders(t, db)

	res := listBlockedScoped(t, svc, nil, []uuid.UUID{shopA})
	if len(res.List) != 1 || res.List[0].OrderID != fxA.orderID.String() {
		t.Fatalf("shop A scope: expected only order A, got %+v", res.List)
	}
	if res.Summary.TotalOpen != 1 {
		t.Fatalf("shop A summary: expected totalOpen 1, got %d", res.Summary.TotalOpen)
	}

	res = listBlockedScoped(t, svc, nil, []uuid.UUID{})
	if len(res.List) != 0 || res.Summary.TotalOpen != 0 {
		t.Fatalf("no-store scope: expected empty result, got %d rows / totalOpen %d", len(res.List), res.Summary.TotalOpen)
	}
}

func TestGetOrderExceptionDetailScope(t *testing.T) {
	db := openProcBlockedTestDB(t)
	svc := &Service{DB: db}
	fxA, fxB, shopA, _ := seedScopedBlockedOrders(t, db)

	scope := ListOrderExceptionsRequest{AllowedShopIDs: []uuid.UUID{shopA}}
	if _, err := svc.GetOrderExceptionDetail(context.Background(), SourceOrderItem, fxA.itemID.String(), scope); err != nil {
		t.Fatalf("in-scope detail should load: %v", err)
	}
	if _, err := svc.GetOrderExceptionDetail(context.Background(), SourceOrderItem, fxB.itemID.String(), scope); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("out-of-scope shop detail: expected ErrRecordNotFound, got %v", err)
	}

	t1 := int64(1)
	tenantScope := ListOrderExceptionsRequest{TenantID: &t1}
	if _, err := svc.GetOrderExceptionDetail(context.Background(), SourceOrderItem, fxB.itemID.String(), tenantScope); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant detail: expected ErrRecordNotFound, got %v", err)
	}
}
