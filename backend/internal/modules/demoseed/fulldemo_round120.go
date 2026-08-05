package demoseed

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/selection"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
)

// demoMarketURLPrefix marks collect captures that belong to the demo dataset:
// the domain only exists inside the seeder, so cleanup by URL prefix can never
// touch real captures.
const demoMarketURLPrefix = "https://market.demo.trademind.local/"

// round120CapturePlan is one repeated-capture series for a demo selection
// candidate source URL (multiple successful captures make the price trend
// drawable; the candidate without history demos the trend empty state).
type round120CapturePlan struct {
	urlSuffix   string // matches the candidate SourceURL built in seedSelection
	prices      []float64
	monthlySold int
	reviewCount int
}

func round120CapturePlans() []round120CapturePlan {
	return []round120CapturePlan{
		// candidate 1: four captures, gently declining price
		{urlSuffix: "item/DEMO-SEL-1", prices: [](float64){24.8, 23.9, 22.5, 21.9}, monthlySold: 1350, reviewCount: 214},
		// candidate 3: two captures — the minimum that draws a line
		{urlSuffix: "item/DEMO-SEL-3", prices: [](float64){35.0, 33.5}, monthlySold: 460, reviewCount: 87},
		// candidate 2 gets no captures on purpose (trend empty-state demo)
	}
}

// seedRound120SourcingInsights adds the选品数据面 demo data: repeated collect
// captures per candidate source URL (price trend + collected sales/review
// facts) and a same-category product link so the category benchmark joins
// existing demo orders. All rows are removable by URL prefix / DEMO- prefix.
func (s *FullDemoSeeder) seedRound120SourcingInsights(tx *gorm.DB, res *FullDemoResult, now time.Time, products []product.Product) error {
	count := func(table string, n int64) { res.Counts[table] += n }
	if !tx.Migrator().HasTable("collect_tasks") {
		return nil
	}

	for _, plan := range round120CapturePlans() {
		url := demoMarketURLPrefix + plan.urlSuffix
		for i, price := range plan.prices {
			finished := now.Add(-time.Duration(len(plan.prices)-i) * 24 * time.Hour)
			raw, err := json.Marshal(map[string]any{
				"source":   "1688",
				"currency": "CNY",
				"title":    "DEMO- 采集留痕样本",
				"skus":     []map[string]any{{"price": price}},
				"attributes": map[string]any{
					"monthlySold": plan.monthlySold,
					"reviewCount": plan.reviewCount,
				},
			})
			if err != nil {
				return fmt.Errorf("demoseed: round120 raw result: %w", err)
			}
			task := collect.CollectTask{TenantID: s.TenantID,
				Source: "1688", SourceURL: url,
				Status: collect.StatusSuccess, RawResult: raw,
				FinishedAt: &finished}
			started := finished.Add(-2 * time.Minute)
			task.StartedAt = &started
			if err := tx.Create(&task).Error; err != nil {
				return fmt.Errorf("demoseed: round120 collect task: %w", err)
			}
			count("collect_tasks", 1)
		}
	}

	// Link the pending-decision demo candidate to a same-category demo
	// product so the站内同类目 benchmark aggregates existing demo orders.
	if len(products) > 1 {
		var cand selection.SelectionCandidate
		err := tx.Where("tenant_id = ? AND title LIKE ? AND source_url LIKE ? AND product_id IS NULL",
			s.TenantID, "DEMO-候选%", demoMarketURLPrefix+"%").
			Order("created_at ASC").First(&cand).Error
		if err == nil {
			if err := tx.Model(&selection.SelectionCandidate{}).Where("id = ?", cand.ID).
				Update("product_id", products[1].ID).Error; err != nil {
				return fmt.Errorf("demoseed: round120 candidate product link: %w", err)
			}
		} else if err != gorm.ErrRecordNotFound {
			return fmt.Errorf("demoseed: round120 candidate lookup: %w", err)
		}
	}

	// Supply-readiness sample: a货源档案 whose offer id matches candidate 1's
	// best 1688 match, so对比 shows one已就绪 row. The row hangs off a demo
	// supplier, so the existing supplier-scoped cleanup removes it.
	if len(products) > 0 {
		var sup sourcing.Supplier
		if err := tx.Where("tenant_id = ? AND name LIKE ?", s.TenantID, "DEMO-%").
			Order("created_at ASC").First(&sup).Error; err == nil {
			src := sourcing.ProductSource{TenantID: s.TenantID, ProductID: products[0].ID, SupplierID: sup.ID,
				SourceURL:     "https://detail.1688.com/offer/DEMO-SEL-1.html",
				SourceOfferID: "DEMO-SEL-OFFER-1",
				Priority:      3, Status: sourcing.SourceStatusActive}
			if err := tx.Create(&src).Error; err != nil {
				return fmt.Errorf("demoseed: round120 supply source: %w", err)
			}
			count("product_sources", 1)
		} else if err != gorm.ErrRecordNotFound {
			return fmt.Errorf("demoseed: round120 supplier lookup: %w", err)
		}
	}
	return nil
}

// cleanupRound120CollectTasks removes demo collect captures by the seeder-only
// URL prefix.
func cleanupRound120CollectTasks(tx *gorm.DB, res *FullDemoResult) error {
	if !tx.Migrator().HasTable("collect_tasks") {
		return nil
	}
	q := tx.Unscoped().Where("source_url LIKE ?", demoMarketURLPrefix+"%").
		Delete(&collect.CollectTask{})
	if q.Error != nil {
		return fmt.Errorf("demoseed cleanup collect_tasks: %w", q.Error)
	}
	res.Counts["collect_tasks"] += q.RowsAffected
	return nil
}
