package finance_test

import (
	"strings"
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/finance"
	"github.com/trademind-ai/trademind/backend/internal/modules/reports"
)

// Cross-tenant payment / expense rows sharing an order ID must not leak into
// another tenant's reconciliation aggregates.
func TestReconciliationCrossTenantAggIsolation(t *testing.T) {
	db := openFinanceTestDB(t)
	svc := newFinanceService(db, &fakeSettings{groups: map[string]map[string]string{
		"report_currency": {"base_currency": "CNY", "rates": `{}`},
	}})
	shopID := seedShop(t, db, 1, "店铺A")
	o := seedPaidOrder(t, db, 1, &shopID, "SO-T1", "CNY", 100)
	now := time.Now()
	mustCreate(t, db, &finance.PaymentRecord{
		TenantID: 2, OrderID: o.ID, Amount: 100, Currency: "CNY", ReceivedAt: now, Source: "manual",
	})
	mustCreate(t, db, &finance.OrderExpense{
		TenantID: 2, OrderID: o.ID, TypeCode: "ads", Amount: 30, Currency: "CNY", IncurredAt: &now,
	})
	r, _ := reports.ResolveRange(30, "", "")
	res, err := svc.Reconciliation(financeTestCtx(1, nil), r, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("rows: %d", len(res.Rows))
	}
	row := res.Rows[0]
	if row.Received != 0 || row.SettlementStatus != finance.SettlementUnpaid {
		t.Fatalf("cross-tenant payment leaked: received=%v status=%s", row.Received, row.SettlementStatus)
	}
	if row.ExpenseBase != nil && *row.ExpenseBase != 0 {
		t.Fatalf("cross-tenant expense leaked: %v", *row.ExpenseBase)
	}
}

// The Currency column must be formula-escaped like the other text columns.
func TestReconciliationCSVCurrencyEscaped(t *testing.T) {
	db := openFinanceTestDB(t)
	svc := newFinanceService(db, &fakeSettings{groups: map[string]map[string]string{
		"report_currency": {"base_currency": "CNY", "rates": `{}`},
	}})
	shopID := seedShop(t, db, 1, "店铺A")
	seedPaidOrder(t, db, 1, &shopID, "SO-EVIL", "=1+2", 100)
	r, _ := reports.ResolveRange(30, "", "")
	data, _, err := svc.ExportReconciliationCSV(financeTestCtx(1, nil), r, "")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "'=1+2") {
		t.Fatalf("currency not escaped: %s", text)
	}
}
