package order_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"gorm.io/gorm"
)

func TestExportShippingListCSV(t *testing.T) {
	db := openImportTestDB(t)
	svc := &order.Service{DB: db}
	a := createFlowTestOrder(t, svc, "SO-EXP-A")
	b := createFlowTestOrder(t, svc, "SO-EXP-B")

	c := importTestCtx(1)
	data, name, err := svc.ExportShippingListCSV(c, []uuid.UUID{a.ID, b.ID})
	if err != nil {
		t.Fatal(err)
	}
	if name != "shipping-list-2.csv" {
		t.Fatalf("unexpected name %s", name)
	}
	body := string(data)
	if !strings.HasPrefix(body, "\xEF\xBB\xBF") {
		t.Fatal("expected UTF-8 BOM")
	}
	if !strings.Contains(body, "订单号") || !strings.Contains(body, "快递单号(回填)") {
		t.Fatalf("missing header columns: %s", body)
	}
	if !strings.Contains(body, "SO-EXP-A") || !strings.Contains(body, "SO-EXP-B") {
		t.Fatalf("missing order rows: %s", body)
	}
}

func TestExportShippingListCSVCrossTenantDenied(t *testing.T) {
	db := openImportTestDB(t)
	svc := &order.Service{DB: db}
	a := createFlowTestOrder(t, svc, "SO-EXP-T1")

	c := importTestCtx(2)
	_, _, err := svc.ExportShippingListCSV(c, []uuid.UUID{a.ID})
	if err != gorm.ErrRecordNotFound {
		t.Fatalf("expected not found for cross-tenant export, got %v", err)
	}
}
