package procurement

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
	"github.com/trademind-ai/trademind/backend/internal/providers/trade"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Domain errors mapped to 4xx by the handler.
var (
	ErrNotFound   = errors.New("procurement: not found")
	ErrBadRequest = errors.New("procurement: bad request")
	ErrConflict   = errors.New("procurement: conflict")
)

// Service implements purchase-order collaboration (manual-order mode).
type Service struct {
	DB       *gorm.DB
	OpLog    *operationlog.Service
	Provider trade.Provider
}

func (s *Service) provider() trade.Provider {
	if s.Provider != nil {
		return s.Provider
	}
	return trade.NewMock1688()
}

func (s *Service) logOp(ctx context.Context, operator *uuid.UUID, action, targetID, detail string) {
	if s.OpLog == nil {
		return
	}
	_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
		AdminUserID: operator,
		Action:      action,
		Resource:    "procurement",
		ResourceID:  targetID,
		Status:      "success",
		Message:     detail,
	})
}

func (s *Service) writeEvent(tx *gorm.DB, poID uuid.UUID, from, to, source string, payload any) error {
	ev := PurchaseOrderEvent{PurchaseOrderID: poID, FromStatus: from, ToStatus: to, Source: source}
	if payload != nil {
		if b, err := json.Marshal(payload); err == nil {
			ev.Payload = datatypes.JSON(b)
		}
	}
	return tx.Create(&ev).Error
}

// transition moves a purchase order along the state machine inside one tx.
func (s *Service) transition(ctx context.Context, id uuid.UUID, to, source string, payload any, mutate func(tx *gorm.DB, po *PurchaseOrder) error) (*PurchaseOrder, error) {
	var po PurchaseOrder
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&po, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if !CanTransition(po.Status, to) {
			return ErrIllegalTransition(po.Status, to)
		}
		from := po.Status
		po.Status = to
		if mutate != nil {
			if err := mutate(tx, &po); err != nil {
				return err
			}
		}
		if err := tx.Save(&po).Error; err != nil {
			return err
		}
		return s.writeEvent(tx, po.ID, from, to, source, payload)
	})
	if err != nil {
		return nil, err
	}
	return &po, nil
}

// GenerateBody creates purchase orders from sales orders.
type GenerateBody struct {
	OrderIDs       []string       `json:"orderIds"`
	Receiver       map[string]any `json:"receiver"`
	IdempotencyKey string         `json:"idempotencyKey"`
}

// Blocker is one unmet precondition (e.g. missing SKU mapping).
type Blocker struct {
	OrderID    string `json:"orderId"`
	LocalSKUID string `json:"localSkuId,omitempty"`
	SKUName    string `json:"skuName,omitempty"`
	Code       string `json:"code"` // sku.unmatched|source.missing|mapping.missing
	Message    string `json:"message"`
}

// GenerateResult reports created purchase orders plus blockers and warnings.
type GenerateResult struct {
	Orders   []PurchaseOrder `json:"orders"`
	Blockers []Blocker       `json:"blockers"`
	Warnings []Blocker       `json:"warnings,omitempty"`
}

type aggLine struct {
	item     PurchaseOrderItem
	supplier sourcing.Supplier
}

