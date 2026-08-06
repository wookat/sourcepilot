package finance_test

import (
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/reports"
)

// The platform column must render the Chinese display label, not the raw
// platform enum key.
func TestReconciliationCSVPlatformLocalized(t *testing.T) {
	db := openFinanceTestDB(t)
	svc := newFinanceService(db, &fakeSettings{groups: map[string]map[string]string{
		"report_currency": {"base_currency": "CNY", "rates": `{}`},
	}})
	shopID := seedShop(t, db, 1, "店铺A")
	seedPaidOrder(t, db, 1, &shopID, "SO-PLAT", "CNY", 100)
	r, _ := reports.ResolveRange(30, "", "")
	data, _, err := svc.ExportReconciliationCSV(financeTestCtx(1, nil), r, "")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "TikTok Shop") {
		t.Fatalf("platform not localized: %s", text)
	}
	if strings.Contains(text, ",tiktok,") {
		t.Fatalf("raw platform enum leaked: %s", text)
	}
}
