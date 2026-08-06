package reports_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/reports"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func openReportsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:reports_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
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
		&procurement.PurchaseOrder{}, &procurement.PurchaseOrderItem{}, &procurement.PurchaseOrderEvent{},
		&inventory.InventoryChangeLog{},
		&inventory.Warehouse{}, &inventory.WarehouseStock{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func reportsTestCtx(tenantID int64, principal *adminperm.Principal) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/reports/profit", nil)
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

func newReportsService(db *gorm.DB, settings *fakeSettings) *reports.Service {
	return &reports.Service{DB: db, Settings: settings, Proc: &procurement.Service{DB: db}}
}

func mustCreate(t *testing.T, db *gorm.DB, v any) {
	t.Helper()
	if err := db.Create(v).Error; err != nil {
		t.Fatal(err)
	}
}

func seedProfitFixture(t *testing.T, db *gorm.DB) (shopID uuid.UUID, productID uuid.UUID, skuID uuid.UUID) {
	t.Helper()
	sh := &shop.Shop{TenantID: 1, Platform: "tiktok", ShopName: "测试店铺", Status: "active", AuthStatus: "authorized"}
	mustCreate(t, db, sh)

	p := &product.Product{TenantID: 1, Source: "1688", Title: "测试商品A", Status: "draft"}
	mustCreate(t, db, p)
	sku := &product.ProductSKU{ProductID: p.ID, SKUCode: "SKU-A", SKUName: "红色"}
	mustCreate(t, db, sku)

	sup := &sourcing.Supplier{TenantID: 1, Platform: "1688", Name: "供应商甲"}
	mustCreate(t, db, sup)
	src := &sourcing.ProductSource{TenantID: 1, ProductID: p.ID, SupplierID: sup.ID, IsPrimary: true, Status: "active"}
	mustCreate(t, db, src)
	price := 30.0
	mustCreate(t, db, &sourcing.ProductSourceSKU{TenantID: 1, ProductSourceID: src.ID, LocalSKUID: sku.ID, CurrentPrice: &price, Currency: "CNY"})

	return sh.ID, p.ID, sku.ID
}

func seedPaidOrder(t *testing.T, db *gorm.DB, tenantID int64, shopID *uuid.UUID, orderNo, currency string, total float64, productID, skuID *uuid.UUID, qty int) *order.Order {
	t.Helper()
	o := &order.Order{
		TenantID: tenantID, Platform: "tiktok", ShopID: shopID, OrderNo: orderNo,
		Status: "pending", PaymentStatus: order.PaymentPaid, Currency: currency, TotalAmount: total,
	}
	mustCreate(t, db, o)
	it := &order.OrderItem{
		OrderID: o.ID, ProductID: productID, ProductSKUID: skuID,
		ProductTitle: "测试商品A", Quantity: qty, UnitPrice: total / float64(qty), TotalPrice: total,
	}
	mustCreate(t, db, it)
	return o
}

func TestResolveRange(t *testing.T) {
	r, err := reports.ResolveRange(7, "", "")
	if err != nil || r.Days != 7 {
		t.Fatalf("days=7: %v %+v", err, r)
	}
	r, err = reports.ResolveRange(0, "2026-01-01", "2026-01-31")
	if err != nil || r.Days != 31 || r.StartDate() != "2026-01-01" || r.EndDate() != "2026-01-31" {
		t.Fatalf("custom range: %v %+v", err, r)
	}
	if _, err = reports.ResolveRange(0, "2026-02-01", "2026-01-01"); err == nil {
		t.Fatal("expected error for reversed range")
	}
	if _, err = reports.ResolveRange(0, "bad", "2026-01-01"); err == nil {
		t.Fatal("expected error for bad date")
	}
	if _, err = reports.ResolveRange(0, "2020-01-01", "2026-01-01"); err == nil {
		t.Fatal("expected error for too-long range")
	}
}

func TestProfitReportConversionAndCost(t *testing.T) {
	db := openReportsTestDB(t)
	shopID, productID, skuID := seedProfitFixture(t, db)
	// Base USD, CNY→USD rate 0.14; EUR has no rate → unconverted.
	svc := newReportsService(db, &fakeSettings{groups: map[string]map[string]string{
		"report_currency": {"base_currency": "USD", "rates": `{"CNY":"0.14"}`},
		"report_profit":   {"fee_items": `[{"name":"平台佣金","mode":"percent","value":10}]`},
	}})

	seedPaidOrder(t, db, 1, &shopID, "SO-1", "USD", 100, &productID, &skuID, 2)
	seedPaidOrder(t, db, 1, &shopID, "SO-2", "EUR", 50, &productID, &skuID, 1)
	// other tenant must not leak
	seedPaidOrder(t, db, 2, nil, "SO-T2", "USD", 999, nil, nil, 1)

	r, _ := reports.ResolveRange(30, "", "")
	res, err := svc.ProfitReport(reportsTestCtx(1, nil), reports.DimensionOrder, r)
	if err != nil {
		t.Fatal(err)
	}
	if res.BaseCurrency != "USD" {
		t.Fatalf("base currency: %s", res.BaseCurrency)
	}
	if res.Summary.OrderCount != 2 {
		t.Fatalf("order count: %d", res.Summary.OrderCount)
	}
	// revenueBase: only USD 100 converts (EUR has no rate)
	if res.Summary.RevenueBase != 100 {
		t.Fatalf("revenueBase: %v", res.Summary.RevenueBase)
	}
	if len(res.Summary.UnconvertedCurrencies) != 1 || res.Summary.UnconvertedCurrencies[0] != "EUR" {
		t.Fatalf("unconverted: %v", res.Summary.UnconvertedCurrencies)
	}
	// cost: 30 CNY × 3 units = 90 CNY → 12.60 USD
	if res.Summary.CostCNY != 90 {
		t.Fatalf("costCNY: %v", res.Summary.CostCNY)
	}
	if res.Summary.CostBase == nil || *res.Summary.CostBase != 12.6 {
		t.Fatalf("costBase: %v", res.Summary.CostBase)
	}
	// fee: 10% of 100 = 10; profit = 100 - 12.6 - 10 = 77.4; margin 77.4%
	if res.Summary.FeeBase != 10 {
		t.Fatalf("feeBase: %v", res.Summary.FeeBase)
	}
	if res.Summary.GrossProfitBase == nil || *res.Summary.GrossProfitBase != 77.4 {
		t.Fatalf("profit: %v", res.Summary.GrossProfitBase)
	}
	if res.Summary.MarginPercent == nil || *res.Summary.MarginPercent != 77.4 {
		t.Fatalf("margin: %v", res.Summary.MarginPercent)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("rows: %d", len(res.Rows))
	}
}

func TestProfitReportNoCNYRate(t *testing.T) {
	db := openReportsTestDB(t)
	shopID, productID, skuID := seedProfitFixture(t, db)
	// Base USD without a CNY rate: cost cannot convert → no profit computed.
	svc := newReportsService(db, &fakeSettings{groups: map[string]map[string]string{
		"report_currency": {"base_currency": "USD", "rates": `{}`},
	}})
	seedPaidOrder(t, db, 1, &shopID, "SO-1", "USD", 100, &productID, &skuID, 1)

	r, _ := reports.ResolveRange(30, "", "")
	res, err := svc.ProfitReport(reportsTestCtx(1, nil), reports.DimensionOrder, r)
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.CostBase != nil || res.Summary.GrossProfitBase != nil {
		t.Fatalf("expected nil costBase/profit without CNY rate: %+v", res.Summary)
	}
	if res.Summary.CostCNY != 30 {
		t.Fatalf("costCNY: %v", res.Summary.CostCNY)
	}
}

func TestProfitReportDimensions(t *testing.T) {
	db := openReportsTestDB(t)
	shopID, productID, skuID := seedProfitFixture(t, db)
	svc := newReportsService(db, &fakeSettings{groups: map[string]map[string]string{
		"report_currency": {"base_currency": "CNY", "rates": `{"USD":"7.2"}`},
	}})
	seedPaidOrder(t, db, 1, &shopID, "SO-1", "USD", 10, &productID, &skuID, 1)
	seedPaidOrder(t, db, 1, nil, "SO-2", "CNY", 80, &productID, &skuID, 2)

	r, _ := reports.ResolveRange(30, "", "")
	c := reportsTestCtx(1, nil)

	byShop, err := svc.ProfitReport(c, reports.DimensionShop, r)
	if err != nil {
		t.Fatal(err)
	}
	if len(byShop.Rows) != 2 {
		t.Fatalf("shop rows: %d", len(byShop.Rows))
	}

	byProduct, err := svc.ProfitReport(reportsTestCtx(1, nil), reports.DimensionProduct, r)
	if err != nil {
		t.Fatal(err)
	}
	if len(byProduct.Rows) != 1 {
		t.Fatalf("product rows: %d", len(byProduct.Rows))
	}
	row := byProduct.Rows[0]
	// revenue: 10 USD × 7.2 + 80 CNY = 152 CNY; cost 3×30 = 90; orders 2
	if row.RevenueBase != 152 || row.CostCNY != 90 || row.OrderCount != 2 || row.Quantity != 3 {
		t.Fatalf("product row: %+v", row)
	}

	if _, err := svc.ProfitReport(reportsTestCtx(1, nil), "bogus", r); err == nil {
		t.Fatal("expected dimension error")
	}
}

func TestProfitReportStoreScope(t *testing.T) {
	db := openReportsTestDB(t)
	shopID, productID, skuID := seedProfitFixture(t, db)
	svc := newReportsService(db, &fakeSettings{})
	seedPaidOrder(t, db, 1, &shopID, "SO-IN", "CNY", 100, &productID, &skuID, 1)
	seedPaidOrder(t, db, 1, nil, "SO-NOSHOP", "CNY", 50, &productID, &skuID, 1)

	// operator granted a different shop: sees nothing from shopID
	other := uuid.New()
	scoped := &adminperm.Principal{
		UserID: uuid.New(), Role: adminperm.RoleOperator,
		StoreGrants: []adminperm.StoreGrant{{StoreID: other, PermissionScope: "operate"}},
	}
	r, _ := reports.ResolveRange(30, "", "")
	res, err := svc.ProfitReport(reportsTestCtx(1, scoped), reports.DimensionOrder, r)
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.OrderCount != 0 {
		t.Fatalf("scoped operator should see 0 orders, got %d", res.Summary.OrderCount)
	}

	granted := &adminperm.Principal{
		UserID: uuid.New(), Role: adminperm.RoleOperator,
		StoreGrants: []adminperm.StoreGrant{{StoreID: shopID, PermissionScope: "operate"}},
	}
	res, err = svc.ProfitReport(reportsTestCtx(1, granted), reports.DimensionOrder, r)
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.OrderCount != 1 {
		t.Fatalf("granted operator should see 1 order, got %d", res.Summary.OrderCount)
	}
}

func TestProfitCSVExport(t *testing.T) {
	db := openReportsTestDB(t)
	shopID, productID, skuID := seedProfitFixture(t, db)
	svc := newReportsService(db, &fakeSettings{groups: map[string]map[string]string{
		"report_currency": {"base_currency": "CNY", "rates": `{"USD":"7.2"}`},
	}})
	seedPaidOrder(t, db, 1, &shopID, "SO-1", "USD", 10, &productID, &skuID, 1)
	seedPaidOrder(t, db, 1, &shopID, "SO-2", "EUR", 5, &productID, &skuID, 1)

	r, _ := reports.ResolveRange(30, "", "")
	data, name, err := svc.ExportProfitCSV(reportsTestCtx(1, nil), reports.DimensionOrder, r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(name, "profit-report-order-") {
		t.Fatalf("filename: %s", name)
	}
	s := string(data)
	if !strings.HasPrefix(s, "\xEF\xBB\xBF") {
		t.Fatal("missing BOM")
	}
	if !strings.Contains(s, "收入(USD)") || !strings.Contains(s, "折算收入(USD→CNY)") {
		t.Fatalf("missing original/converted columns: %s", s)
	}
	if !strings.Contains(s, "72.00") {
		t.Fatalf("expected converted 72.00: %s", s)
	}
	if !strings.Contains(s, "EUR") {
		t.Fatalf("expected unconverted EUR flagged: %s", s)
	}
	// rate-less EUR row: converted cells read「未折算」instead of blanks
	if !strings.Contains(s, "5.00,未折算") {
		t.Fatalf("expected explicit 未折算 for rate-less EUR converted column: %s", s)
	}
}

func TestProfitCSVExportFullRows(t *testing.T) {
	db := openReportsTestDB(t)
	svc := newReportsService(db, &fakeSettings{groups: map[string]map[string]string{
		"report_currency": {"base_currency": "CNY", "rates": `{}`},
	}})
	// > page cap (500) and > one keyset order-load page (1000)
	const n = 1005
	orders := make([]order.Order, 0, n)
	for i := 0; i < n; i++ {
		orders = append(orders, order.Order{
			TenantID: 1, Platform: "tiktok", OrderNo: fmt.Sprintf("SO-BULK-%04d", i),
			Status: "pending", PaymentStatus: order.PaymentPaid, Currency: "CNY", TotalAmount: 100,
		})
	}
	if err := db.CreateInBatches(&orders, 200).Error; err != nil {
		t.Fatal(err)
	}
	r, _ := reports.ResolveRange(30, "", "")

	page, err := svc.ProfitReport(reportsTestCtx(1, nil), reports.DimensionOrder, r)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) != 500 || !page.Truncated {
		t.Fatalf("page cap: rows=%d truncated=%v", len(page.Rows), page.Truncated)
	}
	if page.Summary.OrderCount != n {
		t.Fatalf("summary should cover all orders: %d", page.Summary.OrderCount)
	}

	data, _, err := svc.ExportProfitCSV(reportsTestCtx(1, nil), reports.DimensionOrder, r)
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
	}
	if len(seen) != n {
		t.Fatalf("distinct orders in csv: %d", len(seen))
	}
}