// Generate builds supplier-aggregated purchase orders from sales orders.
// Lines lacking a local SKU link, a primary source or a SKU mapping are
// returned as blockers instead of failing the whole batch.
func (s *Service) Generate(ctx context.Context, body GenerateBody, operator *uuid.UUID) (*GenerateResult, error) {
	if len(body.OrderIDs) == 0 {
		return nil, fmt.Errorf("%w: orderIds required", ErrBadRequest)
	}
	orderIDs := make([]uuid.UUID, 0, len(body.OrderIDs))
	for _, raw := range body.OrderIDs {
		u, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("%w: invalid order id %q", ErrBadRequest, raw)
		}
		orderIDs = append(orderIDs, u)
	}
	idemKey := strings.TrimSpace(body.IdempotencyKey)
	if idemKey == "" {
		idemKey = uuid.NewString()
	}
	var existing []PurchaseOrder
	if err := s.DB.WithContext(ctx).Where("idempotency_key LIKE ?", idemKey+"%").Find(&existing).Error; err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return &GenerateResult{Orders: existing}, nil
	}

	res := &GenerateResult{}
	bySupplier := map[uuid.UUID][]aggLine{}

	for _, oid := range orderIDs {
		var items []order.OrderItem
		if err := s.DB.WithContext(ctx).Where("order_id = ?", oid).Find(&items).Error; err != nil {
			return nil, err
		}
		if len(items) == 0 {
			res.Blockers = append(res.Blockers, Blocker{OrderID: oid.String(), Code: "order.empty", Message: "订单不存在或没有商品行"})
			continue
		}
		for _, it := range items {
			if it.ProductSKUID == nil || it.ProductID == nil {
				res.Blockers = append(res.Blockers, Blocker{
					OrderID: oid.String(), SKUName: it.SKUName,
					Code: "sku.unmatched", Message: "订单行未匹配本地 SKU，请先完成 SKU 匹配",
				})
				continue
			}
			var primary sourcing.ProductSource
			err := s.DB.WithContext(ctx).Preload("Supplier").
				Where("product_id = ? AND is_primary = TRUE AND status <> ?", *it.ProductID, sourcing.SourceStatusDisabled).
				First(&primary).Error
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					res.Blockers = append(res.Blockers, Blocker{
						OrderID: oid.String(), LocalSKUID: it.ProductSKUID.String(), SKUName: it.SKUName,
						Code: "source.missing", Message: "商品没有主货源，请先在货源档案绑定",
					})
					continue
				}
				return nil, err
			}
			var mapping sourcing.ProductSourceSKU
			err = s.DB.WithContext(ctx).
				Where("product_source_id = ? AND local_sku_id = ?", primary.ID, *it.ProductSKUID).
				First(&mapping).Error
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					res.Blockers = append(res.Blockers, Blocker{
						OrderID: oid.String(), LocalSKUID: it.ProductSKUID.String(), SKUName: it.SKUName,
						Code: "mapping.missing", Message: "主货源缺少该 SKU 的映射，请先补全 SKU 映射",
					})
					continue
				}
				return nil, err
			}
			var localSKU product.ProductSKU
			skuName := it.SKUName
			title := it.ProductTitle
			if err := s.DB.WithContext(ctx).First(&localSKU, "id = ?", *it.ProductSKUID).Error; err == nil {
				if skuName == "" {
					skuName = localSKU.SKUName
				}
			}
			if title == "" && it.ProductID != nil {
				var prod product.Product
				if err := s.DB.WithContext(ctx).Select("title").First(&prod, "id = ?", *it.ProductID).Error; err == nil {
					title = prod.Title
				}
			}
			expected := mapping.CurrentPrice
			if expected == nil {
				var hist sourcing.SourcePriceHistory
				if err := s.DB.WithContext(ctx).
					Where("source_sku_id = ?", mapping.ID).
					Order("captured_at DESC").
					First(&hist).Error; err == nil {
					p := hist.Price
					expected = &p
				}
			}
			if expected == nil {
				res.Warnings = append(res.Warnings, Blocker{
					OrderID: oid.String(), LocalSKUID: it.ProductSKUID.String(), SKUName: it.SKUName,
					Code: "price.missing", Message: "SKU 缺少参考进价，采购单金额将不含该行，可在确认前补填",
				})
			}
			salesID := oid
			line := PurchaseOrderItem{
				SalesOrderID:    &salesID,
				LocalSKUID:      *it.ProductSKUID,
				SourceSKUID:     mapping.ID,
				ExternalOfferID: primary.SourceOfferID,
				ExternalSKUID:   mapping.ExternalSKUID,
				SourceURL:       primary.SourceURL,
				ProductTitle:    title,
				SKUName:         skuName,
				Quantity:        it.Quantity,
				ExpectedPrice:   expected,
			}
			if primary.Supplier == nil {
				res.Blockers = append(res.Blockers, Blocker{
					OrderID: oid.String(), LocalSKUID: it.ProductSKUID.String(), SKUName: it.SKUName,
					Code: "source.missing", Message: "主货源供应商缺失",
				})
				continue
			}
			bySupplier[primary.SupplierID] = append(bySupplier[primary.SupplierID], aggLine{item: line, supplier: *primary.Supplier})
		}
	}

	if len(bySupplier) == 0 {
		return res, nil
	}

	receiverJSON := datatypes.JSON([]byte(`{}`))
	if body.Receiver != nil {
		if b, err := json.Marshal(body.Receiver); err == nil {
			receiverJSON = datatypes.JSON(b)
		}
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		idx := 0
		for supplierID, lines := range bySupplier {
			idx++
			total := 0.0
			for _, l := range lines {
				if l.item.ExpectedPrice != nil {
					total += *l.item.ExpectedPrice * float64(l.item.Quantity)
				}
			}
			po := PurchaseOrder{
				SupplierID:      supplierID,
				SupplierName:    lines[0].supplier.Name,
				SourcePlatform:  lines[0].supplier.Platform,
				Status:          StatusDraft,
				TotalAmount:     round2(total),
				Currency:        "CNY",
				PayStatus:       PayStatusUnpaid,
				Receiver:        receiverJSON,
				IdempotencyKey:  fmt.Sprintf("%s:%d", idemKey, idx),
				ConfirmRequired: true,
			}
			if err := tx.Create(&po).Error; err != nil {
				return err
			}
			for _, l := range lines {
				l.item.PurchaseOrderID = po.ID
				if err := tx.Create(&l.item).Error; err != nil {
					return err
				}
			}
			if err := s.writeEvent(tx, po.ID, "", StatusDraft, EventSourceSystem, map[string]any{"orderIds": body.OrderIDs}); err != nil {
				return err
			}
			po.Items = nil
			res.Orders = append(res.Orders, po)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logOp(ctx, operator, "procurement.generate", idemKey, fmt.Sprintf("%d purchase orders, %d blockers", len(res.Orders), len(res.Blockers)))
	return res, nil
}

func round2(v float64) float64 { return float64(int(v*100+0.5)) / 100 }

// UpdateItemPrice sets an item's expected price before the order is confirmed
// (draft / pending_confirm only) and recomputes the order total.
func (s *Service) UpdateItemPrice(ctx context.Context, poID, itemID uuid.UUID, price float64, operator *uuid.UUID) (*PurchaseOrder, error) {
	if price <= 0 {
		return nil, fmt.Errorf("%w: expectedPrice must be greater than 0", ErrBadRequest)
	}
	var po PurchaseOrder
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&po, "id = ?", poID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if po.Status != StatusDraft && po.Status != StatusPendingConfirm {
			return fmt.Errorf("%w: 仅草稿或待确认状态可修改参考价（当前 %s）", ErrConflict, po.Status)
		}
		var item PurchaseOrderItem
		if err := tx.First(&item, "id = ? AND purchase_order_id = ?", itemID, poID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if err := tx.Model(&item).Update("expected_price", price).Error; err != nil {
			return err
		}
		var items []PurchaseOrderItem
		if err := tx.Where("purchase_order_id = ?", poID).Find(&items).Error; err != nil {
			return err
		}
		total := 0.0
		for _, it := range items {
			if it.ID == itemID {
				total += price * float64(it.Quantity)
				continue
			}
			if it.ExpectedPrice != nil {
				total += *it.ExpectedPrice * float64(it.Quantity)
			}
		}
		po.TotalAmount = round2(total)
		return tx.Model(&po).Update("total_amount", po.TotalAmount).Error
	})
	if err != nil {
		return nil, err
	}
	s.logOp(ctx, operator, "procurement.item.update_price", poID.String(), fmt.Sprintf("item %s price %.2f", itemID, price))
	return s.Detail(ctx, poID)
}

