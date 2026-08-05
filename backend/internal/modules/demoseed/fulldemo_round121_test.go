package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/finance"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
)

// Round 121: 财务对账样本（回款 / 费用 / 采购实付价）seed / cleanup / verify
// leave zero residue.
func TestFullDemoSeedFinanceSamples(t *testing.T) {
	db := openFullDemoTestDB(t)
	s := &FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development"}

	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}

	var payments []finance.PaymentRecord
	if err := db.Find(&payments).Error; err != nil {
		t.Fatal(err)
	}
	if len(payments) < 3 {
		t.Fatalf("expected >=3 demo payment records, got %d", len(payments))
	}
	for _, p := range payments {
		if p.TenantID != 1 {
			t.Fatalf("payment %s wrong tenant %d", p.ID, p.TenantID)
		}
		if p.Source != finance.SourceManual {
			t.Fatalf("payment %s expected manual source, got %s", p.ID, p.Source)
		}
	}

	var expenseCount int64
	db.Model(&finance.OrderExpense{}).Count(&expenseCount)
	if expenseCount < 2 {
		t.Fatalf("expected >=2 demo order expenses, got %d", expenseCount)
	}
	var shopExpenseCount int64
	db.Model(&finance.ShopMonthlyExpense{}).Count(&shopExpenseCount)
	if shopExpenseCount < 2 {
		t.Fatalf("expected >=2 demo shop monthly expenses, got %d", shopExpenseCount)
	}

	var actualPriced int64
	db.Model(&procurement.PurchaseOrderItem{}).
		Where("actual_price IS NOT NULL AND sales_order_id IS NOT NULL").Count(&actualPriced)
	if actualPriced < 1 {
		t.Fatalf("expected a purchase item with actual price bound to a sales order, got %d", actualPriced)
	}

	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	verify, err := s.VerifyClean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"finance_payment_records", "finance_order_expenses", "finance_shop_monthly_expenses"} {
		if verify.Counts[table] != 0 {
			t.Fatalf("residual %s rows: %d", table, verify.Counts[table])
		}
	}
}
