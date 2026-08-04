package migrationimport

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

// RowError is one per-row validation failure (RowNumber is the data row
// number starting at 1, i.e. excluding the header row).
type RowError struct {
	RowNumber int    `json:"rowNumber"`
	Field     string `json:"field,omitempty"`
	Message   string `json:"message"`
}

// SKUInput is one normalized product SKU candidate.
type SKUInput struct {
	RowNumber int
	SKUCode   string
	SKUName   string
	Price     *float64
	CostPrice *float64
	Stock     *int
	ImageURL  string
}

// ProductInput groups rows sharing the same title into one draft product.
type ProductInput struct {
	Title       string
	Currency    string
	Description string
	SourceURL   string
	SKUs        []SKUInput
}

// OrderItemInputRow is one normalized order line.
type OrderItemInputRow struct {
	RowNumber    int
	ProductTitle string
	SKUCode      string
	SKUName      string
	Quantity     int
	UnitPrice    float64
}

// OrderInput groups rows sharing the same order no into one order.
type OrderInput struct {
	FirstRowNumber  int
	RowNumbers      []int
	OrderNo         string
	ExternalOrderID string
	CustomerName    string
	CustomerPhone   string
	CustomerEmail   string
	Country         string
	Province        string
	City            string
	Address         string
	ZipCode         string
	Currency        string
	TotalAmount     *float64
	StatusMapping   OrderStatusMapping
	RawStatus       string
	OrderedAt       *time.Time
	PaidAt          *time.Time
	TrackingNo      string
	Items           []OrderItemInputRow
}

// normalizedMapping fills unmapped canonical fields with -1 so missing map
// keys never fall back to column 0.
func normalizedMapping(kind string, mapping map[string]int) map[string]int {
	out := make(map[string]int, len(mapping))
	for _, f := range FieldsForKind(kind) {
		idx, ok := mapping[f.Key]
		if !ok {
			idx = -1
		}
		out[f.Key] = idx
	}
	return out
}