func TestProcurementReport(t *testing.T) {
	db := openReportsTestDB(t)
	svc := newReportsService(db, &fakeSettings{})
	sup1, sup2 := uuid.New(), uuid.New()

	mkPO := func(supID uuid.UUID, supName, status string, amount float64, createdDaysAgo int) *procurement.PurchaseOrder {
		po := &procurement.PurchaseOrder{TenantID: 1, SupplierID: supID, SupplierName: supName, Status: status, TotalAmount: amount, Currency: "CNY", IdempotencyKey: uuid.NewString()}
		mustCreate(t, db, po)
		if createdDaysAgo != 0 {
			db.Model(po).Update("created_at", time.Now().AddDate(0, 0, -createdDaysAgo))
			po.CreatedAt = time.Now().AddDate(0, 0, -createdDaysAgo)
		}
		return po
	}

	delivered := mkPO(sup1, "供应商甲", procurement.StatusDelivered, 300, 10)
	mustCreate(t, db, &procurement.PurchaseOrderEvent{TenantID: 1, PurchaseOrderID: delivered.ID, ToStatus: procurement.StatusDelivered, Source: "manual", CreatedAt: delivered.CreatedAt.AddDate(0, 0, 5)})
	mkPO(sup1, "供应商甲", procurement.StatusShipped, 200, 3)
	mkPO(sup2, "供应商乙", procurement.StatusCancelled, 150, 2)
	mkPO(sup2, "供应商乙", procurement.StatusVoided, 999, 2)
	// other tenant
	other := &procurement.PurchaseOrder{TenantID: 2, SupplierID: sup2, SupplierName: "他租户", Status: procurement.StatusPaid, TotalAmount: 888, Currency: "CNY"}
	mustCreate(t, db, other)

	r, _ := reports.ResolveRange(30, "", "")
	res, err := svc.ProcurementReport(reportsTestCtx(1, nil), r)
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.POCount != 3 { // voided excluded
		t.Fatalf("poCount: %d", res.Summary.POCount)
	}
	if res.Summary.TotalAmount != 500 { // cancelled excluded from amount
		t.Fatalf("totalAmount: %v", res.Summary.TotalAmount)
	}
	if res.Summary.InTransitCount != 1 || res.Summary.DeliveredCount != 1 || res.Summary.CancelledCount != 1 {
		t.Fatalf("status counts: %+v", res.Summary)
	}
	if res.Summary.AvgLeadTimeDays == nil || *res.Summary.AvgLeadTimeDays != 5 {
		t.Fatalf("avg lead time: %v", res.Summary.AvgLeadTimeDays)
	}
	var bucketTotal int64
	for _, b := range res.LeadTime {
		bucketTotal += b.Count
	}
	if bucketTotal != 1 {
		t.Fatalf("lead time buckets: %+v", res.LeadTime)
	}
	if len(res.Suppliers) != 2 || res.Suppliers[0].SupplierName != "供应商甲" || res.Suppliers[0].Amount != 500 {
		t.Fatalf("suppliers: %+v", res.Suppliers)
	}
	if len(res.Daily) != 30 {
		t.Fatalf("daily length: %d", len(res.Daily))
	}
}

