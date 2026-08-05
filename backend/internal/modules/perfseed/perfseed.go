// Package perfseed seeds a large PERF- prefixed load-test dataset (万级订单/
// 采购/库存流水/自动化日志/回款费用) for performance auditing. It is fully
// separate from the demo seed (DEMO-): rows carry the PERF- prefix and are
// removed via the shared prefix cleanup, leaving zero residue.
package perfseed

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/demoseed"
	"github.com/trademind-ai/trademind/backend/internal/modules/finance"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/selection"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
	"gorm.io/gorm"
)

// PerfPrefix marks every seeded row so cleanup can target perf data only.
const PerfPrefix = "PERF-"

const insertBatch = 500

// Scale controls generated row counts. Zero fields fall back to DefaultScale.
type Scale struct {
	Shops              int
	Suppliers          int
	Products           int
	SKUsPerProduct     int
	PriceHistPerSKU    int
	Orders             int
	ItemsPerOrder      int
	PurchaseOrders     int
	InventoryLogs      int
	InventorySyncTasks int
	AutomationRules    int
	AutomationLogs     int
	PaymentRecords     int
	OrderExpenses      int
	SelectionCands     int
}

// DefaultScale is the万级 load profile used by the perf audit.
func DefaultScale() Scale {
	return Scale{
		Shops:              10,
		Suppliers:          50,
		Products:           2000,
		SKUsPerProduct:     2,
		PriceHistPerSKU:    5,
		Orders:             10000,
		ItemsPerOrder:      2,
		PurchaseOrders:     5000,
		InventoryLogs:      30000,
		InventorySyncTasks: 3000,
		AutomationRules:    8,
		AutomationLogs:     20000,
		PaymentRecords:     12000,
		OrderExpenses:      20000,
		SelectionCands:     5000,
	}
}

// Result reports per-table inserted row counts.
type Result struct {
	Action  string           `json:"action"`
	Prefix  string           `json:"prefix"`
	Tenant  int64            `json:"tenant"`
	Seconds float64          `json:"seconds"`
	Counts  map[string]int64 `json:"counts"`
}

// Seeder seeds / cleans the PERF- dataset for one tenant.
type Seeder struct {
	DB       *gorm.DB
	TenantID int64
	AppEnv   string
	Scale    Scale
}

func (s *Seeder) guard() error {
	if config.IsProduction(s.AppEnv) {
		return fmt.Errorf("perfseed refuses to run in production")
	}
	return nil
}

func (s *Seeder) demoSeeder() *demoseed.FullDemoSeeder {
	return &demoseed.FullDemoSeeder{DB: s.DB, TenantID: s.TenantID, AppEnv: s.AppEnv, Prefix: PerfPrefix}
}

// Cleanup removes all PERF- rows via the shared prefix cleanup.
func (s *Seeder) Cleanup(ctx context.Context) (*demoseed.FullDemoResult, error) {
	if err := s.guard(); err != nil {
		return nil, err
	}
	return s.demoSeeder().Cleanup(ctx)
}

// VerifyClean counts residual PERF- rows (all must be zero).
func (s *Seeder) VerifyClean(ctx context.Context) (*demoseed.FullDemoResult, error) {
	if err := s.guard(); err != nil {
		return nil, err
	}
	return s.demoSeeder().VerifyClean(ctx)
}

// Seed removes prior PERF- rows then inserts the full load dataset.
func (s *Seeder) Seed(ctx context.Context) (*Result, error) {
	if err := s.guard(); err != nil {
		return nil, err
	}
	if _, err := s.Cleanup(ctx); err != nil {
		return nil, fmt.Errorf("perfseed pre-clean: %w", err)
	}
	sc := s.Scale
	if sc.Orders == 0 {
		sc = DefaultScale()
	}
	start := time.Now()
	res := &Result{Action: "seed", Prefix: PerfPrefix, Tenant: s.TenantID, Counts: map[string]int64{}}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.seedAll(tx, sc, res)
	})
	if err != nil {
		return nil, err
	}
	res.Seconds = time.Since(start).Seconds()
	return res, nil
}

