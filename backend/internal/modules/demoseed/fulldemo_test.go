package demoseed

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
	"gorm.io/gorm"
)

func TestDemoPurchaseOrderPlansFollowStateMachine(t *testing.T) {
	plans := demoPurchaseOrderPlans()
	if len(plans) == 0 {
		t.Fatal("no purchase order plans")
	}
	for _, p := range plans {
		if err := validatePurchaseChain(p); err != nil {
			t.Errorf("plan %s: %v", p.suffix, err)
		}
	}
}

func TestDemoSalesOrderPlansFollowLifecycle(t *testing.T) {
	plans := demoSalesOrderPlans()
	if len(plans) != 5 {
		t.Fatalf("expected 5 sales order plans (pending/paid/shipped/delivered/cancelled), got %d", len(plans))
	}
	seen := map[string]bool{}
	for _, p := range plans {
		if err := validateSalesChain(p); err != nil {
			t.Errorf("plan %s: %v", p.suffix, err)
		}
		seen[p.status] = true
	}
	for _, st := range []string{"pending", "paid", "shipped", "delivered", "cancelled"} {
		if !seen[st] {
			t.Errorf("missing sales order status %s", st)
		}
	}
}

func TestDemoPlansUseDemoPrefix(t *testing.T) {
	for _, p := range demoPurchaseOrderPlans() {
		if p.externalID != "" && !strings.HasPrefix(p.externalID, DemoPrefix) {
			t.Errorf("plan %s external id %q missing %s prefix", p.suffix, p.externalID, DemoPrefix)
		}
		if p.trackingNo != "" && !strings.HasPrefix(p.trackingNo, DemoPrefix) {
			t.Errorf("plan %s tracking no %q missing %s prefix", p.suffix, p.trackingNo, DemoPrefix)
		}
	}
}

func TestCollectDemoPurchaseOrderIDs(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
		return
	}
	if err := db.AutoMigrate(
		&order.Order{},
		&sourcing.Supplier{},
		&procurement.PurchaseOrder{},
		&procurement.PurchaseOrderItem{},
	); err != nil {
		t.Fatal(err)
	}

	demoSupplier := sourcing.Supplier{Name: DemoPrefix + "供应商A"}
	realSupplier := sourcing.Supplier{Name: "真实供应商"}
	if err := db.Create(&demoSupplier).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&realSupplier).Error; err != nil {
		t.Fatal(err)
	}
	demoOrder := order.Order{Platform: "manual", OrderNo: DemoPrefix + "SO-1", Status: "paid"}
	if err := db.Create(&demoOrder).Error; err != nil {
		t.Fatal(err)
	}

	mkPO := func(supplierID uuid.UUID, idem, external, supplierName string) procurement.PurchaseOrder {
		po := procurement.PurchaseOrder{
			SupplierID:     supplierID,
			SupplierName:   supplierName,
			Status:         procurement.StatusDraft,
			IdempotencyKey: idem,
		}
		po.ExternalOrderID = external
		if err := db.Create(&po).Error; err != nil {
			t.Fatal(err)
		}
		return po
	}

	seeded := mkPO(demoSupplier.ID, DemoPrefix+"po-seeded", "", demoSupplier.Name)
	uiBySupplier := mkPO(demoSupplier.ID, "ui-created-1", "", demoSupplier.Name)
	uiByOrder := mkPO(realSupplier.ID, "ui-created-2", "", realSupplier.Name)
	real := mkPO(realSupplier.ID, "real-po", "", realSupplier.Name)

	item := procurement.PurchaseOrderItem{
		PurchaseOrderID: uiByOrder.ID,
		SalesOrderID:    &demoOrder.ID,
		LocalSKUID:      uuid.New(),
		SourceSKUID:     uuid.New(),
		Quantity:        1,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}

	ids, err := collectDemoPurchaseOrderIDs(
		db, DemoPrefix+"%",
		[]uuid.UUID{demoOrder.ID},
		[]uuid.UUID{demoSupplier.ID},
	)
	if err != nil {
		t.Fatal(err)
	}
	got := map[uuid.UUID]bool{}
	for _, id := range ids {
		got[id] = true
	}
	for name, po := range map[string]procurement.PurchaseOrder{
		"seeded": seeded, "uiBySupplier": uiBySupplier, "uiByOrder": uiByOrder,
	} {
		if !got[po.ID] {
			t.Errorf("expected %s purchase order to be collected", name)
		}
	}
	if got[real.ID] {
		t.Error("real purchase order must not be collected")
	}
}

func TestFullDemoSeederGuards(t *testing.T) {
	var nilSeeder *FullDemoSeeder
	if _, err := nilSeeder.Seed(context.Background()); err == nil {
		t.Error("nil seeder should error")
	}
	s := &FullDemoSeeder{AppEnv: "production"}
	if _, err := s.Seed(context.Background()); err == nil {
		t.Error("production env should be refused")
	}
	if _, err := s.Cleanup(context.Background()); err == nil {
		t.Error("production cleanup should be refused")
	}
}
