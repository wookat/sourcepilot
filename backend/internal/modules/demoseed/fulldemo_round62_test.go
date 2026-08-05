package demoseed

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/bannedwords"
	"github.com/trademind-ai/trademind/backend/internal/modules/carrier"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/migrationimport"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/selection"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
	"github.com/trademind-ai/trademind/backend/internal/modules/waybill"
	"gorm.io/gorm"
)

func openFullDemoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:fulldemo_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&carrier.Carrier{},
		&waybill.Template{},
		&waybill.ShippingRule{},
		&shop.Shop{},
		&product.Product{},
		&product.ProductImage{},
		&product.ProductSKU{},
		&sourcing.Supplier{},
		&sourcing.ProductSource{},
		&sourcing.ProductSourceSKU{},
		&sourcing.SourcePriceHistory{},
		&sourcing.SourceSwitchEvent{},
		&productpublish.ProductPublishBatch{},
		&productpublish.ProductPublishTask{},
		&productpublish.ProductPublication{},
		&productpublish.ProductPublicationSKU{},
		&settings.Setting{},
		&order.Order{},
		&order.OrderItem{},
		&order.OrderItemSKUMatch{},
		&order.OrderShipment{},
		&order.OrderReviewRule{},
		&order.OrderReviewHit{},
		&procurement.PurchaseOrder{},
		&procurement.PurchaseOrderItem{},
		&procurement.PurchaseOrderEvent{},
		&procurement.PurchaseLogistics{},
		&inventory.InventoryChangeLog{},
		&inventory.InventorySyncBatch{},
		&inventory.InventorySyncTask{},
		&inventory.Warehouse{},
		&inventory.WarehouseStock{},
		&ordersync.OrderSyncTask{},
		&orderexception.OrderExceptionMark{},
		&admin.AdminUser{},
		&admin.UserStorePermission{},
		&customerchat.CustomerConversation{},
		&customerchat.CustomerMessage{},
		&customerchat.CustomerReplySuggestion{},
		&customerchat.CustomerFailureEvent{},
		&customerchat.CustomerReplyTemplate{},
		&customerchat.BuyerMessageRule{},
		&customerchat.BuyerMessageDraft{},
		&customersync.CustomerMessageSyncTask{},
		&selection.SelectionTask{},
		&selection.SelectionCandidate{},
		&selection.SelectionSourceMatch{},
		&selection.SelectionEvaluation{},
		&migrationimport.ImportJob{},
		&migrationimport.ImportJobRow{},
		&bannedwords.BannedWord{},
		&bannedwords.BannedWordCategoryState{},
	); err != nil {
		t.Fatal(err)
	}
	if err := operationtask.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

// Seed grants the tenant's operator (with no existing store grants) one DEMO
// shop, and cleanup removes DEMO shop grants without touching real grants.
func TestFullDemoSeedOperatorStoreGrant(t *testing.T) {
	db := openFullDemoTestDB(t)
	if db == nil {
		return
	}
	op := admin.AdminUser{TenantID: 7, Username: admin.NewInternalUsername(),
		Email: "demo_operator@trademind.local", PasswordHash: "x", Role: "operator"}
	if err := db.Create(&op).Error; err != nil {
		t.Fatal(err)
	}
	scoped := admin.AdminUser{TenantID: 7, Username: admin.NewInternalUsername(),
		Email: "scoped_operator@trademind.local", PasswordHash: "x", Role: "operator"}
	if err := db.Create(&scoped).Error; err != nil {
		t.Fatal(err)
	}
	realShop := shop.Shop{TenantID: 7, Platform: "manual", ShopName: "真实店铺", ShopCode: "REAL-1", Status: "active"}
	if err := db.Create(&realShop).Error; err != nil {
		t.Fatal(err)
	}
	realGrant := admin.UserStorePermission{UserID: scoped.ID, StoreID: realShop.ID, PermissionScope: admin.StorePermScopeView}
	if err := db.Create(&realGrant).Error; err != nil {
		t.Fatal(err)
	}

	s := &FullDemoSeeder{DB: db, TenantID: 7, AppEnv: "development"}
	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var grants []admin.UserStorePermission
	if err := db.Where("user_id = ?", op.ID).Find(&grants).Error; err != nil {
		t.Fatal(err)
	}
	if len(grants) != 1 {
		t.Fatalf("expected 1 demo grant for ungranted operator, got %d", len(grants))
	}
	if grants[0].PermissionScope != admin.StorePermScopeOperate {
		t.Fatalf("expected operate scope, got %s", grants[0].PermissionScope)
	}
	var demoShop shop.Shop
	if err := db.First(&demoShop, "id = ?", grants[0].StoreID).Error; err != nil {
		t.Fatal(err)
	}
	if demoShop.ShopCode != "DEMO-SHOP-2" {
		t.Fatalf("expected grant on DEMO-SHOP-2 (manual shop; douyin stays denied sample), got %s", demoShop.ShopCode)
	}

	var n int64
	if err := db.Model(&admin.UserStorePermission{}).Where("user_id = ?", scoped.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("operator with existing grants must not be touched, got %d grants", n)
	}

	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if err := db.Model(&admin.UserStorePermission{}).Where("user_id = ?", op.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("demo shop grants must be cleaned, got %d", n)
	}
	if err := db.Model(&admin.UserStorePermission{}).Where("user_id = ?", scoped.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("real store grant must survive cleanup, got %d", n)
	}
}

