package order

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// BatchShipmentItem fills one tracking number matched by order number.
type BatchShipmentItem struct {
	OrderNo    string `json:"orderNo"`
	TrackingNo string `json:"trackingNo"`
	Carrier    string `json:"carrier"`
}

// BatchShipmentsBody is POST /orders/shipments/batch.
type BatchShipmentsBody struct {
	Items []BatchShipmentItem `json:"items"`
}

// BatchShipmentLineResult reports the outcome of one pasted line.
type BatchShipmentLineResult struct {
	Key     string `json:"key"` // orderNo used as match key
	OrderID string `json:"orderId,omitempty"`
	OK      bool   `json:"ok"`
	Status  string `json:"status,omitempty"` // resulting order status
	Message string `json:"message,omitempty"`
	// InventoryDeducted is set on succeeded lines only: whether the order
	// already has a successful stock deduction (shipping itself never deducts).
	InventoryDeducted *bool `json:"inventoryDeducted,omitempty"`
}

// BatchShipmentsResult aggregates per-line results.
type BatchShipmentsResult struct {
	Succeeded int                       `json:"succeeded"`
	Failed    int                       `json:"failed"`
	Results   []BatchShipmentLineResult `json:"results"`
}

const batchShipmentsMaxItems = 200

func (r *BatchShipmentsResult) add(line BatchShipmentLineResult) {
	if line.OK {
		r.Succeeded++
	} else {
		r.Failed++
	}
	r.Results = append(r.Results, line)
}

// BatchShipments creates shipped shipments for multiple orders matched by
// order number (the key operators copy from carrier shipping lists), so a
// day's shipping can be pasted back in one action. Each line is processed
// independently: one bad line does not fail the whole batch.
func (s *Service) BatchShipments(c *gin.Context, body BatchShipmentsBody, adminID *uuid.UUID) (*BatchShipmentsResult, error) {
	if len(body.Items) == 0 {
		return nil, fmt.Errorf("items required")
	}
	if len(body.Items) > batchShipmentsMaxItems {
		return nil, fmt.Errorf("too many items (max %d)", batchShipmentsMaxItems)
	}
	res := &BatchShipmentsResult{Results: []BatchShipmentLineResult{}}
	seen := map[string]bool{}
	for _, it := range body.Items {
		no := strings.TrimSpace(it.OrderNo)
		tn := strings.TrimSpace(it.TrackingNo)
		line := BatchShipmentLineResult{Key: no}
		if no == "" || tn == "" {
			line.Message = "订单号与快递单号均不能为空"
			res.add(line)
			continue
		}
		if seen[no] {
			line.Message = "重复的订单号，已跳过"
			res.add(line)
			continue
		}
		seen[no] = true
		tx := s.DB.WithContext(c.Request.Context()).Model(&Order{}).Where("order_no = ?", no)
		scoped, _, err := adminperm.ApplyTenantScope(c, tx)
		if err != nil {
			return nil, err
		}
		var orders []Order
		if err := scoped.Limit(2).Find(&orders).Error; err != nil {
			return nil, err
		}
		switch len(orders) {
		case 0:
			line.Message = "没有找到该订单号对应的销售订单"
			res.add(line)
			continue
		case 1:
			// matched
		default:
			line.Message = "该订单号匹配到多个销售订单，请到订单详情单独发货"
			res.add(line)
			continue
		}
		o := orders[0]
		line.OrderID = o.ID.String()
		if orderTerminalRestoreState(&o) {
			line.Message = "订单已取消/关闭/退款，不能发货"
			res.add(line)
			continue
		}
		if strings.TrimSpace(o.PaymentStatus) != PaymentPaid {
			line.Message = "订单未付款，不能发货"
			res.add(line)
			continue
		}
		carrier := strings.TrimSpace(it.Carrier)
		if carrier == "" {
			carrier = "其他快递"
		}
		if _, err := s.AppendShipment(c, o.ID, OrderShipmentInput{
			Carrier:    carrier,
			TrackingNo: tn,
			Status:     ShipmentShipped,
		}, adminID); err != nil {
			line.Message = err.Error()
			res.add(line)
			continue
		}
		var after Order
		if err := s.DB.WithContext(c.Request.Context()).First(&after, "id = ?", o.ID).Error; err == nil {
			line.Status = after.Status
		}
		line.OK = true
		res.add(line)
	}
	return res, nil
}
