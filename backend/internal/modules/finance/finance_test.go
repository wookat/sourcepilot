package finance_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/finance"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/reports"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

func openFinanceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:finance_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&order.Order{}, &order.OrderItem{}, &shop.Shop{},
		&product.Product{}, &product.ProductSKU{},
		&sourcing.Supplier{}, &sourcing.ProductSource{}, &sourcing.ProductSourceSKU{}, &sourcing.SourcePriceHistory{},
		&procurement.PurchaseOrder{}, &procurement.PurchaseOrderItem{}, &procurement.PurchaseOrderEvent{}, &procurement.PurchaseLogistics{},
		&finance.PaymentRecord{}, &finance.OrderExpense{}, &finance.ShopMonthlyExpense{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func financeTestCtx(tenantID int64, principal *adminperm.Principal) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/finance/payments", nil)
	c.Set(ctxkey.TenantID, tenantID)
	if principal != nil {
		c.Set("adminperm.principal", principal)
	}
	return c
}

// fakeSettings implements fxrate.SettingsReader.
type fakeSettings struct {
	groups map[string]map[string]string
}

func (f *fakeSettings) PlainByGroup(_ context.Context, _ int64, groupKey string) (map[string]string, error) {
	if f == nil || f.groups == nil {
		return map[string]string{}, nil
	}
	return f.groups[groupKey], nil
}

func newFinanceService(db *gorm.DB, settings *fakeSettings) *finance.Service {
	if settings == nil {
		settings = &fakeSettings{}
	}
	return &finance.Service{DB: db, Settings: settings, Proc: &procurement.Service{DB: db}}
}

func mustCreate(t *testing.T, db *gorm.DB, v any) {
	t.Helper()
	if err := db.Create(v).Error; err != nil {
		t.Fatal(err)
	}
}

func seedShop(t *testing.T, db *gorm.DB, tenantID int64, name string) uuid.UUID {
	t.Helper()
	sh := &shop.Shop{TenantID: tenantID, Platform: "tiktok", ShopName: name, Status: "active", AuthStatus: "authorized"}
	mustCreate(t, db, sh)
	return sh.ID
}

func seedPaidOrder(t *testing.T, db *gorm.DB, tenantID int64, shopID *uuid.UUID, orderNo, currency string, total float64) *order.Order {
	t.Helper()
	o := &order.Order{
		TenantID: tenantID, Platform: "tiktok", ShopID: shopID, OrderNo: orderNo,
		Status: "pending", PaymentStatus: order.PaymentPaid, Currency: currency, TotalAmount: total,
	}
	mustCreate(t, db, o)
	it := &order.OrderItem{OrderID: o.ID, ProductTitle: "测试商品", Quantity: 1, UnitPrice: total, TotalPrice: total}
	mustCreate(t, db, it)
	return o
}

func payBody(orderID uuid.UUID, amount float64, currency, day string) finance.PaymentBody {
	return finance.PaymentBody{OrderID: orderID.String(), Amount: amount, Currency: currency, ReceivedAt: day}
}

func operatorFor(shopID uuid.UUID, scope string) *adminperm.Principal {
	return &adminperm.Principal{
		UserID: uuid.New(), Role: adminperm.RoleOperator,
		StoreGrants: []adminperm.StoreGrant{{StoreID: shopID, PermissionScope: scope}},
	}
}