// Cleanup removes tenant-0 orphan demo customer conversations (with children)
// while keeping real-tenant and non-demo conversations.
func TestFullDemoCleanupTenantZeroDemoConversations(t *testing.T) {
	db := openFullDemoTestDB(t)
	if db == nil {
		return
	}
	mkConv := func(tenantID int64, name string) customerchat.CustomerConversation {
		c := customerchat.CustomerConversation{TenantID: tenantID, Platform: "manual",
			CustomerName: name, CustomerLanguage: "zh-CN", Status: "open"}
		if err := db.Create(&c).Error; err != nil {
			t.Fatal(err)
		}
		return c
	}
	orphan := mkConv(0, "F8 Demo Send Failed Buyer")
	orphan2 := mkConv(0, "Demo Buyer Pending")
	realTenant := mkConv(7, "Demo Buyer Pending")
	realBuyer := mkConv(0, "普通买家")

	msg := customerchat.CustomerMessage{ConversationID: orphan.ID, Role: "customer",
		Content: "hi", Language: "zh-CN", Source: "manual"}
	if err := db.Create(&msg).Error; err != nil {
		t.Fatal(err)
	}
	ev := customerchat.CustomerFailureEvent{ConversationID: orphan.ID,
		Category: customerchat.FailureCategoryReplySendFailed, Status: customerchat.FailureEventStatusOpen}
	if err := db.Create(&ev).Error; err != nil {
		t.Fatal(err)
	}

	s := &FullDemoSeeder{DB: db, TenantID: 7, AppEnv: "development"}
	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	assertCount := func(model any, where string, args []any, want int64, label string) {
		t.Helper()
		var n int64
		if err := db.Model(model).Where(where, args...).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n != want {
			t.Fatalf("%s: expected %d rows, got %d", label, want, n)
		}
	}
	assertCount(&customerchat.CustomerConversation{}, "id IN ?", []any{[]uuid.UUID{orphan.ID, orphan2.ID}}, 0, "tenant-0 demo conversations")
	assertCount(&customerchat.CustomerMessage{}, "conversation_id = ?", []any{orphan.ID}, 0, "orphan messages")
	assertCount(&customerchat.CustomerFailureEvent{}, "conversation_id = ?", []any{orphan.ID}, 0, "orphan failure events")
	assertCount(&customerchat.CustomerConversation{}, "id = ?", []any{realTenant.ID}, 1, "real tenant conversation")
	assertCount(&customerchat.CustomerConversation{}, "id = ?", []any{realBuyer.ID}, 1, "non-demo tenant-0 conversation")
}

// Seed must persist the disabled reply-template sample as disabled（回归：
// GORM bool default tag 曾把 Enabled:false 零值吞成 true）, and cleanup must
// remove all DEMO- templates.
func TestFullDemoSeedReplyTemplates(t *testing.T) {
	db := openFullDemoTestDB(t)
	if db == nil {
		return
	}

	s := &FullDemoSeeder{DB: db, TenantID: 7, AppEnv: "development"}
	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var total, disabled int64
	if err := db.Model(&customerchat.CustomerReplyTemplate{}).
		Where("tenant_id = ? AND name LIKE ?", 7, DemoPrefix+"%").Count(&total).Error; err != nil {
		t.Fatal(err)
	}
	if total != int64(len(demoReplyTemplatePlans())) {
		t.Fatalf("seeded templates: got %d, want %d", total, len(demoReplyTemplatePlans()))
	}
	if err := db.Model(&customerchat.CustomerReplyTemplate{}).
		Where("tenant_id = ? AND name LIKE ? AND enabled = ?", 7, DemoPrefix+"%", false).
		Count(&disabled).Error; err != nil {
		t.Fatal(err)
	}
	if disabled != 1 {
		t.Fatalf("disabled sample template: got %d rows, want 1", disabled)
	}

	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	var left int64
	if err := db.Unscoped().Model(&customerchat.CustomerReplyTemplate{}).
		Where("name LIKE ?", DemoPrefix+"%").Count(&left).Error; err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatalf("cleanup residual templates: %d", left)
	}
}