func batchInsert[T any](tx *gorm.DB, res *Result, table string, rows []T) error {
	if len(rows) == 0 {
		return nil
	}
	if err := tx.CreateInBatches(rows, insertBatch).Error; err != nil {
		return fmt.Errorf("perfseed insert %s: %w", table, err)
	}
	res.Counts[table] += int64(len(rows))
	return nil
}

func fptr(v float64) *float64 { return &v }
func iptr(v int) *int         { return &v }

func (s *Seeder) seedAll(tx *gorm.DB, sc Scale, res *Result) error {
	rng := rand.New(rand.NewSource(122)) // deterministic dataset
	now := time.Now().UTC().Truncate(time.Hour)
	tid := s.TenantID
	daysBack := func(maxDays int) time.Time {
		return now.Add(-time.Duration(rng.Intn(maxDays*24)) * time.Hour)
	}

	// ---- shops ----
	platforms := []string{"douyin", "tiktok", "shopee"}
	currencies := map[string]string{"douyin": "CNY", "tiktok": "USD", "shopee": "SGD"}
	shops := make([]shop.Shop, sc.Shops)
	for i := range shops {
		p := platforms[i%len(platforms)]
		shops[i] = shop.Shop{
			TenantID: tid, Platform: p,
			ShopName: fmt.Sprintf("%sShop-%02d", PerfPrefix, i+1),
			ShopCode: fmt.Sprintf("%sSHOP-%02d", PerfPrefix, i+1),
			Status:   "active", AuthStatus: "authorized", Currency: currencies[p],
		}
	}
	if err := batchInsert(tx, res, "shops", shops); err != nil {
		return err
	}

	// ---- suppliers ----
	sups := make([]sourcing.Supplier, sc.Suppliers)
	for i := range sups {
		sups[i] = sourcing.Supplier{
			TenantID: tid, Platform: "1688",
			Name: fmt.Sprintf("%sSupplier-%03d", PerfPrefix, i+1), Status: "active",
		}
	}
	if err := batchInsert(tx, res, "suppliers", sups); err != nil {
		return err
	}

	// ---- products + SKUs ----
	prods := make([]product.Product, sc.Products)
	for i := range prods {
		prods[i] = product.Product{
			TenantID: tid, Source: "1688",
			SourceURL:     fmt.Sprintf("https://detail.1688.com/offer/perf%d.html", i+1),
			OriginalTitle: fmt.Sprintf("%s商品-%04d", PerfPrefix, i+1),
			Title:         fmt.Sprintf("%s商品-%04d", PerfPrefix, i+1),
			Currency:      "CNY", Status: product.StatusReady,
		}
		prods[i].CreatedAt = daysBack(180)
	}
	if err := batchInsert(tx, res, "products", prods); err != nil {
		return err
	}
	skus := make([]product.ProductSKU, 0, sc.Products*sc.SKUsPerProduct)
	for i := range prods {
		for k := 0; k < sc.SKUsPerProduct; k++ {
			stock := rng.Intn(500)
			skus = append(skus, product.ProductSKU{
				ProductID: prods[i].ID,
				SKUCode:   fmt.Sprintf("%sSKU-%04d-%d", PerfPrefix, i+1, k+1),
				SKUName:   fmt.Sprintf("规格%d", k+1),
				Price:     fptr(float64(rng.Intn(9000)+1000) / 100),
				CostPrice: fptr(float64(rng.Intn(4000)+500) / 100),
				Stock:     iptr(stock),
			})
		}
	}
	if err := batchInsert(tx, res, "product_skus", skus); err != nil {
		return err
	}

	// ---- sources + source SKUs + price history ----
	srcs := make([]sourcing.ProductSource, sc.Products)
	for i := range prods {
		srcs[i] = sourcing.ProductSource{
			TenantID: tid, ProductID: prods[i].ID, SupplierID: sups[i%len(sups)].ID,
			SourceOfferID: fmt.Sprintf("perf-offer-%d", i+1),
			Priority:      100, IsPrimary: true, Status: "active",
		}
	}
	if err := batchInsert(tx, res, "product_sources", srcs); err != nil {
		return err
	}
	srcSKUs := make([]sourcing.ProductSourceSKU, 0, len(skus))
	for i := range prods {
		for k := 0; k < sc.SKUsPerProduct; k++ {
			idx := i*sc.SKUsPerProduct + k
			row := sourcing.ProductSourceSKU{
				TenantID: tid, ProductSourceID: srcs[i].ID, LocalSKUID: skus[idx].ID,
				ExternalSKUID: fmt.Sprintf("perf-ext-%d", idx+1), Currency: "CNY", Status: "active",
			}
			if idx%2 == 0 { // half rely on price history fallback
				row.CurrentPrice = fptr(float64(rng.Intn(4000)+500) / 100)
			}
			srcSKUs = append(srcSKUs, row)
		}
	}
	if err := batchInsert(tx, res, "product_source_skus", srcSKUs); err != nil {
		return err
	}
	hist := make([]sourcing.SourcePriceHistory, 0, len(srcSKUs)*sc.PriceHistPerSKU)
	for i := range srcSKUs {
		for h := 0; h < sc.PriceHistPerSKU; h++ {
			hist = append(hist, sourcing.SourcePriceHistory{
				TenantID: tid, SourceSKUID: srcSKUs[i].ID,
				Price:         float64(rng.Intn(4000)+500) / 100,
				Stock:         iptr(rng.Intn(1000)),
				CapturedAt:    now.Add(-time.Duration(h*24+rng.Intn(24)) * time.Hour),
				CaptureSource: "collect",
			})
		}
	}
	if err := batchInsert(tx, res, "source_price_history", hist); err != nil {
		return err
	}

	// ---- orders + items + SKU matches ----
	statuses := []string{order.StatusPending, order.StatusProcessing, order.StatusShipped, order.StatusCancelled}
	orders := make([]order.Order, sc.Orders)
	for i := range orders {
		sh := shops[i%len(shops)]
		st := statuses[rng.Intn(len(statuses))]
		payment := order.PaymentUnpaid
		var paidAt *time.Time
		if rng.Intn(10) < 7 {
			payment = order.PaymentPaid
			t := daysBack(90)
			paidAt = &t
		}
		created := daysBack(90)
		orders[i] = order.Order{
			TenantID: tid, Platform: sh.Platform, ShopID: &sh.ID,
			OrderNo:      fmt.Sprintf("%sO-%06d", PerfPrefix, i+1),
			CustomerName: fmt.Sprintf("%sCustomer-%04d", PerfPrefix, rng.Intn(3000)+1),
			Status:       st, PaymentStatus: payment, FulfillmentStatus: "unfulfilled",
			Currency: sh.Currency, TotalAmount: float64(rng.Intn(50000)+500) / 100,
			PaidAt: paidAt,
		}
		orders[i].CreatedAt = created
		if i%17 == 0 {
			orders[i].ReviewStatus = order.ReviewStatusPending
		}
	}
	if err := batchInsert(tx, res, "orders", orders); err != nil {
		return err
	}
	items := make([]order.OrderItem, 0, sc.Orders*sc.ItemsPerOrder)
	matches := make([]order.OrderItemSKUMatch, 0, sc.Orders*sc.ItemsPerOrder/2)
	for i := range orders {
		for k := 0; k < sc.ItemsPerOrder; k++ {
			pIdx := rng.Intn(len(prods))
			skuIdx := pIdx*sc.SKUsPerProduct + rng.Intn(sc.SKUsPerProduct)
			it := order.OrderItem{
				OrderID:      orders[i].ID,
				ProductTitle: prods[pIdx].Title,
				SKUName:      skus[skuIdx].SKUName,
				Quantity:     rng.Intn(5) + 1,
				UnitPrice:    *skus[skuIdx].Price,
			}
			it.TotalPrice = it.UnitPrice * float64(it.Quantity)
			matched := (i*sc.ItemsPerOrder+k)%2 == 0
			if matched { // half matched to local SKUs
				it.ProductID = &prods[pIdx].ID
				it.ProductSKUID = &skus[skuIdx].ID
			}
			items = append(items, it)
		}
	}
	if err := batchInsert(tx, res, "order_items", items); err != nil {
		return err
	}
	for idx := range items {
		if items[idx].ProductSKUID == nil {
			continue
		}
		matches = append(matches, order.OrderItemSKUMatch{
			OrderID: items[idx].OrderID, OrderItemID: items[idx].ID,
			Platform:  "perf",
			ProductID: items[idx].ProductID, ProductSKUID: items[idx].ProductSKUID,
			MatchType: "auto", MatchStatus: "matched", Confidence: 95,
		})
	}
	if err := batchInsert(tx, res, "order_item_sku_matches", matches); err != nil {
		return err
	}

	// ---- purchase orders + items + events ----
	poStatuses := []string{procurement.StatusDraft, procurement.StatusPendingConfirm, procurement.StatusPlaced, procurement.StatusPaid, procurement.StatusShipped, procurement.StatusDelivered}
	pos := make([]procurement.PurchaseOrder, sc.PurchaseOrders)
	for i := range pos {
		pos[i] = procurement.PurchaseOrder{
			TenantID: tid, SupplierID: sups[i%len(sups)].ID,
			SupplierName: sups[i%len(sups)].Name, SourcePlatform: "1688",
			Status:      poStatuses[rng.Intn(len(poStatuses))],
			TotalAmount: float64(rng.Intn(80000)+1000) / 100, Currency: "CNY",
			PayStatus:      procurement.PayStatusUnpaid,
			IdempotencyKey: fmt.Sprintf("%sPO-%06d", PerfPrefix, i+1),
		}
		pos[i].CreatedAt = daysBack(90)
	}
	if err := batchInsert(tx, res, "purchase_orders", pos); err != nil {
		return err
	}
	poItems := make([]procurement.PurchaseOrderItem, 0, sc.PurchaseOrders*2)
	poEvents := make([]procurement.PurchaseOrderEvent, 0, sc.PurchaseOrders*3)
	for i := range pos {
		for k := 0; k < 2; k++ {
			pIdx := rng.Intn(len(prods))
			skuIdx := pIdx*sc.SKUsPerProduct + rng.Intn(sc.SKUsPerProduct)
			salesID := orders[rng.Intn(len(orders))].ID
			poItems = append(poItems, procurement.PurchaseOrderItem{
				TenantID: tid, PurchaseOrderID: pos[i].ID, SalesOrderID: &salesID,
				LocalSKUID: skus[skuIdx].ID, SourceSKUID: srcSKUs[skuIdx].ID,
				ProductTitle: prods[pIdx].Title, Quantity: rng.Intn(10) + 1,
				ExpectedPrice: fptr(float64(rng.Intn(4000)+500) / 100),
			})
		}
		prev := ""
		for _, st := range []string{procurement.StatusDraft, procurement.StatusPendingConfirm, pos[i].Status} {
			poEvents = append(poEvents, procurement.PurchaseOrderEvent{
				TenantID: tid, PurchaseOrderID: pos[i].ID,
				FromStatus: prev, ToStatus: st, Source: procurement.EventSourceSystem,
				CreatedAt: pos[i].CreatedAt.Add(time.Duration(len(poEvents)%48) * time.Minute),
			})
			prev = st
		}
	}
	if err := batchInsert(tx, res, "purchase_order_items", poItems); err != nil {
		return err
	}
	if err := batchInsert(tx, res, "purchase_order_events", poEvents); err != nil {
		return err
	}

	// ---- inventory change logs ----
	changeTypes := []string{"order_deduct", "order_restore", "manual_adjust", "purchase_inbound"}
	invLogs := make([]inventory.InventoryChangeLog, sc.InventoryLogs)
	for i := range invLogs {
		pIdx := rng.Intn(len(prods))
		skuIdx := pIdx*sc.SKUsPerProduct + rng.Intn(sc.SKUsPerProduct)
		before := rng.Intn(500)
		delta := rng.Intn(21) - 10
		invLogs[i] = inventory.InventoryChangeLog{
			TenantID: tid, ProductID: prods[pIdx].ID, ProductSKUID: skus[skuIdx].ID,
			ChangeType:  changeTypes[rng.Intn(len(changeTypes))],
			BeforeStock: before, AfterStock: before + delta, Delta: delta,
			Reason:           "perf-seed",
			BusinessEventKey: fmt.Sprintf("%sINV-%06d", PerfPrefix, i+1),
		}
		invLogs[i].CreatedAt = daysBack(90)
	}
	if err := batchInsert(tx, res, "inventory_change_logs", invLogs); err != nil {
		return err
	}

	// ---- inventory sync tasks ----
	taskStatuses := []string{"pending", "success", "failed"}
	invTasks := make([]inventory.InventorySyncTask, sc.InventorySyncTasks)
	for i := range invTasks {
		pIdx := rng.Intn(len(prods))
		skuIdx := pIdx*sc.SKUsPerProduct + rng.Intn(sc.SKUsPerProduct)
		sh := shops[i%len(shops)]
		invTasks[i] = inventory.InventorySyncTask{
			TenantID: tid, ProductID: prods[pIdx].ID, ProductSKUID: &skus[skuIdx].ID,
			ShopID: sh.ID, Platform: sh.Platform,
			TaskType: "stock_push", Status: taskStatuses[rng.Intn(len(taskStatuses))],
			Mode: "manual", TargetStock: rng.Intn(500),
		}
		invTasks[i].CreatedAt = daysBack(60)
	}
	if err := batchInsert(tx, res, "inventory_sync_tasks", invTasks); err != nil {
		return err
	}

	// ---- automation rules + logs ----
	rules := make([]order.OrderAutomationRule, sc.AutomationRules)
	events := order.ValidAutomationEvents()
	for i := range rules {
		ev := events[i%len(events)]
		action := ""
		switch ev {
		case order.AutomationEventOrderCreated:
			action = order.AutomationActionConfirmPayment
		case order.AutomationEventOrderPaid:
			action = order.AutomationActionGenerateProcurement
		default:
			action = order.AutomationActionNotifyShipping
		}
		rules[i] = order.OrderAutomationRule{
			TenantID: tid, Name: fmt.Sprintf("%sRule-%02d", PerfPrefix, i+1),
			Priority: i, Enabled: true, TriggerEvent: ev, Action: action,
			MaxAmount: fptr(1000),
		}
	}
	if err := batchInsert(tx, res, "order_automation_rules", rules); err != nil {
		return err
	}
	logStatuses := []string{order.AutomationLogSuccess, order.AutomationLogSkipped, order.AutomationLogFailed}
	autoLogs := make([]order.OrderAutomationLog, sc.AutomationLogs)
	for i := range autoLogs {
		o := orders[i%len(orders)]
		r := rules[(i/len(orders))%len(rules)]
		autoLogs[i] = order.OrderAutomationLog{
			TenantID: tid, RuleID: r.ID, RuleName: r.Name,
			OrderID: o.ID, OrderNo: o.OrderNo,
			TriggerEvent: r.TriggerEvent, Action: r.Action,
			Status: logStatuses[rng.Intn(len(logStatuses))],
			Reason: "perf-seed", Attempts: 1,
			DedupKey: fmt.Sprintf("%d:%s:%s:%s", tid, r.ID, o.ID, r.TriggerEvent),
		}
		autoLogs[i].CreatedAt = daysBack(60)
	}
	if err := batchInsert(tx, res, "order_automation_logs", autoLogs); err != nil {
		return err
	}

	// ---- finance: payments + order expenses + shop monthly ----
	pays := make([]finance.PaymentRecord, sc.PaymentRecords)
	for i := range pays {
		o := orders[i%len(orders)]
		pays[i] = finance.PaymentRecord{
			TenantID: tid, OrderID: o.ID, ShopID: o.ShopID,
			Amount: o.TotalAmount * (0.5 + rng.Float64()*0.5), Currency: o.Currency,
			FeeAmount: o.TotalAmount * 0.02, ReceivedAt: daysBack(60),
			Channel: "platform", Remark: fmt.Sprintf("%sPAY-%06d", PerfPrefix, i+1),
			Source: finance.SourceManual,
		}
	}
	if err := batchInsert(tx, res, "finance_payment_records", pays); err != nil {
		return err
	}
	expTypes := []string{"commission", "promotion", "shipping", "other"}
	exps := make([]finance.OrderExpense, sc.OrderExpenses)
	for i := range exps {
		o := orders[rng.Intn(len(orders))]
		t := daysBack(60)
		exps[i] = finance.OrderExpense{
			TenantID: tid, OrderID: o.ID, ShopID: o.ShopID,
			TypeCode: expTypes[rng.Intn(len(expTypes))],
			Amount:   float64(rng.Intn(2000)+10) / 100, Currency: o.Currency,
			IncurredAt: &t, Remark: fmt.Sprintf("%sEXP-%06d", PerfPrefix, i+1),
		}
	}
	if err := batchInsert(tx, res, "finance_order_expenses", exps); err != nil {
		return err
	}
	var monthly []finance.ShopMonthlyExpense
	for i := range shops {
		for m := 0; m < 12; m++ {
			monthly = append(monthly, finance.ShopMonthlyExpense{
				TenantID: tid, ShopID: shops[i].ID,
				Month:    now.AddDate(0, -m, 0).Format("2006-01"),
				TypeCode: "rent", Amount: float64(rng.Intn(50000)+5000) / 100,
				Currency: shops[i].Currency,
				Remark:   fmt.Sprintf("%sMONTHLY-%02d-%02d", PerfPrefix, i+1, m+1),
			})
		}
	}
	if err := batchInsert(tx, res, "finance_shop_monthly_expenses", monthly); err != nil {
		return err
	}

	// ---- selection task + candidates + matches + evaluations ----
	task := selection.SelectionTask{
		TenantID: tid, Name: fmt.Sprintf("%sSelection-Task", PerfPrefix),
		TargetPlatform: "tiktok", TargetCountry: "US", Status: "success",
	}
	if err := tx.Create(&task).Error; err != nil {
		return fmt.Errorf("perfseed insert selection_tasks: %w", err)
	}
	res.Counts["selection_tasks"]++
	cands := make([]selection.SelectionCandidate, sc.SelectionCands)
	for i := range cands {
		cands[i] = selection.SelectionCandidate{
			TenantID: tid, TaskID: task.ID,
			Title:          fmt.Sprintf("%sCandidate-%04d", PerfPrefix, i+1),
			MarketPlatform: "tiktok",
			MarketPrice:    fptr(float64(rng.Intn(9000)+1000) / 100),
			MarketCurrency: "USD", MarketSales30d: iptr(rng.Intn(5000)),
			Status: "evaluated",
		}
	}
	if err := batchInsert(tx, res, "selection_candidates", cands); err != nil {
		return err
	}
	sMatches := make([]selection.SelectionSourceMatch, len(cands))
	evals := make([]selection.SelectionEvaluation, len(cands))
	for i := range cands {
		sim := 0.5 + rng.Float64()*0.5
		sMatches[i] = selection.SelectionSourceMatch{
			TenantID: tid, CandidateID: cands[i].ID, SourcePlatform: "1688",
			SourceOfferID: fmt.Sprintf("perf-sel-%d", i+1), MatchMethod: "title",
			Similarity: &sim,
		}
		score := rng.Float64() * 100
		evals[i] = selection.SelectionEvaluation{
			TenantID: tid, CandidateID: cands[i].ID,
			PurchaseCost: fptr(float64(rng.Intn(4000)+500) / 100),
			EstProfit:    fptr(float64(rng.Intn(3000)) / 100),
			AIScore:      &score, Decision: "pending",
		}
	}
	if err := batchInsert(tx, res, "selection_source_matches", sMatches); err != nil {
		return err
	}
	return batchInsert(tx, res, "selection_evaluations", evals)
}
