package sourcing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/providers/sourceinfo"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Domain errors mapped to 4xx by the handler.
var (
	ErrNotFound   = errors.New("sourcing: not found")
	ErrBadRequest = errors.New("sourcing: bad request")
	ErrConflict   = errors.New("sourcing: conflict")
)

// Service implements supplier / product-source business logic.
type Service struct {
	DB       *gorm.DB
	Settings *settings.Service
	OpLog    *operationlog.Service
	Provider sourceinfo.Provider
}

func (s *Service) provider() sourceinfo.Provider {
	if s.Provider != nil {
		return s.Provider
	}
	return &sourceinfo.Mock{}
}

func (s *Service) logOp(ctx context.Context, operator *uuid.UUID, action, targetID, detail string) {
	if s.OpLog == nil {
		return
	}
	_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
		AdminUserID: operator,
		Action:      action,
		Resource:    "sourcing",
		ResourceID:  targetID,
		Status:      "success",
		Message:     detail,
	})
}

// ---------- suppliers ----------

// SupplierListQuery filters supplier list.
type SupplierListQuery struct {
	Page     int
	PageSize int
	Keyword  string
	Platform string
	Status   string
}

// SupplierList is a paginated supplier result.
type SupplierList struct {
	Items    []Supplier `json:"items"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
}

// ListSuppliers returns suppliers with pagination.
func (s *Service) ListSuppliers(ctx context.Context, q SupplierListQuery) (*SupplierList, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 200 {
		q.PageSize = 20
	}
	tx := s.DB.WithContext(ctx).Model(&Supplier{})
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		like := "%" + kw + "%"
		tx = tx.Where("name ILIKE ? OR external_id ILIKE ?", like, like)
	}
	if q.Platform != "" {
		tx = tx.Where("platform = ?", q.Platform)
	}
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []Supplier
	if err := tx.Order("created_at DESC").
		Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return &SupplierList{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// SupplierBody is the create/update payload.
type SupplierBody struct {
	Platform   string          `json:"platform"`
	ExternalID string          `json:"externalId"`
	Name       string          `json:"name"`
	Rating     *float64        `json:"rating"`
	Contact    json.RawMessage `json:"contact"`
	Remark     string          `json:"remark"`
	Status     string          `json:"status"`
}

// CreateSupplier inserts a supplier.
func (s *Service) CreateSupplier(ctx context.Context, body SupplierBody, operator *uuid.UUID) (*Supplier, error) {
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: name required", ErrBadRequest)
	}
	sup := Supplier{
		Platform:   defaultStr(strings.TrimSpace(body.Platform), "1688"),
		ExternalID: strings.TrimSpace(body.ExternalID),
		Name:       name,
		Rating:     body.Rating,
		Remark:     strings.TrimSpace(body.Remark),
		Status:     defaultStr(strings.TrimSpace(body.Status), SupplierStatusActive),
	}
	if len(body.Contact) > 0 {
		sup.Contact = datatypes.JSON(body.Contact)
	}
	if sup.ExternalID != "" {
		var dup int64
		if err := s.DB.WithContext(ctx).Model(&Supplier{}).
			Where("platform = ? AND external_id = ?", sup.Platform, sup.ExternalID).
			Count(&dup).Error; err != nil {
			return nil, err
		}
		if dup > 0 {
			return nil, fmt.Errorf("%w: supplier external id already exists", ErrConflict)
		}
	}
	if err := s.DB.WithContext(ctx).Create(&sup).Error; err != nil {
		return nil, err
	}
	s.logOp(ctx, operator, "sourcing.supplier.create", sup.ID.String(), sup.Name)
	return &sup, nil
}

// UpdateSupplier patches mutable supplier fields.
func (s *Service) UpdateSupplier(ctx context.Context, id uuid.UUID, body SupplierBody, operator *uuid.UUID) (*Supplier, error) {
	var sup Supplier
	if err := s.DB.WithContext(ctx).First(&sup, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	updates := map[string]any{}
	if v := strings.TrimSpace(body.Name); v != "" {
		updates["name"] = v
	}
	if v := strings.TrimSpace(body.ExternalID); v != "" {
		updates["external_id"] = v
	}
	if body.Rating != nil {
		updates["rating"] = *body.Rating
	}
	if len(body.Contact) > 0 {
		updates["contact"] = datatypes.JSON(body.Contact)
	}
	if body.Remark != "" {
		updates["remark"] = strings.TrimSpace(body.Remark)
	}
	if v := strings.TrimSpace(body.Status); v == SupplierStatusActive || v == SupplierStatusDisabled {
		updates["status"] = v
	}
	if len(updates) > 0 {
		if err := s.DB.WithContext(ctx).Model(&sup).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	s.logOp(ctx, operator, "sourcing.supplier.update", sup.ID.String(), sup.Name)
	return &sup, nil
}

// DeleteSupplier soft-deletes a supplier without active sources.
func (s *Service) DeleteSupplier(ctx context.Context, id uuid.UUID, operator *uuid.UUID) error {
	var cnt int64
	if err := s.DB.WithContext(ctx).Model(&ProductSource{}).
		Where("supplier_id = ?", id).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return fmt.Errorf("%w: supplier has bound sources", ErrConflict)
	}
	res := s.DB.WithContext(ctx).Delete(&Supplier{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	s.logOp(ctx, operator, "sourcing.supplier.delete", id.String(), "")
	return nil
}

// ---------- product sources ----------

// ListProductSources returns all sources of a product with SKU mappings.
func (s *Service) ListProductSources(ctx context.Context, productID uuid.UUID) ([]ProductSource, error) {
	var items []ProductSource
	if err := s.DB.WithContext(ctx).
		Preload("Supplier").Preload("SKUs").
		Where("product_id = ?", productID).
		Order("is_primary DESC, priority ASC, created_at ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// BindSourceBody binds a product to a supplier offer.
type BindSourceBody struct {
	SupplierID   string   `json:"supplierId"`
	SupplierName string   `json:"supplierName"`
	Platform     string   `json:"platform"`
	ExternalID   string   `json:"externalId"`
	SourceURL    string   `json:"sourceUrl"`
	SourceOffer  string   `json:"sourceOfferId"`
	Priority     *int     `json:"priority"`
	MOQ          *int     `json:"moq"`
	LeadTimeDays *int     `json:"leadTimeDays"`
	SetPrimary   bool     `json:"setPrimary"`
	Rating       *float64 `json:"rating"`
}

// BindSource creates a product source, auto-creating the supplier when only
// a name/external id is given. The first source of a product becomes primary.
func (s *Service) BindSource(ctx context.Context, productID uuid.UUID, body BindSourceBody, operator *uuid.UUID) (*ProductSource, error) {
	offerID := strings.TrimSpace(body.SourceOffer)
	if offerID == "" {
		offerID = extractOfferID(body.SourceURL)
	}
	var src *ProductSource
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sup Supplier
		if sid := strings.TrimSpace(body.SupplierID); sid != "" {
			u, err := uuid.Parse(sid)
			if err != nil {
				return fmt.Errorf("%w: invalid supplierId", ErrBadRequest)
			}
			if err := tx.First(&sup, "id = ?", u).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("%w: supplier", ErrNotFound)
				}
				return err
			}
		} else {
			name := strings.TrimSpace(body.SupplierName)
			if name == "" {
				return fmt.Errorf("%w: supplierId or supplierName required", ErrBadRequest)
			}
			platform := defaultStr(strings.TrimSpace(body.Platform), "1688")
			ext := strings.TrimSpace(body.ExternalID)
			q := tx.Where("platform = ? AND name = ?", platform, name)
			if ext != "" {
				q = tx.Where("platform = ? AND external_id = ?", platform, ext)
			}
			if err := q.First(&sup).Error; err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				sup = Supplier{Platform: platform, ExternalID: ext, Name: name, Rating: body.Rating, Status: SupplierStatusActive}
				if err := tx.Create(&sup).Error; err != nil {
					return err
				}
			}
		}

		var dup int64
		if err := tx.Model(&ProductSource{}).
			Where("product_id = ? AND supplier_id = ? AND source_offer_id = ?", productID, sup.ID, offerID).
			Count(&dup).Error; err != nil {
			return err
		}
		if dup > 0 {
			return fmt.Errorf("%w: source already bound", ErrConflict)
		}

		var existing int64
		if err := tx.Model(&ProductSource{}).Where("product_id = ?", productID).Count(&existing).Error; err != nil {
			return err
		}
		priority := 100
		if body.Priority != nil {
			priority = *body.Priority
		}
		ps := ProductSource{
			ProductID:     productID,
			SupplierID:    sup.ID,
			SourceURL:     strings.TrimSpace(body.SourceURL),
			SourceOfferID: offerID,
			Priority:      priority,
			IsPrimary:     existing == 0 || body.SetPrimary,
			Status:        SourceStatusActive,
			MOQ:           body.MOQ,
			LeadTimeDays:  body.LeadTimeDays,
		}
		if ps.IsPrimary && existing > 0 {
			if err := tx.Model(&ProductSource{}).
				Where("product_id = ? AND is_primary = TRUE", productID).
				Update("is_primary", false).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&ps).Error; err != nil {
			return err
		}
		ps.Supplier = &sup
		src = &ps
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logOp(ctx, operator, "sourcing.source.bind", src.ID.String(), src.SourceURL)
	return src, nil
}

// UpdateSourceBody patches priority/locked/status/moq/leadTime.
type UpdateSourceBody struct {
	Priority     *int    `json:"priority"`
	Locked       *bool   `json:"locked"`
	Status       *string `json:"status"`
	MOQ          *int    `json:"moq"`
	LeadTimeDays *int    `json:"leadTimeDays"`
	SourceURL    *string `json:"sourceUrl"`
}

// UpdateSource patches a product source.
func (s *Service) UpdateSource(ctx context.Context, id uuid.UUID, body UpdateSourceBody, operator *uuid.UUID) (*ProductSource, error) {
	var ps ProductSource
	if err := s.DB.WithContext(ctx).First(&ps, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	updates := map[string]any{}
	if body.Priority != nil {
		updates["priority"] = *body.Priority
	}
	if body.Locked != nil {
		updates["locked"] = *body.Locked
	}
	if body.Status != nil {
		st := strings.TrimSpace(*body.Status)
		switch st {
		case SourceStatusActive, SourceStatusOutOfStock, SourceStatusPriceAlert, SourceStatusDisabled:
			updates["status"] = st
		default:
			return nil, fmt.Errorf("%w: invalid status", ErrBadRequest)
		}
	}
	if body.MOQ != nil {
		updates["moq"] = *body.MOQ
	}
	if body.LeadTimeDays != nil {
		updates["lead_time_days"] = *body.LeadTimeDays
	}
	if body.SourceURL != nil {
		updates["source_url"] = strings.TrimSpace(*body.SourceURL)
	}
	if len(updates) > 0 {
		if err := s.DB.WithContext(ctx).Model(&ps).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	s.logOp(ctx, operator, "sourcing.source.update", ps.ID.String(), "")
	return &ps, nil
}

// SetPrimary manually switches the primary source (mode=manual switch event).
func (s *Service) SetPrimary(ctx context.Context, id uuid.UUID, operator *uuid.UUID) (*ProductSource, error) {
	var target ProductSource
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&target, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if target.Status == SourceStatusDisabled {
			return fmt.Errorf("%w: source disabled", ErrConflict)
		}
		if target.IsPrimary {
			return nil
		}
		var prev ProductSource
		var fromID *uuid.UUID
		if err := tx.Where("product_id = ? AND is_primary = TRUE", target.ProductID).First(&prev).Error; err == nil {
			fromID = &prev.ID
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Model(&ProductSource{}).
			Where("product_id = ? AND is_primary = TRUE", target.ProductID).
			Update("is_primary", false).Error; err != nil {
			return err
		}
		if err := tx.Model(&target).Update("is_primary", true).Error; err != nil {
			return err
		}
		ev := SourceSwitchEvent{
			ProductID:    target.ProductID,
			FromSourceID: fromID,
			ToSourceID:   target.ID,
			Reason:       SwitchReasonManual,
			Mode:         SwitchModeManual,
			Operator:     operator,
		}
		return tx.Create(&ev).Error
	})
	if err != nil {
		return nil, err
	}
	target.IsPrimary = true
	s.logOp(ctx, operator, "sourcing.source.set_primary", target.ID.String(), "")
	return &target, nil
}

// SKUMappingBody is one local SKU ↔ external SKU mapping row.
type SKUMappingBody struct {
	LocalSKUID    string          `json:"localSkuId"`
	ExternalSKUID string          `json:"externalSkuId"`
	ExternalSpec  json.RawMessage `json:"externalSpec"`
	CurrentPrice  *float64        `json:"currentPrice"`
	CurrentStock  *int            `json:"currentStock"`
}

// SaveSKUMappings upserts SKU mappings for one product source. Manual price
// input writes a price-history row with capture_source=manual.
func (s *Service) SaveSKUMappings(ctx context.Context, sourceID uuid.UUID, rows []SKUMappingBody, operator *uuid.UUID) ([]ProductSourceSKU, error) {
	var ps ProductSource
	if err := s.DB.WithContext(ctx).First(&ps, "id = ?", sourceID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var out []ProductSourceSKU
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		for _, r := range rows {
			localID, err := uuid.Parse(strings.TrimSpace(r.LocalSKUID))
			if err != nil {
				return fmt.Errorf("%w: invalid localSkuId", ErrBadRequest)
			}
			var m ProductSourceSKU
			err = tx.Where("product_source_id = ? AND local_sku_id = ?", sourceID, localID).First(&m).Error
			isNew := errors.Is(err, gorm.ErrRecordNotFound)
			if err != nil && !isNew {
				return err
			}
			if isNew {
				m = ProductSourceSKU{ProductSourceID: sourceID, LocalSKUID: localID, Currency: "CNY", Status: "active"}
			}
			m.ExternalSKUID = strings.TrimSpace(r.ExternalSKUID)
			if len(r.ExternalSpec) > 0 {
				m.ExternalSpec = datatypes.JSON(r.ExternalSpec)
			}
			priceChanged := false
			if r.CurrentPrice != nil {
				priceChanged = m.CurrentPrice == nil || *m.CurrentPrice != *r.CurrentPrice
				m.CurrentPrice = r.CurrentPrice
			}
			if r.CurrentStock != nil {
				m.CurrentStock = r.CurrentStock
			}
			if err := tx.Save(&m).Error; err != nil {
				return err
			}
			if priceChanged && r.CurrentPrice != nil {
				h := SourcePriceHistory{
					SourceSKUID:   m.ID,
					Price:         *r.CurrentPrice,
					Stock:         m.CurrentStock,
					CapturedAt:    now,
					CaptureSource: CaptureSourceManual,
				}
				if err := tx.Create(&h).Error; err != nil {
					return err
				}
			}
			out = append(out, m)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logOp(ctx, operator, "sourcing.sku_mapping.save", sourceID.String(), fmt.Sprintf("%d rows", len(rows)))
	return out, nil
}

// DeleteSKUMapping soft-deletes one local↔external SKU mapping row.
func (s *Service) DeleteSKUMapping(ctx context.Context, id uuid.UUID, operator *uuid.UUID) error {
	res := s.DB.WithContext(ctx).Delete(&ProductSourceSKU{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	s.logOp(ctx, operator, "sourcing.sku_mapping.delete", id.String(), "")
	return nil
}

// PriceHistory returns recent price/stock snapshots for a source SKU.
func (s *Service) PriceHistory(ctx context.Context, sourceSKUID uuid.UUID, days int) ([]SourcePriceHistory, error) {
	if days <= 0 || days > 365 {
		days = 90
	}
	since := time.Now().UTC().AddDate(0, 0, -days)
	var items []SourcePriceHistory
	if err := s.DB.WithContext(ctx).
		Where("source_sku_id = ? AND captured_at >= ?", sourceSKUID, since).
		Order("captured_at DESC").Limit(500).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListSwitchEvents returns switch audit rows, optionally by product.
func (s *Service) ListSwitchEvents(ctx context.Context, productID *uuid.UUID, page, pageSize int) (map[string]any, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	tx := s.DB.WithContext(ctx).Model(&SourceSwitchEvent{})
	if productID != nil {
		tx = tx.Where("product_id = ?", *productID)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []SourceSwitchEvent
	if err := tx.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	return map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize}, nil
}

// RefreshResult reports one product refresh run.
type RefreshResult struct {
	ProductID uuid.UUID       `json:"productId"`
	Refreshed int             `json:"refreshed"`
	Alerts    []string        `json:"alerts"`
	Switched  *ProductSource  `json:"switched,omitempty"`
	Sources   []ProductSource `json:"sources"`
}

// RefreshProductSources fetches current price/stock via the sourceinfo
// Provider (mock for now), appends price history and applies switch rules.
func (s *Service) RefreshProductSources(ctx context.Context, productID uuid.UUID, operator *uuid.UUID) (*RefreshResult, error) {
	sources, err := s.ListProductSources(ctx, productID)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("%w: no sources bound", ErrNotFound)
	}
	cfg, err := s.ruleConfig(ctx)
	if err != nil {
		return nil, err
	}
	res := &RefreshResult{ProductID: productID}
	now := time.Now().UTC()
	prov := s.provider()
	for i := range sources {
		src := &sources[i]
		if src.Status == SourceStatusDisabled || len(src.SKUs) == 0 {
			continue
		}
		extIDs := make([]string, 0, len(src.SKUs))
		for _, m := range src.SKUs {
			extIDs = append(extIDs, defaultStr(m.ExternalSKUID, m.LocalSKUID.String()))
		}
		quote, err := prov.FetchOffer(ctx, defaultStr(src.SourceOfferID, src.ID.String()), extIDs)
		if err != nil {
			res.Alerts = append(res.Alerts, fmt.Sprintf("source %s fetch failed: %v", src.ID, err))
			continue
		}
		byExt := map[string]sourceinfo.SKUQuote{}
		for _, q := range quote.SKUs {
			byExt[q.ExternalSKUID] = q
		}
		outOfStock := true
		priceAlert := false
		for j := range src.SKUs {
			m := &src.SKUs[j]
			q, ok := byExt[defaultStr(m.ExternalSKUID, m.LocalSKUID.String())]
			if !ok {
				continue
			}
			oldPrice := m.CurrentPrice
			stock := q.Stock
			price := q.Price
			m.CurrentPrice = &price
			m.CurrentStock = &stock
			if err := s.DB.WithContext(ctx).Model(m).
				Updates(map[string]any{"current_price": price, "current_stock": stock}).Error; err != nil {
				return nil, err
			}
			h := SourcePriceHistory{SourceSKUID: m.ID, Price: price, Stock: &stock, CapturedAt: now, CaptureSource: CaptureSourceCrawl}
			if err := s.DB.WithContext(ctx).Create(&h).Error; err != nil {
				return nil, err
			}
			if stock > 0 {
				outOfStock = false
			}
			if oldPrice != nil && *oldPrice > 0 && (price-*oldPrice) / *oldPrice * 100 > cfg.PriceIncreaseThresholdPercent {
				priceAlert = true
			}
			res.Refreshed++
		}
		newStatus := SourceStatusActive
		if outOfStock {
			newStatus = SourceStatusOutOfStock
		} else if priceAlert {
			newStatus = SourceStatusPriceAlert
			res.Alerts = append(res.Alerts, fmt.Sprintf("source %s price increased beyond %.1f%%", src.ID, cfg.PriceIncreaseThresholdPercent))
		}
		if err := s.DB.WithContext(ctx).Model(src).
			Updates(map[string]any{"status": newStatus, "last_checked_at": now}).Error; err != nil {
			return nil, err
		}
		src.Status = newStatus
		src.LastCheckedAt = &now
	}
	switched, alerts, err := s.applySwitchRules(ctx, productID, sources, cfg)
	if err != nil {
		return nil, err
	}
	res.Switched = switched
	res.Alerts = append(res.Alerts, alerts...)
	res.Sources, err = s.ListProductSources(ctx, productID)
	if err != nil {
		return nil, err
	}
	s.logOp(ctx, operator, "sourcing.source.refresh", productID.String(), fmt.Sprintf("%d skus refreshed", res.Refreshed))
	return res, nil
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// extractOfferID pulls the numeric offer id from a 1688 offer URL.
func extractOfferID(rawURL string) string {
	u := strings.TrimSpace(rawURL)
	idx := strings.Index(u, "/offer/")
	if idx < 0 {
		return ""
	}
	rest := u[idx+len("/offer/"):]
	end := strings.IndexFunc(rest, func(r rune) bool { return r < '0' || r > '9' })
	if end == -1 {
		end = len(rest)
	}
	if _, err := strconv.Atoi(rest[:end]); err != nil {
		return ""
	}
	return rest[:end]
}
