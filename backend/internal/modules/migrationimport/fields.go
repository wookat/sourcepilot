package migrationimport

import "strings"

// FieldDef describes one canonical import field for a kind.
type FieldDef struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Required bool     `json:"required"`
	Aliases  []string `json:"-"`
}

// Product canonical field keys.
const (
	FTitle       = "title"
	FSKUCode     = "skuCode"
	FSKUName     = "skuName"
	FPrice       = "price"
	FCostPrice   = "costPrice"
	FStock       = "stock"
	FCurrency    = "currency"
	FImageURL    = "imageUrl"
	FDescription = "description"
	FSourceURL   = "sourceUrl"
)

// Inventory opening import canonical field keys (quantity reuses FQuantity).
const (
	FWarehouseCode = "warehouseCode"
)

// Source archive import canonical field keys.
const (
	FSupplierName  = "supplierName"
	FSourceLink    = "sourceLink"
	FRefPrice      = "refPrice"
	FExternalSKUID = "externalSkuId"
)

// Payment (回款) import canonical field keys (orderNo reuses FOrderNo,
// currency reuses FCurrency).
const (
	FPaymentAmount = "paymentAmount"
	FFeeAmount     = "feeAmount"
	FReceivedAt    = "receivedAt"
	FChannel       = "channel"
	FRemark        = "remark"
)

// Order canonical field keys.
const (
	FOrderNo         = "orderNo"
	FExternalOrderID = "externalOrderId"
	FCustomerName    = "customerName"
	FCustomerPhone   = "customerPhone"
	FCustomerEmail   = "customerEmail"
	FCountry         = "country"
	FProvince        = "province"
	FCity            = "city"
	FAddress         = "address"
	FZipCode         = "zipCode"
	FProductTitle    = "productTitle"
	FQuantity        = "quantity"
	FUnitPrice       = "unitPrice"
	FTotalAmount     = "totalAmount"
	FStatus          = "status"
	FOrderedAt       = "orderedAt"
	FPaidAt          = "paidAt"
	FTrackingNo      = "trackingNo"
)

// ProductFields lists canonical product import fields (one file row = one SKU;
// rows sharing the same title become one product draft).
func ProductFields() []FieldDef {
	return []FieldDef{
		{Key: FTitle, Label: "商品名称", Required: true,
			Aliases: []string{"商品名称", "产品名称", "品名", "商品标题", "产品标题", "标题", "宝贝标题", "商品名", "产品名", "名称", "product name", "name", "title"}},
		{Key: FSKUCode, Label: "SKU编码",
			Aliases: []string{"sku", "库存sku", "库存sku编号", "商品sku", "商品编号", "货号", "sku编码", "sku编号", "seller sku"}},
		{Key: FSKUName, Label: "规格名称",
			Aliases: []string{"规格", "规格名称", "属性", "变体", "多物品选项", "子sku名称", "variation"}},
		{Key: FPrice, Label: "售价",
			Aliases: []string{"售价", "销售价", "价格", "单价", "刊登价", "price"}},
		{Key: FCostPrice, Label: "成本价",
			Aliases: []string{"成本", "成本价", "采购价", "采购参考价", "进价", "cost"}},
		{Key: FStock, Label: "库存数量",
			Aliases: []string{"数量", "库存", "库存数量", "可用库存", "quantity", "stock"}},
		{Key: FCurrency, Label: "币种",
			Aliases: []string{"币种", "货币", "currency"}},
		{Key: FImageURL, Label: "图片链接",
			Aliases: []string{"图片", "图片链接", "图片地址", "库存图片地址", "主图", "主图链接", "image", "image url"}},
		{Key: FDescription, Label: "商品描述",
			Aliases: []string{"描述", "商品描述", "产品描述", "description"}},
		{Key: FSourceURL, Label: "来源链接",
			Aliases: []string{"来源链接", "商品链接", "链接", "url", "source url"}},
	}
}

