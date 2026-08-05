package migrationimport_test

import (
	"encoding/csv"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/finance"
	"github.com/trademind-ai/trademind/backend/internal/modules/migrationimport"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

func openPaymentTestDB(t *testing.T) *gorm.DB {
	db := openTestDB(t)
	if err := db.AutoMigrate(&finance.PaymentRecord{}, &finance.OrderExpense{}, &finance.ShopMonthlyExpense{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func newPaymentSvc(db *gorm.DB) *migrationimport.Service {
	return &migrationimport.Service{
		DB:       db,
		Products: &product.Service{DB: db},
		Orders:   &order.Service{DB: db},
		Finance:  &finance.Service{DB: db, Proc: &procurement.Service{DB: db}},
	}
}

func seedPaymentOrder(t *testing.T, db *gorm.DB, tenantID int64, shopID *uuid.UUID, orderNo string, total float64) *order.Order {
	t.Helper()
	o := &order.Order{
		TenantID: tenantID, Platform: "tiktok", ShopID: shopID, OrderNo: orderNo,
		Status: "pending", PaymentStatus: order.PaymentPaid, Currency: "CNY", TotalAmount: total,
	}
	if err := db.Create(o).Error; err != nil {
		t.Fatal(err)
	}
	return o
}

func paymentBody(rows [][]string, hash string) migrationimport.WizardBody {
	return migrationimport.WizardBody{
		Kind:    migrationimport.KindPayment,
		Columns: []string{"订单号", "回款金额", "币种", "手续费", "回款日期"},
		Rows:    rows,
		Mapping: map[string]int{
			"orderNo": 0, "paymentAmount": 1, "currency": 2, "feeAmount": 3, "receivedAt": 4,
		},
		FileName:     "payments.csv",
		FileHash:     hash,
		SourceFormat: migrationimport.SourceCustom,
	}
}

func TestValidatePayments(t *testing.T) {
	db := openPaymentTestDB(t)
	svc := newPaymentSvc(db)
	c := testCtx(1)

	out, err := svc.Validate(c, paymentBody([][]string{
		{"SO-1", "100.00", "CNY", "5.00", "2026-01-05"},
		{"SO-1", "100.00", "CNY", "5.00", "2026-01-05"}, // duplicate in file
		{"SO-2", "-3", "CNY", "", "2026-01-05"},         // bad amount
		{"SO-3", "50", "CNY", "60", "2026-01-05"},       // fee > amount
		{"SO-4", "50", "CNY", "", "not-a-date"},         // bad date
		{"SO-5", "50", "BAD!", "", "2026-01-05"},        // bad currency
	}, "hash-pay-validate"))
	if err != nil {
		t.Fatal(err)
	}
	if out.TotalRows != 6 || out.ValidRows != 1 || out.ErrorRows != 5 {
		t.Fatalf("unexpected: %+v", out)
	}
}

func TestCommitPaymentsAndDuplicates(t *testing.T) {
	db := openPaymentTestDB(t)
	svc := newPaymentSvc(db)
	c := testCtx(1)
	o := seedPaymentOrder(t, db, 1, nil, "SO-PAY-1", 100)
	seedPaymentOrder(t, db, 2, nil, "SO-OTHER-TENANT", 100)

	body := paymentBody([][]string{
		{"SO-PAY-1", "60.00", "CNY", "3.00", "2026-01-05"},
		{"SO-MISSING", "10.00", "CNY", "", "2026-01-05"},
		{"SO-OTHER-TENANT", "10.00", "CNY", "", "2026-01-05"}, // cross tenant → not found
	}, "hash-pay-commit")
	out, err := svc.Commit(c, body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.SuccessRows != 1 || out.FailedRows != 2 {
		t.Fatalf("commit: %+v", out)
	}
	var n int64
	if err := db.Model(&finance.PaymentRecord{}).Where("order_id = ?", o.ID).Count(&n).Error; err != nil || n != 1 {
		t.Fatalf("payment rows: %d %v", n, err)
	}
	var rec finance.PaymentRecord
	if err := db.First(&rec, "order_id = ?", o.ID).Error; err != nil {
		t.Fatal(err)
	}
	if rec.Source != finance.SourceImport || rec.Amount != 60 || rec.FeeAmount != 3 {
		t.Fatalf("record: %+v", rec)
	}

	// batch replay: identical file hash returns the previous summary.
	replay, err := svc.Commit(c, body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Replayed {
		t.Fatalf("expected replay: %+v", replay)
	}

	// same payment row in a new file is skipped as duplicate.
	dup := paymentBody([][]string{{"SO-PAY-1", "60.00", "CNY", "3.00", "2026-01-05"}}, "hash-pay-commit-2")
	out2, err := svc.Commit(c, dup, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out2.SuccessRows != 0 || out2.DuplicateRows != 1 {
		t.Fatalf("duplicate commit: %+v", out2)
	}
	if err := db.Model(&finance.PaymentRecord{}).Where("order_id = ?", o.ID).Count(&n).Error; err != nil || n != 1 {
		t.Fatalf("still one payment row: %d %v", n, err)
	}
}

func TestCommitPaymentsStoreScope(t *testing.T) {
	db := openPaymentTestDB(t)
	svc := newPaymentSvc(db)
	shopA := seedShop(t, db, 1)
	shopB := seedShop(t, db, 1)
	seedPaymentOrder(t, db, 1, &shopA, "SO-SHOP-A", 100)

	// operator granted only shopB: shopA's order is invisible → row fails.
	c := testCtx(1)
	c.Set("adminperm.principal", &adminperm.Principal{
		UserID: uuid.New(), Role: adminperm.RoleOperator,
		StoreGrants: []adminperm.StoreGrant{{StoreID: shopB, PermissionScope: "operate"}},
	})
	out, err := svc.Commit(c, paymentBody([][]string{
		{"SO-SHOP-A", "100.00", "CNY", "", "2026-01-05"},
	}, "hash-pay-scope"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.SuccessRows != 0 || out.FailedRows != 1 {
		t.Fatalf("scoped commit: %+v", out)
	}

	// view-only grant on shopA: visible but not operable → row fails too.
	c2 := testCtx(1)
	c2.Set("adminperm.principal", &adminperm.Principal{
		UserID: uuid.New(), Role: adminperm.RoleOperator,
		StoreGrants: []adminperm.StoreGrant{{StoreID: shopA, PermissionScope: "view"}},
	})
	out2, err := svc.Commit(c2, paymentBody([][]string{
		{"SO-SHOP-A", "100.00", "CNY", "", "2026-01-05"},
	}, "hash-pay-scope-2"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if out2.SuccessRows != 0 || out2.FailedRows != 1 {
		t.Fatalf("view-only commit: %+v", out2)
	}
	var n int64
	if err := db.Model(&finance.PaymentRecord{}).Count(&n).Error; err != nil || n != 0 {
		t.Fatalf("no payment rows expected: %d %v", n, err)
	}
}

func exportPaymentsCSV(t *testing.T, svc *migrationimport.Service, c *gin.Context, rec *httptest.ResponseRecorder) [][]string {
	t.Helper()
	h := &migrationimport.Handler{Svc: svc}
	c.Params = gin.Params{{Key: "kind", Value: "payment"}}
	c.Request = httptest.NewRequest("GET", "/api/v1/imports/export/payment", nil)
	h.ExportCSV(c)
	body := strings.TrimPrefix(rec.Body.String(), "\xef\xbb\xbf")
	rows, err := csv.NewReader(strings.NewReader(body)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestExportPaymentsCSV(t *testing.T) {
	db := openPaymentTestDB(t)
	svc := newPaymentSvc(db)
	shopA := seedShop(t, db, 1)
	shopB := seedShop(t, db, 1)
	o1 := seedPaymentOrder(t, db, 1, &shopA, "SO-EXP-1", 100)
	o2 := seedPaymentOrder(t, db, 1, &shopB, "SO-EXP-2", 200)
	oOther := seedPaymentOrder(t, db, 2, nil, "SO-EXP-OTHER", 50)
	seed := func(o *order.Order, tenantID int64, amount float64, source string, day string) {
		t.Helper()
		at, err := time.Parse("2006-01-02", day)
		if err != nil {
			t.Fatal(err)
		}
		rec := &finance.PaymentRecord{
			TenantID: tenantID, OrderID: o.ID, ShopID: o.ShopID,
			Amount: amount, Currency: "CNY", FeeAmount: 1.5, ReceivedAt: at,
			Channel: "平台结算", Source: source,
		}
		if err := db.Create(rec).Error; err != nil {
			t.Fatal(err)
		}
	}
	seed(o1, 1, 100, finance.SourceManual, "2026-01-05")
	seed(o2, 1, 60, finance.SourceImport, "2026-01-06")
	seed(oOther, 2, 50, finance.SourceManual, "2026-01-07") // other tenant

	// Admin sees both tenant-1 rows, never the other tenant's.
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set(ctxkey.TenantID, int64(1))
	rows := exportPaymentsCSV(t, svc, c, rec)
	if len(rows) != 3 {
		t.Fatalf("expected header + 2 rows, got %d: %v", len(rows), rows)
	}
	wantHeader := []string{"订单号", "回款金额", "币种", "手续费", "回款日期", "回款渠道", "备注", "来源"}
	for i, hcol := range wantHeader {
		if rows[0][i] != hcol {
			t.Fatalf("header[%d] = %q, want %q", i, rows[0][i], hcol)
		}
	}
	if rows[1][0] != "SO-EXP-1" || rows[1][7] != "手工" || rows[1][4] != "2026-01-05" {
		t.Fatalf("row1: %v", rows[1])
	}
	if rows[2][0] != "SO-EXP-2" || rows[2][7] != "导入" || rows[2][1] != "60" {
		t.Fatalf("row2: %v", rows[2])
	}

	// Operator scoped to shopA only sees shopA's payment.
	rec2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(rec2)
	c2.Set(ctxkey.TenantID, int64(1))
	c2.Set("adminperm.principal", &adminperm.Principal{
		UserID: uuid.New(), Role: adminperm.RoleOperator,
		StoreGrants: []adminperm.StoreGrant{{StoreID: shopA, PermissionScope: "view"}},
	})
	rows2 := exportPaymentsCSV(t, svc, c2, rec2)
	if len(rows2) != 2 || rows2[1][0] != "SO-EXP-1" {
		t.Fatalf("scoped export: %v", rows2)
	}
}
