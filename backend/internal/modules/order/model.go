package order

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// Order is manually managed internal draft order (no marketplace sync).
type Order struct {
	model.Base
	TenantID        int64      `gorm:"default:0;index;uniqueIndex:idx_orders_tenant_order_no,priority:1" json:"tenantId"`
	Platform        string     `gorm:"size:64;index;not null" json:"platform"`
	ShopID          *uuid.UUID `gorm:"type:char(36);index" json:"shopId,omitempty"`
	ExternalOrderID *string    `gorm:"size:255;index" json:"externalOrderId,omitempty"`
	// OrderNo is unique per tenant, never globally: a global unique index
	// leaks (and lets a tenant squat) other tenants' order numbers.
	OrderNo       string `gorm:"size:128;uniqueIndex:idx_orders_tenant_order_no,priority:2;not null" json:"orderNo"`
	CustomerName  string `gorm:"size:255;index;not null" json:"customerName"`
	CustomerEmail string `gorm:"size:255" json:"customerEmail,omitempty"`
	CustomerPhone string `gorm:"size:64" json:"customerPhone,omitempty"`
	Status        string `gorm:"size:32;index;not null" json:"status"`
	// ReviewStatus is the审单 state (see ReviewStatus* constants). Empty means
	// the order never entered the review flow and is not blocked.
	ReviewStatus      string     `gorm:"size:32;index;default:''" json:"reviewStatus"`
	PaymentStatus     string     `gorm:"size:32;index;not null" json:"paymentStatus"`
	FulfillmentStatus string     `gorm:"size:32;index;not null" json:"fulfillmentStatus"`
	Currency          string     `gorm:"size:16;not null" json:"currency"`
	TotalAmount       float64    `gorm:"type:decimal(18,4);default:0" json:"totalAmount"`
	PaidAt            *time.Time `json:"paidAt,omitempty"`
	OrderedAt         *time.Time `json:"orderedAt,omitempty"`
	ShippedAt         *time.Time `json:"shippedAt,omitempty"`
	// WaybillPrintedAt marks when picking/waybill sheets were last printed
	// (打单状态); it never gates the shipping flow.
	WaybillPrintedAt *time.Time `gorm:"index" json:"waybillPrintedAt,omitempty"`
	// ShipReadyNotifiedAt marks when the shipping workbench was notified that
	// the order's procurement was signed in (自动化「通知发货工作台」动作); it
	// is informational and never gates the shipping flow.
	ShipReadyNotifiedAt *time.Time `gorm:"index" json:"shipReadyNotifiedAt,omitempty"`
	// PlannedCarrier* snapshot the R111发货规则 outcome landed by the自动化
	// 「自动应用发货规则」 action (or a future manual pick). The plan is advisory:
	// the ship flow still lets the operator choose any carrier (人工覆盖).
	PlannedCarrierCode string `gorm:"size:64" json:"plannedCarrierCode,omitempty"`
	PlannedCarrierName string `gorm:"size:128" json:"plannedCarrierName,omitempty"`
	// PlannedCarrierMode is recommend (仅推荐) or apply (直接应用).
	PlannedCarrierMode string     `gorm:"size:16" json:"plannedCarrierMode,omitempty"`
	PlannedCarrierRule string     `gorm:"size:128" json:"plannedCarrierRule,omitempty"`
	PlannedCarrierAt   *time.Time `json:"plannedCarrierAt,omitempty"`
	// AssignedWarehouse* snapshot the自动分仓 outcome; deductions without an
	// explicit warehouse pin to the assigned warehouse (多仓扣减联动).
	AssignedWarehouseID       *uuid.UUID     `gorm:"type:char(36);index" json:"assignedWarehouseId,omitempty"`
	AssignedWarehouseName     string         `gorm:"size:128" json:"assignedWarehouseName,omitempty"`
	AssignedWarehouseStrategy string         `gorm:"size:32" json:"assignedWarehouseStrategy,omitempty"`
	WarehouseAssignedAt       *time.Time     `json:"warehouseAssignedAt,omitempty"`
	DeliveredAt               *time.Time     `json:"deliveredAt,omitempty"`
	PlatformUpdatedAt         *time.Time     `gorm:"index" json:"platformUpdatedAt,omitempty"`
	PlatformRevision          string         `gorm:"size:128;index" json:"platformRevision,omitempty"`
	Remark                    string         `gorm:"type:text" json:"remark,omitempty"`
	RawData                   datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
	CreatedBy                 *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`

	Items     []OrderItem     `gorm:"foreignKey:OrderID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"items,omitempty"`
	Shipments []OrderShipment `gorm:"foreignKey:OrderID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"shipments,omitempty"`
}

func (Order) TableName() string { return "orders" }

// OrderItem is a line item (optional link to product draft).
type OrderItem struct {
	model.HardDeleteBase
	OrderID        uuid.UUID      `gorm:"type:char(36);index;not null" json:"orderId"`
	ProductID      *uuid.UUID     `gorm:"type:char(36);index" json:"productId,omitempty"`
	ProductSKUID   *uuid.UUID     `gorm:"column:product_sku_id;type:char(36);index" json:"productSkuId,omitempty"`
	ExternalItemID *string        `gorm:"size:255" json:"externalItemId,omitempty"`
	ExternalSKUID  *string        `gorm:"column:external_sku_id;size:256" json:"externalSkuId,omitempty"`
	SellerSKU      string         `gorm:"size:128" json:"sellerSku,omitempty"`
	ProductTitle   string         `gorm:"size:512;not null" json:"productTitle"`
	SKUName        string         `gorm:"size:512" json:"skuName,omitempty"`
	SKUCode        string         `gorm:"size:128" json:"skuCode,omitempty"`
	Quantity       int            `gorm:"not null" json:"quantity"`
	UnitPrice      float64        `gorm:"type:decimal(18,4);default:0" json:"unitPrice"`
	TotalPrice     float64        `gorm:"type:decimal(18,4);default:0" json:"totalPrice"`
	ImageURL       string         `gorm:"type:text" json:"imageUrl,omitempty"`
	Attrs          datatypes.JSON `gorm:"type:jsonb" json:"attrs,omitempty"`
	RawData        datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
}

func (OrderItem) TableName() string { return "order_items" }

// OrderShipment is logistics info for one package / tracking segment.
type OrderShipment struct {
	model.HardDeleteBase
	OrderID uuid.UUID `gorm:"type:char(36);index;not null" json:"orderId"`
	// Carrier keeps the display-name snapshot; CarrierID / CarrierCode link
	// the tenant carrier the waybill was booked with (nil for legacy rows).
	Carrier     string         `gorm:"size:128;not null" json:"carrier"`
	CarrierID   *uuid.UUID     `gorm:"type:char(36);index" json:"carrierId,omitempty"`
	CarrierCode string         `gorm:"size:64;index" json:"carrierCode,omitempty"`
	TrackingNo  string         `gorm:"size:255;not null;index" json:"trackingNo"`
	TrackingURL string         `gorm:"type:text" json:"trackingUrl,omitempty"`
	Status      string         `gorm:"size:32;index;not null" json:"status"`
	ShippedAt   *time.Time     `json:"shippedAt,omitempty"`
	DeliveredAt *time.Time     `json:"deliveredAt,omitempty"`
	RawData     datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
}

func (OrderShipment) TableName() string { return "order_shipments" }
