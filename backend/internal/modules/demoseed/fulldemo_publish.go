package demoseed

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/providers/fxrate"
	"gorm.io/gorm"
)

// demoSettingRemark marks seeded settings rows so cleanup targets demo
// presets only and never touches operator-managed configuration.
const demoSettingRemark = DemoPrefix + "演示配置（种子数据）"

// demoPublishPresetRow is one settings row of a platform publish preset.
type demoPublishPresetRow struct {
	key, value, valueType string
}

// demoPublishPresets maps settings group key to the required fields with
// obviously-fake DEMO values. Each preset only unlocks the degraded
// local_draft_only publish capability (no platform API is ever called on
// that path), so no real credential is fabricated.
var demoPublishPresets = map[string][]demoPublishPresetRow{
	"platform_tiktok": {
		{"app_key", "DEMO-tiktok-app-key", "string"},
		{"app_secret", "DEMO-not-a-real-secret", "string"},
		{"auth_base_url", "https://auth.tiktok-shops.com", "string"},
		{"api_base_url", "https://open-api.tiktokglobalshop.com", "string"},
		{"redirect_uri", "https://demo.trademind.local/api/v1/stores/tiktok/callback", "string"},
		{"api_version", "202309", "string"},
		{"timeout_sec", "30", "number"},
	},
	"platform_shopee": {
		{"partner_id", "1000000", "string"},
		{"partner_key", "DEMO-not-a-real-partner-key", "string"},
		{"auth_base_url", "https://partner.shopeemobile.com", "string"},
		{"api_base_url", "https://partner.shopeemobile.com", "string"},
		{"redirect_uri", "https://demo.trademind.local/api/v1/stores/shopee/callback", "string"},
		{"timeout_sec", "30", "number"},
	},
}

func demoPublishPresetSettings(groupKey string) []settings.Setting {
	rows := demoPublishPresets[groupKey]
	out := make([]settings.Setting, 0, len(rows))
	for _, r := range rows {
		out = append(out, settings.Setting{
			TenantID:  0,
			GroupKey:  groupKey,
			ItemKey:   r.key,
			ItemValue: r.value,
			ValueType: r.valueType,
			Remark:    demoSettingRemark,
		})
	}
	return out
}

// seedPublishCapabilityPreset inserts the platform_tiktok / platform_shopee
// app config presets so the TikTok and Shopee publish targets resolve to the
// degraded local_draft_only capability out of the box. Real configuration is
// never modified: a preset is skipped entirely when its group already has
// any row.
func (s *FullDemoSeeder) seedPublishCapabilityPreset(tx *gorm.DB, res *FullDemoResult) error {
	if !tx.Migrator().HasTable("settings") {
		return nil
	}
	for _, groupKey := range []string{"platform_tiktok", "platform_shopee"} {
		var n int64
		if err := tx.Model(&settings.Setting{}).
			Where("tenant_id = 0 AND group_key = ?", groupKey).Count(&n).Error; err != nil {
			return fmt.Errorf("demoseed: count %s settings: %w", groupKey, err)
		}
		if n > 0 {
			continue
		}
		for _, row := range demoPublishPresetSettings(groupKey) {
			row := row
			if err := tx.Create(&row).Error; err != nil {
				return fmt.Errorf("demoseed: %s preset: %w", groupKey, err)
			}
			res.Counts["settings"]++
		}
	}
	return nil
}

