package procurement

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// Purchase order statuses (manual-order transition mode).
const (
	StatusDraft          = "draft"           // generated, price/stock re-checked
	StatusPendingConfirm = "pending_confirm" // waiting for operator confirmation
	StatusPlacing        = "placing"         // confirmed; operator ordering on 1688 by hand
	StatusPlaced         = "placed"          // 1688 order number backfilled
	StatusPaid           = "paid"            // payment done (manual mark-paid)
	StatusShipped        = "shipped"         // tracking number backfilled
	StatusDelivered      = "delivered"       // cloud warehouse received
	StatusFailed         = "failed"
	StatusCancelled      = "cancelled"
	StatusVoided         = "voided" // manual write-off of a terminal order (audit rows kept)
)

// Pay statuses.
const (
	PayStatusUnpaid = "unpaid"
	PayStatusPaid   = "paid"
)

// Event sources.
const (
	EventSourceManual = "manual"
	EventSourceSystem = "system"
	EventSourceAPI    = "api"
)

// PurchaseOrder aggregates purchase lines for one supplier.
type PurchaseOrder struct {
	model.Base
	TenantID        int64          `gorm:"default:0;index;uniqueIndex:idx_po_idem" json:"tenantId"`
	SupplierID      uuid.UUID      `gorm:"type:char(36);not null;index" json:"supplierId"`
	SupplierName    string         `gorm:"size:255" json:"supplierName"`
	SourcePlatform  string         `gorm:"size:32;not null;default:'1688'" json:"sourcePlatform"`
	ExternalOrderID string         `gorm:"size:64;index" json:"externalOrderId,omitempty"`
	Status          string         `gorm:"size:32;not null;index" json:"status"`
	TotalAmount     float64        `gorm:"type:decimal(12,2);default:0" json:"totalAmount"`
	Currency        string         `gorm:"size:8;default:'CNY'" json:"currency"`
	PayStatus       string         `gorm:"size:16;default:'unpaid'" json:"payStatus"`
	PayChannel      string         `gorm:"size:32" json:"payChannel,omitempty"`
	PaidAt          *time.Time     `json:"paidAt,omitempty"`
	Receiver        datatypes.JSON `gorm:"type:jsonb" json:"receiver,omitempty"`
	IdempotencyKey  string         `gorm:"size:128;not null;uniqueIndex:idx_po_idem" json:"idempotencyKey"`
	ErrorMessage    string         `gorm:"type:text" json:"errorMessage,omitempty"`
	RetryCount      int            `gorm:"default:0" json:"retryCount"`
	MaxRetries      int            `gorm:"default:3" json:"maxRetries"`
	ConfirmRequired bool           `gorm:"not null;default:true" json:"confirmRequired"`
	ConfirmedBy     *uuid.UUID     `gorm:"type:char(36)" json:"confirmedBy,omitempty"`
	ConfirmedAt     *time.Time     `json:"confirmedAt,omitempty"`
	RawCreateReq    datatypes.JSON `gorm:"type:jsonb" json:"rawCreateReq,omitempty"`
	RawCreateResp   datatypes.JSON `gorm:"type:jsonb" json:"rawCreateResp,omitempty"`

	Items     []PurchaseOrderItem  `gorm:"foreignKey:PurchaseOrderID" json:"items,omitempty"`
	Events    []PurchaseOrderEvent `gorm:"foreignKey:PurchaseOrderID" json:"events,omitempty"`
	Logistics []PurchaseLogistics  `gorm:"foreignKey:PurchaseOrderID" json:"logistics,omitempty"`
}

func (PurchaseOrder) TableName() string { return "purchase_orders" }

// PurchaseOrderItem is one SKU purchase line linked back to sales orders.
type PurchaseOrderItem struct {
	model.HardDeleteBase
	TenantID        int64      `gorm:"default:0;index" json:"tenantId"`
	PurchaseOrderID uuid.UUID  `gorm:"type:char(36);not null;index" json:"purchaseOrderId"`
	SalesOrderID    *uuid.UUID `gorm:"type:char(36);index" json:"salesOrderId,omitempty"`
	LocalSKUID      uuid.UUID  `gorm:"column:local_sku_id;type:char(36);not null;index" json:"localSkuId"`
	SourceSKUID     uuid.UUID  `gorm:"column:source_sku_id;type:char(36);not null;index" json:"sourceSkuId"`
	ExternalOfferID string     `gorm:"size:64" json:"externalOfferId,omitempty"`
	ExternalSKUID   string     `gorm:"column:external_sku_id;size:64" json:"externalSkuId,omitempty"`
	SourceURL       string     `gorm:"size:2048" json:"sourceUrl,omitempty"`
	ProductTitle    string     `gorm:"size:512" json:"productTitle,omitempty"`
	SKUName         string     `gorm:"size:512" json:"skuName,omitempty"`
	Quantity        int        `gorm:"not null" json:"quantity"`
	ExpectedPrice   *float64   `gorm:"type:decimal(12,2)" json:"expectedPrice,omitempty"`
	ActualPrice     *float64   `gorm:"type:decimal(12,2)" json:"actualPrice,omitempty"`
}

func (PurchaseOrderItem) TableName() string { return "purchase_order_items" }

// PurchaseOrderEvent is an append-only status transition audit row.
type PurchaseOrderEvent struct {
	ID              int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID        int64          `gorm:"default:0;index" json:"tenantId"`
	PurchaseOrderID uuid.UUID      `gorm:"type:char(36);not null;index" json:"purchaseOrderId"`
	FromStatus      string         `gorm:"size:32" json:"fromStatus,omitempty"`
	ToStatus        string         `gorm:"size:32;not null" json:"toStatus"`
	Source          string         `gorm:"size:16;not null" json:"source"`
	Payload         datatypes.JSON `gorm:"type:jsonb" json:"payload,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
}

func (PurchaseOrderEvent) TableName() string { return "purchase_order_events" }

// PurchaseLogistics is the shipping record backfilled by the operator.
type PurchaseLogistics struct {
	model.Base
	TenantID        int64          `gorm:"default:0;index" json:"tenantId"`
	PurchaseOrderID uuid.UUID      `gorm:"type:char(36);not null;index" json:"purchaseOrderId"`
	TrackingNo      string         `gorm:"size:64" json:"trackingNo,omitempty"`
	Carrier         string         `gorm:"size:64" json:"carrier,omitempty"`
	Status          string         `gorm:"size:32" json:"status,omitempty"` // pending|in_transit|delivered
	Traces          datatypes.JSON `gorm:"type:jsonb" json:"traces,omitempty"`
	WarehouseID     *uuid.UUID     `gorm:"type:char(36)" json:"warehouseId,omitempty"`
	InboundAt       *time.Time     `json:"inboundAt,omitempty"`
}

func (PurchaseLogistics) TableName() string { return "purchase_logistics" }
