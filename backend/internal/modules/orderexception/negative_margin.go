package orderexception

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

// negativeMarginScanLimit bounds how many candidate orders are cost-estimated
// per listing (most recently updated first).
const negativeMarginScanLimit = 200

// collectNegativeMargin surfaces paid, not-yet-fulfilled orders whose
// estimated gross profit (primary-source reference cost vs. order amount)
// is negative, so they can be reviewed before procurement/shipping.
func (s *Service) collectNegativeMargin(ctx context.Context, req ListOrderExceptionsRequest) ([]aggRow, error) {
	if s == nil || s.DB == nil || s.Cost == nil {
		return nil, nil
	}
	mig := s.DB.Migrator()
	if !mig.HasTable("orders") || !mig.HasTable("order_items") {
		return nil, nil
	}

	q := s.DB.WithContext(ctx).Model(&order.Order{}).
		Where("payment_status = ?", order.PaymentPaid).
		Where("status NOT IN (?, ?, ?)", order.StatusCancelled, order.StatusRefunded, order.StatusClosed).
		Where("fulfillment_status = ?", order.FulfillmentUnfulfilled)
	if req.Platform != "" {
		q = q.Where("LOWER(platform) = ?", strings.ToLower(strings.TrimSpace(req.Platform)))
	}
	if req.ShopID != "" {
		if sid, err := uuid.Parse(strings.TrimSpace(req.ShopID)); err == nil {
			q = q.Where("shop_id = ?", sid)
		}
	}
	if req.OrderID != "" {
		if oid, err := uuid.Parse(strings.TrimSpace(req.OrderID)); err == nil {
			q = q.Where("id = ?", oid)
		}
	}
	var orders []order.Order
	if err := q.Order("updated_at DESC").Limit(negativeMarginScanLimit).Find(&orders).Error; err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return nil, nil
	}

	ids := make([]uuid.UUID, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.ID)
	}
	estimates, err := s.Cost.EstimateOrderCostBatch(ctx, ids)
	if err != nil {
		return nil, err
	}

	out := make([]aggRow, 0)
	for _, o := range orders {
		est, ok := estimates[o.ID.String()]
		if !ok || est.GrossProfit == nil || *est.GrossProfit >= 0 {
			continue
		}
		extOid := ""
		if o.ExternalOrderID != nil {
			extOid = strings.TrimSpace(*o.ExternalOrderID)
		}
		msg := fmt.Sprintf("订单预估毛利为负：售价 %.2f %s，预估采购成本 %.2f CNY，预估毛利 %.2f %s",
			o.TotalAmount, o.Currency, est.EstimatedCostCNY, *est.GrossProfit, o.Currency)
		if est.MarginPercent != nil {
			msg += fmt.Sprintf("（毛利率 %.2f%%）", *est.MarginPercent)
		}
		out = append(out, aggRow{
			exceptionType:   TypeNegativeMargin,
			severity:        SeverityHigh,
			sourceType:      SourceOrder,
			sourceID:        o.ID,
			orderID:         o.ID,
			platform:        strings.TrimSpace(o.Platform),
			shopID:          o.ShopID,
			orderNo:         strings.TrimSpace(o.OrderNo),
			externalOrderID: extOid,
			errorMessage:    msg,
			suggestedAction: "请在发货前复核：调整售价、切换更低价货源，或取消/关闭该订单避免亏损。",
			createdAt:       o.CreatedAt,
			updatedAt:       o.UpdatedAt,
		})
	}
	return out, nil
}
