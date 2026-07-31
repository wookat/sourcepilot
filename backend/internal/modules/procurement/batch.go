package procurement

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BatchPlacedItem backfills one 1688 order number by purchase order id.
type BatchPlacedItem struct {
	PurchaseOrderID string `json:"purchaseOrderId"`
	ExternalOrderID string `json:"externalOrderId"`
}

// BatchMarkPlacedBody is POST /procurement/orders/batch-mark-placed.
type BatchMarkPlacedBody struct {
	Items []BatchPlacedItem `json:"items"`
}

// BatchLogisticsItem backfills one tracking number matched by 1688 order number.
type BatchLogisticsItem struct {
	ExternalOrderID string `json:"externalOrderId"`
	TrackingNo      string `json:"trackingNo"`
	Carrier         string `json:"carrier"`
}

// BatchLogisticsBody is POST /procurement/orders/batch-logistics.
type BatchLogisticsBody struct {
	Items []BatchLogisticsItem `json:"items"`
}

// BatchLineResult reports the outcome of one backfill line.
type BatchLineResult struct {
	Key             string `json:"key"` // purchaseOrderId or externalOrderId used as match key
	PurchaseOrderID string `json:"purchaseOrderId,omitempty"`
	SupplierName    string `json:"supplierName,omitempty"`
	OK              bool   `json:"ok"`
	Status          string `json:"status,omitempty"` // resulting purchase order status
	Message         string `json:"message,omitempty"`
}

// BatchResult aggregates per-line results.
type BatchResult struct {
	Succeeded int               `json:"succeeded"`
	Failed    int               `json:"failed"`
	Results   []BatchLineResult `json:"results"`
}

const batchMaxItems = 200

func (r *BatchResult) add(line BatchLineResult) {
	if line.OK {
		r.Succeeded++
	} else {
		r.Failed++
	}
	r.Results = append(r.Results, line)
}

// BatchMarkPlaced backfills 1688 order numbers for multiple purchase orders.
// Each line is processed independently: one illegal transition does not fail
// the whole batch.
func (s *Service) BatchMarkPlaced(ctx context.Context, body BatchMarkPlacedBody, operator *uuid.UUID) (*BatchResult, error) {
	if len(body.Items) == 0 {
		return nil, fmt.Errorf("%w: items required", ErrBadRequest)
	}
	if len(body.Items) > batchMaxItems {
		return nil, fmt.Errorf("%w: too many items (max %d)", ErrBadRequest, batchMaxItems)
	}
	res := &BatchResult{Results: []BatchLineResult{}}
	seen := map[string]bool{}
	for _, it := range body.Items {
		key := strings.TrimSpace(it.PurchaseOrderID)
		ext := strings.TrimSpace(it.ExternalOrderID)
		line := BatchLineResult{Key: key}
		if key == "" || ext == "" {
			line.Message = "采购单与 1688 订单号均不能为空"
			res.add(line)
			continue
		}
		if seen[key] {
			line.Message = "重复的采购单，已跳过"
			res.add(line)
			continue
		}
		seen[key] = true
		id, err := uuid.Parse(key)
		if err != nil {
			line.Message = "采购单 ID 无效"
			res.add(line)
			continue
		}
		po, err := s.MarkPlaced(ctx, id, MarkPlacedBody{ExternalOrderID: ext}, operator)
		if err != nil {
			line.Message = batchErrMessage(err)
			res.add(line)
			continue
		}
		line.OK = true
		line.PurchaseOrderID = po.ID.String()
		line.SupplierName = po.SupplierName
		line.Status = po.Status
		res.add(line)
	}
	s.logOp(ctx, operator, "procurement.batch_mark_placed", "", fmt.Sprintf("%d ok, %d failed", res.Succeeded, res.Failed))
	return res, nil
}

// BatchFillLogistics backfills tracking numbers, matching purchase orders by
// their backfilled 1688 order number (the same key printed on 1688 shipping
// pages, so operators can paste "订单号 运单号 [快递]" lines directly).
func (s *Service) BatchFillLogistics(ctx context.Context, body BatchLogisticsBody, operator *uuid.UUID) (*BatchResult, error) {
	if len(body.Items) == 0 {
		return nil, fmt.Errorf("%w: items required", ErrBadRequest)
	}
	if len(body.Items) > batchMaxItems {
		return nil, fmt.Errorf("%w: too many items (max %d)", ErrBadRequest, batchMaxItems)
	}
	res := &BatchResult{Results: []BatchLineResult{}}
	seen := map[string]bool{}
	for _, it := range body.Items {
		ext := strings.TrimSpace(it.ExternalOrderID)
		tn := strings.TrimSpace(it.TrackingNo)
		line := BatchLineResult{Key: ext}
		if ext == "" || tn == "" {
			line.Message = "1688 订单号与运单号均不能为空"
			res.add(line)
			continue
		}
		if seen[ext] {
			line.Message = "重复的 1688 订单号，已跳过"
			res.add(line)
			continue
		}
		seen[ext] = true
		var pos []PurchaseOrder
		if err := s.DB.WithContext(ctx).Where("external_order_id = ?", ext).Limit(2).Find(&pos).Error; err != nil {
			return nil, err
		}
		switch len(pos) {
		case 0:
			line.Message = "没有找到该 1688 订单号对应的采购单，请先回填订单号"
			res.add(line)
			continue
		case 1:
			// matched
		default:
			line.Message = "该 1688 订单号匹配到多个采购单，请到采购单详情单独回填"
			res.add(line)
			continue
		}
		po := pos[0]
		line.PurchaseOrderID = po.ID.String()
		line.SupplierName = po.SupplierName
		if po.Status == StatusPlaced {
			// Operators usually paste tracking numbers after paying on 1688;
			// auto mark-paid keeps the state machine intact for this flow.
			if _, err := s.MarkPaid(ctx, po.ID, MarkPaidBody{}, operator); err != nil {
				line.Message = batchErrMessage(err)
				res.add(line)
				continue
			}
		}
		updated, err := s.FillLogistics(ctx, po.ID, LogisticsBody{TrackingNo: tn, Carrier: it.Carrier}, operator)
		if err != nil {
			line.Message = batchErrMessage(err)
			res.add(line)
			continue
		}
		line.OK = true
		line.Status = updated.Status
		res.add(line)
	}
	s.logOp(ctx, operator, "procurement.batch_logistics", "", fmt.Sprintf("%d ok, %d failed", res.Succeeded, res.Failed))
	return res, nil
}

func batchErrMessage(err error) string {
	switch {
	case errors.Is(err, ErrNotFound):
		return "采购单不存在"
	case errors.Is(err, ErrConflict), errors.Is(err, ErrBadRequest):
		msg := err.Error()
		if i := strings.Index(msg, ": "); i >= 0 {
			msg = msg[i+2:]
		}
		return msg
	case errors.Is(err, gorm.ErrRecordNotFound):
		return "采购单不存在"
	default:
		return err.Error()
	}
}
