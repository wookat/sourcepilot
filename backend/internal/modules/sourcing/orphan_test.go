package sourcing

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/providers/sourceinfo"
	"gorm.io/gorm"
)

func newOrphanTestService(t *testing.T) *Service {
	t.Helper()
	dsn := fmt.Sprintf("file:sourcing_orphan_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(
		&Supplier{}, &ProductSource{}, &ProductSourceSKU{},
		&SourcePriceHistory{}, &SourceSwitchEvent{},
		&product.Product{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return &Service{DB: db, Provider: &sourceinfo.Mock{}}
}

func TestOrphanSourceDetectionAndUnbind(t *testing.T) {
	svc := newOrphanTestService(t)
	ctx := context.Background()

	liveProduct := product.Product{Title: "live product", Status: "draft"}
	deadProduct := product.Product{Title: "dead product", Status: "draft"}
	if err := svc.DB.Create(&liveProduct).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.DB.Create(&deadProduct).Error; err != nil {
		t.Fatal(err)
	}

	liveSrc := mustBind(t, svc, liveProduct.ID, "supplier-live", "111", 10)
	deadSrc := mustBind(t, svc, deadProduct.ID, "supplier-dead", "222", 10)
	price := 9.9
	if err := svc.DB.Create(&ProductSourceSKU{
		ProductSourceID: deadSrc.ID, LocalSKUID: uuid.New(), ExternalSKUID: "ext-1",
		Currency: "CNY", Status: "active", CurrentPrice: &price,
	}).Error; err != nil {
		t.Fatal(err)
	}

	// no orphans while both products live
	rows, err := svc.ListOrphanSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected no orphan yet, got %+v", rows)
	}

	// non-orphan source cannot be unbound
	if err := svc.DeleteSource(ctx, liveSrc.ID, nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict for live product source, got %v", err)
	}

	// soft-delete one product → its source becomes an orphan
	if err := svc.DB.Delete(&product.Product{}, "id = ?", deadProduct.ID).Error; err != nil {
		t.Fatal(err)
	}
	rows, err = svc.ListOrphanSources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].SourceID != deadSrc.ID {
		t.Fatalf("expected 1 orphan (dead source), got %+v", rows)
	}
	if rows[0].SupplierName != "supplier-dead" || rows[0].SKUCount != 1 {
		t.Fatalf("unexpected orphan row %+v", rows[0])
	}

	// supplier deletion blocked while the orphan source exists
	if err := svc.DeleteSupplier(ctx, deadSrc.SupplierID, nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected supplier delete conflict, got %v", err)
	}

	// unbind the orphan → source + mappings soft-deleted
	if err := svc.DeleteSource(ctx, deadSrc.ID, nil); err != nil {
		t.Fatalf("unbind orphan: %v", err)
	}
	var srcCnt, mapCnt int64
	if err := svc.DB.Model(&ProductSource{}).Where("id = ?", deadSrc.ID).Count(&srcCnt).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.DB.Model(&ProductSourceSKU{}).Where("product_source_id = ?", deadSrc.ID).Count(&mapCnt).Error; err != nil {
		t.Fatal(err)
	}
	if srcCnt != 0 || mapCnt != 0 {
		t.Fatalf("expected source and mappings soft-deleted, got src=%d map=%d", srcCnt, mapCnt)
	}
	// audit rows survive soft delete
	var keptSrc int64
	if err := svc.DB.Unscoped().Model(&ProductSource{}).Where("id = ?", deadSrc.ID).Count(&keptSrc).Error; err != nil {
		t.Fatal(err)
	}
	if keptSrc != 1 {
		t.Fatalf("expected soft-deleted source row kept, got %d", keptSrc)
	}

	// supplier is now deletable
	if err := svc.DeleteSupplier(ctx, deadSrc.SupplierID, nil); err != nil {
		t.Fatalf("supplier delete after unbind: %v", err)
	}

	// unknown source id → not found
	if err := svc.DeleteSource(ctx, uuid.New(), nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}
