package order

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/repository"
)

// maxPrintSheets caps one picking/shipping print request.
const maxPrintSheets = 50

// PrintSheetItem is one SKU line on a picking/shipping sheet.
type PrintSheetItem struct {
	ProductTitle string `json:"productTitle"`
	SKUName      string `json:"skuName,omitempty"`
	SKUCode      string `json:"skuCode,omitempty"`
	SellerSKU    string `json:"sellerSku,omitempty"`
	Quantity     int    `json:"quantity"`
}

// PrintSheetShipment is one waybill line on a picking/shipping sheet.
type PrintSheetShipment struct {
	Carrier     string `json:"carrier"`
	CarrierCode string `json:"carrierCode,omitempty"`
	TrackingNo  string `json:"trackingNo,omitempty"`
	Status      string `json:"status"`
}

// PrintSheet is a manual picking/shipping document for one order (人工贴单
// 用拣货/发货单，不做电子面单).
type PrintSheet struct {
	OrderID       uuid.UUID            `json:"orderId"`
	OrderNo       string               `json:"orderNo"`
	Platform      string               `json:"platform"`
	ShopName      string               `json:"shopName,omitempty"`
	CustomerName  string               `json:"customerName"`
	CustomerPhone string               `json:"customerPhone,omitempty"`
	CustomerEmail string               `json:"customerEmail,omitempty"`
	Remark        string               `json:"remark,omitempty"`
	OrderedAt     *time.Time           `json:"orderedAt,omitempty"`
	Items         []PrintSheetItem     `json:"items"`
	Shipments     []PrintSheetShipment `json:"shipments"`
}

// PrintSheets builds picking/shipping sheets for up to maxPrintSheets orders,
// enforcing tenant + store scope per order.
func (s *Service) PrintSheets(c *gin.Context, ids []uuid.UUID) ([]PrintSheet, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("ids required")
	}
	if len(ids) > maxPrintSheets {
		return nil, fmt.Errorf("too many orders (max %d)", maxPrintSheets)
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	out := make([]PrintSheet, 0, len(ids))
	for _, id := range ids {
		var o Order
		if err := repository.FindByID(c.Request.Context(), s.DB, &o, tid, id); err != nil {
			return nil, err
		}
		if o.ShopID != nil {
			if err := adminperm.EnsureStoreVisible(c, s.DB, o.ShopID); err != nil {
				return nil, err
			}
		}
		sheet := PrintSheet{
			OrderID:       o.ID,
			OrderNo:       o.OrderNo,
			Platform:      o.Platform,
			CustomerName:  o.CustomerName,
			CustomerPhone: o.CustomerPhone,
			CustomerEmail: o.CustomerEmail,
			Remark:        o.Remark,
			OrderedAt:     o.OrderedAt,
			Items:         []PrintSheetItem{},
			Shipments:     []PrintSheetShipment{},
		}
		if o.ShopID != nil && s.Shops != nil {
			if sum, err := s.Shops.GetSummary(c, *o.ShopID); err == nil && sum != nil {
				sheet.ShopName = sum.ShopName
			}
		}
		var items []OrderItem
		_ = s.DB.WithContext(c.Request.Context()).Where("order_id = ?", o.ID).Order("created_at ASC").Find(&items).Error
		for _, it := range items {
			sheet.Items = append(sheet.Items, PrintSheetItem{
				ProductTitle: it.ProductTitle,
				SKUName:      it.SKUName,
				SKUCode:      it.SKUCode,
				SellerSKU:    it.SellerSKU,
				Quantity:     it.Quantity,
			})
		}
		var ships []OrderShipment
		_ = s.DB.WithContext(c.Request.Context()).Where("order_id = ?", o.ID).Order("created_at ASC").Find(&ships).Error
		for _, sh := range ships {
			sheet.Shipments = append(sheet.Shipments, PrintSheetShipment{
				Carrier:     sh.Carrier,
				CarrierCode: sh.CarrierCode,
				TrackingNo:  sh.TrackingNo,
				Status:      sh.Status,
			})
		}
		out = append(out, sheet)
	}
	return out, nil
}
