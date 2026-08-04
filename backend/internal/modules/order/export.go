package order

import (
	"bytes"
	"encoding/csv"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/csvsafe"
	"gorm.io/gorm"
)

// shippingCSVHeader is the manual shipping list header. 快递单号/承运商 columns
// stay empty so the operator can fill them offline and paste back into 批量发货.
var shippingCSVHeader = []string{
	"订单号", "客户名", "客户电话", "商品标题", "SKU名称", "数量", "币种", "订单金额", "快递单号(回填)", "承运商(回填)",
}

// ExportShippingListCSV renders one merged shipping list covering several
// sales orders (one row per item, 订单号 column distinguishes orders).
func (s *Service) ExportShippingListCSV(c *gin.Context, ids []uuid.UUID) ([]byte, string, error) {
	if s == nil || s.DB == nil {
		return nil, "", fmt.Errorf("order: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, "", err
	}
	var rows []Order
	if err := s.DB.WithContext(c.Request.Context()).
		Preload("Items").
		Where("tenant_id = ? AND id IN ?", tid, ids).
		Find(&rows).Error; err != nil {
		return nil, "", err
	}
	if len(rows) != len(ids) {
		return nil, "", gorm.ErrRecordNotFound
	}
	byID := make(map[uuid.UUID]*Order, len(rows))
	for i := range rows {
		byID[rows[i].ID] = &rows[i]
	}

	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM for Excel
	w := csv.NewWriter(&buf)
	if err := w.Write(shippingCSVHeader); err != nil {
		return nil, "", err
	}
	for _, id := range ids {
		o := byID[id]
		if o == nil {
			return nil, "", gorm.ErrRecordNotFound
		}
		if len(o.Items) == 0 {
			row := []string{
				o.OrderNo, o.CustomerName, o.CustomerPhone, "", "", "0",
				o.Currency, fmt.Sprintf("%.2f", o.TotalAmount), "", "",
			}
			if err := w.Write(csvsafe.Row(row)); err != nil {
				return nil, "", err
			}
			continue
		}
		for _, it := range o.Items {
			skuName := it.SKUName
			if skuName == "" {
				skuName = it.SellerSKU
			}
			row := []string{
				o.OrderNo,
				o.CustomerName,
				o.CustomerPhone,
				it.ProductTitle,
				skuName,
				fmt.Sprintf("%d", it.Quantity),
				o.Currency,
				fmt.Sprintf("%.2f", o.TotalAmount),
				"",
				"",
			}
			if err := w.Write(csvsafe.Row(row)); err != nil {
				return nil, "", err
			}
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", err
	}
	name := fmt.Sprintf("shipping-list-%d.csv", len(ids))
	return buf.Bytes(), name, nil
}
