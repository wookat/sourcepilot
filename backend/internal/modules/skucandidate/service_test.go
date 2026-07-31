package skucandidate_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/skucandidate"
	"gorm.io/gorm"
)

func openCandidateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:sku_candidate_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&order.Order{}, &order.OrderItem{}, &order.OrderItemSKUMatch{},
		&product.Product{}, &product.ProductSKU{},
		&productpublish.ProductPublication{}, &productpublish.ProductPublicationSKU{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

// Regression: product_skus is hard-delete (no deleted_at column); candidate
// queries must not reference skus.deleted_at or the whole suggestion fails
// with SQL error 42703 on PostgreSQL.
func TestSuggestForOrderItemPublicationChannel(t *testing.T) {
	db := openCandidateTestDB(t)

	p := product.Product{Title: "收纳盒 大号"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	sku := product.ProductSKU{ProductID: p.ID, SKUCode: "BOX-L-BLUE"}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatal(err)
	}
	shopID := uuid.New()
	pub := productpublish.ProductPublication{ProductID: p.ID, Platform: "shopee", ShopID: shopID}
	if err := db.Create(&pub).Error; err != nil {
		t.Fatal(err)
	}
	pps := productpublish.ProductPublicationSKU{PublicationID: pub.ID, ProductSKUID: &sku.ID, ExternalSKUID: "EXT-1", SKUCode: "BOX-L-BLUE"}
	if err := db.Create(&pps).Error; err != nil {
		t.Fatal(err)
	}

	o := order.Order{OrderNo: "R9-CAND-1", Platform: "shopee", ShopID: &shopID, Status: order.StatusPending}
	if err := db.Create(&o).Error; err != nil {
		t.Fatal(err)
	}
	ext := "EXT-1"
	it := order.OrderItem{OrderID: o.ID, ProductTitle: "收纳盒 大号 蓝色", SKUCode: "BOX-L-BLUE", ExternalSKUID: &ext, Quantity: 1}
	if err := db.Create(&it).Error; err != nil {
		t.Fatal(err)
	}

	svc := &skucandidate.Service{DB: db}
	got, err := svc.SuggestForOrderItem(context.Background(), it.ID, skucandidate.SuggestOpts{Limit: 10})
	if err != nil {
		t.Fatalf("SuggestForOrderItem: %v", err)
	}
	if len(got.List) == 0 {
		t.Fatal("expected at least one candidate")
	}
	found := false
	for _, c := range got.List {
		if c.ProductSKUID == sku.ID.String() {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected candidate for sku %s, got %+v", sku.ID, got.List)
	}
}