func TestCreatePaymentValidation(t *testing.T) {
	db := openFinanceTestDB(t)
	svc := newFinanceService(db, nil)
	shopID := seedShop(t, db, 1, "店铺A")
	o := seedPaidOrder(t, db, 1, &shopID, "SO-1", "CNY", 100)

	cases := []finance.PaymentBody{
		{OrderID: "bad-uuid", Amount: 10, ReceivedAt: "2026-01-05"},
		{OrderID: o.ID.String(), Amount: 0, ReceivedAt: "2026-01-05"},
		{OrderID: o.ID.String(), Amount: 10, FeeAmount: -1, ReceivedAt: "2026-01-05"},
		{OrderID: o.ID.String(), Amount: 10, FeeAmount: 11, ReceivedAt: "2026-01-05"},
		{OrderID: o.ID.String(), Amount: 10, Currency: "XYZ123", ReceivedAt: "2026-01-05"},
		{OrderID: o.ID.String(), Amount: 10, ReceivedAt: "05/01/2026"},
		{OrderID: o.ID.String(), Amount: 10, ReceivedAt: ""},
	}
	for i, body := range cases {
		if _, err := svc.CreatePayment(financeTestCtx(1, nil), body, nil); !errors.Is(err, finance.ErrBadRequest) {
			t.Fatalf("case %d: want ErrBadRequest, got %v", i, err)
		}
	}
	if _, err := svc.CreatePayment(financeTestCtx(1, nil), payBody(uuid.New(), 10, "CNY", "2026-01-05"), nil); !errors.Is(err, finance.ErrNotFound) {
		t.Fatalf("unknown order: want ErrNotFound, got %v", err)
	}
	if _, err := svc.CreatePayment(financeTestCtx(1, nil), payBody(o.ID, 50, "", "2026-01-05"), nil); err != nil {
		t.Fatalf("valid payment (order currency fallback): %v", err)
	}
}