func TestInventoryReport(t *testing.T) {
	db := openReportsTestDB(t)
	svc := newReportsService(db, &fakeSettings{})

	p := &product.Product{TenantID: 1, Source: "1688", Title: "库存商品", Status: "draft"}
	mustCreate(t, db, p)
	stockA, stockB, stockZero := 100, 3, 0
	cost := 10.0
	skuA := &product.ProductSKU{ProductID: p.ID, SKUCode: "A", Stock: &stockA, WarningStock: 5, CostPrice: &cost}
	skuB := &product.ProductSKU{ProductID: p.ID, SKUCode: "B", Stock: &stockB, WarningStock: 5}
	skuC := &product.ProductSKU{ProductID: p.ID, SKUCode: "C", Stock: &stockZero, WarningStock: 5}
	mustCreate(t, db, skuA)
	mustCreate(t, db, skuB)
	mustCreate(t, db, skuC)

	// primary source price for skuB (overrides missing cost price)
	sup := &sourcing.Supplier{TenantID: 1, Platform: "1688", Name: "供应商甲"}
	mustCreate(t, db, sup)
	src := &sourcing.ProductSource{TenantID: 1, ProductID: p.ID, SupplierID: sup.ID, IsPrimary: true, Status: "active"}
	mustCreate(t, db, src)
	bPrice := 20.0
	mustCreate(t, db, &sourcing.ProductSourceSKU{TenantID: 1, ProductSourceID: src.ID, LocalSKUID: skuB.ID, CurrentPrice: &bPrice, Currency: "CNY"})

	// recent outbound for skuA (not slow-moving); skuB has no outbound (slow)
	mustCreate(t, db, &inventory.InventoryChangeLog{TenantID: 1, ProductID: p.ID, ProductSKUID: skuA.ID, ChangeType: inventory.ChangeOrderDeduct, BeforeStock: 103, AfterStock: 100, Delta: -3, BusinessEventKey: "k1"})

	// other tenant product must not leak
	p2 := &product.Product{TenantID: 2, Source: "1688", Title: "他租户", Status: "draft"}
	mustCreate(t, db, p2)
	mustCreate(t, db, &product.ProductSKU{ProductID: p2.ID, SKUCode: "X", Stock: &stockA})

	res, err := svc.InventoryReport(reportsTestCtx(1, nil), 30, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.SKUCount != 3 || res.Summary.TotalStock != 103 {
		t.Fatalf("sku/stock: %+v", res.Summary)
	}
	// value: skuA 100×10 + skuB 3×20 = 1060; skuC unvalued
	if res.Summary.StockValueCNY != 1060 || res.Summary.ValuedSKUCount != 2 || res.Summary.UnvaluedSKUCount != 1 {
		t.Fatalf("stock value: %+v", res.Summary)
	}
	if res.Summary.LowStockCount != 1 || res.Summary.OutOfStockCount != 1 {
		t.Fatalf("low/out counts: %+v", res.Summary)
	}
	if len(res.LowStock) != 2 { // skuB (3<=5) and skuC (0<=5)
		t.Fatalf("lowStock rows: %d", len(res.LowStock))
	}
	// slow-moving: skuB only (skuA had recent outbound, skuC has no stock)
	if res.Summary.SlowMovingCount != 1 || len(res.SlowMoving) != 1 || res.SlowMoving[0].SKUCode != "B" {
		t.Fatalf("slowMoving: %+v", res.SlowMoving)
	}
	// turnover: outbound 3 over 30 days = 0.1/day → 103/0.1 = 1030 days
	if res.Summary.AvgDailyOutbound == nil || *res.Summary.AvgDailyOutbound != 0.1 {
		t.Fatalf("avgDailyOutbound: %v", res.Summary.AvgDailyOutbound)
	}
	if res.Summary.TurnoverDays == nil || *res.Summary.TurnoverDays != 1030 {
		t.Fatalf("turnoverDays: %v", res.Summary.TurnoverDays)
	}
}