func cellAt(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func parseFloatCell(v string) (*float64, error) {
	v = strings.TrimSpace(strings.ReplaceAll(v, ",", ""))
	if v == "" {
		return nil, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < 0 {
		return nil, fmt.Errorf("需为非负数字")
	}
	return &f, nil
}

func parseIntCell(v string) (*int, error) {
	v = strings.TrimSpace(strings.ReplaceAll(v, ",", ""))
	if v == "" {
		return nil, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < 0 || f != float64(int(f)) {
		return nil, fmt.Errorf("需为非负整数")
	}
	n := int(f)
	return &n, nil
}

var timeLayouts = []string{
	"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02",
	"2006/01/02 15:04:05", "2006/01/02 15:04", "2006/01/02",
	time.RFC3339,
}

func parseTimeCell(v string) (*time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	for _, layout := range timeLayouts {
		if t, err := time.ParseInLocation(layout, v, time.Local); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("时间格式无法识别（如 2026-01-02 15:04:05）")
}

var currencyOK = func(v string) bool {
	if len(v) != 3 {
		return false
	}
	for _, r := range v {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// BuildProducts validates and groups product rows. Rows with errors are
// reported and excluded; remaining rows group by title into draft inputs.
func BuildProducts(columns []string, rows [][]string, mapping map[string]int) ([]ProductInput, []RowError) {
	mapping = normalizedMapping(KindProduct, mapping)
	var errs []RowError
	byTitle := map[string]*ProductInput{}
	var titleOrder []string
	seenSKU := map[string]int{}
	for i, row := range rows {
		rowNo := i + 1
		title := cellAt(row, mapping[FTitle])
		if title == "" {
			errs = append(errs, RowError{rowNo, FTitle, "商品名称不能为空"})
			continue
		}
		skuCode := cellAt(row, mapping[FSKUCode])
		if skuCode != "" {
			if prev, dup := seenSKU[skuCode]; dup {
				errs = append(errs, RowError{rowNo, FSKUCode, fmt.Sprintf("SKU 编码与第 %d 行重复", prev)})
				continue
			}
			seenSKU[skuCode] = rowNo
		}
		price, err := parseFloatCell(cellAt(row, mapping[FPrice]))
		if err != nil {
			errs = append(errs, RowError{rowNo, FPrice, "售价" + err.Error()})
			continue
		}
		cost, err := parseFloatCell(cellAt(row, mapping[FCostPrice]))
		if err != nil {
			errs = append(errs, RowError{rowNo, FCostPrice, "成本价" + err.Error()})
			continue
		}
		stock, err := parseIntCell(cellAt(row, mapping[FStock]))
		if err != nil {
			errs = append(errs, RowError{rowNo, FStock, "库存数量" + err.Error()})
			continue
		}
		currency := strings.ToUpper(cellAt(row, mapping[FCurrency]))
		if currency != "" && !currencyOK(currency) {
			errs = append(errs, RowError{rowNo, FCurrency, "币种需为 3 位字母代码（如 CNY / USD）"})
			continue
		}
		p := byTitle[title]
		if p == nil {
			p = &ProductInput{
				Title:       title,
				Currency:    currency,
				Description: cellAt(row, mapping[FDescription]),
				SourceURL:   cellAt(row, mapping[FSourceURL]),
			}
			byTitle[title] = p
			titleOrder = append(titleOrder, title)
		}
		skuName := cellAt(row, mapping[FSKUName])
		if skuName == "" {
			skuName = skuCode
		}
		if skuName == "" {
			skuName = "默认规格"
		}
		p.SKUs = append(p.SKUs, SKUInput{
			RowNumber: rowNo,
			SKUCode:   skuCode,
			SKUName:   skuName,
			Price:     price,
			CostPrice: cost,
			Stock:     stock,
			ImageURL:  cellAt(row, mapping[FImageURL]),
		})
	}
	out := make([]ProductInput, 0, len(titleOrder))
	for _, t := range titleOrder {
		out = append(out, *byTitle[t])
	}
	return out, errs
}

// BuildOrders validates and groups order rows by order no. A row error
// excludes only that row; header fields come from the first row of a group.
func BuildOrders(columns []string, rows [][]string, mapping map[string]int) ([]OrderInput, []RowError) {
	mapping = normalizedMapping(KindOrder, mapping)
	var errs []RowError
	byNo := map[string]*OrderInput{}
	var noOrder []string
	for i, row := range rows {
		rowNo := i + 1
		orderNo := cellAt(row, mapping[FOrderNo])
		if orderNo == "" {
			errs = append(errs, RowError{rowNo, FOrderNo, "订单号不能为空"})
			continue
		}
		productTitle := cellAt(row, mapping[FProductTitle])
		if productTitle == "" {
			errs = append(errs, RowError{rowNo, FProductTitle, "商品名称不能为空"})
			continue
		}
		qty, err := parseIntCell(cellAt(row, mapping[FQuantity]))
		if err != nil || qty == nil || *qty < 1 {
			errs = append(errs, RowError{rowNo, FQuantity, "数量需为正整数"})
			continue
		}
		unitPrice, err := parseFloatCell(cellAt(row, mapping[FUnitPrice]))
		if err != nil {
			errs = append(errs, RowError{rowNo, FUnitPrice, "单价" + err.Error()})
			continue
		}
		existing := byNo[orderNo]
		if existing == nil {
			customerName := cellAt(row, mapping[FCustomerName])
			if customerName == "" {
				errs = append(errs, RowError{rowNo, FCustomerName, "收件人不能为空"})
				continue
			}
			rawStatus := cellAt(row, mapping[FStatus])
			if rawStatus == "" {
				errs = append(errs, RowError{rowNo, FStatus, "订单状态不能为空"})
				continue
			}
			sm, ok := MapOrderStatus(rawStatus)
			if !ok {
				errs = append(errs, RowError{rowNo, FStatus, fmt.Sprintf("无法识别的订单状态「%s」", rawStatus)})
				continue
			}
			currency := strings.ToUpper(cellAt(row, mapping[FCurrency]))
			if currency != "" && !currencyOK(currency) {
				errs = append(errs, RowError{rowNo, FCurrency, "币种需为 3 位字母代码（如 CNY / USD）"})
				continue
			}
			total, err := parseFloatCell(cellAt(row, mapping[FTotalAmount]))
			if err != nil {
				errs = append(errs, RowError{rowNo, FTotalAmount, "订单金额" + err.Error()})
				continue
			}
			orderedAt, err := parseTimeCell(cellAt(row, mapping[FOrderedAt]))
			if err != nil {
				errs = append(errs, RowError{rowNo, FOrderedAt, "下单时间" + err.Error()})
				continue
			}
			paidAt, err := parseTimeCell(cellAt(row, mapping[FPaidAt]))
			if err != nil {
				errs = append(errs, RowError{rowNo, FPaidAt, "付款时间" + err.Error()})
				continue
			}
			existing = &OrderInput{
				FirstRowNumber:  rowNo,
				OrderNo:         orderNo,
				ExternalOrderID: cellAt(row, mapping[FExternalOrderID]),
				CustomerName:    customerName,
				CustomerPhone:   cellAt(row, mapping[FCustomerPhone]),
				CustomerEmail:   cellAt(row, mapping[FCustomerEmail]),
				Country:         cellAt(row, mapping[FCountry]),
				Province:        cellAt(row, mapping[FProvince]),
				City:            cellAt(row, mapping[FCity]),
				Address:         cellAt(row, mapping[FAddress]),
				ZipCode:         cellAt(row, mapping[FZipCode]),
				Currency:        currency,
				TotalAmount:     total,
				StatusMapping:   sm,
				RawStatus:       rawStatus,
				OrderedAt:       orderedAt,
				PaidAt:          paidAt,
				TrackingNo:      cellAt(row, mapping[FTrackingNo]),
			}
			byNo[orderNo] = existing
			noOrder = append(noOrder, orderNo)
		}
		existing.RowNumbers = append(existing.RowNumbers, rowNo)
		existing.Items = append(existing.Items, OrderItemInputRow{
			RowNumber:    rowNo,
			ProductTitle: productTitle,
			SKUCode:      cellAt(row, mapping[FSKUCode]),
			SKUName:      cellAt(row, mapping[FSKUName]),
			Quantity:     *qty,
			UnitPrice:    derefFloat(unitPrice),
		})
	}
	out := make([]OrderInput, 0, len(noOrder))
	for _, no := range noOrder {
		out = append(out, *byNo[no])
	}
	return out, errs
}

func derefFloat(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

// ToCreateBody converts one grouped order input into the order module create payload.
func (oi *OrderInput) ToCreateBody(sourceFormat string) order.CreateBody {
	body := order.CreateBody{
		Platform:          "migration",
		OrderNo:           oi.OrderNo,
		CustomerName:      oi.CustomerName,
		CustomerPhone:     oi.CustomerPhone,
		CustomerEmail:     oi.CustomerEmail,
		Status:            oi.StatusMapping.Status,
		PaymentStatus:     oi.StatusMapping.PaymentStatus,
		FulfillmentStatus: oi.StatusMapping.FulfillmentStatus,
		Currency:          oi.Currency,
		OrderedAt:         oi.OrderedAt,
		PaidAt:            oi.PaidAt,
	}
	if oi.ExternalOrderID != "" {
		ext := oi.ExternalOrderID
		body.ExternalOrderID = &ext
	}
	total := 0.0
	for _, it := range oi.Items {
		lineTotal := float64(it.Quantity) * it.UnitPrice
		total += lineTotal
		body.Items = append(body.Items, order.OrderItemInput{
			ProductTitle: it.ProductTitle,
			SKUCode:      it.SKUCode,
			SKUName:      it.SKUName,
			Quantity:     it.Quantity,
			UnitPrice:    it.UnitPrice,
			TotalPrice:   lineTotal,
		})
	}
	if oi.TotalAmount != nil {
		body.TotalAmount = *oi.TotalAmount
	} else {
		body.TotalAmount = total
	}
	if oi.TrackingNo != "" {
		body.Shipments = []order.OrderShipmentInput{{
			Carrier:    "unknown",
			TrackingNo: oi.TrackingNo,
			Status:     shipmentStatusFor(oi.StatusMapping.Status),
		}}
	}
	receiver := map[string]string{}
	for k, v := range map[string]string{
		"country": oi.Country, "province": oi.Province, "city": oi.City,
		"address": oi.Address, "zipCode": oi.ZipCode,
	} {
		if v != "" {
			receiver[k] = v
		}
	}
	raw := map[string]any{
		"importSource":    sourceFormat,
		"importRawStatus": oi.RawStatus,
	}
	if len(receiver) > 0 {
		raw["receiver"] = receiver
		body.Remark = receiverSummary(oi)
	}
	if b, err := json.Marshal(raw); err == nil {
		body.RawData = b
	}
	return body
}

func receiverSummary(oi *OrderInput) string {
	parts := make([]string, 0, 6)
	for _, v := range []string{oi.Country, oi.Province, oi.City, oi.Address, oi.ZipCode} {
		if v != "" {
			parts = append(parts, v)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "收件地址: " + strings.Join(parts, " ")
}

func shipmentStatusFor(orderStatus string) string {
	switch orderStatus {
	case order.StatusDelivered:
		return order.ShipmentDelivered
	case order.StatusShipped:
		return order.ShipmentShipped
	default:
		return order.ShipmentPending
	}
}