func TestSettlementStatuses(t *testing.T) {
	db := openFinanceTestDB(t)
	svc := newFinanceService(db, nil)
	shopID := seedShop(t, db, 1, "店铺A")
	c := financeTestCtx(1, nil)

	unpaid := seedPaidOrder(t, db, 1, &shopID, "SO-UNPAID", "CNY", 100)
	short := seedPaidOrder(t, db, 1, &shopID, "SO-SHORT", "CNY", 100)
	over := seedPaidOrder(t, db, 1, &shopID, "SO-OVER", "CNY", 100)
	settled := seedPaidOrder(t, db, 1, &shopID, "SO-SETTLED", "CNY", 100)
	boundary := seedPaidOrder(t, db, 1, &shopID, "SO-BOUNDARY", "CNY", 100)

	for _, p := range []struct {
		o      *order.Order
		amount float64
	}{{short, 80}, {over, 120.5}, {settled, 100}, {boundary, 99.99}} {
		if _, err := svc.CreatePayment(c, payBody(p.o.ID, p.amount, "CNY", "2026-01-05"), nil); err != nil {
			t.Fatal(err)
		}
	}
	rows, _, err := svc.ListPayments(c, finance.ListPaymentsQuery{PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, r := range rows {
		got[r.OrderNo] = r.SettlementStatus
	}
	want := map[string]string{
		"SO-SHORT":    finance.SettlementShort,
		"SO-OVER":     finance.SettlementOver,
		"SO-SETTLED":  finance.SettlementSettled,
		"SO-BOUNDARY": finance.SettlementSettled, // within ±0.01 tolerance
	}
	for no, status := range want {
		if got[no] != status {
			t.Fatalf("%s: want %s, got %s", no, status, got[no])
		}
	}
	sum, err := svc.OrderSummary(c, unpaid.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Finance.SettlementStatus != finance.SettlementUnpaid {
		t.Fatalf("unpaid order: got %s", sum.Finance.SettlementStatus)
	}
}

func TestPaymentStoreScopeAndTenantIsolation(t *testing.T) {
	db := openFinanceTestDB(t)
	svc := newFinanceService(db, nil)
	shopA := seedShop(t, db, 1, "店铺A")
	shopB := seedShop(t, db, 1, "店铺B")
	oA := seedPaidOrder(t, db, 1, &shopA, "SO-A", "CNY", 100)
	oT2 := seedPaidOrder(t, db, 2, nil, "SO-T2", "CNY", 100)

	// operator granted only shopB cannot see shopA's order → 404
	other := operatorFor(shopB, "operate")
	if _, err := svc.CreatePayment(financeTestCtx(1, other), payBody(oA.ID, 10, "CNY", "2026-01-05"), nil); !errors.Is(err, finance.ErrNotFound) {
		t.Fatalf("out-of-scope order: want ErrNotFound, got %v", err)
	}
	// view-only grant on shopA can see but not operate → 403
	viewer := operatorFor(shopA, "view")
	if _, err := svc.CreatePayment(financeTestCtx(1, viewer), payBody(oA.ID, 10, "CNY", "2026-01-05"), nil); !errors.Is(err, finance.ErrForbidden) {
		t.Fatalf("view-only grant: want ErrForbidden, got %v", err)
	}
	// operate grant works
	op := operatorFor(shopA, "operate")
	rec, err := svc.CreatePayment(financeTestCtx(1, op), payBody(oA.ID, 10, "CNY", "2026-01-05"), nil)
	if err != nil {
		t.Fatal(err)
	}
	// tenant 1 cannot reach tenant 2's order → 404
	if _, err := svc.CreatePayment(financeTestCtx(1, nil), payBody(oT2.ID, 10, "CNY", "2026-01-05"), nil); !errors.Is(err, finance.ErrNotFound) {
		t.Fatalf("cross tenant: want ErrNotFound, got %v", err)
	}
	// list scope: shopB operator sees no rows
	rows, _, err := svc.ListPayments(financeTestCtx(1, other), finance.ListPaymentsQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("scoped list: want 0 rows, got %d", len(rows))
	}
	// delete scope: shopB operator gets 404 for shopA's record
	if err := svc.DeletePayment(financeTestCtx(1, other), rec.ID, nil); !errors.Is(err, finance.ErrNotFound) {
		t.Fatalf("out-of-scope delete: want ErrNotFound, got %v", err)
	}
	if err := svc.DeletePayment(financeTestCtx(1, op), rec.ID, nil); err != nil {
		t.Fatal(err)
	}
}

func TestFindDuplicatePayment(t *testing.T) {
	db := openFinanceTestDB(t)
	svc := newFinanceService(db, nil)
	o := seedPaidOrder(t, db, 1, nil, "SO-DUP", "CNY", 100)
	c := financeTestCtx(1, nil)
	if _, err := svc.CreatePayment(c, payBody(o.ID, 50, "CNY", "2026-01-05"), nil); err != nil {
		t.Fatal(err)
	}
	day, _ := time.ParseInLocation("2006-01-02", "2026-01-05", time.Local)
	dup, err := svc.FindDuplicatePayment(context.Background(), 1, o.ID, 50, "CNY", day)
	if err != nil || !dup {
		t.Fatalf("want duplicate, got %v %v", dup, err)
	}
	dup, err = svc.FindDuplicatePayment(context.Background(), 1, o.ID, 50.01, "CNY", day)
	if err != nil || dup {
		t.Fatalf("different amount should not be duplicate: %v %v", dup, err)
	}
	dup, err = svc.FindDuplicatePayment(context.Background(), 1, o.ID, 50, "CNY", day.AddDate(0, 0, 1))
	if err != nil || dup {
		t.Fatalf("different day should not be duplicate: %v %v", dup, err)
	}
}

func TestOrderExpenseCRUD(t *testing.T) {
	db := openFinanceTestDB(t)
	svc := newFinanceService(db, nil)
	shopID := seedShop(t, db, 1, "店铺A")
	o := seedPaidOrder(t, db, 1, &shopID, "SO-EXP", "CNY", 100)
	c := financeTestCtx(1, nil)

	if _, err := svc.CreateOrderExpense(c, finance.OrderExpenseBody{OrderID: o.ID.String(), TypeCode: "nope", Amount: 5}, nil); !errors.Is(err, finance.ErrBadRequest) {
		t.Fatalf("invalid type: want ErrBadRequest, got %v", err)
	}
	if _, err := svc.CreateOrderExpense(c, finance.OrderExpenseBody{OrderID: o.ID.String(), TypeCode: "promotion", Amount: 0}, nil); !errors.Is(err, finance.ErrBadRequest) {
		t.Fatalf("zero amount: want ErrBadRequest, got %v", err)
	}
	exp, err := svc.CreateOrderExpense(c, finance.OrderExpenseBody{OrderID: o.ID.String(), TypeCode: "promotion", Amount: 12.5, IncurredAt: "2026-01-06"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if exp.Currency != "CNY" || exp.ShopID == nil || *exp.ShopID != shopID {
		t.Fatalf("expense defaults: %+v", exp)
	}
	// readonly-style viewer cannot delete
	viewer := operatorFor(shopID, "view")
	if err := svc.DeleteOrderExpense(financeTestCtx(1, viewer), exp.ID, nil); !errors.Is(err, finance.ErrForbidden) {
		t.Fatalf("view-only delete: want ErrForbidden, got %v", err)
	}
	if err := svc.DeleteOrderExpense(c, exp.ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteOrderExpense(c, exp.ID, nil); !errors.Is(err, finance.ErrNotFound) {
		t.Fatalf("re-delete: want ErrNotFound, got %v", err)
	}
}

func TestShopExpenseValidationAndScope(t *testing.T) {
	db := openFinanceTestDB(t)
	svc := newFinanceService(db, nil)
	shopID := seedShop(t, db, 1, "店铺A")
	shopT2 := seedShop(t, db, 2, "店铺T2")
	c := financeTestCtx(1, nil)

	if _, err := svc.CreateShopExpense(c, finance.ShopExpenseBody{ShopID: shopID.String(), Month: "2026/01", TypeCode: "other", Amount: 10}, nil); !errors.Is(err, finance.ErrBadRequest) {
		t.Fatalf("bad month: want ErrBadRequest, got %v", err)
	}
	if _, err := svc.CreateShopExpense(c, finance.ShopExpenseBody{ShopID: shopT2.String(), Month: "2026-01", TypeCode: "other", Amount: 10}, nil); !errors.Is(err, finance.ErrNotFound) {
		t.Fatalf("cross-tenant shop: want ErrNotFound, got %v", err)
	}
	exp, err := svc.CreateShopExpense(c, finance.ShopExpenseBody{ShopID: shopID.String(), Month: "2026-01", TypeCode: "other", Amount: 200}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rows, total, err := svc.ListShopExpenses(c, shopID.String(), "2026-01", 1, 20)
	if err != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("list shop expenses: %v total=%d rows=%d", err, total, len(rows))
	}
	if err := svc.DeleteShopExpense(c, exp.ID, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExpenseTypesConfig(t *testing.T) {
	db := openFinanceTestDB(t)
	svc := newFinanceService(db, &fakeSettings{groups: map[string]map[string]string{
		"finance": {"expense_types": `[{"code":"storage","label":"仓储费"},{"code":"promotion","label":"重复忽略"},{"code":"","label":"无效"}]`},
	}})
	types := svc.ExpenseTypes(context.Background(), 1)
	if len(types) != len(finance.DefaultExpenseTypes())+1 {
		t.Fatalf("want defaults + storage, got %+v", types)
	}
	last := types[len(types)-1]
	if last.Code != "storage" || last.Label != "仓储费" {
		t.Fatalf("custom type: %+v", last)
	}
	_ = db
}

func seedActualCost(t *testing.T, db *gorm.DB, tenantID int64, salesOrderID uuid.UUID, qty int, actual *float64, currency, status string) {
	t.Helper()
	po := &procurement.PurchaseOrder{TenantID: tenantID, Status: status, Currency: currency, IdempotencyKey: uuid.NewString()}
	mustCreate(t, db, po)
	item := &procurement.PurchaseOrderItem{
		TenantID: tenantID, PurchaseOrderID: po.ID, SalesOrderID: &salesOrderID,
		SourceSKUID: uuid.New(), Quantity: qty, ActualPrice: actual,
	}
	mustCreate(t, db, item)
}

func TestOrderSummaryActualVsEstimated(t *testing.T) {
	db := openFinanceTestDB(t)
	svc := newFinanceService(db, &fakeSettings{groups: map[string]map[string]string{
		"report_currency": {"base_currency": "CNY", "rates": `{"USD":"7.0"}`},
	}})
	shopID := seedShop(t, db, 1, "店铺A")
	o := seedPaidOrder(t, db, 1, &shopID, "SO-PROFIT", "USD", 100)
	c := financeTestCtx(1, nil)

	// 回款 100 USD - 手续费 5 USD → 净回款 665 CNY
	if _, err := svc.CreatePayment(c, finance.PaymentBody{OrderID: o.ID.String(), Amount: 100, Currency: "USD", FeeAmount: 5, ReceivedAt: "2026-01-05"}, nil); err != nil {
		t.Fatal(err)
	}
	// 采购实付 2 × 150 CNY = 300 CNY
	actual := 150.0
	seedActualCost(t, db, 1, o.ID, 2, &actual, "CNY", procurement.StatusPaid)
	// 订单费用 65 CNY
	if _, err := svc.CreateOrderExpense(c, finance.OrderExpenseBody{OrderID: o.ID.String(), TypeCode: "promotion", Amount: 65, Currency: "CNY"}, nil); err != nil {
		t.Fatal(err)
	}
	sum, err := svc.OrderSummary(c, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	fin := sum.Finance
	if fin.ReceivedBase == nil || *fin.ReceivedBase != 665 {
		t.Fatalf("receivedBase: %+v", fin.ReceivedBase)
	}
	if fin.ActualCostBase == nil || *fin.ActualCostBase != 300 {
		t.Fatalf("actualCostBase: %+v", fin.ActualCostBase)
	}
	if fin.ExpenseBase == nil || *fin.ExpenseBase != 65 {
		t.Fatalf("expenseBase: %+v", fin.ExpenseBase)
	}
	// 实算毛利 = 665 - 300 - 65 = 300
	if fin.ActualProfitBase == nil || *fin.ActualProfitBase != 300 {
		t.Fatalf("actualProfitBase: %+v", fin.ActualProfitBase)
	}
	// 估算毛利 = 700（无参考成本、无估算费用）
	if fin.EstimatedProfitBase == nil || *fin.EstimatedProfitBase != 700 {
		t.Fatalf("estimatedProfitBase: %+v", fin.EstimatedProfitBase)
	}
	// 差异 -400 → 差异较大
	if fin.ProfitDiffBase == nil || *fin.ProfitDiffBase != -400 || !fin.LargeDiff {
		t.Fatalf("profitDiff: %+v largeDiff=%v", fin.ProfitDiffBase, fin.LargeDiff)
	}
	if fin.SettlementStatus != finance.SettlementSettled {
		t.Fatalf("settlement: %s", fin.SettlementStatus)
	}
}

func TestOrderSummaryMissingRateStaysNil(t *testing.T) {
	db := openFinanceTestDB(t)
	svc := newFinanceService(db, &fakeSettings{groups: map[string]map[string]string{
		"report_currency": {"base_currency": "CNY", "rates": `{}`},
	}})
	o := seedPaidOrder(t, db, 1, nil, "SO-EUR", "EUR", 100)
	c := financeTestCtx(1, nil)
	if _, err := svc.CreatePayment(c, payBody(o.ID, 100, "EUR", "2026-01-05"), nil); err != nil {
		t.Fatal(err)
	}
	sum, err := svc.OrderSummary(c, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Finance.ReceivedBase != nil || sum.Finance.ActualProfitBase != nil || sum.Finance.EstimatedProfitBase != nil {
		t.Fatalf("missing rate must not fabricate base amounts: %+v", sum.Finance)
	}
	if sum.Finance.SettlementStatus != finance.SettlementSettled {
		t.Fatalf("order-currency settlement still works: %s", sum.Finance.SettlementStatus)
	}
}

func TestReconciliationFilterAndCSV(t *testing.T) {
	db := openFinanceTestDB(t)
	svc := newFinanceService(db, &fakeSettings{groups: map[string]map[string]string{
		"report_currency": {"base_currency": "CNY", "rates": `{}`},
	}})
	shopID := seedShop(t, db, 1, "店铺A")
	c := financeTestCtx(1, nil)
	settled := seedPaidOrder(t, db, 1, &shopID, "SO-OK", "CNY", 100)
	seedPaidOrder(t, db, 1, &shopID, "SO-NONE", "CNY", 80)
	short := seedPaidOrder(t, db, 1, &shopID, "SO-LESS", "CNY", 100)
	if _, err := svc.CreatePayment(c, payBody(settled.ID, 100, "CNY", "2026-01-05"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreatePayment(c, payBody(short.ID, 60, "CNY", "2026-01-05"), nil); err != nil {
		t.Fatal(err)
	}
	r, _ := reports.ResolveRange(30, "", "")
	res, err := svc.Reconciliation(c, r, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.OrderCount != 3 || res.Summary.SettledCount != 1 || res.Summary.UnpaidCount != 1 || res.Summary.ShortCount != 1 {
		t.Fatalf("summary: %+v", res.Summary)
	}
	if res.Summary.FlaggedCount != 2 {
		t.Fatalf("flagged: %+v", res.Summary)
	}
	flagged, err := svc.Reconciliation(c, r, "flagged")
	if err != nil || len(flagged.Rows) != 2 {
		t.Fatalf("flagged rows: %v %d", err, len(flagged.Rows))
	}
	if _, err := svc.Reconciliation(c, r, "bogus"); !errors.Is(err, finance.ErrBadRequest) {
		t.Fatalf("bogus status: want ErrBadRequest, got %v", err)
	}
	data, name, err := svc.ExportReconciliationCSV(c, r, "short")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "SO-LESS") || strings.Contains(text, "SO-OK") {
		t.Fatalf("csv filter: %s", text)
	}
	if !strings.Contains(text, "少款") || !strings.HasPrefix(name, "finance-reconciliation-") {
		t.Fatalf("csv content/name: %s %s", text, name)
	}
}

func TestReconciliationCSVFullExport(t *testing.T) {
	db := openFinanceTestDB(t)
	svc := newFinanceService(db, &fakeSettings{groups: map[string]map[string]string{
		"report_currency": {"base_currency": "CNY", "rates": `{}`},
	}})
	shopID := seedShop(t, db, 1, "店铺A")
	// > page cap (500) and > one keyset order-load page (1000)
	const n = 1005
	orders := make([]order.Order, 0, n)
	for i := 0; i < n; i++ {
		orders = append(orders, order.Order{
			TenantID: 1, Platform: "tiktok", ShopID: &shopID, OrderNo: fmt.Sprintf("SO-BULK-%04d", i),
			Status: "pending", PaymentStatus: order.PaymentPaid, Currency: "CNY", TotalAmount: 100,
		})
	}
	if err := db.CreateInBatches(&orders, 200).Error; err != nil {
		t.Fatal(err)
	}
	c := financeTestCtx(1, nil)
	r, _ := reports.ResolveRange(30, "", "")

	page, err := svc.Reconciliation(c, r, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) != 500 || !page.Truncated {
		t.Fatalf("page cap: rows=%d truncated=%v", len(page.Rows), page.Truncated)
	}
	if page.Summary.OrderCount != n {
		t.Fatalf("summary should cover all orders: %d", page.Summary.OrderCount)
	}

	data, _, err := svc.ExportReconciliationCSV(c, r, "")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != n+1 {
		t.Fatalf("csv rows: want %d data rows + header, got %d lines", n, len(lines))
	}
	seen := map[string]bool{}
	for _, line := range lines[1:] {
		no := strings.SplitN(line, ",", 2)[0]
		if seen[no] {
			t.Fatalf("duplicate order in csv: %s", no)
		}
		seen[no] = true
		if !strings.Contains(line, "100.00") {
			t.Fatalf("row values: %s", line)
		}
	}
	if len(seen) != n {
		t.Fatalf("distinct orders in csv: %d", len(seen))
	}
}

func TestReportShopMonthAggregation(t *testing.T) {
	db := openFinanceTestDB(t)
	svc := newFinanceService(db, &fakeSettings{groups: map[string]map[string]string{
		"report_currency": {"base_currency": "CNY", "rates": `{}`},
	}})
	shopID := seedShop(t, db, 1, "店铺A")
	c := financeTestCtx(1, nil)
	o := seedPaidOrder(t, db, 1, &shopID, "SO-R1", "CNY", 200)
	if _, err := svc.CreatePayment(c, payBody(o.ID, 200, "CNY", "2026-01-05"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateOrderExpense(c, finance.OrderExpenseBody{OrderID: o.ID.String(), TypeCode: "shipping", Amount: 20}, nil); err != nil {
		t.Fatal(err)
	}
	month := o.CreatedAt.Format("2006-01")
	if _, err := svc.CreateShopExpense(c, finance.ShopExpenseBody{ShopID: shopID.String(), Month: month, TypeCode: "other", Amount: 30}, nil); err != nil {
		t.Fatal(err)
	}
	r, _ := reports.ResolveRange(30, "", "")
	res, err := svc.Report(c, r)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("rows: %+v", res.Rows)
	}
	row := res.Rows[0]
	if row.ShopName != "店铺A" || row.Month != month || row.OrderCount != 1 {
		t.Fatalf("row identity: %+v", row)
	}
	if row.ReceivableBase == nil || *row.ReceivableBase != 200 || row.ReceivedBase == nil || *row.ReceivedBase != 200 {
		t.Fatalf("receivable/received: %+v", row)
	}
	if row.ReturnRatePercent == nil || *row.ReturnRatePercent != 100 {
		t.Fatalf("return rate: %+v", row.ReturnRatePercent)
	}
	if len(row.FeesByType) != 1 || row.FeesByType[0].TypeCode != "shipping" || row.FeesByType[0].Base != 20 {
		t.Fatalf("fee parts: %+v", row.FeesByType)
	}
	if row.ShopExpenseBase == nil || *row.ShopExpenseBase != 30 {
		t.Fatalf("shop expense: %+v", row.ShopExpenseBase)
	}
	// 实算毛利 = 200(净回款) - 0(实付) - 20(订单费用) - 30(店铺月费) = 150
	if row.ActualProfitBase == nil || *row.ActualProfitBase != 150 {
		t.Fatalf("actual profit: %+v", row.ActualProfitBase)
	}
	data, name, err := svc.ExportReportCSV(c, r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "店铺A") || !strings.HasPrefix(name, "finance-report-") {
		t.Fatalf("report csv: %s %s", string(data), name)
	}
}

func TestReconciliationStoreScope(t *testing.T) {
	db := openFinanceTestDB(t)
	svc := newFinanceService(db, &fakeSettings{groups: map[string]map[string]string{
		"report_currency": {"base_currency": "CNY", "rates": `{}`},
	}})
	shopA := seedShop(t, db, 1, "店铺A")
	shopB := seedShop(t, db, 1, "店铺B")
	seedPaidOrder(t, db, 1, &shopA, "SO-A1", "CNY", 100)
	seedPaidOrder(t, db, 1, &shopB, "SO-B1", "CNY", 100)
	seedPaidOrder(t, db, 2, nil, "SO-OTHER-TENANT", "CNY", 100)

	r, _ := reports.ResolveRange(30, "", "")
	res, err := svc.Reconciliation(financeTestCtx(1, operatorFor(shopA, "view")), r, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.OrderCount != 1 || len(res.Rows) != 1 || res.Rows[0].OrderNo != "SO-A1" {
		t.Fatalf("store scope: %+v", res.Rows)
	}
	res, err = svc.Reconciliation(financeTestCtx(1, nil), r, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.OrderCount != 2 {
		t.Fatalf("tenant isolation: %+v", res.Summary)
	}
}

func TestUpdateItemActualPrice(t *testing.T) {
	db := openFinanceTestDB(t)
	proc := &procurement.Service{DB: db}
	po := &procurement.PurchaseOrder{TenantID: 1, Status: procurement.StatusPlaced, Currency: "CNY", IdempotencyKey: uuid.NewString()}
	mustCreate(t, db, po)
	exp := 40.0
	item := &procurement.PurchaseOrderItem{
		TenantID: 1, PurchaseOrderID: po.ID, SourceSKUID: uuid.New(), Quantity: 2, ExpectedPrice: &exp,
	}
	mustCreate(t, db, item)

	if _, err := proc.UpdateItemActualPrice(context.Background(), po.ID, item.ID, 0, nil); !errors.Is(err, procurement.ErrBadRequest) {
		t.Fatalf("zero price: want ErrBadRequest, got %v", err)
	}
	out, err := proc.UpdateItemActualPrice(context.Background(), po.ID, item.ID, 45.5, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.TotalAmount != 91 {
		t.Fatalf("total should use actual price: %v", out.TotalAmount)
	}
	// draft order cannot register actual price
	draft := &procurement.PurchaseOrder{TenantID: 1, Status: procurement.StatusDraft, Currency: "CNY", IdempotencyKey: uuid.NewString()}
	mustCreate(t, db, draft)
	dItem := &procurement.PurchaseOrderItem{TenantID: 1, PurchaseOrderID: draft.ID, SourceSKUID: uuid.New(), Quantity: 1}
	mustCreate(t, db, dItem)
	if _, err := proc.UpdateItemActualPrice(context.Background(), draft.ID, dItem.ID, 10, nil); !errors.Is(err, procurement.ErrConflict) {
		t.Fatalf("draft: want ErrConflict, got %v", err)
	}
	if _, err := proc.UpdateItemActualPrice(context.Background(), po.ID, uuid.New(), 10, nil); !errors.Is(err, procurement.ErrNotFound) {
		t.Fatalf("missing item: want ErrNotFound, got %v", err)
	}
}
