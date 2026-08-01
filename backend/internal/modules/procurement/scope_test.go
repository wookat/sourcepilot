package procurement

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

func int64Ptr(v int64) *int64 { return &v }

// setupScopedFixture generates one purchase order and returns it with the
// tenant/shop of the sales order it was generated from.
func setupScopedFixture(t *testing.T) (fixture, PurchaseOrder, uuid.UUID) {
	t.Helper()
	f := setupFixture(t)
	shopID := uuid.New()
	if err := f.svc.DB.Model(&order.Order{}).Where("id = ?", f.orderID).
		Updates(map[string]any{"tenant_id": int64(1), "shop_id": shopID}).Error; err != nil {
		t.Fatal(err)
	}
	po := generate(t, f, "scope-key").Orders[0]
	if err := f.svc.DB.Model(&PurchaseOrder{}).Where("id = ?", po.ID).
		Update("tenant_id", int64(1)).Error; err != nil {
		t.Fatal(err)
	}
	return f, po, shopID
}

func TestListAppliesTenantAndStoreScope(t *testing.T) {
	f, po, shopID := setupScopedFixture(t)
	ctx := context.Background()

	// same tenant, admin (nil shop list) sees the order
	res, err := f.svc.List(ctx, ListQuery{Scope: Scope{TenantID: int64Ptr(1)}})
	if err != nil || res.Total != 1 || res.Items[0].ID != po.ID {
		t.Fatalf("expected tenant match, got %v %+v", err, res)
	}
	// other tenant sees nothing
	res, err = f.svc.List(ctx, ListQuery{Scope: Scope{TenantID: int64Ptr(2)}})
	if err != nil || res.Total != 0 {
		t.Fatalf("expected cross-tenant empty, got %v %+v", err, res)
	}
	// granted shop sees the order
	res, err = f.svc.List(ctx, ListQuery{Scope: Scope{TenantID: int64Ptr(1), AllowedShopIDs: []uuid.UUID{shopID}}})
	if err != nil || res.Total != 1 {
		t.Fatalf("expected shop-granted match, got %v %+v", err, res)
	}
	// non-admin without grants sees nothing
	res, err = f.svc.List(ctx, ListQuery{Scope: Scope{TenantID: int64Ptr(1), AllowedShopIDs: []uuid.UUID{}}})
	if err != nil || res.Total != 0 {
		t.Fatalf("expected no-grant empty, got %v %+v", err, res)
	}
	// grant for another shop sees nothing
	res, err = f.svc.List(ctx, ListQuery{Scope: Scope{TenantID: int64Ptr(1), AllowedShopIDs: []uuid.UUID{uuid.New()}}})
	if err != nil || res.Total != 0 {
		t.Fatalf("expected foreign-shop empty, got %v %+v", err, res)
	}
}

func TestPOInScope(t *testing.T) {
	f, po, shopID := setupScopedFixture(t)
	ctx := context.Background()

	cases := []struct {
		name string
		sc   Scope
		want bool
	}{
		{"same tenant admin", Scope{TenantID: int64Ptr(1)}, true},
		{"cross tenant", Scope{TenantID: int64Ptr(2)}, false},
		{"granted shop", Scope{TenantID: int64Ptr(1), AllowedShopIDs: []uuid.UUID{shopID}}, true},
		{"no grants", Scope{TenantID: int64Ptr(1), AllowedShopIDs: []uuid.UUID{}}, false},
		{"foreign shop", Scope{TenantID: int64Ptr(1), AllowedShopIDs: []uuid.UUID{uuid.New()}}, false},
	}
	for _, tc := range cases {
		got, err := f.svc.POInScope(ctx, po.ID, tc.sc)
		if err != nil || got != tc.want {
			t.Fatalf("%s: expected %v, got %v (%v)", tc.name, tc.want, got, err)
		}
	}
	if got, err := f.svc.POInScope(ctx, uuid.New(), Scope{TenantID: int64Ptr(1)}); err != nil || got {
		t.Fatalf("missing po must be out of scope, got %v (%v)", got, err)
	}
}

func TestSalesOrderInScope(t *testing.T) {
	f, _, shopID := setupScopedFixture(t)
	ctx := context.Background()

	cases := []struct {
		name string
		sc   Scope
		want bool
	}{
		{"same tenant admin", Scope{TenantID: int64Ptr(1)}, true},
		{"cross tenant", Scope{TenantID: int64Ptr(2)}, false},
		{"granted shop", Scope{TenantID: int64Ptr(1), AllowedShopIDs: []uuid.UUID{shopID}}, true},
		{"no grants", Scope{TenantID: int64Ptr(1), AllowedShopIDs: []uuid.UUID{}}, false},
	}
	for _, tc := range cases {
		got, err := f.svc.SalesOrderInScope(ctx, f.orderID, tc.sc)
		if err != nil || got != tc.want {
			t.Fatalf("%s: expected %v, got %v (%v)", tc.name, tc.want, got, err)
		}
	}
}

func TestBatchMarkPlacedSkipsOutOfScopePO(t *testing.T) {
	f, po, _ := setupScopedFixture(t)
	advanceToPlacing(t, f, po)
	ctx := context.Background()

	res, err := f.svc.BatchMarkPlaced(ctx, BatchMarkPlacedBody{Items: []BatchPlacedItem{
		{PurchaseOrderID: po.ID.String(), ExternalOrderID: "1688-SC-1"},
	}}, Scope{TenantID: int64Ptr(2)}, nil)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if res.Succeeded != 0 || res.Failed != 1 || res.Results[0].Message != "采购单不存在" {
		t.Fatalf("expected out-of-scope line to fail as not found, got %+v", res)
	}
	got, err := f.svc.Detail(ctx, po.ID)
	if err != nil || got.ExternalOrderID != "" {
		t.Fatalf("cross-tenant batch must not mutate, got %v %+v", err, got)
	}
}

func TestBatchFillLogisticsSkipsOutOfScopePO(t *testing.T) {
	f, po, _ := setupScopedFixture(t)
	advanceToPlacing(t, f, po)
	ctx := context.Background()
	if _, err := f.svc.MarkPlaced(ctx, po.ID, MarkPlacedBody{ExternalOrderID: "1688-SC-2"}, nil); err != nil {
		t.Fatalf("mark placed: %v", err)
	}

	res, err := f.svc.BatchFillLogistics(ctx, BatchLogisticsBody{Items: []BatchLogisticsItem{
		{ExternalOrderID: "1688-SC-2", TrackingNo: "SF-1"},
	}}, Scope{TenantID: int64Ptr(2)}, nil)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if res.Succeeded != 0 || res.Failed != 1 {
		t.Fatalf("expected out-of-scope match to fail, got %+v", res)
	}
}
