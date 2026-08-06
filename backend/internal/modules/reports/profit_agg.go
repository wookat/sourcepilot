package reports

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

// orderCurrencyAgg is one (row key, currency) revenue group over scoped
// orders. Amount is the exact decimal(18,4) sum of order totals (sums of
// 4-decimal values stay 4-decimal, so the float64 round-trip through
// AmountRat is lossless); N counts the orders in the group.
type orderCurrencyAgg struct {
	Key      string  `gorm:"column:grp"`
	Currency string  `gorm:"column:currency"`
	Amount   float64 `gorm:"column:amount"`
	N        int64   `gorm:"column:n"`
}

// orderCurrencyAggs groups scoped orders by (keyExpr, currency) in SQL. An
// empty keyExpr groups by currency only (all rows share Key "").
func orderCurrencyAggs(build func() (*gorm.DB, error), keyExpr string) ([]orderCurrencyAgg, error) {
	tx, err := build()
	if err != nil {
		return nil, err
	}
	sel, grp := "currency, SUM(total_amount) AS amount, COUNT(*) AS n", "currency"
	if keyExpr != "" {
		sel = keyExpr + " AS grp, " + sel
		grp = keyExpr + ", " + grp
	}
	var rows []orderCurrencyAgg
	if err := tx.Select(sel).Group(grp).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// linePairAgg is one (row key, product, sku, quantity) line group over
// scoped order items. Reference costs only depend on the (product, sku)
// pair and the per-line cost keeps its per-line round2(unit × quantity), so
// grouping by quantity too reproduces the ungrouped math exactly.
type linePairAgg struct {
	Key          string     `gorm:"column:grp"`
	ProductID    *uuid.UUID `gorm:"column:product_id"`
	ProductSKUID *uuid.UUID `gorm:"column:product_sku_id"`
	Quantity     int        `gorm:"column:quantity"`
	N            int64      `gorm:"column:n"`
}

// linePairAggs groups scoped order lines by (keyExpr, product, sku,
// quantity) in SQL. The scoped orders query joins in as a subquery aliased o.
func (s *Service) linePairAggs(ctx context.Context, build func() (*gorm.DB, error), keyExpr string) ([]linePairAgg, error) {
	sub, err := build()
	if err != nil {
		return nil, err
	}
	sub = sub.Select("id, shop_id")
	sel := "oi.product_id AS product_id, oi.product_sku_id AS product_sku_id, oi.quantity AS quantity, COUNT(*) AS n"
	grp := "oi.product_id, oi.product_sku_id, oi.quantity"
	if keyExpr != "" {
		sel = keyExpr + " AS grp, " + sel
		grp = keyExpr + ", " + grp
	}
	var rows []linePairAgg
	if err := s.DB.WithContext(ctx).Table("order_items AS oi").
		Joins("JOIN (?) AS o ON oi.order_id = o.id", sub).
		Select(sel).Group(grp).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// itemCurrencyAgg is one (product key, order currency) revenue group over
// scoped order items (product dimension rows price by line total).
type itemCurrencyAgg struct {
	Key      string  `gorm:"column:grp"`
	Currency string  `gorm:"column:currency"`
	Amount   float64 `gorm:"column:amount"`
}

// itemCurrencyAggs sums line revenue per (product key, order currency).
func (s *Service) itemCurrencyAggs(ctx context.Context, build func() (*gorm.DB, error)) ([]itemCurrencyAgg, error) {
	sub, err := build()
	if err != nil {
		return nil, err
	}
	sub = sub.Select("id, currency")
	var rows []itemCurrencyAgg
	if err := s.DB.WithContext(ctx).Table("order_items AS oi").
		Joins("JOIN (?) AS o ON oi.order_id = o.id", sub).
		Select("COALESCE(oi.product_id, '" + KeyUnmatchedProduct + "') AS grp, o.currency AS currency, SUM(oi.total_price) AS amount").
		Group("COALESCE(oi.product_id, '" + KeyUnmatchedProduct + "'), o.currency").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// productOrderCounts counts distinct orders per product key.
func (s *Service) productOrderCounts(ctx context.Context, build func() (*gorm.DB, error)) (map[string]int64, error) {
	sub, err := build()
	if err != nil {
		return nil, err
	}
	sub = sub.Select("id")
	type row struct {
		Key string `gorm:"column:grp"`
		N   int64  `gorm:"column:n"`
	}
	var rows []row
	if err := s.DB.WithContext(ctx).Table("order_items AS oi").
		Joins("JOIN (?) AS o ON oi.order_id = o.id", sub).
		Select("COALESCE(oi.product_id, '" + KeyUnmatchedProduct + "') AS grp, COUNT(DISTINCT oi.order_id) AS n").
		Group("COALESCE(oi.product_id, '" + KeyUnmatchedProduct + "')").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.Key] = r.N
	}
	return out, nil
}

// keyFirstSeen is the newest scoped order (or line) of a row key: it carries
// the row's first-seen attributes (platform / title) and the iteration order
// of the pre-sort row list (orders scan created_at DESC).
type keyFirstSeen struct {
	Key      string `gorm:"column:grp"`
	Platform string `gorm:"column:platform"`
	Title    string `gorm:"column:title"`
}

// shopFirstSeen resolves each shop key's newest order (platform + insertion
// order), mirroring the previous created_at DESC iteration.
func (s *Service) shopFirstSeen(ctx context.Context, build func() (*gorm.DB, error)) ([]keyFirstSeen, error) {
	sub, err := build()
	if err != nil {
		return nil, err
	}
	sub = sub.Select("id, shop_id, platform, created_at")
	keyExpr := "COALESCE(o.shop_id, '" + KeyNoShop + "')"
	inner := s.DB.Table("(?) AS o", sub).
		Select(keyExpr + " AS grp, o.platform AS platform, o.created_at AS created_at, " +
			"ROW_NUMBER() OVER (PARTITION BY " + keyExpr + " ORDER BY o.created_at DESC, o.id DESC) AS rn")
	var rows []keyFirstSeen
	if err := s.DB.WithContext(ctx).Table("(?) AS t", inner).
		Select("t.grp AS grp, t.platform AS platform").
		Where("t.rn = 1").Order("t.created_at DESC, t.grp").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// productFirstSeen resolves each product key's line from the newest scoped
// order (title + insertion order), mirroring the previous created_at DESC
// iteration.
func (s *Service) productFirstSeen(ctx context.Context, build func() (*gorm.DB, error)) ([]keyFirstSeen, error) {
	sub, err := build()
	if err != nil {
		return nil, err
	}
	sub = sub.Select("id, created_at")
	keyExpr := "COALESCE(oi.product_id, '" + KeyUnmatchedProduct + "')"
	inner := s.DB.Table("order_items AS oi").
		Joins("JOIN (?) AS o ON oi.order_id = o.id", sub).
		Select(keyExpr + " AS grp, oi.product_title AS title, o.created_at AS created_at, " +
			"ROW_NUMBER() OVER (PARTITION BY " + keyExpr + " ORDER BY o.created_at DESC, o.id DESC, oi.id) AS rn")
	var rows []keyFirstSeen
	if err := s.DB.WithContext(ctx).Table("(?) AS t", inner).
		Select("t.grp AS grp, t.title AS title").
		Where("t.rn = 1").Order("t.created_at DESC, t.grp").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// pairKey identifies a (product, sku) reference-cost pair.
type pairKey struct {
	product string
	sku     string
}

func pairKeyOf(productID, skuID *uuid.UUID) pairKey {
	k := pairKey{}
	if productID != nil {
		k.product = productID.String()
	}
	if skuID != nil {
		k.sku = skuID.String()
	}
	return k
}

// resolvePairRefs resolves the reference unit cost of each distinct
// (product, sku) pair via the shared procurement resolver (the resolution
// only depends on the pair, so one synthetic line per pair is exact).
func (s *Service) resolvePairRefs(ctx context.Context, pairs []linePairAgg) (map[pairKey]*float64, error) {
	seen := map[pairKey]bool{}
	synth := make([]order.OrderItem, 0, len(pairs))
	byID := map[uuid.UUID]pairKey{}
	for _, p := range pairs {
		k := pairKeyOf(p.ProductID, p.ProductSKUID)
		if seen[k] {
			continue
		}
		seen[k] = true
		it := order.OrderItem{ProductID: p.ProductID, ProductSKUID: p.ProductSKUID, Quantity: 1}
		it.ID = uuid.New()
		byID[it.ID] = k
		synth = append(synth, it)
	}
	out := make(map[pairKey]*float64, len(synth))
	if len(synth) == 0 {
		return out, nil
	}
	refs, err := s.Proc.ResolveLineCostRefs(ctx, synth)
	if err != nil {
		return nil, err
	}
	for id, ref := range refs {
		out[byID[id]] = ref.UnitCostCNY
	}
	return out, nil
}
