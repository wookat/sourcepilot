package migrationimport

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/csvsafe"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// MaxExportRows caps one full CSV export (defensive bound, not pagination).
const MaxExportRows = 50000

const exportBatchSize = 500

func exportTimeText(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func exportFloatText(f *float64) string {
	if f == nil {
		return ""
	}
	return strconv.FormatFloat(*f, 'f', -1, 64)
}

func exportIntText(n *int) string {
	if n == nil {
		return ""
	}
	return strconv.Itoa(*n)
}

// ExportCSV GET /imports/export/:kind — unified full-data CSV export for
// migrating out (product drafts / orders / inventory / source archives).
func (h *Handler) ExportCSV(c *gin.Context) {
	kind, err := normalizeKind(c.Param("kind"))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=trademind-export-%s.csv", kind))
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM for Excel
	w := csv.NewWriter(c.Writer)
	defer w.Flush()
	switch kind {
	case KindProduct:
		err = h.Svc.exportProducts(c, w)
	case KindOrder:
		err = h.Svc.exportOrders(c, w)
	case KindInventory:
		err = h.Svc.exportInventory(c, w)
	case KindSource:
		err = h.Svc.exportSources(c, w)
	case KindPayment:
		err = h.Svc.exportPayments(c, w)
	}
	if err != nil {
		// Headers are already sent; append an error row instead of a broken 200.
		_ = w.Write(csvsafe.Row([]string{"导出中断", err.Error()}))
	}
}

// exportProducts writes all tenant product drafts, one row per SKU (products
// without SKUs still get one row). Currency is the per-product currency.
func (s *Service) exportProducts(c *gin.Context, w *csv.Writer) error {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	if err := w.Write([]string{
		"商品ID", "商品名称", "状态", "币种", "SKU编码", "规格名称", "售价", "成本价", "库存数量", "商品描述", "来源", "来源链接",
	}); err != nil {
		return err
	}
	written := 0
	for offset := 0; written < MaxExportRows; offset += exportBatchSize {
		var products []product.Product
		if err := s.DB.WithContext(c.Request.Context()).
			Preload("SKUs").
			Where("tenant_id = ?", tid).
			Order("created_at ASC, id ASC").
			Offset(offset).Limit(exportBatchSize).Find(&products).Error; err != nil {
			return err
		}
		if len(products) == 0 {
			return nil
		}
		for _, p := range products {
			skus := p.SKUs
			if len(skus) == 0 {
				skus = []product.ProductSKU{{}}
			}
			for _, sku := range skus {
				if written >= MaxExportRows {
					return nil
				}
				if err := w.Write(csvsafe.Row([]string{
					p.ID.String(), p.Title, p.Status, p.Currency,
					sku.SKUCode, sku.SKUName,
					exportFloatText(sku.Price), exportFloatText(sku.CostPrice), exportIntText(sku.Stock),
					p.Description, p.Source, p.SourceURL,
				})); err != nil {
					return err
				}
				written++
			}
		}
	}
	return nil
}

// exportOrders writes all store-visible orders, one row per line item.
// Currency is the per-order currency.
func (s *Service) exportOrders(c *gin.Context, w *csv.Writer) error {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	if err := w.Write([]string{
		"订单号", "平台订单号", "平台", "收件人", "电话", "邮箱", "订单状态", "支付状态", "履约状态",
		"币种", "订单金额", "商品名称", "SKU编码", "规格名称", "数量", "单价", "小计", "下单时间", "付款时间",
	}); err != nil {
		return err
	}
	written := 0
	for offset := 0; written < MaxExportRows; offset += exportBatchSize {
		tx := s.DB.WithContext(c.Request.Context()).Model(&order.Order{}).Where("tenant_id = ?", tid)
		tx, err := adminperm.ApplyStoreScope(c, s.DB, tx, "shop_id")
		if err != nil {
			return err
		}
		var orders []order.Order
		if err := tx.Preload("Items").
			Order("created_at ASC, id ASC").
			Offset(offset).Limit(exportBatchSize).Find(&orders).Error; err != nil {
			return err
		}
		if len(orders) == 0 {
			return nil
		}
		for _, o := range orders {
			extID := ""
			if o.ExternalOrderID != nil {
				extID = *o.ExternalOrderID
			}
			items := o.Items
			if len(items) == 0 {
				items = []order.OrderItem{{}}
			}
			for _, it := range items {
				if written >= MaxExportRows {
					return nil
				}
				if err := w.Write(csvsafe.Row([]string{
					o.OrderNo, extID, o.Platform, o.CustomerName, o.CustomerPhone, o.CustomerEmail,
					o.Status, o.PaymentStatus, o.FulfillmentStatus,
					o.Currency, strconv.FormatFloat(o.TotalAmount, 'f', -1, 64),
					it.ProductTitle, it.SKUCode, it.SKUName,
					strconv.Itoa(it.Quantity), strconv.FormatFloat(it.UnitPrice, 'f', -1, 64),
					strconv.FormatFloat(it.TotalPrice, 'f', -1, 64),
					exportTimeText(o.OrderedAt), exportTimeText(o.PaidAt),
				})); err != nil {
					return err
				}
				written++
			}
		}
	}
	return nil
}

type inventoryExportRow struct {
	SKUID         uuid.UUID `gorm:"column:sku_id"`
	SKUCode       string    `gorm:"column:sku_code"`
	SKUName       string    `gorm:"column:sku_name"`
	Title         string    `gorm:"column:title"`
	Currency      string    `gorm:"column:currency"`
	TotalStock    *int      `gorm:"column:total_stock"`
	CostPrice     *float64  `gorm:"column:cost_price"`
	WarehouseCode string    `gorm:"column:warehouse_code"`
	WarehouseName string    `gorm:"column:warehouse_name"`
	NonDefault    *int      `gorm:"column:non_default_stock"`
}

// exportInventory writes one row per SKU × warehouse. The default warehouse
// stock is derived: total SKU stock minus the sum of non-default warehouse
// lines (same semantics as the inventory center); SKUs without any warehouse
// rows export the total as the default line.
func (s *Service) exportInventory(c *gin.Context, w *csv.Writer) error {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	if err := w.Write([]string{
		"SKU编码", "商品名称", "规格名称", "仓库编码", "仓库名称", "库存数量", "参考进价", "币种",
	}); err != nil {
		return err
	}
	var defWh inventory.Warehouse
	hasDefault := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND is_default = ?", tid, true).First(&defWh).Error == nil
	defCode, defName := "", "默认仓"
	if hasDefault {
		defCode, defName = defWh.Code, defWh.Name
	}
	written := 0
	for offset := 0; written < MaxExportRows; offset += exportBatchSize {
		var skus []inventoryExportRow
		if err := s.DB.WithContext(c.Request.Context()).
			Table("product_skus").
			Select(`product_skus.id AS sku_id, product_skus.sku_code, product_skus.sku_name,
				products.title, products.currency, product_skus.stock AS total_stock, product_skus.cost_price`).
			Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
			Where("products.tenant_id = ?", tid).
			Order("product_skus.created_at ASC, product_skus.id ASC").
			Offset(offset).Limit(exportBatchSize).Scan(&skus).Error; err != nil {
			return err
		}
		if len(skus) == 0 {
			return nil
		}
		ids := make([]uuid.UUID, 0, len(skus))
		for _, r := range skus {
			ids = append(ids, r.SKUID)
		}
		var whRows []inventoryExportRow
		if err := s.DB.WithContext(c.Request.Context()).
			Table("warehouse_stocks").
			Select(`warehouse_stocks.product_sku_id AS sku_id, warehouses.code AS warehouse_code,
				warehouses.name AS warehouse_name, warehouse_stocks.stock AS non_default_stock`).
			Joins("JOIN warehouses ON warehouses.id = warehouse_stocks.warehouse_id AND warehouses.deleted_at IS NULL").
			Where("warehouse_stocks.tenant_id = ? AND warehouses.is_default = ? AND warehouse_stocks.product_sku_id IN ?", tid, false, ids).
			Order("warehouses.priority ASC, warehouses.code ASC").
			Scan(&whRows).Error; err != nil {
			return err
		}
		bySKU := map[uuid.UUID][]inventoryExportRow{}
		for _, r := range whRows {
			bySKU[r.SKUID] = append(bySKU[r.SKUID], r)
		}
		for _, sku := range skus {
			total := 0
			if sku.TotalStock != nil {
				total = *sku.TotalStock
			}
			nonDefaultSum := 0
			for _, r := range bySKU[sku.SKUID] {
				if r.NonDefault != nil {
					nonDefaultSum += *r.NonDefault
				}
			}
			defaultStock := total - nonDefaultSum
			rows := append([]inventoryExportRow{{
				WarehouseCode: defCode, WarehouseName: defName, NonDefault: &defaultStock,
			}}, bySKU[sku.SKUID]...)
			for _, r := range rows {
				if written >= MaxExportRows {
					return nil
				}
				if err := w.Write(csvsafe.Row([]string{
					sku.SKUCode, sku.Title, sku.SKUName, r.WarehouseCode, r.WarehouseName,
					exportIntText(r.NonDefault), exportFloatText(sku.CostPrice), sku.Currency,
				})); err != nil {
					return err
				}
				written++
			}
		}
	}
	return nil
}

type paymentExportRow struct {
	OrderNo    string    `gorm:"column:order_no"`
	Amount     float64   `gorm:"column:amount"`
	Currency   string    `gorm:"column:currency"`
	FeeAmount  float64   `gorm:"column:fee_amount"`
	ReceivedAt time.Time `gorm:"column:received_at"`
	Channel    string    `gorm:"column:channel"`
	Remark     string    `gorm:"column:remark"`
	Source     string    `gorm:"column:source"`
}

// exportPayments writes all store-visible payment records, one row per record.
func (s *Service) exportPayments(c *gin.Context, w *csv.Writer) error {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	if err := w.Write([]string{
		"订单号", "回款金额", "币种", "手续费", "回款日期", "回款渠道", "备注", "来源",
	}); err != nil {
		return err
	}
	written := 0
	for offset := 0; written < MaxExportRows; offset += exportBatchSize {
		tx := s.DB.WithContext(c.Request.Context()).
			Table("finance_payment_records").
			Select(`orders.order_no, finance_payment_records.amount, finance_payment_records.currency,
				finance_payment_records.fee_amount, finance_payment_records.received_at,
				finance_payment_records.channel, finance_payment_records.remark, finance_payment_records.source`).
			Joins("JOIN orders ON orders.id = finance_payment_records.order_id AND orders.deleted_at IS NULL").
			Where("finance_payment_records.tenant_id = ? AND finance_payment_records.deleted_at IS NULL", tid)
		tx, err := adminperm.ApplyStoreScope(c, s.DB, tx, "orders.shop_id")
		if err != nil {
			return err
		}
		var rows []paymentExportRow
		if err := tx.Order("finance_payment_records.received_at ASC, finance_payment_records.id ASC").
			Offset(offset).Limit(exportBatchSize).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, r := range rows {
			if written >= MaxExportRows {
				return nil
			}
			sourceLabel := "手工"
			if r.Source == "import" {
				sourceLabel = "导入"
			}
			if err := w.Write(csvsafe.Row([]string{
				r.OrderNo, strconv.FormatFloat(r.Amount, 'f', -1, 64), r.Currency,
				strconv.FormatFloat(r.FeeAmount, 'f', -1, 64), r.ReceivedAt.Local().Format("2006-01-02"),
				r.Channel, r.Remark, sourceLabel,
			})); err != nil {
				return err
			}
			written++
		}
	}
	return nil
}

type sourceExportRow struct {
	SupplierName string   `gorm:"column:supplier_name"`
	Platform     string   `gorm:"column:platform"`
	Title        string   `gorm:"column:title"`
	SKUCode      string   `gorm:"column:sku_code"`
	SourceURL    string   `gorm:"column:source_url"`
	ExternalSKU  string   `gorm:"column:external_sku_id"`
	Price        *float64 `gorm:"column:current_price"`
	Currency     string   `gorm:"column:currency"`
	IsPrimary    bool     `gorm:"column:is_primary"`
	Status       string   `gorm:"column:status"`
}

// exportSources writes one row per source ↔ SKU mapping; sources without any
// SKU mapping still export one row (empty SKU columns).
func (s *Service) exportSources(c *gin.Context, w *csv.Writer) error {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	if err := w.Write([]string{
		"供应商名称", "货源平台", "商品名称", "SKU编码", "货源链接", "货源SKU", "参考价", "币种", "是否主货源", "状态",
	}); err != nil {
		return err
	}
	written := 0
	for offset := 0; written < MaxExportRows; offset += exportBatchSize {
		var rows []sourceExportRow
		if err := s.DB.WithContext(c.Request.Context()).
			Model(&sourcing.ProductSource{}).
			Select(`suppliers.name AS supplier_name, suppliers.platform, products.title,
				product_skus.sku_code, product_sources.source_url, product_source_skus.external_sku_id,
				product_source_skus.current_price, COALESCE(product_source_skus.currency, '') AS currency,
				product_sources.is_primary, product_sources.status`).
			Joins("JOIN suppliers ON suppliers.id = product_sources.supplier_id AND suppliers.deleted_at IS NULL").
			Joins("JOIN products ON products.id = product_sources.product_id AND products.deleted_at IS NULL").
			Joins("LEFT JOIN product_source_skus ON product_source_skus.product_source_id = product_sources.id AND product_source_skus.deleted_at IS NULL").
			Joins("LEFT JOIN product_skus ON product_skus.id = product_source_skus.local_sku_id").
			Where("product_sources.tenant_id = ?", tid).
			Order("suppliers.name ASC, product_sources.created_at ASC, product_source_skus.created_at ASC").
			Offset(offset).Limit(exportBatchSize).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, r := range rows {
			if written >= MaxExportRows {
				return nil
			}
			primary := "否"
			if r.IsPrimary {
				primary = "是"
			}
			if err := w.Write(csvsafe.Row([]string{
				r.SupplierName, r.Platform, r.Title, r.SKUCode, r.SourceURL, r.ExternalSKU,
				exportFloatText(r.Price), r.Currency, primary, r.Status,
			})); err != nil {
				return err
			}
			written++
		}
	}
	return nil
}
