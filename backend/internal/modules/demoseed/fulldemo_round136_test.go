package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/finance"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

// Round 136: reconciliation volume top-up (25+ paid orders so the 20-per-page
// workbench paginates) plus the operator-scope automation positive sample
// DEMO-AT-1005, all cleaned with zero residue and idempotent.
func TestFullDemoSeedRound136FinanceVolumeAndOperatorSample(t *testing.T) {
	db := openFullDemoTestDB(t)
	s := &FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development"}

	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}

	// 对账工作台行 = 已付款订单：必须补足到 25+（默认页大小 20 → 出现第二页）。
	var paid int64
	if err := db.Model(&order.Order{}).
		Where("tenant_id = ? AND order_no LIKE ? AND payment_status = ?",
			s.TenantID, "DEMO-%", order.PaymentPaid).Count(&paid).Error; err != nil {
		t.Fatal(err)
	}
	if paid < 25 {
		t.Fatalf("expected at least 25 paid DEMO orders for reconciliation paging, got %d", paid)
	}
	if paid < round136FinanceOrderTarget {
		t.Fatalf("expected top-up to reach target %d, got %d", round136FinanceOrderTarget, paid)
	}

	// 量样本必须覆盖结清/少款/多款/未回款四类，合计区各计数非零可演示。
	for _, label := range []string{"已结清样本", "少款样本", "多款样本"} {
		var n int64
		db.Model(&finance.PaymentRecord{}).Where("remark LIKE ?", "%"+label+"%").Count(&n)
		if n == 0 {
			t.Fatalf("expected round136 payment sample %q", label)
		}
	}
	var unpaidVolume int64
	db.Model(&order.Order{}).
		Where("order_no LIKE ? AND id NOT IN (?)", "DEMO-FIN-2%",
			db.Model(&finance.PaymentRecord{}).Select("order_id")).Count(&unpaidVolume)
	if unpaidVolume == 0 {
		t.Fatal("expected at least one round136 order without payment (未回款 sample)")
	}

	// operator 正样本 DEMO-AT-1005：落在授权手工店、未付款、审单通过、SKU 已匹配。
	var op order.Order
	if err := db.First(&op, "order_no = ?", "DEMO-AT-1005").Error; err != nil {
		t.Fatalf("expected DEMO-AT-1005 operator sample order: %v", err)
	}
	if op.PaymentStatus != order.PaymentUnpaid || op.ReviewStatus != order.ReviewStatusAutoPassed {
		t.Fatalf("unexpected DEMO-AT-1005 state: %+v", op)
	}
	var manualShop shop.Shop
	if err := db.First(&manualShop, "shop_code = ?", "DEMO-SHOP-2").Error; err != nil {
		t.Fatal(err)
	}
	if op.ShopID == nil || *op.ShopID != manualShop.ID {
		t.Fatalf("DEMO-AT-1005 must live on the operator-granted manual shop")
	}
	var operatorUser admin.AdminUser
	if err := db.First(&operatorUser, "tenant_id = ? AND lower(email) = ?",
		s.TenantID, "demo_operator@trademind.local").Error; err != nil {
		t.Fatal(err)
	}
	var grants int64
	db.Model(&admin.UserStorePermission{}).
		Where("user_id = ? AND store_id = ?", operatorUser.ID, manualShop.ID).Count(&grants)
	if grants == 0 {
		t.Fatal("demo operator must hold a store grant on the manual shop for DEMO-AT-1005")
	}
	var matchRows int64
	db.Model(&order.OrderItemSKUMatch{}).
		Where("order_id = ? AND match_status = ?", op.ID, order.MatchStatusMatched).Count(&matchRows)
	if matchRows == 0 {
		t.Fatal("DEMO-AT-1005 must carry matched order_item_sku_matches rows")
	}

	// 幂等：重跑 seed 不产生重复订单。
	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}
	var paidAgain int64
	db.Model(&order.Order{}).Where("tenant_id = ? AND order_no LIKE ? AND payment_status = ?",
		s.TenantID, "DEMO-%", order.PaymentPaid).Count(&paidAgain)
	if paidAgain != paid {
		t.Fatalf("seed not idempotent: paid orders %d -> %d", paid, paidAgain)
	}

	// clean / verify 零残留。
	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	res, err := s.VerifyClean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for table, n := range res.Counts {
		if n > 0 {
			t.Fatalf("residual rows after cleanup in %s: %d", table, n)
		}
	}
}