// seedReportCurrencyRates fills the manual USD report rate when the tenant-0
// rate table is still empty, so demo USD orders convert in reports out of the
// box instead of showing the "unconverted currency" notice. A non-empty rate
// table (real configuration) is never modified.
func (s *FullDemoSeeder) seedReportCurrencyRates(tx *gorm.DB, res *FullDemoResult) error {
	if !tx.Migrator().HasTable("settings") {
		return nil
	}
	const demoRates = `{"USD":"7.20"}`
	var row settings.Setting
	err := tx.Where("tenant_id = 0 AND group_key = ? AND item_key = ?",
		fxrate.SettingsGroup, fxrate.KeyRates).First(&row).Error
	switch {
	case err == nil:
		if v := strings.TrimSpace(row.ItemValue); v != "" && v != "{}" {
			return nil
		}
		if err := tx.Model(&settings.Setting{}).Where("id = ?", row.ID).
			Update("item_value", demoRates).Error; err != nil {
			return fmt.Errorf("demoseed: update report currency rates: %w", err)
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		create := settings.Setting{TenantID: 0, GroupKey: fxrate.SettingsGroup,
			ItemKey: fxrate.KeyRates, ItemValue: demoRates, ValueType: "json", Remark: demoSettingRemark}
		if err := tx.Create(&create).Error; err != nil {
			return fmt.Errorf("demoseed: create report currency rates: %w", err)
		}
		res.Counts["settings"]++
	default:
		return fmt.Errorf("demoseed: load report currency rates: %w", err)
	}
	return nil
}

// seedDouyinPublicationSample creates one already-published douyin
// publication (bound external listing) with SKU binding rows covering the
// bound + unmatched states, so the publication detail / SKU binding views
// are non-empty out of the box.
func (s *FullDemoSeeder) seedDouyinPublicationSample(tx *gorm.DB, res *FullDemoResult, now time.Time,
	douyinShop shop.Shop, prod product.Product, skus []product.ProductSKU) error {
	if len(skus) < 2 {
		return fmt.Errorf("demoseed: douyin publication sample needs 2 skus")
	}
	publishedAt := now.Add(-72 * time.Hour)
	syncedAt := now.Add(-2 * time.Hour)
	pub := productpublish.ProductPublication{
		ProductID:          prod.ID,
		ShopID:             douyinShop.ID,
		Platform:           douyinShop.Platform,
		ExternalProductID:  "DEMO-DY-3502001",
		Status:             productpublish.StatusPublishedRecord,
		PublishStatus:      productpublish.StatusSuccess,
		Title:              prod.Title,
		Currency:           "CNY",
		ExternalURL:        "https://haohuo.jinritemai.com/views/product/detail?id=DEMO-DY-3502001",
		PublishedAt:        &publishedAt,
		LastSyncedAt:       &syncedAt,
		SkuBindingSyncedAt: &syncedAt,
		RawData: mustJSON(map[string]any{
			"seedPrefix": DemoPrefix,
			"platformSkus": []map[string]any{
				{"platformSkuId": "DEMO-DY-SKU-1", "specName": "默认规格-1", "priceYuan": derefOr(skus[0].Price, 29.9), "stock": derefOrInt(skus[0].Stock, 50)},
				{"platformSkuId": "DEMO-DY-SKU-2", "specName": "默认规格-2", "priceYuan": derefOr(skus[1].Price, 32.9), "stock": derefOrInt(skus[1].Stock, 45)},
			},
		}),
	}
	if err := tx.Create(&pub).Error; err != nil {
		return fmt.Errorf("demoseed: douyin publication: %w", err)
	}
	res.Counts["product_publications"]++

	bindRows := []productpublish.ProductPublicationSKU{
		{PublicationID: pub.ID, ProductSKUID: &skus[0].ID, ExternalSKUID: "DEMO-DY-SKU-1",
			SKUCode: skus[0].SKUCode, Price: skus[0].Price, Stock: skus[0].Stock,
			BindStatus: productpublish.BindStatusBound, BindConfidence: 100,
			BindMessage: "DEMO- 种子数据：SKU 编码精确匹配", LastSyncedAt: &syncedAt},
		{PublicationID: pub.ID, ProductSKUID: &skus[1].ID,
			SKUCode: skus[1].SKUCode, Price: skus[1].Price, Stock: skus[1].Stock,
			BindStatus: productpublish.BindStatusUnmatched, BindConfidence: 0,
			BindMessage: "DEMO- 演示：平台无对应规格，可手动绑定", LastSyncedAt: &syncedAt},
	}
	for i := range bindRows {
		if err := tx.Create(&bindRows[i]).Error; err != nil {
			return fmt.Errorf("demoseed: publication sku: %w", err)
		}
		res.Counts["product_publication_skus"]++
	}
	return nil
}

// seedPublishBatchWithTasks creates one finished multi-product publish batch
// plus its child tasks (success / failed-retryable / pending) so the publish
// task page's 批次 and 子任务 tabs are non-empty out of the box. All rows carry
// the DEMO- prefix (batch name / task title / idempotency key) for cleanup.
func (s *FullDemoSeeder) seedPublishBatchWithTasks(tx *gorm.DB, res *FullDemoResult, now time.Time,
	tiktokShop shop.Shop, products []product.Product) error {
	if len(products) < 2 {
		return fmt.Errorf("demoseed: publish batch sample needs 2 products")
	}
	createdAt := now.Add(-36 * time.Hour)
	finishedAt := now.Add(-35 * time.Hour)
	batch := productpublish.ProductPublishBatch{
		TenantID:       s.TenantID,
		BatchType:      productpublish.BatchTypeMultiProduct,
		Name:           DemoPrefix + "刊登批次（TikTok 本地草稿演示）",
		Status:         productpublish.BatchPartialSuccess,
		ProductCount:   2,
		TargetCount:    1,
		TaskCount:      3,
		ReadyCount:     2,
		SuccessCount:   1,
		FailedCount:    1,
		IdempotencyKey: DemoPrefix + "publish-batch-1",
		Summary: mustJSON(map[string]any{
			"seedPrefix": DemoPrefix,
			"note":       "演示批次：本地草稿能力（local_draft_only），不调用真实平台 API",
		}),
		FinishedAt: &finishedAt,
	}
	batch.CreatedAt = createdAt
	if err := tx.Create(&batch).Error; err != nil {
		return fmt.Errorf("demoseed: publish batch: %w", err)
	}
	res.Counts["product_publish_batches"]++

	targetKey := "tiktok:" + tiktokShop.ID.String()
	successFin := createdAt.Add(2 * time.Minute)
	failedFin := createdAt.Add(3 * time.Minute)
	tasks := []productpublish.ProductPublishTask{
		{
			TenantID: s.TenantID, ProductID: products[0].ID,
			ShopID: tiktokShop.ID, TargetStoreID: tiktokShop.ID,
			BatchID: &batch.ID, TargetKey: targetKey, Platform: tiktokShop.Platform,
			TaskType: productpublish.TaskTypeLocalDraftCreate,
			Status:   productpublish.TaskSuccess, PublishStatus: productpublish.StatusDraftCreated,
			Mode: productpublish.PublishModeSaveAsPlatformDraft, PublishMode: productpublish.PublishModeSaveAsPlatformDraft,
			Title:      products[0].Title,
			Output:     mustJSON(map[string]any{"seedPrefix": DemoPrefix, "capability": productpublish.CapLocalDraftOnly}),
			FinishedAt: &successFin,
		},
		{
			TenantID: s.TenantID, ProductID: products[1].ID,
			ShopID: tiktokShop.ID, TargetStoreID: tiktokShop.ID,
			BatchID: &batch.ID, TargetKey: targetKey, Platform: tiktokShop.Platform,
			TaskType: productpublish.TaskTypeLocalDraftCreate,
			Status:   productpublish.TaskFailed, PublishStatus: productpublish.StatusPubFailed,
			Mode: productpublish.PublishModeSaveAsPlatformDraft, PublishMode: productpublish.PublishModeSaveAsPlatformDraft,
			Title:        products[1].Title,
			Retryable:    true,
			ErrorCode:    "DEMO_READINESS_BLOCKED",
			ErrorMessage: DemoPrefix + " 演示失败样本：商品缺少主图，可在补齐后重试",
			FinishedAt:   &failedFin,
		},
		{
			TenantID: s.TenantID, ProductID: products[0].ID,
			ShopID: tiktokShop.ID, TargetStoreID: tiktokShop.ID,
			BatchID: &batch.ID, TargetKey: targetKey, Platform: tiktokShop.Platform,
			TaskType: productpublish.TaskTypeLocalDraftCreate,
			Status:   productpublish.TaskPending, PublishStatus: productpublish.StatusReady,
			Mode: productpublish.PublishModeSaveAsPlatformDraft, PublishMode: productpublish.PublishModeSaveAsPlatformDraft,
			Title: products[0].Title,
		},
	}
	for i := range tasks {
		tasks[i].CreatedAt = createdAt.Add(time.Duration(i) * time.Minute)
		if err := tx.Create(&tasks[i]).Error; err != nil {
			return fmt.Errorf("demoseed: publish task: %w", err)
		}
		res.Counts["product_publish_tasks"]++
	}
	return nil
}

// cleanupPublishSamples removes SKU binding rows attached to demo
// publications plus the DEMO- settings preset. Publications themselves are
// removed by the existing product-scoped cleanup.
func cleanupPublishSamples(tx *gorm.DB, res *FullDemoResult, like string, productIDs []uuid.UUID, includeDemoSettings bool) error {
	del := func(table string, q *gorm.DB) error {
		if q.Error != nil {
			return fmt.Errorf("demoseed cleanup %s: %w", table, q.Error)
		}
		res.Counts[table] += q.RowsAffected
		return nil
	}
	if tx.Migrator().HasTable("product_publish_tasks") && tx.Migrator().HasTable("product_publish_batches") {
		batchIDs := tx.Model(&productpublish.ProductPublishBatch{}).Unscoped().Select("id").
			Where("name LIKE ? OR idempotency_key LIKE ?", like, like)
		taskQ := tx.Unscoped().Where("title LIKE ? OR error_message LIKE ? OR batch_id IN (?)", like, like, batchIDs)
		if len(productIDs) > 0 {
			taskQ = tx.Unscoped().Where("title LIKE ? OR error_message LIKE ? OR batch_id IN (?) OR product_id IN ?",
				like, like, batchIDs, productIDs)
		}
		if err := del("product_publish_tasks", taskQ.Delete(&productpublish.ProductPublishTask{})); err != nil {
			return err
		}
		if err := del("product_publish_batches",
			tx.Unscoped().Where("name LIKE ? OR idempotency_key LIKE ?", like, like).
				Delete(&productpublish.ProductPublishBatch{})); err != nil {
			return err
		}
	}
	pubIDs := tx.Model(&productpublish.ProductPublication{}).Unscoped().Select("id").
		Where("title LIKE ? OR external_product_id LIKE ?", like, like)
	if len(productIDs) > 0 {
		pubIDs = tx.Model(&productpublish.ProductPublication{}).Unscoped().Select("id").
			Where("title LIKE ? OR external_product_id LIKE ? OR product_id IN ?", like, like, productIDs)
	}
	if err := del("product_publication_skus",
		tx.Unscoped().Where("external_sku_id LIKE ? OR publication_id IN (?)", like, pubIDs).
			Delete(&productpublish.ProductPublicationSKU{})); err != nil {
		return err
	}
	if includeDemoSettings && tx.Migrator().HasTable("settings") {
		if err := del("settings",
			tx.Where("remark = ?", demoSettingRemark).Delete(&settings.Setting{})); err != nil {
			return err
		}
	}
	return nil
}

func derefOr(v *float64, def float64) float64 {
	if v == nil {
		return def
	}
	return *v
}

func derefOrInt(v *int, def int) int {
	if v == nil {
		return def
	}
	return *v
}
