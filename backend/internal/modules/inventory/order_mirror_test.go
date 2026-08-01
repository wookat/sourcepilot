package inventory

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// orderLineMirror reads the order module's `order_items` table by column name;
// GORM's default naming turns ProductSKUID into `product_sk_uid`, so the
// column tag must pin `product_sku_id` or the field silently stays nil and
// every deduction is skipped with missing_product_sku_id.
func TestOrderLineMirrorReadsProductSKUIDColumn(t *testing.T) {
	dsn := fmt.Sprintf("file:mirror_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	// Schema as created by the order module (real column name).
	if err := db.Exec(`CREATE TABLE order_items (
		id TEXT PRIMARY KEY,
		created_at DATETIME,
		updated_at DATETIME,
		order_id TEXT NOT NULL,
		product_id TEXT,
		product_sku_id TEXT,
		external_item_id TEXT,
		quantity INTEGER NOT NULL
	)`).Error; err != nil {
		t.Fatal(err)
	}
	orderID := uuid.New()
	skuID := uuid.New()
	if err := db.Exec(
		`INSERT INTO order_items (id, order_id, product_sku_id, quantity) VALUES (?, ?, ?, 2)`,
		uuid.NewString(), orderID.String(), skuID.String(),
	).Error; err != nil {
		t.Fatal(err)
	}

	svc := &Service{DB: db}
	items, err := svc.loadOrderItems(context.Background(), orderID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ProductSKUID == nil || *items[0].ProductSKUID != skuID {
		t.Fatalf("ProductSKUID = %v, want %s", items[0].ProductSKUID, skuID)
	}
}
