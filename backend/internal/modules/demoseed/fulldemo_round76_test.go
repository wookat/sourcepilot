package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

// Round 76: customer-service conversation samples must span both the admin
// douyin demo shop and the operator/readonly-granted manual DEMO shop, so the
// customer-service page is non-empty for scoped roles out of the box.
func TestFullDemoCustomerConversationsSpanManualShop(t *testing.T) {
	db := openFullDemoTestDB(t)
	if db == nil {
		return
	}
	s := &FullDemoSeeder{DB: db, TenantID: 7, AppEnv: "development"}
	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	shopConvCount := func(shopCode string) int64 {
		t.Helper()
		var sh shop.Shop
		if err := db.Where("shop_code = ?", shopCode).First(&sh).Error; err != nil {
			t.Fatalf("shop %s: %v", shopCode, err)
		}
		var n int64
		if err := db.Model(&customerchat.CustomerConversation{}).
			Where("customer_name LIKE ? AND shop_id = ?", "DEMO-%", sh.ID).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := shopConvCount("DEMO-SHOP-1"); n == 0 {
		t.Error("expected conversations on the admin douyin demo shop")
	}
	if n := shopConvCount("DEMO-SHOP-2"); n == 0 {
		t.Error("expected conversations on the manual DEMO shop")
	}

	// clean must leave zero residue including manual-shop conversations
	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatalf("clean: %v", err)
	}
	var n int64
	if err := db.Model(&customerchat.CustomerConversation{}).
		Where("customer_name LIKE ?", "DEMO-%").Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("residual demo conversations after clean: %d", n)
	}
}