// OrderFields lists canonical order import fields (one file row = one order
// line item; rows sharing the same order no become one order).
func OrderFields() []FieldDef {
	return []FieldDef{
		{Key: FOrderNo, Label: "订单号", Required: true,
			Aliases: []string{"订单号", "订单编号", "订单id", "order number", "order no", "order id"}},
		{Key: FExternalOrderID, Label: "平台订单号",
			Aliases: []string{"平台订单号", "交易单号", "平台单号", "transaction number", "交易号"}},
		{Key: FCustomerName, Label: "收件人", Required: true,
			Aliases: []string{"收件人", "收货人姓名", "收货人", "客户姓名", "买家姓名", "customer name", "客户账号", "买家账号"}},
		{Key: FCustomerPhone, Label: "电话",
			Aliases: []string{"电话", "手机", "手机号", "联系电话", "收件人电话", "telephone", "phone"}},
		{Key: FCustomerEmail, Label: "邮箱",
			Aliases: []string{"邮箱", "买家邮箱", "email", "paypal邮箱"}},
		{Key: FCountry, Label: "国家",
			Aliases: []string{"国家", "国家（中文）", "国家(中文)", "country"}},
		{Key: FProvince, Label: "省/州",
			Aliases: []string{"省", "州省", "省/州", "所属地区", "state", "province"}},
		{Key: FCity, Label: "城市",
			Aliases: []string{"城市", "所属城市", "city"}},
		{Key: FAddress, Label: "详细地址",
			Aliases: []string{"详细地址", "邮寄地址", "地址", "地址1", "mailing address", "address"}},
		{Key: FZipCode, Label: "邮编",
			Aliases: []string{"邮编", "邮政编码", "zip code", "zip", "postcode"}},
		{Key: FProductTitle, Label: "商品名称", Required: true,
			Aliases: []string{"商品名称", "产品名称", "品名", "商品标题", "产品标题", "标题", "商品名", "产品名", "goods to named", "product name"}},
		{Key: FSKUCode, Label: "SKU编码",
			Aliases: []string{"sku", "商品sku", "商品编号", "库存sku", "货号", "seller sku"}},
		{Key: FSKUName, Label: "规格名称",
			Aliases: []string{"规格", "规格名称", "多物品选项", "属性", "variation"}},
		{Key: FQuantity, Label: "数量", Required: true,
			Aliases: []string{"数量", "商品数量", "购买数量", "quantity", "qty"}},
		{Key: FUnitPrice, Label: "单价",
			Aliases: []string{"单价", "商品单价", "unit price"}},
		{Key: FTotalAmount, Label: "订单金额",
			Aliases: []string{"订单金额", "总金额", "原始金额", "金额", "total amount", "total"}},
		{Key: FCurrency, Label: "币种",
			Aliases: []string{"币种", "货币", "currency"}},
		{Key: FStatus, Label: "订单状态", Required: true,
			Aliases: []string{"订单状态", "状态", "status", "order status"}},
		{Key: FOrderedAt, Label: "下单时间",
			Aliases: []string{"下单时间", "订单时间", "创建时间", "order time", "created time"}},
		{Key: FPaidAt, Label: "付款时间",
			Aliases: []string{"付款时间", "支付时间", "time of payment", "paid time"}},
		{Key: FTrackingNo, Label: "运单号",
			Aliases: []string{"运单号", "货运单号", "快递单号", "物流单号", "tracking number", "tracking no"}},
	}
}

// InventoryFields lists canonical inventory opening import fields (one file
// row = one SKU + warehouse opening quantity).
func InventoryFields() []FieldDef {
	return []FieldDef{
		{Key: FSKUCode, Label: "SKU编码", Required: true,
			Aliases: []string{"sku", "sku编码", "sku编号", "商品sku", "商品编号", "库存sku", "货号", "seller sku"}},
		{Key: FWarehouseCode, Label: "仓库编码",
			Aliases: []string{"仓库", "仓库编码", "仓库代码", "仓库名称", "warehouse", "warehouse code"}},
		{Key: FQuantity, Label: "期初数量", Required: true,
			Aliases: []string{"数量", "库存", "库存数量", "期初数量", "期初库存", "可用库存", "quantity", "stock", "qty"}},
		{Key: FCostPrice, Label: "参考进价",
			Aliases: []string{"成本", "成本价", "进价", "参考进价", "采购价", "采购参考价", "cost", "cost price"}},
	}
}

