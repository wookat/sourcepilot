package sourcing

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// Supplier statuses.
const (
	SupplierStatusActive   = "active"
	SupplierStatusDisabled = "disabled"
)

// ProductSource statuses.
const (
	SourceStatusActive     = "active"
	SourceStatusOutOfStock = "out_of_stock"
	SourceStatusPriceAlert = "price_alert"
	SourceStatusDisabled   = "disabled"
)

// Switch event reasons / modes.
const (
	SwitchReasonOutOfStock    = "out_of_stock"
	SwitchReasonPriceIncrease = "price_increase"
	SwitchReasonManual        = "manual"

	SwitchModeAuto      = "auto"
	SwitchModeManual    = "manual"
	SwitchModeSuggested = "suggested"
)

// Price history capture sources.
const (
	CaptureSourceCrawl  = "crawl"
	CaptureSourceOrder  = "order"
	CaptureSourceManual = "manual"
	CaptureSourceAPI    = "api"
)

// Supplier is a purchasing supplier (1688 shop by default).
type Supplier struct {
	model.Base
	TenantID   int64          `gorm:"default:0;index" json:"tenantId"`
	Platform   string         `gorm:"size:32;not null;default:'1688';index:idx_supplier_ext" json:"platform"`
	ExternalID string         `gorm:"size:64;index:idx_supplier_ext" json:"externalId,omitempty"`
	Name       string         `gorm:"size:255;not null;index" json:"name"`
	Rating     *float64       `gorm:"type:decimal(4,2)" json:"rating,omitempty"`
	Contact    datatypes.JSON `gorm:"type:jsonb" json:"contact,omitempty"`
	Remark     string         `gorm:"size:255" json:"remark,omitempty"`
	Status     string         `gorm:"size:16;not null;default:'active';index" json:"status"`
	RawData    datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
}

func (Supplier) TableName() string { return "suppliers" }

// ProductSource binds a product to a supplier offer (one product, many sources).
type ProductSource struct {
	model.Base
	TenantID      int64          `gorm:"default:0;index;uniqueIndex:idx_product_source" json:"tenantId"`
	ProductID     uuid.UUID      `gorm:"type:char(36);not null;index;uniqueIndex:idx_product_source" json:"productId"`
	SupplierID    uuid.UUID      `gorm:"type:char(36);not null;index;uniqueIndex:idx_product_source" json:"supplierId"`
	SourceURL     string         `gorm:"size:2048" json:"sourceUrl,omitempty"`
	SourceOfferID string         `gorm:"size:64;uniqueIndex:idx_product_source" json:"sourceOfferId,omitempty"`
	Priority      int            `gorm:"not null;default:100" json:"priority"`
	IsPrimary     bool           `gorm:"not null;default:false;index" json:"isPrimary"`
	Locked        bool           `gorm:"not null;default:false" json:"locked"`
	Status        string         `gorm:"size:16;not null;default:'active';index" json:"status"`
	MOQ           *int           `gorm:"column:moq" json:"moq,omitempty"`
	LeadTimeDays  *int           `json:"leadTimeDays,omitempty"`
	LastCheckedAt *time.Time     `json:"lastCheckedAt,omitempty"`
	RawData       datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`

	Supplier *Supplier          `gorm:"foreignKey:SupplierID" json:"supplier,omitempty"`
	SKUs     []ProductSourceSKU `gorm:"foreignKey:ProductSourceID" json:"skus,omitempty"`
}

func (ProductSource) TableName() string { return "product_sources" }

// ProductSourceSKU maps a local product SKU to the supplier-side SKU/spec.
type ProductSourceSKU struct {
	model.Base
	TenantID        int64          `gorm:"default:0;index;uniqueIndex:idx_source_sku" json:"tenantId"`
	ProductSourceID uuid.UUID      `gorm:"type:char(36);not null;index;uniqueIndex:idx_source_sku" json:"productSourceId"`
	LocalSKUID      uuid.UUID      `gorm:"column:local_sku_id;type:char(36);not null;index;uniqueIndex:idx_source_sku" json:"localSkuId"`
	ExternalSKUID   string         `gorm:"column:external_sku_id;size:64" json:"externalSkuId,omitempty"`
	ExternalSpec    datatypes.JSON `gorm:"type:jsonb" json:"externalSpec,omitempty"`
	CurrentPrice    *float64       `gorm:"type:decimal(12,2)" json:"currentPrice,omitempty"`
	Currency        string         `gorm:"size:8;default:'CNY'" json:"currency"`
	CurrentStock    *int           `json:"currentStock,omitempty"`
	Status          string         `gorm:"size:16;not null;default:'active'" json:"status"`
}

func (ProductSourceSKU) TableName() string { return "product_source_skus" }

// SourcePriceHistory is an append-only price/stock snapshot for a source SKU.
type SourcePriceHistory struct {
	ID            int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID      int64          `gorm:"default:0;index" json:"tenantId"`
	SourceSKUID   uuid.UUID      `gorm:"column:source_sku_id;type:char(36);not null;index:idx_sph_sku_time,priority:1" json:"sourceSkuId"`
	Price         float64        `gorm:"type:decimal(12,2);not null" json:"price"`
	Stock         *int           `json:"stock,omitempty"`
	CapturedAt    time.Time      `gorm:"not null;index:idx_sph_sku_time,priority:2,sort:desc" json:"capturedAt"`
	CaptureSource string         `gorm:"size:16;not null" json:"captureSource"`
	RawData       datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
}

func (SourcePriceHistory) TableName() string { return "source_price_history" }

// SourceSwitchEvent records primary-source switches / suggestions (audit).
type SourceSwitchEvent struct {
	model.HardDeleteBase
	TenantID     int64          `gorm:"default:0;index" json:"tenantId"`
	ProductID    uuid.UUID      `gorm:"type:char(36);not null;index" json:"productId"`
	FromSourceID *uuid.UUID     `gorm:"type:char(36)" json:"fromSourceId,omitempty"`
	ToSourceID   uuid.UUID      `gorm:"type:char(36);not null" json:"toSourceId"`
	Reason       string         `gorm:"size:32;not null" json:"reason"`
	Detail       datatypes.JSON `gorm:"type:jsonb" json:"detail,omitempty"`
	Mode         string         `gorm:"size:16;not null" json:"mode"`
	Operator     *uuid.UUID     `gorm:"type:char(36)" json:"operator,omitempty"`
}

func (SourceSwitchEvent) TableName() string { return "source_switch_events" }
