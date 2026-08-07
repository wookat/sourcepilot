package demoseed

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/bannedwords"
	"github.com/trademind-ai/trademind/backend/internal/modules/carrier"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectrule"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/finance"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/selection"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
	"github.com/trademind-ai/trademind/backend/internal/modules/waybill"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// DemoPrefix marks every seeded row so cleanup can target demo data only.
const DemoPrefix = "DEMO-"

// demoImageDataURI returns an inline SVG placeholder (neutral background +
// "DEMO-n" label) so demo product images render offline without external
// image requests. Loading paths that reject data: URLs (e.g. Douyin image
// upload) skip these the same way they skip other non-fetchable sources.
func demoImageDataURI(n int) string {
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200" viewBox="0 0 200 200"><rect width="200" height="200" fill="#f0f2f5"/><rect x="40" y="50" width="120" height="90" rx="8" fill="none" stroke="#bfbfbf" stroke-width="4"/><circle cx="70" cy="80" r="9" fill="#bfbfbf"/><path d="M52 128l30-30 22 22 22-26 22 34" fill="none" stroke="#bfbfbf" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"/><text x="100" y="172" text-anchor="middle" font-family="sans-serif" font-size="20" fill="#8c8c8c">DEMO-%d</text></svg>`, n)
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}

// cleanPrefixPattern constrains custom cleanup prefixes to a safe shape:
// alphanumeric segments ending with "-", no SQL LIKE wildcards (% _).
var cleanPrefixPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,30}-$`)

// ValidateCleanPrefix checks a custom cleanup/verify prefix (e.g. "QA-").
func ValidateCleanPrefix(prefix string) error {
	if !cleanPrefixPattern.MatchString(prefix) {
		return fmt.Errorf("demoseed: invalid prefix %q (want 2-32 chars of [A-Za-z0-9-] ending with %q, e.g. %q)", prefix, "-", "QA-")
	}
	return nil
}

// FullDemoSeeder seeds a coherent cross-module demo dataset for one tenant.
// Seed is idempotent: it always removes previously seeded DEMO- data first,
// then inserts a fresh, internally consistent dataset.
type FullDemoSeeder struct {
	DB       *gorm.DB
	TenantID int64
	AppEnv   string
	// Prefix optionally overrides the row prefix targeted by Cleanup and
	// VerifyClean (default DemoPrefix). Seed always writes DemoPrefix rows.
	Prefix string
}

// mustJSONStrings marshals a string list into a JSON column value.
func mustJSONStrings(items ...string) datatypes.JSON {
	b, _ := json.Marshal(items)
	return datatypes.JSON(b)
}

// cleanPrefix returns the prefix targeted by Cleanup/VerifyClean.
func (s *FullDemoSeeder) cleanPrefix() string {
	if p := strings.TrimSpace(s.Prefix); p != "" {
		return p
	}
	return DemoPrefix
}

// FullDemoResult reports per-table row counts for seed / cleanup / verify.
// SoftDeleted (verify only) lists prefixed rows that are soft-deleted
// (deleted_at set): they are historical residue invisible to the app and are
// reported separately instead of being counted as live residual rows.
type FullDemoResult struct {
	Action      string           `json:"action"`
	Counts      map[string]int64 `json:"counts"`
	SoftDeleted map[string]int64 `json:"softDeleted,omitempty"`
}

// verifyCheck is one residual-row counter; softDeleted is set only for
// soft-delete models to report deleted_at-marked residue separately.
type verifyCheck struct {
	table       string
	count       func() (int64, error)
	softDeleted func() (int64, error)
}

// splitSoftDeleted builds live + soft-deleted residual counters for a
// soft-delete model; base must apply Model + Unscoped + the prefix filter.
func splitSoftDeleted(table string, base func() *gorm.DB) verifyCheck {
	return verifyCheck{
		table: table,
		count: func() (int64, error) {
			var n int64
			return n, base().Where(table + ".deleted_at IS NULL").Count(&n).Error
		},
		softDeleted: func() (int64, error) {
			var n int64
			return n, base().Where(table + ".deleted_at IS NOT NULL").Count(&n).Error
		},
	}
}

// purchaseOrderPlan describes one demo purchase order walked through the real
// procurement state machine (see statemachine.go).
type purchaseOrderPlan struct {
	suffix      string
	chain       []string // statuses after draft, applied in order
	payStatus   string
	externalID  string
	trackingNo  string
	withInbound bool
}

// demoPurchaseOrderPlans covers 草稿→已签收 (draft → delivered) plus failed/cancelled.
func demoPurchaseOrderPlans() []purchaseOrderPlan {
	return []purchaseOrderPlan{
		{suffix: "PO-DRAFT", chain: nil, payStatus: procurement.PayStatusUnpaid},
		{suffix: "PO-CONFIRM", chain: []string{procurement.StatusPendingConfirm}, payStatus: procurement.PayStatusUnpaid},
		{suffix: "PO-PLACING", chain: []string{procurement.StatusPendingConfirm, procurement.StatusPlacing}, payStatus: procurement.PayStatusUnpaid},
		{suffix: "PO-PLACED", chain: []string{procurement.StatusPendingConfirm, procurement.StatusPlacing, procurement.StatusPlaced}, payStatus: procurement.PayStatusUnpaid, externalID: "DEMO-1688-2001"},
		{suffix: "PO-PAID", chain: []string{procurement.StatusPendingConfirm, procurement.StatusPlacing, procurement.StatusPlaced, procurement.StatusPaid}, payStatus: procurement.PayStatusPaid, externalID: "DEMO-1688-2002"},
		{suffix: "PO-SHIPPED", chain: []string{procurement.StatusPendingConfirm, procurement.StatusPlacing, procurement.StatusPlaced, procurement.StatusPaid, procurement.StatusShipped}, payStatus: procurement.PayStatusPaid, externalID: "DEMO-1688-2003", trackingNo: "DEMO-TRK-PO-1"},
		{suffix: "PO-DELIVERED", chain: []string{procurement.StatusPendingConfirm, procurement.StatusPlacing, procurement.StatusPlaced, procurement.StatusPaid, procurement.StatusShipped, procurement.StatusDelivered}, payStatus: procurement.PayStatusPaid, externalID: "DEMO-1688-2004", trackingNo: "DEMO-TRK-PO-2", withInbound: true},
		{suffix: "PO-FAILED", chain: []string{procurement.StatusPendingConfirm, procurement.StatusPlacing, procurement.StatusFailed}, payStatus: procurement.PayStatusUnpaid},
		{suffix: "PO-CANCELLED", chain: []string{procurement.StatusCancelled}, payStatus: procurement.PayStatusUnpaid},
	}
}

// salesOrderPlan describes one demo sales order; lifecycle steps are validated
// with order.ValidateOrderStateTransition so no illegal state is produced.
type salesOrderPlan struct {
	suffix      string
	status      string
	payment     string
	fulfillment string
	// steps are intermediate lifecycle points from the initial state.
	steps         [][3]string
	withShipment  bool
	shipmentState string
	withDeduct    bool
	unmatchedItem bool
}

func demoSalesOrderPlans() []salesOrderPlan {
	return []salesOrderPlan{
		{suffix: "SO-PENDING", status: order.StatusPending, payment: order.PaymentUnpaid, fulfillment: order.FulfillmentUnfulfilled},
		{suffix: "SO-PAID", status: order.StatusPaid, payment: order.PaymentPaid, fulfillment: order.FulfillmentUnfulfilled,
			steps:      [][3]string{{order.StatusPaid, order.PaymentPaid, order.FulfillmentUnfulfilled}},
			withDeduct: true},
		{suffix: "SO-SHIPPED", status: order.StatusShipped, payment: order.PaymentPaid, fulfillment: order.FulfillmentFulfilled,
			steps: [][3]string{
				{order.StatusPaid, order.PaymentPaid, order.FulfillmentUnfulfilled},
				{order.StatusShipped, order.PaymentPaid, order.FulfillmentFulfilled},
			},
			withShipment: true, shipmentState: order.ShipmentInTransit},
		{suffix: "SO-DELIVERED", status: order.StatusDelivered, payment: order.PaymentPaid, fulfillment: order.FulfillmentFulfilled,
			steps: [][3]string{
				{order.StatusPaid, order.PaymentPaid, order.FulfillmentUnfulfilled},
				{order.StatusShipped, order.PaymentPaid, order.FulfillmentFulfilled},
				{order.StatusDelivered, order.PaymentPaid, order.FulfillmentFulfilled},
			},
			withShipment: true, shipmentState: order.ShipmentDelivered},
		{suffix: "SO-CANCELLED", status: order.StatusCancelled, payment: order.PaymentUnpaid, fulfillment: order.FulfillmentUnfulfilled,
			steps:         [][3]string{{order.StatusCancelled, order.PaymentUnpaid, order.FulfillmentUnfulfilled}},
			unmatchedItem: true},
	}
}

func validatePurchaseChain(plan purchaseOrderPlan) error {
	from := procurement.StatusDraft
	for _, to := range plan.chain {
		if !procurement.CanTransition(from, to) {
			return procurement.ErrIllegalTransition(from, to)
		}
		from = to
	}
	return nil
}

func validateSalesChain(plan salesOrderPlan) error {
	cur := [3]string{order.StatusPending, order.PaymentUnpaid, order.FulfillmentUnfulfilled}
	for _, next := range plan.steps {
		if !order.ValidateOrderStateTransition(cur[0], cur[1], cur[2], next[0], next[1], next[2]) {
			return fmt.Errorf("demoseed: illegal order transition %v -> %v", cur, next)
		}
		cur = next
	}
	if cur[0] != plan.status || cur[1] != plan.payment || cur[2] != plan.fulfillment {
		return fmt.Errorf("demoseed: order plan %s final state mismatch", plan.suffix)
	}
	return nil
}

// Seed removes previous DEMO- data then inserts a fresh demo dataset.
func (s *FullDemoSeeder) Seed(ctx context.Context) (*FullDemoResult, error) {
	if err := s.guard(); err != nil {
		return nil, err
	}
	if s.TenantID <= 0 {
		return nil, fmt.Errorf("demoseed: positive tenant id required (got %d): task workers reject tenant_id<=0 rows, so seeded publish/order demos would never run", s.TenantID)
	}
	if s.cleanPrefix() != DemoPrefix {
		return nil, fmt.Errorf("demoseed: seed only supports the %s prefix", DemoPrefix)
	}
	if _, err := s.Cleanup(ctx); err != nil {
		return nil, err
	}
	res := &FullDemoResult{Action: "seed", Counts: map[string]int64{}}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.seedAll(tx, res)
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *FullDemoSeeder) guard() error {
	if s == nil {
		return fmt.Errorf("demoseed: service unavailable")
	}
	if config.IsProduction(s.AppEnv) {
		return ErrProductionForbidden
	}
	if s.DB == nil {
		return fmt.Errorf("demoseed: service unavailable")
	}
	return nil
}

//nolint:gocyclo // linear dataset construction
func (s *FullDemoSeeder) seedAll(tx *gorm.DB, res *FullDemoResult) error {
	now := time.Now().UTC()
	count := func(table string, n int64) { res.Counts[table] += n }

	// ---- banned words：预置违禁词基础库（幂等，供合规检测演示）----
	if err := bannedwords.EnsurePresets(context.Background(), tx, s.TenantID); err != nil {
		return fmt.Errorf("demoseed: banned words: %w", err)
	}
	var bannedCount int64
	if err := tx.Model(&bannedwords.BannedWord{}).Where("tenant_id = ?", s.TenantID).Count(&bannedCount).Error; err != nil {
		return fmt.Errorf("demoseed: banned words count: %w", err)
	}
	count("banned_words", bannedCount)

	// ---- carriers：预置国内常用物流商（幂等，供发货/打印演示）----
	if err := carrier.EnsurePresets(context.Background(), tx, s.TenantID); err != nil {
		return fmt.Errorf("demoseed: carriers: %w", err)
	}
	var carrierCount int64
	if err := tx.Model(&carrier.Carrier{}).Where("tenant_id = ?", s.TenantID).Count(&carrierCount).Error; err != nil {
		return fmt.Errorf("demoseed: carriers count: %w", err)
	}
	count("carriers", carrierCount)

	// ---- waybill templates：预置三种尺寸打印模板（幂等，供打单演示）----
	if err := waybill.EnsureTemplatePresets(context.Background(), tx, s.TenantID); err != nil {
		return fmt.Errorf("demoseed: waybill templates: %w", err)
	}
	var waybillTplCount int64
	if err := tx.Model(&waybill.Template{}).Where("tenant_id = ?", s.TenantID).Count(&waybillTplCount).Error; err != nil {
		return fmt.Errorf("demoseed: waybill templates count: %w", err)
	}
	count("waybill_templates", waybillTplCount)

	// ---- shipping rules：DEMO- 发货规则样本（省份/平台/金额段 → 物流商）----
	minHeavy, maxStd := 5.0, 5.0
	minAmt := 500.0
	shippingRules := []waybill.ShippingRule{
		{TenantID: s.TenantID, Name: "DEMO-江浙沪标准件走中通", Priority: 10, Enabled: true,
			Provinces: mustJSONStrings("上海", "江苏", "浙江"), MaxWeightKg: &maxStd, CarrierCode: "zto"},
		{TenantID: s.TenantID, Name: "DEMO-高客单价订单走顺丰", Priority: 20, Enabled: true,
			MinAmount: &minAmt, CarrierCode: "sf"},
		{TenantID: s.TenantID, Name: "DEMO-重货走德邦", Priority: 30, Enabled: true,
			MinWeightKg: &minHeavy, CarrierCode: "deppon"},
		{TenantID: s.TenantID, Name: "DEMO-抖店订单默认韵达", Priority: 40, Enabled: true,
			Platforms: mustJSONStrings("douyin_shop"), CarrierCode: "yunda"},
	}
	for i := range shippingRules {
		if err := tx.Create(&shippingRules[i]).Error; err != nil {
			return fmt.Errorf("demoseed: shipping rule: %w", err)
		}
	}
	count("shipping_rules", int64(len(shippingRules)))
	var sfCarrier carrier.Carrier
	if err := tx.First(&sfCarrier, "tenant_id = ? AND code = ?", s.TenantID, "sf").Error; err != nil {
		return fmt.Errorf("demoseed: sf carrier: %w", err)
	}

	// ---- shops ×4（抖店 / 手工渠道 / TikTok / 虾皮降级刊登演示）----
	shops := []shop.Shop{
		{TenantID: s.TenantID, Platform: "douyin_shop", ShopName: "DEMO-抖店旗舰店", ShopCode: "DEMO-SHOP-1",
			Status: "active", AuthStatus: "authorized", Region: "CN", Currency: "CNY", Remark: "DEMO- 演示店铺（种子数据）"},
		{TenantID: s.TenantID, Platform: "manual", ShopName: "DEMO-手工渠道店", ShopCode: "DEMO-SHOP-2",
			Status: "active", AuthStatus: "authorized", Currency: "CNY", Remark: "DEMO- 演示店铺（种子数据）"},
		{TenantID: s.TenantID, Platform: "tiktok", ShopName: "DEMO-TikTok 演示店", ShopCode: "DEMO-SHOP-3",
			Status: "active", AuthStatus: "authorized", Region: "SG", Currency: "USD", Remark: "DEMO- 演示店铺（种子数据）"},
		{TenantID: s.TenantID, Platform: "shopee", ShopName: "DEMO-虾皮演示店", ShopCode: "DEMO-SHOP-4",
			Status: "active", AuthStatus: "authorized", Region: "SG", Currency: "SGD", Remark: "DEMO- 演示店铺（种子数据）"},
	}
	for i := range shops {
		if err := tx.Create(&shops[i]).Error; err != nil {
			return fmt.Errorf("demoseed: shop: %w", err)
		}
	}
	count("shops", int64(len(shops)))

	// ---- demo RBAC accounts (admin / operator / readonly): guarantee they
	// exist with the documented passwords on every platform (the PowerShell
	// seed script is not required) and reset drifted passwords idempotently.
	if err := s.ensureDemoAccounts(tx, count); err != nil {
		return err
	}

	// ---- operator / readonly demo accounts: grant the manual DEMO shop so
	// scoped positive paths are testable out of the box (readonly gets view
	// scope to exercise the read-visible / write-denied permission boundary).
	// The douyin DEMO shop stays ungranted as the denied sample, matching the
	// seed-demo-permissions convention. Only users with zero existing store
	// grants are touched; real scope configuration is never modified.
	var operators []admin.AdminUser
	if err := tx.Where("tenant_id = ? AND role IN ?", s.TenantID, []string{"operator", "readonly"}).Find(&operators).Error; err != nil {
		return fmt.Errorf("demoseed: list operators: %w", err)
	}
	for _, op := range operators {
		var n int64
		if err := tx.Model(&admin.UserStorePermission{}).Where("user_id = ?", op.ID).Count(&n).Error; err != nil {
			return fmt.Errorf("demoseed: count store grants: %w", err)
		}
		if n > 0 {
			continue
		}
		scope := admin.StorePermScopeOperate
		if strings.EqualFold(op.Role, "readonly") {
			scope = admin.StorePermScopeView
		}
		grant := admin.UserStorePermission{UserID: op.ID, StoreID: shops[1].ID,
			Platform: shops[1].Platform, PermissionScope: scope}
		if err := tx.Create(&grant).Error; err != nil {
			return fmt.Errorf("demoseed: operator store grant: %w", err)
		}
		count("user_store_permissions", 1)
	}

	// ---- product drafts（采集来源 + 手动来源，AI 优化前后文案）----
	type productSpec struct {
		source, status, title, aiTitle, origTitle, desc, aiDesc, url string
	}
	specs := []productSpec{
		{source: "collect", status: product.StatusReady,
			origTitle: "DEMO-1688原始标题 陶瓷马克杯 350ml 批发",
			title:     "DEMO-北欧风陶瓷马克杯 350ml 简约咖啡杯",
			aiTitle:   "DEMO-AI优化 | Nordic Ceramic Mug 350ml Minimalist Coffee Cup",
			desc:      "DEMO-采集原始描述：陶瓷马克杯，350ml，多色可选。",
			aiDesc:    "DEMO-AI 生成描述：北欧极简设计陶瓷马克杯，350ml 黄金容量，釉面细腻，适合咖啡/茶/牛奶，支持洗碗机清洗。",
			url:       "https://detail.1688.com/offer/DEMO-810001.html"},
		{source: "collect", status: product.StatusPublished,
			origTitle: "DEMO-1688原始标题 不锈钢保温杯 500ml",
			title:     "DEMO-便携不锈钢保温杯 500ml 车载水杯",
			aiTitle:   "DEMO-AI优化 | Portable Stainless Steel Thermos 500ml",
			desc:      "DEMO-采集原始描述：304不锈钢保温杯。",
			aiDesc:    "DEMO-AI 生成描述：304 食品级不锈钢真空保温杯，12 小时长效保温，防漏杯盖，一键开合。",
			url:       "https://detail.1688.com/offer/DEMO-810002.html"},
		{source: "collect", status: product.StatusDraft,
			origTitle: "DEMO-1688原始标题 硅胶手机壳 适用iPhone",
			title:     "DEMO-液态硅胶手机壳 全包防摔",
			desc:      "DEMO-采集原始描述：液态硅胶手机壳。",
			url:       "https://detail.1688.com/offer/DEMO-810003.html"},
		{source: "manual", status: product.StatusReady,
			title:   "DEMO-手动录入 桌面收纳盒 三层抽屉式",
			aiTitle: "DEMO-AI优化 | 3-Tier Desktop Drawer Organizer",
			desc:    "DEMO-手动录入描述：桌面收纳盒，三层抽屉。",
			aiDesc:  "DEMO-AI 生成描述：三层抽屉式桌面收纳盒，磨砂质感，适合办公桌、化妆台收纳。"},
		{source: "manual", status: product.StatusDraft,
			title: "DEMO-手动录入 帆布托特包 大容量",
			desc:  "DEMO-手动录入描述：帆布托特包。"},
	}
	products := make([]product.Product, 0, len(specs))
	skus := make([]product.ProductSKU, 0, len(specs)*2)
	for i, sp := range specs {
		p := product.Product{TenantID: s.TenantID, Source: sp.source, SourceURL: sp.url,
			OriginalTitle: sp.origTitle, Title: sp.title, AITitle: sp.aiTitle,
			Description: sp.desc, AIDescription: sp.aiDesc, Currency: "CNY", Status: sp.status}
		if err := tx.Create(&p).Error; err != nil {
			return fmt.Errorf("demoseed: product: %w", err)
		}
		products = append(products, p)
		img := product.ProductImage{ProductID: p.ID, ImageType: product.ImageTypeMain,
			Source: product.ImageSourceCollect, OriginURL: demoImageDataURI(i + 1), SortOrder: 0}
		if err := tx.Create(&img).Error; err != nil {
			return fmt.Errorf("demoseed: product image: %w", err)
		}
		count("product_images", 1)
		for j := 0; j < 2; j++ {
			stock := 40 + i*10 + j*5
			price := 19.9 + float64(i*10+j*3)
			cost := price * 0.45
			sku := product.ProductSKU{ProductID: p.ID,
				SKUCode: fmt.Sprintf("DEMO-SKU-%d-%d", i+1, j+1),
				SKUName: fmt.Sprintf("默认规格-%d", j+1),
				Price:   &price, CostPrice: &cost, Stock: &stock, WarningStock: 10}
			if err := tx.Create(&sku).Error; err != nil {
				return fmt.Errorf("demoseed: sku: %w", err)
			}
			skus = append(skus, sku)
		}
	}
	count("products", int64(len(products)))
	count("product_skus", int64(len(skus)))
	// one low-stock SKU to light up inventory alerts
	low := 2
	if err := tx.Model(&product.ProductSKU{}).Where("id = ?", skus[len(skus)-1].ID).
		Updates(map[string]any{"stock": low, "stock_status": "low_stock"}).Error; err != nil {
		return fmt.Errorf("demoseed: low stock sku: %w", err)
	}

	// ---- supplier + product sources + SKU mappings + price history ----
	rating := 4.8
	supplier := sourcing.Supplier{TenantID: s.TenantID, Platform: "1688", ExternalID: "DEMO-SUP-1",
		Name: "DEMO-义乌市优选家居用品厂", Rating: &rating, Status: sourcing.SupplierStatusActive,
		Remark: "DEMO- 演示供应商（种子数据）"}
	if err := tx.Create(&supplier).Error; err != nil {
		return fmt.Errorf("demoseed: supplier: %w", err)
	}
	count("suppliers", 1)
	moq := 10
	lead := 3
	for i := 0; i < 2; i++ {
		src := sourcing.ProductSource{TenantID: s.TenantID, ProductID: products[i].ID, SupplierID: supplier.ID,
			SourceURL: products[i].SourceURL, SourceOfferID: fmt.Sprintf("DEMO-OFFER-%d", i+1),
			Priority: 1, IsPrimary: true, Status: sourcing.SourceStatusActive, MOQ: &moq, LeadTimeDays: &lead}
		if err := tx.Create(&src).Error; err != nil {
			return fmt.Errorf("demoseed: product source: %w", err)
		}
		count("product_sources", 1)
		for j := 0; j < 2; j++ {
			localSKU := skus[i*2+j]
			price := *localSKU.CostPrice
			stk := 500
			ssku := sourcing.ProductSourceSKU{TenantID: s.TenantID, ProductSourceID: src.ID, LocalSKUID: localSKU.ID,
				ExternalSKUID: fmt.Sprintf("DEMO-EXT-SKU-%d-%d", i+1, j+1),
				CurrentPrice:  &price, Currency: "CNY", CurrentStock: &stk, Status: "active"}
			if err := tx.Create(&ssku).Error; err != nil {
				return fmt.Errorf("demoseed: source sku: %w", err)
			}
			count("product_source_skus", 1)
			hist := sourcing.SourcePriceHistory{TenantID: s.TenantID, SourceSKUID: ssku.ID, Price: price,
				Stock: &stk, CapturedAt: now.Add(-24 * time.Hour), CaptureSource: sourcing.CaptureSourceManual}
			if err := tx.Create(&hist).Error; err != nil {
				return fmt.Errorf("demoseed: price history: %w", err)
			}
			count("source_price_history", 1)
		}
	}

	// ---- product publications: link every demo product to the granted
	// manual DEMO shop (DEMO-SHOP-2, same grant as the operator/readonly
	// demo accounts) so scoped roles see non-empty product lists out of
	// the box. Titles carry the DEMO- prefix for cleanup targeting. ----
	for i := range products {
		pub := productpublish.ProductPublication{ProductID: products[i].ID, ShopID: shops[1].ID,
			Platform: shops[1].Platform, Status: productpublish.StatusDraft,
			PublishStatus: productpublish.StatusDraftCreated,
			Title:         products[i].Title, Currency: "CNY"}
		if err := tx.Create(&pub).Error; err != nil {
			return fmt.Errorf("demoseed: product publication: %w", err)
		}
		count("product_publications", 1)
	}

	// ---- publish-link samples: degraded TikTok publish capability preset
	// (local_draft_only) plus one bound douyin publication with SKU binding
	// rows so publications / sku-bindings views are non-empty. ----
	if err := s.seedPublishCapabilityPreset(tx, res); err != nil {
		return err
	}
	if err := s.seedReportCurrencyRates(tx, res); err != nil {
		return err
	}
	if err := s.seedDouyinPublicationSample(tx, res, now, shops[0], products[1], skus[2:4]); err != nil {
		return err
	}
	if err := s.seedPublishBatchWithTasks(tx, res, now, shops[2], products[:2]); err != nil {
		return err
	}

	// ---- sourcing alerts: one price-increase and one out-of-stock source
	// plus a backup supplier so the open switch suggestions are adoptable
	// (adopt switches primary to the backup) or ignorable. ----
	backupRating := 4.5
	backupSupplier := sourcing.Supplier{TenantID: s.TenantID, Platform: "1688", ExternalID: "DEMO-SUP-2",
		Name: "DEMO-广州备选供应链公司", Rating: &backupRating, Status: sourcing.SupplierStatusActive,
		Remark: "DEMO- 演示备选供应商（种子数据）"}
	if err := tx.Create(&backupSupplier).Error; err != nil {
		return fmt.Errorf("demoseed: backup supplier: %w", err)
	}
	count("suppliers", 1)

	lastChecked := now.Add(-time.Hour)
	alertPlans := []struct {
		productIdx int
		status     string
		reason     string
	}{
		{productIdx: 0, status: sourcing.SourceStatusPriceAlert, reason: sourcing.SwitchReasonPriceIncrease},
		{productIdx: 1, status: sourcing.SourceStatusOutOfStock, reason: sourcing.SwitchReasonOutOfStock},
	}
	for k, ap := range alertPlans {
		prod := products[ap.productIdx]
		var primary sourcing.ProductSource
		if err := tx.Where("product_id = ? AND supplier_id = ?", prod.ID, supplier.ID).First(&primary).Error; err != nil {
			return fmt.Errorf("demoseed: alert primary source: %w", err)
		}
		if err := tx.Model(&sourcing.ProductSource{}).Where("id = ?", primary.ID).
			Updates(map[string]any{"status": ap.status, "last_checked_at": lastChecked}).Error; err != nil {
			return fmt.Errorf("demoseed: alert source status: %w", err)
		}
		if ap.status == sourcing.SourceStatusPriceAlert {
			// a fresher, higher price point so the history modal shows the jump
			var ssku sourcing.ProductSourceSKU
			if err := tx.Where("product_source_id = ?", primary.ID).First(&ssku).Error; err != nil {
				return fmt.Errorf("demoseed: alert source sku: %w", err)
			}
			raised := 0.0
			if ssku.CurrentPrice != nil {
				raised = *ssku.CurrentPrice * 1.3
			}
			stk := 300
			hist := sourcing.SourcePriceHistory{TenantID: s.TenantID, SourceSKUID: ssku.ID, Price: raised,
				Stock: &stk, CapturedAt: lastChecked, CaptureSource: sourcing.CaptureSourceCrawl}
			if err := tx.Create(&hist).Error; err != nil {
				return fmt.Errorf("demoseed: alert price history: %w", err)
			}
			count("source_price_history", 1)
			if err := tx.Model(&sourcing.ProductSourceSKU{}).Where("id = ?", ssku.ID).
				Update("current_price", raised).Error; err != nil {
				return fmt.Errorf("demoseed: alert sku price: %w", err)
			}
		} else {
			if err := tx.Model(&sourcing.ProductSourceSKU{}).Where("product_source_id = ?", primary.ID).
				Update("current_stock", 0).Error; err != nil {
				return fmt.Errorf("demoseed: alert sku stock: %w", err)
			}
		}

		backup := sourcing.ProductSource{TenantID: s.TenantID, ProductID: prod.ID, SupplierID: backupSupplier.ID,
			SourceURL:     fmt.Sprintf("https://detail.1688.com/offer/DEMO-820%03d.html", k+1),
			SourceOfferID: fmt.Sprintf("DEMO-BAK-OFFER-%d", k+1),
			Priority:      2, IsPrimary: false, Status: sourcing.SourceStatusActive,
			MOQ: &moq, LeadTimeDays: &lead, LastCheckedAt: &lastChecked}
		if err := tx.Create(&backup).Error; err != nil {
			return fmt.Errorf("demoseed: backup source: %w", err)
		}
		count("product_sources", 1)
		localSKU := skus[ap.productIdx*2]
		bakPrice := *localSKU.CostPrice * 0.95
		bakStock := 800
		bakSKU := sourcing.ProductSourceSKU{TenantID: s.TenantID, ProductSourceID: backup.ID, LocalSKUID: localSKU.ID,
			ExternalSKUID: fmt.Sprintf("DEMO-BAK-EXT-SKU-%d", k+1),
			CurrentPrice:  &bakPrice, Currency: "CNY", CurrentStock: &bakStock, Status: "active"}
		if err := tx.Create(&bakSKU).Error; err != nil {
			return fmt.Errorf("demoseed: backup source sku: %w", err)
		}
		count("product_source_skus", 1)
		hist := sourcing.SourcePriceHistory{TenantID: s.TenantID, SourceSKUID: bakSKU.ID, Price: bakPrice,
			Stock: &bakStock, CapturedAt: lastChecked, CaptureSource: sourcing.CaptureSourceManual}
		if err := tx.Create(&hist).Error; err != nil {
			return fmt.Errorf("demoseed: backup price history: %w", err)
		}
		count("source_price_history", 1)

		suggestion := sourcing.SourceSwitchEvent{TenantID: s.TenantID, ProductID: prod.ID,
			FromSourceID: &primary.ID, ToSourceID: backup.ID, Reason: ap.reason,
			Detail: mustJSON(map[string]any{"seedPrefix": DemoPrefix}),
			Mode:   sourcing.SwitchModeSuggested, Status: sourcing.SuggestionOpen}
		if err := tx.Create(&suggestion).Error; err != nil {
			return fmt.Errorf("demoseed: switch suggestion: %w", err)
		}
		count("source_switch_events", 1)
	}

	// ---- sales orders across all statuses ----
	plans := demoSalesOrderPlans()
	for idx, plan := range plans {
		if err := validateSalesChain(plan); err != nil {
			return err
		}
		sku := skus[idx%len(skus)]
		unit := 0.0
		if sku.Price != nil {
			unit = *sku.Price
		}
		qty := 2
		orderedAt := now.Add(-time.Duration(48-idx*6) * time.Hour)
		o := order.Order{TenantID: s.TenantID, Platform: shops[idx%2].Platform, ShopID: &shops[idx%2].ID,
			OrderNo:      fmt.Sprintf("DEMO-%s-%04d", plan.suffix, idx+1),
			CustomerName: fmt.Sprintf("DEMO-买家%d", idx+1), Status: plan.status,
			PaymentStatus: plan.payment, FulfillmentStatus: plan.fulfillment,
			Currency: "CNY", TotalAmount: unit * float64(qty), OrderedAt: &orderedAt,
			Remark: "DEMO- 演示订单（种子数据）"}
		if plan.payment == order.PaymentPaid {
			paidAt := orderedAt.Add(30 * time.Minute)
			o.PaidAt = &paidAt
		}
		if plan.status == order.StatusShipped || plan.status == order.StatusDelivered {
			shippedAt := orderedAt.Add(6 * time.Hour)
			o.ShippedAt = &shippedAt
		}
		if plan.status == order.StatusDelivered {
			deliveredAt := orderedAt.Add(48 * time.Hour)
			o.DeliveredAt = &deliveredAt
		}
		if err := tx.Create(&o).Error; err != nil {
			return fmt.Errorf("demoseed: order: %w", err)
		}
		count("orders", 1)
		item := order.OrderItem{OrderID: o.ID, ProductID: &sku.ProductID, ProductSKUID: &sku.ID,
			ProductTitle: products[(idx%len(skus))/2].Title, SKUName: sku.SKUName, SKUCode: sku.SKUCode,
			Quantity: qty, UnitPrice: unit, TotalPrice: unit * float64(qty)}
		if plan.unmatchedItem {
			item.ProductID = nil
			item.ProductSKUID = nil
			item.SKUCode = ""
			item.SellerSKU = "DEMO-UNKNOWN-SKU"
			item.ProductTitle = "DEMO-未匹配商品（演示异常）"
		}
		if err := tx.Create(&item).Error; err != nil {
			return fmt.Errorf("demoseed: order item: %w", err)
		}
		count("order_items", 1)

		match := order.OrderItemSKUMatch{OrderID: o.ID, OrderItemID: item.ID, Platform: o.Platform,
			SellerSKU: item.SellerSKU, SKUCode: item.SKUCode,
			ProductID: item.ProductID, ProductSKUID: item.ProductSKUID,
			MatchType: order.MatchTypeLocalSKUCode, MatchStatus: order.MatchStatusMatched, Confidence: 100,
			Reason: "DEMO- 种子数据本地 SKU 匹配"}
		if plan.unmatchedItem {
			match.MatchType = order.MatchTypeNone
			match.MatchStatus = order.MatchStatusUnmatched
			match.Confidence = 0
			match.Reason = "DEMO- 演示：外部 SKU 无法匹配本地档案"
		}
		if err := tx.Create(&match).Error; err != nil {
			return fmt.Errorf("demoseed: sku match: %w", err)
		}
		count("order_item_sku_matches", 1)

		if plan.withShipment {
			sh := order.OrderShipment{OrderID: o.ID, Carrier: sfCarrier.Name,
				CarrierID: &sfCarrier.ID, CarrierCode: sfCarrier.Code,
				TrackingNo: fmt.Sprintf("SF10000000000%d", idx+1), Status: plan.shipmentState,
				TrackingURL: sfCarrier.TrackingURLFor(fmt.Sprintf("SF10000000000%d", idx+1)),
				ShippedAt:   o.ShippedAt, DeliveredAt: o.DeliveredAt}
			if err := tx.Create(&sh).Error; err != nil {
				return fmt.Errorf("demoseed: shipment: %w", err)
			}
			count("order_shipments", 1)
		}

		if plan.withDeduct && sku.Stock != nil {
			before := *sku.Stock
			after := before - qty
			log := inventory.InventoryChangeLog{TenantID: s.TenantID, ProductID: sku.ProductID, ProductSKUID: sku.ID,
				ChangeType: inventory.ChangeOrderDeduct, BeforeStock: before, AfterStock: after, Delta: -qty,
				Reason: "order_paid", Remark: "DEMO- 订单支付扣减（种子数据）",
				RefOrderID: &o.ID, RefOrderItemID: &item.ID,
				BusinessEventKey: fmt.Sprintf("DEMO-EVT-DEDUCT-%d", idx+1)}
			if err := tx.Create(&log).Error; err != nil {
				return fmt.Errorf("demoseed: deduct log: %w", err)
			}
			count("inventory_change_logs", 1)
			if err := tx.Model(&product.ProductSKU{}).Where("id = ?", sku.ID).Update("stock", after).Error; err != nil {
				return fmt.Errorf("demoseed: deduct stock: %w", err)
			}
		}
	}

	// ---- USD sales orders: multi-currency samples so the report base
	// currency conversion / unconverted hint is verifiable out of the box ----
	for i, amt := range []float64{129.99, 58.5} {
		orderedAt := now.Add(-time.Duration(12+i*20) * time.Hour)
		paidAt := orderedAt.Add(30 * time.Minute)
		usd := order.Order{TenantID: s.TenantID, Platform: shops[2].Platform, ShopID: &shops[2].ID,
			OrderNo:      fmt.Sprintf("DEMO-USD-%04d", i+1),
			CustomerName: fmt.Sprintf("DEMO-海外买家%d", i+1), Status: order.StatusPaid,
			PaymentStatus: order.PaymentPaid, FulfillmentStatus: order.FulfillmentUnfulfilled,
			Currency: "USD", TotalAmount: amt, OrderedAt: &orderedAt, PaidAt: &paidAt,
			Remark: "DEMO- 演示订单（USD 多币种样本）"}
		if err := tx.Create(&usd).Error; err != nil {
			return fmt.Errorf("demoseed: usd order: %w", err)
		}
		count("orders", 1)
		sku := skus[i%len(skus)]
		usdItem := order.OrderItem{OrderID: usd.ID, ProductID: &sku.ProductID, ProductSKUID: &sku.ID,
			ProductTitle: products[(i%len(skus))/2].Title, SKUName: sku.SKUName, SKUCode: sku.SKUCode,
			Quantity: 1, UnitPrice: amt, TotalPrice: amt}
		if err := tx.Create(&usdItem).Error; err != nil {
			return fmt.Errorf("demoseed: usd order item: %w", err)
		}
		count("order_items", 1)
	}

	// ---- purchase orders via real state machine ----
	for i, plan := range demoPurchaseOrderPlans() {
		if err := validatePurchaseChain(plan); err != nil {
			return err
		}
		sku := skus[i%4]
		srcPrice := 9.9 + float64(i)
		po := procurement.PurchaseOrder{TenantID: s.TenantID, SupplierID: supplier.ID, SupplierName: supplier.Name,
			SourcePlatform: "1688", ExternalOrderID: plan.externalID, Status: procurement.StatusDraft,
			TotalAmount: srcPrice * 5, Currency: "CNY", PayStatus: plan.payStatus,
			IdempotencyKey: fmt.Sprintf("DEMO-%s", plan.suffix)}
		if plan.payStatus == procurement.PayStatusPaid {
			paidAt := now.Add(-time.Duration(24-i) * time.Hour)
			po.PaidAt = &paidAt
		}
		if err := tx.Create(&po).Error; err != nil {
			return fmt.Errorf("demoseed: purchase order: %w", err)
		}
		count("purchase_orders", 1)
		item := procurement.PurchaseOrderItem{TenantID: s.TenantID, PurchaseOrderID: po.ID,
			LocalSKUID: sku.ID, SourceSKUID: sku.ID, ProductTitle: "DEMO-采购商品", SKUName: sku.SKUName,
			Quantity: 5, ExpectedPrice: &srcPrice}
		if err := tx.Create(&item).Error; err != nil {
			return fmt.Errorf("demoseed: purchase item: %w", err)
		}
		count("purchase_order_items", 1)

		from := procurement.StatusDraft
		for step, to := range plan.chain {
			if !procurement.CanTransition(from, to) {
				return procurement.ErrIllegalTransition(from, to)
			}
			ev := procurement.PurchaseOrderEvent{TenantID: s.TenantID, PurchaseOrderID: po.ID,
				FromStatus: from, ToStatus: to, Source: procurement.EventSourceManual,
				CreatedAt: now.Add(-time.Duration(len(plan.chain)-step) * time.Hour)}
			if err := tx.Create(&ev).Error; err != nil {
				return fmt.Errorf("demoseed: po event: %w", err)
			}
			count("purchase_order_events", 1)
			from = to
		}
		if from != procurement.StatusDraft {
			if err := tx.Model(&procurement.PurchaseOrder{}).Where("id = ?", po.ID).Update("status", from).Error; err != nil {
				return fmt.Errorf("demoseed: po status: %w", err)
			}
		}

		if plan.trackingNo != "" {
			logi := procurement.PurchaseLogistics{TenantID: s.TenantID, PurchaseOrderID: po.ID,
				TrackingNo: plan.trackingNo, Carrier: "中通快递", Status: "in_transit"}
			if plan.withInbound {
				logi.Status = "delivered"
				inboundAt := now.Add(-2 * time.Hour)
				logi.InboundAt = &inboundAt
			}
			if err := tx.Create(&logi).Error; err != nil {
				return fmt.Errorf("demoseed: po logistics: %w", err)
			}
			count("purchase_logistics", 1)
		}

		if plan.withInbound && sku.Stock != nil {
			var cur product.ProductSKU
			if err := tx.Where("id = ?", sku.ID).First(&cur).Error; err != nil {
				return fmt.Errorf("demoseed: inbound sku read: %w", err)
			}
			before := 0
			if cur.Stock != nil {
				before = *cur.Stock
			}
			after := before + item.Quantity
			log := inventory.InventoryChangeLog{TenantID: s.TenantID, ProductID: sku.ProductID, ProductSKUID: sku.ID,
				ChangeType: inventory.ChangePurchaseInbound, BeforeStock: before, AfterStock: after, Delta: item.Quantity,
				Reason: "purchase_delivered", Remark: "DEMO- 采购签收入库（种子数据）",
				BusinessEventKey: fmt.Sprintf("DEMO-EVT-INBOUND-%d", i+1)}
			if err := tx.Create(&log).Error; err != nil {
				return fmt.Errorf("demoseed: inbound log: %w", err)
			}
			count("inventory_change_logs", 1)
			if err := tx.Model(&product.ProductSKU{}).Where("id = ?", sku.ID).Update("stock", after).Error; err != nil {
				return fmt.Errorf("demoseed: inbound stock: %w", err)
			}
		}
	}

	// ---- manual inventory adjust log ----
	adjSKU := skus[2]
	if adjSKU.Stock != nil {
		before := *adjSKU.Stock
		after := before + 10
		log := inventory.InventoryChangeLog{TenantID: s.TenantID, ProductID: adjSKU.ProductID, ProductSKUID: adjSKU.ID,
			ChangeType: inventory.ChangeManualAdjust, BeforeStock: before, AfterStock: after, Delta: 10,
			Reason: "restock", Remark: "DEMO- 手动盘点调整（种子数据）",
			BusinessEventKey: "DEMO-EVT-ADJUST-1"}
		if err := tx.Create(&log).Error; err != nil {
			return fmt.Errorf("demoseed: adjust log: %w", err)
		}
		count("inventory_change_logs", 1)
		if err := tx.Model(&product.ProductSKU{}).Where("id = ?", adjSKU.ID).Update("stock", after).Error; err != nil {
			return fmt.Errorf("demoseed: adjust stock: %w", err)
		}
	}

	// ---- inventory sync batch + tasks（成功 + 失败样本）----
	started := now.Add(-30 * time.Minute)
	finished := now.Add(-25 * time.Minute)
	batch := inventory.InventorySyncBatch{TenantID: s.TenantID, BatchNo: "DEMO-INV-BATCH-1",
		Source: inventory.BatchSourceManual, Status: inventory.BatchStatusPartialSuccess,
		Platform: shops[0].Platform, ShopID: &shops[0].ID,
		TotalCount: 2, SuccessCount: 1, FailedCount: 1,
		StartedAt: &started, FinishedAt: &finished}
	if err := tx.Create(&batch).Error; err != nil {
		return fmt.Errorf("demoseed: inv batch: %w", err)
	}
	count("inventory_sync_batches", 1)
	taskStates := []struct {
		status, errMsg string
		sku            product.ProductSKU
	}{
		{status: inventory.StatusSuccess, sku: skus[0]},
		{status: inventory.StatusFailed, errMsg: "DEMO- 演示：规格未绑定平台 SKU，库存同步被阻断", sku: skus[1]},
	}
	var failedInvTaskID uuid.UUID
	for _, ts := range taskStates {
		target := 0
		if ts.sku.Stock != nil {
			target = *ts.sku.Stock
		}
		t := inventory.InventorySyncTask{TenantID: s.TenantID, BatchID: &batch.ID, BatchNo: batch.BatchNo,
			ProductID: ts.sku.ProductID, ProductSKUID: &ts.sku.ID, ShopID: shops[0].ID, Platform: shops[0].Platform,
			TaskType: inventory.TaskTypeInventorySync, Status: ts.status, Mode: inventory.ModeManual,
			TargetStock: target, StartedAt: &started, FinishedAt: &finished, ErrorMessage: ts.errMsg}
		if err := tx.Create(&t).Error; err != nil {
			return fmt.Errorf("demoseed: inv task: %w", err)
		}
		count("inventory_sync_tasks", 1)
		if ts.status == inventory.StatusFailed {
			failedInvTaskID = t.ID
		}
	}

	// ---- order sync task partial_success（异常工作台样本）----
	syncTask := ordersync.OrderSyncTask{TenantID: s.TenantID, ShopID: shops[0].ID, Platform: shops[0].Platform,
		TaskType: "order_sync", Status: "partial_success", Mode: "manual",
		StartedAt: &started, FinishedAt: &finished,
		TotalCount: 60, SuccessCount: 50, FailedCount: 10,
		ErrorMessage: "DEMO- 演示：部分订单页同步失败",
		Input:        mustJSON(map[string]any{"seedPrefix": DemoPrefix}),
		Output: mustJSON(map[string]any{
			"totalPages": 3, "successPages": 2, "failedPages": 1,
			"pageErrors": []map[string]any{{"page": 3, "error": "DEMO- simulated page fetch failure"}},
		})}
	if err := tx.Create(&syncTask).Error; err != nil {
		return fmt.Errorf("demoseed: order sync task: %w", err)
	}
	count("order_sync_tasks", 1)

	// ---- selection center: DEMO- 选品任务全状态 + 候选/评估样本 ----
	if err := s.seedSelection(tx, res, now, products); err != nil {
		return err
	}

	// ---- 选品数据面：多次采集留痕 + 同类目经营数据关联（Round 120）----
	if err := s.seedRound120SourcingInsights(tx, res, now, products); err != nil {
		return err
	}

	// ---- AI 客服：DEMO- 会话/消息/AI 建议草稿 + 同步任务成功/失败 ----
	if err := s.seedCustomerService(tx, res, now, shops); err != nil {
		return err
	}

	// ---- 运营任务中心：DEMO- 任务全生命周期（建议/待审/驳回/成功/失败）----
	if err := s.seedOperationTasks(tx, res, now, shops, products); err != nil {
		return err
	}

	// ---- 多仓演示：第二仓 + 调拨/入库样本流水（Round 112）----
	if err := s.seedRound112Warehouses(tx, res, now, skus); err != nil {
		return err
	}

	// ---- 审单规则演示：规则 + 待审/挂起命中样本（Round 114）----
	if err := s.seedRound114OrderReview(tx, res, now, shops); err != nil {
		return err
	}

	// ---- 自动化订单规则演示：规则 + 成功/失败/跳过执行日志（Round 119）----
	if err := s.seedRound119OrderAutomation(tx, res, now, shops, products, skus); err != nil {
		return err
	}

	// ---- operator 视角自动化正样本：授权店 DEMO-AT-1005（Round 136）----
	if err := s.seedRound136OperatorAutomation(tx, res, now, shops, products, skus); err != nil {
		return err
	}

	// ---- 自动化动作面扩展演示：自动应用发货规则 + 自动分仓（Round 126）----
	if err := s.seedRound126AutoActions(tx, res, now, shops, products, skus); err != nil {
		return err
	}

	// ---- 采集规则演示：启用/停用自定义规则样本（Round 135）----
	if err := s.seedRound135CollectRules(tx, res); err != nil {
		return err
	}

	// ---- 订单标签演示：租户标签 + 打标样本 + 自动打标签规则（Round 135）----
	if err := s.seedRound135OrderTags(tx, res, shops); err != nil {
		return err
	}

	// ---- 买家自动消息演示：节点规则 + 待发/已发送/已忽略草稿样本（Round 119）----
	if err := s.seedRound119BuyerMessages(tx, res, now, shops); err != nil {
		return err
	}

	// ---- 财务对账演示：回款/费用/采购实付价样本（Round 121）----
	if err := s.seedRound121Finance(tx, res, now, shops, &supplier, skus[0]); err != nil {
		return err
	}

	// ---- 财务对账工作台量样本：补足 25+ 行让分页/合计可演示（Round 136）----
	if err := s.seedRound136FinanceVolume(tx, res, now, shops, skus); err != nil {
		return err
	}

	// ---- exception workbench handled mark（演示处理动作留痕）----
	mark := orderexception.OrderExceptionMark{
		ExceptionType: orderexception.TypeInventorySyncFailed,
		SourceType:    orderexception.SourceInventorySyncTask,
		SourceID:      failedInvTaskID.String(),
		MarkType:      orderexception.MarkHandled,
		Remark:        "DEMO- 演示：已人工确认该库存同步失败样本"}
	if err := tx.Create(&mark).Error; err != nil {
		return fmt.Errorf("demoseed: exception mark: %w", err)
	}
	count("order_exception_marks", 1)

	// ---- MCP 只读 token 演示样本 + 审计样本（Round 147）----
	if err := s.seedRound147MCPToken(tx, res, now); err != nil {
		return err
	}

	// ---- 大屏汇率折算演示：今日多币种订单样本（Round 156）----
	if err := s.seedRound156ScreenFXOrders(tx, res, shops, products, skus); err != nil {
		return err
	}

	// ---- 第二业务租户演示：独立租户 + admin 账号 + 少量业务数据（Round 128）----
	if err := s.seedSecondTenant(tx, res); err != nil {
		return err
	}

	return nil
}

// collectDemoPurchaseOrderIDs returns purchase orders that belong to the demo
// dataset: seeded rows (DEMO- idempotency key / external id) plus rows created
// from the UI against DEMO- suppliers or DEMO- sales orders. Real purchase
// orders are never matched.
func collectDemoPurchaseOrderIDs(tx *gorm.DB, like string, demoOrderIDs, demoSupplierIDs []uuid.UUID) ([]uuid.UUID, error) {
	seen := map[uuid.UUID]struct{}{}
	add := func(ids []uuid.UUID) {
		for _, id := range ids {
			seen[id] = struct{}{}
		}
	}

	var ids []uuid.UUID
	if err := tx.Model(&procurement.PurchaseOrder{}).Unscoped().
		Where("idempotency_key LIKE ? OR external_order_id LIKE ? OR supplier_name LIKE ?", like, like, like).
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	add(ids)

	if len(demoSupplierIDs) > 0 {
		ids = ids[:0]
		if err := tx.Model(&procurement.PurchaseOrder{}).Unscoped().
			Where("supplier_id IN ?", demoSupplierIDs).
			Pluck("id", &ids).Error; err != nil {
			return nil, err
		}
		add(ids)
	}

	if len(demoOrderIDs) > 0 {
		ids = ids[:0]
		if err := tx.Model(&procurement.PurchaseOrderItem{}).Unscoped().
			Where("sales_order_id IN ?", demoOrderIDs).
			Distinct().
			Pluck("purchase_order_id", &ids).Error; err != nil {
			return nil, err
		}
		add(ids)
	}

	out := make([]uuid.UUID, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	return out, nil
}

// collectDemoProductIDs returns products that belong to the demo dataset:
// DEMO- titled rows plus products still owning DEMO- SKUs, so a seeded
// product renamed from the UI is still cleaned with all its children. Real
// products are never matched.
func collectDemoProductIDs(tx *gorm.DB, like string) ([]uuid.UUID, error) {
	seen := map[uuid.UUID]struct{}{}
	var ids []uuid.UUID
	if err := tx.Model(&product.Product{}).Unscoped().Where("title LIKE ?", like).Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	ids = ids[:0]
	if err := tx.Model(&product.ProductSKU{}).Unscoped().Where("sku_code LIKE ?", like).Distinct().Pluck("product_id", &ids).Error; err != nil {
		return nil, err
	}
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	out := make([]uuid.UUID, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	return out, nil
}

// Cleanup hard-deletes all rows carrying the target prefix (default DEMO-)
// and their children.
func (s *FullDemoSeeder) Cleanup(ctx context.Context) (*FullDemoResult, error) {
	if err := s.guard(); err != nil {
		return nil, err
	}
	if err := ValidateCleanPrefix(s.cleanPrefix()); err != nil {
		return nil, err
	}
	defaultPrefix := s.cleanPrefix() == DemoPrefix
	res := &FullDemoResult{Action: "cleanup", Counts: map[string]int64{}}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		del := func(table string, q *gorm.DB) error {
			r := q
			if r.Error != nil {
				return fmt.Errorf("demoseed cleanup %s: %w", table, r.Error)
			}
			res.Counts[table] += r.RowsAffected
			return nil
		}
		like := s.cleanPrefix() + "%"

		var orderIDs []uuid.UUID
		if err := tx.Model(&order.Order{}).Unscoped().Where("order_no LIKE ?", like).Pluck("id", &orderIDs).Error; err != nil {
			return err
		}
		finPayCond := tx.Unscoped().Where("remark LIKE ?", like)
		finExpCond := tx.Unscoped().Where("remark LIKE ?", like)
		if len(orderIDs) > 0 {
			finPayCond = tx.Unscoped().Where("order_id IN ? OR remark LIKE ?", orderIDs, like)
			finExpCond = tx.Unscoped().Where("order_id IN ? OR remark LIKE ?", orderIDs, like)
		}
		if err := del("finance_payment_records", finPayCond.Delete(&finance.PaymentRecord{})); err != nil {
			return err
		}
		if err := del("finance_order_expenses", finExpCond.Delete(&finance.OrderExpense{})); err != nil {
			return err
		}
		if len(orderIDs) > 0 {
			if err := del("order_tag_links", tx.Unscoped().Where("order_id IN ?", orderIDs).Delete(&order.OrderTagLink{})); err != nil {
				return err
			}
			if err := del("order_review_hits", tx.Unscoped().Where("order_id IN ?", orderIDs).Delete(&order.OrderReviewHit{})); err != nil {
				return err
			}
			if err := del("order_automation_logs", tx.Unscoped().Where("order_id IN ?", orderIDs).Delete(&order.OrderAutomationLog{})); err != nil {
				return err
			}
			if err := del("order_item_sku_matches", tx.Unscoped().Where("order_id IN ?", orderIDs).Delete(&order.OrderItemSKUMatch{})); err != nil {
				return err
			}
			if err := del("order_items", tx.Unscoped().Where("order_id IN ?", orderIDs).Delete(&order.OrderItem{})); err != nil {
				return err
			}
			if err := del("order_shipments", tx.Unscoped().Where("order_id IN ?", orderIDs).Delete(&order.OrderShipment{})); err != nil {
				return err
			}
			if err := del("orders", tx.Unscoped().Where("id IN ?", orderIDs).Delete(&order.Order{})); err != nil {
				return err
			}
		}

		if err := del("order_review_hits", tx.Unscoped().Where("rule_name LIKE ?", like).Delete(&order.OrderReviewHit{})); err != nil {
			return err
		}
		if err := del("order_review_rules", tx.Unscoped().Where("name LIKE ?", like).Delete(&order.OrderReviewRule{})); err != nil {
			return err
		}
		if err := del("order_automation_logs", tx.Unscoped().Where("rule_name LIKE ?", like).Delete(&order.OrderAutomationLog{})); err != nil {
			return err
		}
		if err := del("order_automation_rules", tx.Unscoped().Where("name LIKE ?", like).Delete(&order.OrderAutomationRule{})); err != nil {
			return err
		}

		var demoTagIDs []uuid.UUID
		if err := tx.Model(&order.OrderTag{}).Unscoped().Where("name LIKE ?", like).Pluck("id", &demoTagIDs).Error; err != nil {
			return err
		}
		if len(demoTagIDs) > 0 {
			if err := del("order_tag_links", tx.Unscoped().Where("tag_id IN ?", demoTagIDs).Delete(&order.OrderTagLink{})); err != nil {
				return err
			}
			if err := del("order_tags", tx.Unscoped().Where("id IN ?", demoTagIDs).Delete(&order.OrderTag{})); err != nil {
				return err
			}
		}

		if err := del("collect_rules", tx.Unscoped().Where("name LIKE ?", like).Delete(&collectrule.CollectRule{})); err != nil {
			return err
		}

		var demoSupplierIDs []uuid.UUID
		if err := tx.Model(&sourcing.Supplier{}).Unscoped().Where("name LIKE ?", like).Pluck("id", &demoSupplierIDs).Error; err != nil {
			return err
		}

		poIDs, err := collectDemoPurchaseOrderIDs(tx, like, orderIDs, demoSupplierIDs)
		if err != nil {
			return err
		}
		if len(poIDs) > 0 {
			if err := del("purchase_order_items", tx.Unscoped().Where("purchase_order_id IN ?", poIDs).Delete(&procurement.PurchaseOrderItem{})); err != nil {
				return err
			}
			if err := del("purchase_order_events", tx.Unscoped().Where("purchase_order_id IN ?", poIDs).Delete(&procurement.PurchaseOrderEvent{})); err != nil {
				return err
			}
			if err := del("purchase_logistics", tx.Unscoped().Where("purchase_order_id IN ?", poIDs).Delete(&procurement.PurchaseLogistics{})); err != nil {
				return err
			}
			if err := del("purchase_orders", tx.Unscoped().Where("id IN ?", poIDs).Delete(&procurement.PurchaseOrder{})); err != nil {
				return err
			}
		}

		supplierIDs := demoSupplierIDs
		var sourceIDs []uuid.UUID
		if len(supplierIDs) > 0 {
			if err := tx.Model(&sourcing.ProductSource{}).Unscoped().Where("supplier_id IN ?", supplierIDs).Pluck("id", &sourceIDs).Error; err != nil {
				return err
			}
		}
		if len(sourceIDs) > 0 {
			var sourceSKUIDs []uuid.UUID
			if err := tx.Model(&sourcing.ProductSourceSKU{}).Unscoped().Where("product_source_id IN ?", sourceIDs).Pluck("id", &sourceSKUIDs).Error; err != nil {
				return err
			}
			if len(sourceSKUIDs) > 0 {
				if err := del("source_price_history", tx.Unscoped().Where("source_sku_id IN ?", sourceSKUIDs).Delete(&sourcing.SourcePriceHistory{})); err != nil {
					return err
				}
			}
			if err := del("product_source_skus", tx.Unscoped().Where("product_source_id IN ?", sourceIDs).Delete(&sourcing.ProductSourceSKU{})); err != nil {
				return err
			}
			if err := del("product_sources", tx.Unscoped().Where("id IN ?", sourceIDs).Delete(&sourcing.ProductSource{})); err != nil {
				return err
			}
		}
		if len(supplierIDs) > 0 {
			if err := del("suppliers", tx.Unscoped().Where("id IN ?", supplierIDs).Delete(&sourcing.Supplier{})); err != nil {
				return err
			}
		}

		productIDs, err := collectDemoProductIDs(tx, like)
		if err != nil {
			return err
		}
		if err := cleanupPublishSamples(tx, res, like, productIDs, defaultPrefix); err != nil {
			return err
		}
		if len(productIDs) > 0 {
			if err := del("source_switch_events", tx.Unscoped().Where("product_id IN ?", productIDs).Delete(&sourcing.SourceSwitchEvent{})); err != nil {
				return err
			}
			if err := del("product_publications", tx.Unscoped().Where("product_id IN ? OR title LIKE ? OR external_product_id LIKE ?", productIDs, like, like).Delete(&productpublish.ProductPublication{})); err != nil {
				return err
			}
		} else {
			if err := del("product_publications", tx.Unscoped().Where("title LIKE ? OR external_product_id LIKE ?", like, like).Delete(&productpublish.ProductPublication{})); err != nil {
				return err
			}
		}
		var demoWarehouseIDs []uuid.UUID
		if err := tx.Model(&inventory.Warehouse{}).Unscoped().
			Where("code LIKE ? OR name LIKE ?", like, like).Pluck("id", &demoWarehouseIDs).Error; err != nil {
			return err
		}
		if len(demoWarehouseIDs) > 0 {
			if err := del("warehouse_stocks", tx.Unscoped().Where("warehouse_id IN ?", demoWarehouseIDs).Delete(&inventory.WarehouseStock{})); err != nil {
				return err
			}
			if err := del("warehouses", tx.Unscoped().Where("id IN ?", demoWarehouseIDs).Delete(&inventory.Warehouse{})); err != nil {
				return err
			}
		}
		if len(productIDs) > 0 {
			if err := del("warehouse_stocks", tx.Unscoped().Where("product_id IN ?", productIDs).Delete(&inventory.WarehouseStock{})); err != nil {
				return err
			}
		}
		if len(productIDs) > 0 {
			if err := del("inventory_change_logs", tx.Unscoped().Where("product_id IN ?", productIDs).Delete(&inventory.InventoryChangeLog{})); err != nil {
				return err
			}
			if err := del("inventory_sync_tasks", tx.Unscoped().Where("product_id IN ?", productIDs).Delete(&inventory.InventorySyncTask{})); err != nil {
				return err
			}
			if err := del("product_skus", tx.Unscoped().Where("product_id IN ?", productIDs).Delete(&product.ProductSKU{})); err != nil {
				return err
			}
			if err := del("product_images", tx.Unscoped().Where("product_id IN ?", productIDs).Delete(&product.ProductImage{})); err != nil {
				return err
			}
			if err := del("products", tx.Unscoped().Where("id IN ?", productIDs).Delete(&product.Product{})); err != nil {
				return err
			}
		}

		if err := del("inventory_sync_batches", tx.Unscoped().Where("batch_no LIKE ?", like).Delete(&inventory.InventorySyncBatch{})); err != nil {
			return err
		}

		var shopIDs []uuid.UUID
		if err := tx.Model(&shop.Shop{}).Unscoped().Where("shop_code LIKE ?", like).Pluck("id", &shopIDs).Error; err != nil {
			return err
		}
		if err := cleanupMigrationImports(tx, res, like, shopIDs); err != nil {
			return err
		}
		if err := del("finance_shop_monthly_expenses", tx.Unscoped().Where("remark LIKE ?", like).Delete(&finance.ShopMonthlyExpense{})); err != nil {
			return err
		}
		if len(shopIDs) > 0 {
			if err := del("finance_shop_monthly_expenses", tx.Unscoped().Where("shop_id IN ?", shopIDs).Delete(&finance.ShopMonthlyExpense{})); err != nil {
				return err
			}
			if err := del("order_sync_tasks", tx.Unscoped().Where("shop_id IN ? AND error_message LIKE ?", shopIDs, like).Delete(&ordersync.OrderSyncTask{})); err != nil {
				return err
			}
			if err := del("user_store_permissions", tx.Unscoped().Where("store_id IN ?", shopIDs).Delete(&admin.UserStorePermission{})); err != nil {
				return err
			}
			if err := del("shops", tx.Unscoped().Where("id IN ?", shopIDs).Delete(&shop.Shop{})); err != nil {
				return err
			}
		}

		if err := cleanupRound120CollectTasks(tx, res); err != nil {
			return err
		}
		if err := cleanupSelection(tx, res, like); err != nil {
			return err
		}
		if err := cleanupOperationTasks(tx, res, like); err != nil {
			return err
		}
		if err := cleanupCustomerSyncTasks(tx, res, like); err != nil {
			return err
		}

		// demo customer conversations: prefixed rows on any tenant, plus (for
		// the default DEMO- prefix only) F8 edge-case demo rows on any tenant
		// and tenant-0 orphans created by older demo seeds/scripts before
		// conversations stamped tenant_id.
		var convIDs []uuid.UUID
		convQ := tx.Model(&customerchat.CustomerConversation{}).Unscoped().
			Where("customer_name LIKE ?", like)
		if defaultPrefix {
			convQ = tx.Model(&customerchat.CustomerConversation{}).Unscoped().
				Where("customer_name LIKE ? OR customer_name LIKE ? OR (tenant_id = 0 AND customer_name LIKE ?)", like, "F8 Demo%", "Demo %")
		}
		if err := convQ.Pluck("id", &convIDs).Error; err != nil {
			return err
		}
		if len(convIDs) > 0 {
			if err := del("customer_messages", tx.Unscoped().Where("conversation_id IN ?", convIDs).Delete(&customerchat.CustomerMessage{})); err != nil {
				return err
			}
			if err := del("customer_reply_suggestions", tx.Unscoped().Where("conversation_id IN ?", convIDs).Delete(&customerchat.CustomerReplySuggestion{})); err != nil {
				return err
			}
			if err := del("customer_failure_events", tx.Unscoped().Where("conversation_id IN ?", convIDs).Delete(&customerchat.CustomerFailureEvent{})); err != nil {
				return err
			}
			if err := del("customer_conversations", tx.Unscoped().Where("id IN ?", convIDs).Delete(&customerchat.CustomerConversation{})); err != nil {
				return err
			}
		}

		if err := del("buyer_message_drafts", tx.Unscoped().Where("order_no LIKE ? OR template_name LIKE ? OR customer_name LIKE ?", like, like, like).Delete(&customerchat.BuyerMessageDraft{})); err != nil {
			return err
		}
		if err := del("buyer_message_rules", tx.Unscoped().Where("name LIKE ?", like).Delete(&customerchat.BuyerMessageRule{})); err != nil {
			return err
		}

		if err := del("customer_reply_templates", tx.Unscoped().Where("name LIKE ? OR content LIKE ?", like, like).Delete(&customerchat.CustomerReplyTemplate{})); err != nil {
			return err
		}

		if err := del("order_exception_marks", tx.Unscoped().Where("remark LIKE ?", like).Delete(&orderexception.OrderExceptionMark{})); err != nil {
			return err
		}

		if err := del("shipping_rules", tx.Unscoped().Where("name LIKE ?", like).Delete(&waybill.ShippingRule{})); err != nil {
			return err
		}
		if err := del("waybill_templates", tx.Unscoped().Where("is_preset = ? AND name LIKE ?", false, like).Delete(&waybill.Template{})); err != nil {
			return err
		}
		if tx.Migrator().HasTable("mcp_api_tokens") {
			if err := del("mcp_api_tokens", tx.Unscoped().Where("name LIKE ?", like).Delete(&mcptoken.Token{})); err != nil {
				return err
			}
		}
		if err := cleanupSecondTenant(tx, res, like); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// VerifyClean counts residual rows carrying the target prefix per table (all
// should be zero after cleanup).
func (s *FullDemoSeeder) VerifyClean(ctx context.Context) (*FullDemoResult, error) {
	if err := s.guard(); err != nil {
		return nil, err
	}
	if err := ValidateCleanPrefix(s.cleanPrefix()); err != nil {
		return nil, err
	}
	defaultPrefix := s.cleanPrefix() == DemoPrefix
	res := &FullDemoResult{Action: "verify", Counts: map[string]int64{}}
	tx := s.DB.WithContext(ctx)
	like := s.cleanPrefix() + "%"
	checks := []verifyCheck{
		splitSoftDeleted("shops", func() *gorm.DB {
			return tx.Model(&shop.Shop{}).Unscoped().Where("shop_code LIKE ?", like)
		}),
		splitSoftDeleted("products", func() *gorm.DB {
			return tx.Model(&product.Product{}).Unscoped().Where("title LIKE ?", like)
		}),
		{table: "product_skus", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&product.ProductSKU{}).Unscoped().Where("sku_code LIKE ?", like).Count(&n).Error
		}},
		splitSoftDeleted("product_publications", func() *gorm.DB {
			return tx.Model(&productpublish.ProductPublication{}).Unscoped().Where("title LIKE ? OR external_product_id LIKE ?", like, like)
		}),
		{table: "product_publication_skus", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&productpublish.ProductPublicationSKU{}).Unscoped().
				Where("external_sku_id LIKE ? OR bind_message LIKE ? OR publication_id IN (?)", like, like,
					tx.Model(&productpublish.ProductPublication{}).Unscoped().Select("id").
						Where("title LIKE ? OR external_product_id LIKE ?", like, like)).Count(&n).Error
		}},
		{table: "product_publish_batches", count: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("product_publish_batches") {
				return 0, nil
			}
			return n, tx.Model(&productpublish.ProductPublishBatch{}).Unscoped().
				Where("name LIKE ? OR idempotency_key LIKE ?", like, like).Count(&n).Error
		}},
		{table: "product_publish_tasks", count: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("product_publish_tasks") || !tx.Migrator().HasTable("product_publish_batches") {
				return 0, nil
			}
			return n, tx.Model(&productpublish.ProductPublishTask{}).Unscoped().
				Where("title LIKE ? OR error_message LIKE ? OR batch_id IN (?)", like, like,
					tx.Model(&productpublish.ProductPublishBatch{}).Unscoped().Select("id").
						Where("name LIKE ? OR idempotency_key LIKE ?", like, like)).Count(&n).Error
		}},
		{table: "settings", count: func() (int64, error) {
			var n int64
			if !defaultPrefix || !tx.Migrator().HasTable("settings") {
				return 0, nil
			}
			return n, tx.Model(&settings.Setting{}).Where("remark = ?", demoSettingRemark).Count(&n).Error
		}},
		{table: "source_switch_events", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&sourcing.SourceSwitchEvent{}).Unscoped().
				Where("product_id IN (?)", tx.Model(&product.Product{}).Unscoped().
					Select("id").Where("title LIKE ?", like)).Count(&n).Error
		}},
		splitSoftDeleted("suppliers", func() *gorm.DB {
			return tx.Model(&sourcing.Supplier{}).Unscoped().Where("name LIKE ?", like)
		}),
		splitSoftDeleted("orders", func() *gorm.DB {
			return tx.Model(&order.Order{}).Unscoped().Where("order_no LIKE ?", like)
		}),
		splitSoftDeleted("purchase_orders", func() *gorm.DB {
			return tx.Model(&procurement.PurchaseOrder{}).Unscoped().
				Where("idempotency_key LIKE ? OR external_order_id LIKE ? OR supplier_name LIKE ?", like, like, like)
		}),
		{table: "finance_payment_records", count: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("finance_payment_records") {
				return 0, nil
			}
			return n, tx.Model(&finance.PaymentRecord{}).Unscoped().
				Where("remark LIKE ? OR order_id IN (?)", like, tx.Model(&order.Order{}).Unscoped().
					Select("id").Where("order_no LIKE ?", like)).Count(&n).Error
		}},
		{table: "finance_order_expenses", count: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("finance_order_expenses") {
				return 0, nil
			}
			return n, tx.Model(&finance.OrderExpense{}).Unscoped().
				Where("remark LIKE ? OR order_id IN (?)", like, tx.Model(&order.Order{}).Unscoped().
					Select("id").Where("order_no LIKE ?", like)).Count(&n).Error
		}},
		{table: "finance_shop_monthly_expenses", count: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("finance_shop_monthly_expenses") {
				return 0, nil
			}
			return n, tx.Model(&finance.ShopMonthlyExpense{}).Unscoped().
				Where("remark LIKE ? OR shop_id IN (?)", like, tx.Model(&shop.Shop{}).Unscoped().
					Select("id").Where("shop_code LIKE ?", like)).Count(&n).Error
		}},
		{table: "inventory_change_logs", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&inventory.InventoryChangeLog{}).Where("business_event_key LIKE ?", like).Count(&n).Error
		}},
		{table: "warehouses", count: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("warehouses") {
				return 0, nil
			}
			return n, tx.Model(&inventory.Warehouse{}).Unscoped().
				Where("(code LIKE ? OR name LIKE ?) AND warehouses.deleted_at IS NULL", like, like).Count(&n).Error
		}, softDeleted: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("warehouses") {
				return 0, nil
			}
			return n, tx.Model(&inventory.Warehouse{}).Unscoped().
				Where("(code LIKE ? OR name LIKE ?) AND warehouses.deleted_at IS NOT NULL", like, like).Count(&n).Error
		}},
		{table: "warehouse_stocks", count: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("warehouse_stocks") {
				return 0, nil
			}
			return n, tx.Model(&inventory.WarehouseStock{}).
				Where("warehouse_id IN (?) OR product_id IN (?)",
					tx.Model(&inventory.Warehouse{}).Unscoped().Select("id").
						Where("code LIKE ? OR name LIKE ?", like, like),
					tx.Model(&product.Product{}).Unscoped().Select("id").
						Where("title LIKE ?", like)).Count(&n).Error
		}},
		{table: "inventory_sync_batches", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&inventory.InventorySyncBatch{}).Unscoped().Where("batch_no LIKE ?", like).Count(&n).Error
		}},
		{table: "order_sync_tasks", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&ordersync.OrderSyncTask{}).Where("error_message LIKE ?", like).Count(&n).Error
		}},
		{table: "order_review_rules", count: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("order_review_rules") {
				return 0, nil
			}
			return n, tx.Model(&order.OrderReviewRule{}).Where("name LIKE ?", like).Count(&n).Error
		}},
		{table: "order_review_hits", count: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("order_review_hits") {
				return 0, nil
			}
			return n, tx.Model(&order.OrderReviewHit{}).
				Where("rule_name LIKE ? OR order_id IN (?)", like,
					tx.Model(&order.Order{}).Unscoped().Select("id").Where("order_no LIKE ?", like)).
				Count(&n).Error
		}},
		{table: "order_automation_rules", count: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("order_automation_rules") {
				return 0, nil
			}
			return n, tx.Model(&order.OrderAutomationRule{}).Where("name LIKE ?", like).Count(&n).Error
		}},
		{table: "order_automation_logs", count: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("order_automation_logs") {
				return 0, nil
			}
			return n, tx.Model(&order.OrderAutomationLog{}).
				Where("rule_name LIKE ? OR order_id IN (?)", like,
					tx.Model(&order.Order{}).Unscoped().Select("id").Where("order_no LIKE ?", like)).
				Count(&n).Error
		}},
		{table: "order_tags", count: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("order_tags") {
				return 0, nil
			}
			return n, tx.Model(&order.OrderTag{}).Where("name LIKE ?", like).Count(&n).Error
		}},
		{table: "order_tag_links", count: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("order_tag_links") {
				return 0, nil
			}
			return n, tx.Model(&order.OrderTagLink{}).
				Where("tag_id IN (?) OR order_id IN (?)",
					tx.Model(&order.OrderTag{}).Unscoped().Select("id").Where("name LIKE ?", like),
					tx.Model(&order.Order{}).Unscoped().Select("id").Where("order_no LIKE ?", like)).
				Count(&n).Error
		}},
		{table: "collect_rules", count: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("collect_rules") {
				return 0, nil
			}
			return n, tx.Model(&collectrule.CollectRule{}).Where("name LIKE ?", like).Count(&n).Error
		}},
		{table: "order_exception_marks", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&orderexception.OrderExceptionMark{}).Where("remark LIKE ?", like).Count(&n).Error
		}},
		splitSoftDeleted("customer_conversations", func() *gorm.DB {
			if defaultPrefix {
				return tx.Model(&customerchat.CustomerConversation{}).Unscoped().
					Where("customer_name LIKE ? OR customer_name LIKE ? OR (tenant_id = 0 AND customer_name LIKE ?)", like, "F8 Demo%", "Demo %")
			}
			return tx.Model(&customerchat.CustomerConversation{}).Unscoped().
				Where("customer_name LIKE ?", like)
		}),
		{table: "customer_messages", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&customerchat.CustomerMessage{}).
				Where("content LIKE ?", like).Count(&n).Error
		}},
		{table: "customer_reply_suggestions", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&customerchat.CustomerReplySuggestion{}).Unscoped().
				Where("prompt_code LIKE ? OR suggested_reply LIKE ?", like, like).Count(&n).Error
		}},
		splitSoftDeleted("customer_reply_templates", func() *gorm.DB {
			return tx.Model(&customerchat.CustomerReplyTemplate{}).Unscoped().
				Where("name LIKE ? OR content LIKE ?", like, like)
		}),
		splitSoftDeleted("buyer_message_rules", func() *gorm.DB {
			return tx.Model(&customerchat.BuyerMessageRule{}).Unscoped().
				Where("name LIKE ?", like)
		}),
		splitSoftDeleted("buyer_message_drafts", func() *gorm.DB {
			return tx.Model(&customerchat.BuyerMessageDraft{}).Unscoped().
				Where("order_no LIKE ? OR template_name LIKE ? OR customer_name LIKE ?", like, like, like)
		}),
		{table: "shipping_rules", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&waybill.ShippingRule{}).
				Where("name LIKE ?", like).Count(&n).Error
		}},
		{table: "waybill_templates", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&waybill.Template{}).
				Where("is_preset = ? AND name LIKE ?", false, like).Count(&n).Error
		}},
		{table: "customer_message_sync_tasks", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&customersync.CustomerMessageSyncTask{}).Unscoped().
				Where("cursor LIKE ? OR error_message LIKE ?", like, like).Count(&n).Error
		}},
		{table: "selection_tasks", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&selection.SelectionTask{}).Unscoped().
				Where("name LIKE ?", like).Count(&n).Error
		}},
		{table: "selection_candidates", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&selection.SelectionCandidate{}).Unscoped().
				Where("title LIKE ?", like).Count(&n).Error
		}},
		{table: "selection_source_matches", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&selection.SelectionSourceMatch{}).Unscoped().
				Where("source_offer_id LIKE ? OR supplier_name LIKE ?", like, like).Count(&n).Error
		}},
		{table: "selection_evaluations", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&selection.SelectionEvaluation{}).Unscoped().
				Where("candidate_id IN (?)", tx.Model(&selection.SelectionCandidate{}).Unscoped().
					Select("id").Where("title LIKE ?", like)).Count(&n).Error
		}},
		{table: "mcp_api_tokens", count: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("mcp_api_tokens") {
				return 0, nil
			}
			return n, tx.Model(&mcptoken.Token{}).Unscoped().Where("name LIKE ?", like).Count(&n).Error
		}},
		{table: "collect_tasks", count: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("collect_tasks") {
				return 0, nil
			}
			return n, tx.Model(&collect.CollectTask{}).Unscoped().
				Where("source_url LIKE ?", demoMarketURLPrefix+"%").Count(&n).Error
		}},
	}
	checks = append(checks, secondTenantVerifyChecks(tx, like)...)
	checks = append(checks, operationTaskVerifyChecks(tx, like)...)
	checks = append(checks, migrationImportVerifyChecks(tx, like, func() *gorm.DB {
		return tx.Model(&shop.Shop{}).Unscoped().Select("id").Where("shop_code LIKE ?", like)
	})...)
	for _, c := range checks {
		n, err := c.count()
		if err != nil {
			return nil, fmt.Errorf("demoseed verify %s: %w", c.table, err)
		}
		res.Counts[c.table] = n
		if c.softDeleted == nil {
			continue
		}
		d, err := c.softDeleted()
		if err != nil {
			return nil, fmt.Errorf("demoseed verify %s (soft-deleted): %w", c.table, err)
		}
		if d > 0 {
			if res.SoftDeleted == nil {
				res.SoftDeleted = map[string]int64{}
			}
			res.SoftDeleted[c.table] = d
		}
	}
	return res, nil
}