// SourceFields lists canonical source-archive import fields (one file row =
// one supplier offer ↔ local SKU mapping).
func SourceFields() []FieldDef {
	return []FieldDef{
		{Key: FSupplierName, Label: "供应商名称", Required: true,
			Aliases: []string{"供应商", "供应商名称", "厂家", "档口", "supplier", "supplier name"}},
		{Key: FSKUCode, Label: "SKU编码", Required: true,
			Aliases: []string{"sku", "sku编码", "sku编号", "商品sku", "商品编号", "库存sku", "货号", "seller sku"}},
		{Key: FSourceLink, Label: "货源链接",
			Aliases: []string{"货源链接", "1688链接", "采购链接", "商品链接", "链接", "url", "source url"}},
		{Key: FRefPrice, Label: "参考价",
			Aliases: []string{"参考价", "采购价", "参考单价", "进价", "单价", "price", "ref price"}},
		{Key: FExternalSKUID, Label: "货源SKU",
			Aliases: []string{"货源sku", "外部sku", "供应商sku", "external sku"}},
	}
}

// PaymentFields lists canonical payment (回款) import fields (one file row =
// one payment record against an existing order).
func PaymentFields() []FieldDef {
	return []FieldDef{
		{Key: FOrderNo, Label: "订单号", Required: true,
			Aliases: []string{"订单号", "订单编号", "订单id", "order number", "order no", "order id"}},
		{Key: FPaymentAmount, Label: "回款金额", Required: true,
			Aliases: []string{"回款金额", "到账金额", "结算金额", "放款金额", "金额", "amount", "payout amount", "settlement amount"}},
		{Key: FCurrency, Label: "币种",
			Aliases: []string{"币种", "货币", "currency"}},
		{Key: FFeeAmount, Label: "手续费",
			Aliases: []string{"手续费", "平台手续费", "服务费", "fee", "transaction fee"}},
		{Key: FReceivedAt, Label: "回款日期", Required: true,
			Aliases: []string{"回款日期", "到账日期", "到账时间", "结算日期", "放款日期", "date", "payout date", "settlement date"}},
		{Key: FChannel, Label: "回款渠道",
			Aliases: []string{"回款渠道", "渠道", "收款渠道", "收款方式", "channel", "payment method"}},
		{Key: FRemark, Label: "备注",
			Aliases: []string{"备注", "说明", "remark", "note", "memo"}},
	}
}

// FieldsForKind returns the canonical field list for an import kind.
func FieldsForKind(kind string) []FieldDef {
	switch kind {
	case KindOrder:
		return OrderFields()
	case KindInventory:
		return InventoryFields()
	case KindSource:
		return SourceFields()
	case KindPayment:
		return PaymentFields()
	default:
		return ProductFields()
	}
}

func normalizeHeader(h string) string {
	h = strings.TrimSpace(strings.ToLower(h))
	h = strings.TrimPrefix(h, "*")
	h = strings.TrimSuffix(h, "*")
	return strings.TrimSpace(h)
}

// GuessMapping maps canonical field key -> column index (-1 when not found).
func GuessMapping(kind string, columns []string) map[string]int {
	out := map[string]int{}
	norm := make([]string, len(columns))
	for i, c := range columns {
		norm[i] = normalizeHeader(c)
	}
	for _, f := range FieldsForKind(kind) {
		out[f.Key] = -1
		for i, h := range norm {
			if h == "" {
				continue
			}
			for _, a := range f.Aliases {
				if h == a {
					out[f.Key] = i
					break
				}
			}
			if out[f.Key] >= 0 {
				break
			}
		}
	}
	return out
}

// DetectSourceFormat guesses the export source from header names.
func DetectSourceFormat(columns []string) string {
	joined := " "
	for _, c := range columns {
		joined += normalizeHeader(c) + " "
	}
	mabangMarkers := []string{"库存sku", "邮寄地址", "交易单号", "货运单号", "多物品选项", "商品报关名"}
	dxmMarkers := []string{"收货人姓名", "平台订单号", "详细地址", "采购参考价", "买家账号", "店铺名称"}
	mb, dx := 0, 0
	for _, m := range mabangMarkers {
		if strings.Contains(joined, " "+m+" ") {
			mb++
		}
	}
	for _, m := range dxmMarkers {
		if strings.Contains(joined, " "+m+" ") {
			dx++
		}
	}
	switch {
	case mb >= 2 && mb > dx:
		return SourceMabang
	case dx >= 2 && dx > mb:
		return SourceDianxiaomi
	case mb == 1 && dx == 0:
		return SourceMabang
	case dx == 1 && mb == 0:
		return SourceDianxiaomi
	default:
		return SourceCustom
	}
}
