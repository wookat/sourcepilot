package procurement

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
)

// Marking a purchase order delivered must add each line quantity to local
// SKU stock exactly once, with an append-only inventory change log.
func TestMarkDeliveredInboundsStock(t *testing.T) {
	f := setupFixture(t)
	db := f.svc.DB
	ctx := context.Background()

	// Attach a real local SKU (stock 5) to the fixture mapping.
	var mapping sourcing.ProductSourceSKU
	if err := db.First(&mapping).Error; err != nil {
		t.Fatal(err)
	}
	p := product.Product{Title: "inbound product"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	stock := 5
	sku := product.ProductSKU{ProductID: p.ID, SKUCode: "INB-1", Stock: &stock}
	sku.ID = mapping.LocalSKUID
	if err := db.Create(&sku).Error; err != nil {
		t.Fatal(err)
	}

	po := generate(t, f, "key-inbound").Orders[0]
	if _, err := f.svc.Submit(ctx, po.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Confirm(ctx, po.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.MarkPlaced(ctx, po.ID, MarkPlacedBody{ExternalOrderID: "1688-INB-1"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.MarkPaid(ctx, po.ID, MarkPaidBody{}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.FillLogistics(ctx, po.ID, LogisticsBody{TrackingNo: "SF-INB"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.MarkDelivered(ctx, po.ID, nil); err != nil {
		t.Fatal(err)
	}

	var after product.ProductSKU
	if err := db.First(&after, "id = ?", sku.ID).Error; err != nil {
		t.Fatal(err)
	}
	// Fixture order line quantity is 3.
	if after.Stock == nil || *after.Stock != 8 {
		t.Fatalf("stock = %v, want 8", after.Stock)
	}
	var logs []inventory.InventoryChangeLog
	if err := db.Where("product_sku_id = ? AND change_type = ?", sku.ID, inventory.ChangePurchaseInbound).Find(&logs).Error; err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].Delta != 3 || logs[0].BeforeStock != 5 || logs[0].AfterStock != 8 {
		t.Fatalf("unexpected change logs: %+v", logs)
	}

	// Re-running the inbound for the same purchase order must be a no-op.
	var poRow PurchaseOrder
	if err := db.First(&poRow, "id = ?", po.ID).Error; err != nil {
		t.Fatal(err)
	}
	lines, err := inboundStockTx(db, &poRow, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range lines {
		if !l.Skipped || l.Reason != "already_inbound" {
			t.Fatalf("expected already_inbound skip, got %+v", l)
		}
	}
	var after2 product.ProductSKU
	if err := db.First(&after2, "id = ?", sku.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after2.Stock == nil || *after2.Stock != 8 {
		t.Fatalf("stock changed on repeat inbound: %v", after2.Stock)
	}
}