// ListQuery filters purchase orders.
type ListQuery struct {
	Page       int
	PageSize   int
	Status     string
	SupplierID string
	Keyword    string
}

// ListResult is a paginated purchase-order page.
type ListResult struct {
	Items    []PurchaseOrder `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
}

// List returns purchase orders.
func (s *Service) List(ctx context.Context, q ListQuery) (*ListResult, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 200 {
		q.PageSize = 20
	}
	tx := s.DB.WithContext(ctx).Model(&PurchaseOrder{})
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}
	if q.SupplierID != "" {
		tx = tx.Where("supplier_id = ?", q.SupplierID)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		like := "%" + kw + "%"
		tx = tx.Where("supplier_name ILIKE ? OR external_order_id ILIKE ?", like, like)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []PurchaseOrder
	if err := tx.Order("created_at DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	return &ListResult{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// Detail returns one purchase order with items, events and logistics.
func (s *Service) Detail(ctx context.Context, id uuid.UUID) (*PurchaseOrder, error) {
	var po PurchaseOrder
	err := s.DB.WithContext(ctx).
		Preload("Items").
		Preload("Events", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC") }).
		Preload("Logistics").
		First(&po, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &po, nil
}

// Submit re-checks price/stock via the trade provider and moves draft →
// pending_confirm.
func (s *Service) Submit(ctx context.Context, id uuid.UUID, operator *uuid.UUID) (*PurchaseOrder, error) {
	var items []PurchaseOrderItem
	if err := s.DB.WithContext(ctx).Where("purchase_order_id = ?", id).Find(&items).Error; err != nil {
		return nil, err
	}
	req := trade.PreviewRequest{}
	for _, it := range items {
		exp := 0.0
		if it.ExpectedPrice != nil {
			exp = *it.ExpectedPrice
		}
		req.Items = append(req.Items, trade.PreviewItem{
			OfferID:       it.ExternalOfferID,
			ExternalSKUID: it.ExternalSKUID,
			Quantity:      it.Quantity,
			ExpectedPrice: exp,
		})
	}
	preview, err := s.provider().PreviewOrder(ctx, req)
	if err != nil {
		return nil, err
	}
	po, err := s.transition(ctx, id, StatusPendingConfirm, EventSourceSystem, map[string]any{"preview": preview}, func(tx *gorm.DB, po *PurchaseOrder) error {
		if preview != nil && preview.TotalAmount > 0 {
			po.TotalAmount = round2(preview.TotalAmount)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logOp(ctx, operator, "procurement.submit", id.String(), "")
	return po, nil
}

// Confirm records manual approval and moves pending_confirm → placing. The
// mock provider returns Manual=true, so the order stays in placing until the
// operator orders on 1688 and backfills the order number via MarkPlaced.
func (s *Service) Confirm(ctx context.Context, id uuid.UUID, operator *uuid.UUID) (*PurchaseOrder, error) {
	po, err := s.transition(ctx, id, StatusPlacing, EventSourceManual, nil, func(tx *gorm.DB, po *PurchaseOrder) error {
		now := time.Now().UTC()
		po.ConfirmedBy = operator
		po.ConfirmedAt = &now
		var items []PurchaseOrderItem
		if err := tx.Where("purchase_order_id = ?", po.ID).Find(&items).Error; err != nil {
			return err
		}
		req := trade.CreateOrderRequest{IdempotencyKey: po.IdempotencyKey}
		for _, it := range items {
			exp := 0.0
			if it.ExpectedPrice != nil {
				exp = *it.ExpectedPrice
			}
			req.Items = append(req.Items, trade.PreviewItem{
				OfferID:       it.ExternalOfferID,
				ExternalSKUID: it.ExternalSKUID,
				Quantity:      it.Quantity,
				ExpectedPrice: exp,
			})
		}
		if b, err := json.Marshal(req); err == nil {
			po.RawCreateReq = datatypes.JSON(b)
		}
		resp, err := s.provider().CreateOrder(ctx, req)
		if err != nil {
			return err
		}
		if b, err := json.Marshal(resp); err == nil {
			po.RawCreateResp = datatypes.JSON(b)
		}
		if resp.ExternalOrderID != "" {
			po.ExternalOrderID = resp.ExternalOrderID
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logOp(ctx, operator, "procurement.confirm", id.String(), "")
	return po, nil
}

// MarkPlacedBody backfills the manually placed 1688 order.
type MarkPlacedBody struct {
	ExternalOrderID string             `json:"externalOrderId"`
	ActualPrices    map[string]float64 `json:"actualPrices"` // item id → actual unit price
}

// MarkPlaced moves placing → placed, stores the 1688 order number, records
// actual prices and appends order-sourced price history.
func (s *Service) MarkPlaced(ctx context.Context, id uuid.UUID, body MarkPlacedBody, operator *uuid.UUID) (*PurchaseOrder, error) {
	ext := strings.TrimSpace(body.ExternalOrderID)
	if ext == "" {
		return nil, fmt.Errorf("%w: externalOrderId required", ErrBadRequest)
	}
	po, err := s.transition(ctx, id, StatusPlaced, EventSourceManual, map[string]any{"externalOrderId": ext}, func(tx *gorm.DB, po *PurchaseOrder) error {
		po.ExternalOrderID = ext
		var items []PurchaseOrderItem
		if err := tx.Where("purchase_order_id = ?", po.ID).Find(&items).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		total := 0.0
		for i := range items {
			it := &items[i]
			if p, ok := body.ActualPrices[it.ID.String()]; ok && p > 0 {
				it.ActualPrice = &p
				if err := tx.Model(it).Update("actual_price", p).Error; err != nil {
					return err
				}
				h := sourcing.SourcePriceHistory{
					SourceSKUID:   it.SourceSKUID,
					Price:         p,
					CapturedAt:    now,
					CaptureSource: sourcing.CaptureSourceOrder,
				}
				if err := tx.Create(&h).Error; err != nil {
					return err
				}
			}
			price := it.ExpectedPrice
			if it.ActualPrice != nil {
				price = it.ActualPrice
			}
			if price != nil {
				total += *price * float64(it.Quantity)
			}
		}
		if total > 0 {
			po.TotalAmount = round2(total)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if mock, ok := s.provider().(*trade.Mock1688); ok {
		mock.RegisterManualOrder(ext, po.TotalAmount)
	}
	s.logOp(ctx, operator, "procurement.mark_placed", id.String(), ext)
	return po, nil
}

// MarkPaidBody backfills manual payment info.
type MarkPaidBody struct {
	PayChannel string `json:"payChannel"`
}

// MarkPaid moves placed → paid (manual payment fallback per design 3.4).
func (s *Service) MarkPaid(ctx context.Context, id uuid.UUID, body MarkPaidBody, operator *uuid.UUID) (*PurchaseOrder, error) {
	po, err := s.transition(ctx, id, StatusPaid, EventSourceManual, map[string]any{"payChannel": body.PayChannel}, func(tx *gorm.DB, po *PurchaseOrder) error {
		now := time.Now().UTC()
		po.PayStatus = PayStatusPaid
		po.PayChannel = defaultStr(strings.TrimSpace(body.PayChannel), "manual")
		po.PaidAt = &now
		return nil
	})
	if err != nil {
		return nil, err
	}
	if mock, ok := s.provider().(*trade.Mock1688); ok && po.ExternalOrderID != "" {
		_ = mock.AdvancePay(po.ExternalOrderID)
	}
	s.logOp(ctx, operator, "procurement.mark_paid", id.String(), "")
	return po, nil
}

// LogisticsBody backfills tracking info.
type LogisticsBody struct {
	TrackingNo string `json:"trackingNo"`
	Carrier    string `json:"carrier"`
}

// FillLogistics moves paid → shipped and stores the tracking record.
func (s *Service) FillLogistics(ctx context.Context, id uuid.UUID, body LogisticsBody, operator *uuid.UUID) (*PurchaseOrder, error) {
	tn := strings.TrimSpace(body.TrackingNo)
	if tn == "" {
		return nil, fmt.Errorf("%w: trackingNo required", ErrBadRequest)
	}
	po, err := s.transition(ctx, id, StatusShipped, EventSourceManual, map[string]any{"trackingNo": tn}, func(tx *gorm.DB, po *PurchaseOrder) error {
		lg := PurchaseLogistics{
			PurchaseOrderID: po.ID,
			TrackingNo:      tn,
			Carrier:         strings.TrimSpace(body.Carrier),
			Status:          "in_transit",
		}
		return tx.Create(&lg).Error
	})
	if err != nil {
		return nil, err
	}
	if mock, ok := s.provider().(*trade.Mock1688); ok && po.ExternalOrderID != "" {
		_ = mock.AdvanceShip(po.ExternalOrderID, body.Carrier, tn)
	}
	s.logOp(ctx, operator, "procurement.fill_logistics", id.String(), tn)
	return po, nil
}

// MarkDelivered moves shipped → delivered (cloud warehouse inbound).
func (s *Service) MarkDelivered(ctx context.Context, id uuid.UUID, operator *uuid.UUID) (*PurchaseOrder, error) {
	po, err := s.transition(ctx, id, StatusDelivered, EventSourceManual, nil, func(tx *gorm.DB, po *PurchaseOrder) error {
		now := time.Now().UTC()
		return tx.Model(&PurchaseLogistics{}).
			Where("purchase_order_id = ?", po.ID).
			Updates(map[string]any{"status": "delivered", "inbound_at": now}).Error
	})
	if err != nil {
		return nil, err
	}
	s.logOp(ctx, operator, "procurement.mark_delivered", id.String(), "")
	return po, nil
}

// Cancel aborts a not-yet-shipped purchase order.
func (s *Service) Cancel(ctx context.Context, id uuid.UUID, reason string, operator *uuid.UUID) (*PurchaseOrder, error) {
	po, err := s.transition(ctx, id, StatusCancelled, EventSourceManual, map[string]any{"reason": reason}, nil)
	if err != nil {
		return nil, err
	}
	if po.ExternalOrderID != "" {
		_ = s.provider().CancelOrder(ctx, po.ExternalOrderID, reason)
	}
	s.logOp(ctx, operator, "procurement.cancel", id.String(), reason)
	return po, nil
}

// Retry moves failed → placing for another manual attempt.
func (s *Service) Retry(ctx context.Context, id uuid.UUID, operator *uuid.UUID) (*PurchaseOrder, error) {
	po, err := s.transition(ctx, id, StatusPlacing, EventSourceManual, nil, func(tx *gorm.DB, po *PurchaseOrder) error {
		if po.RetryCount >= po.MaxRetries {
			return fmt.Errorf("%w: retry limit reached", ErrConflict)
		}
		po.RetryCount++
		po.ErrorMessage = ""
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logOp(ctx, operator, "procurement.retry", id.String(), "")
	return po, nil
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
