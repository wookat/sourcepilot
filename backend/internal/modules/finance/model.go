// Package finance implements the R121 financial reconciliation loop:
// order payment records (回款), order-level / shop-monthly expense entries
// (费用记账) and the actual-vs-estimated gross profit views built on them.
// No platform API is required: records enter manually or via the migration
// import wizard and reconcile against order receivables.
package finance

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// Payment record sources.
const (
	SourceManual = "manual"
	SourceImport = "import"
)

// Settlement statuses derived from payments vs the order receivable.
const (
	SettlementUnpaid  = "unpaid"  // 未回款
	SettlementShort   = "short"   // 少款
	SettlementOver    = "over"    // 多款
	SettlementSettled = "settled" // 已结清
)

// SettlementTolerance is the amount (order currency) within which the
// received total is considered settled.
const SettlementTolerance = 0.01

// PaymentRecord is one platform payout registered against a sales order.
// ShopID snapshots the order's shop so store scope filters apply without a
// join; Amount is the gross payout and FeeAmount the platform手续费 deducted
// from it (net-in = Amount - FeeAmount).
type PaymentRecord struct {
	model.Base
	TenantID   int64      `gorm:"default:0;index" json:"tenantId"`
	OrderID    uuid.UUID  `gorm:"type:char(36);not null;index" json:"orderId"`
	ShopID     *uuid.UUID `gorm:"type:char(36);index" json:"shopId,omitempty"`
	Amount     float64    `gorm:"type:decimal(18,4);not null" json:"amount"`
	Currency   string     `gorm:"size:16;not null" json:"currency"`
	FeeAmount  float64    `gorm:"type:decimal(18,4);default:0" json:"feeAmount"`
	ReceivedAt time.Time  `gorm:"index;not null" json:"receivedAt"`
	Channel    string     `gorm:"size:64" json:"channel,omitempty"`
	Remark     string     `gorm:"size:512" json:"remark,omitempty"`
	Source     string     `gorm:"size:16;not null;default:'manual'" json:"source"`
	CreatedBy  *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

// TableName maps PaymentRecord to finance_payment_records.
func (PaymentRecord) TableName() string { return "finance_payment_records" }

// OrderExpense is one order-level expense entry (平台佣金 / 推广费 / 运费 /
// 其他, type codes are tenant-configurable via settings).
type OrderExpense struct {
	model.Base
	TenantID   int64      `gorm:"default:0;index" json:"tenantId"`
	OrderID    uuid.UUID  `gorm:"type:char(36);not null;index" json:"orderId"`
	ShopID     *uuid.UUID `gorm:"type:char(36);index" json:"shopId,omitempty"`
	TypeCode   string     `gorm:"size:64;not null" json:"typeCode"`
	Amount     float64    `gorm:"type:decimal(18,4);not null" json:"amount"`
	Currency   string     `gorm:"size:16;not null" json:"currency"`
	IncurredAt *time.Time `gorm:"index" json:"incurredAt,omitempty"`
	Remark     string     `gorm:"size:512" json:"remark,omitempty"`
	CreatedBy  *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

// TableName maps OrderExpense to finance_order_expenses.
func (OrderExpense) TableName() string { return "finance_order_expenses" }

// ShopMonthlyExpense is one shop-level monthly expense entry (Month is a
// local YYYY-MM string).
type ShopMonthlyExpense struct {
	model.Base
	TenantID  int64      `gorm:"default:0;index" json:"tenantId"`
	ShopID    uuid.UUID  `gorm:"type:char(36);not null;index" json:"shopId"`
	Month     string     `gorm:"size:7;not null;index" json:"month"`
	TypeCode  string     `gorm:"size:64;not null" json:"typeCode"`
	Amount    float64    `gorm:"type:decimal(18,4);not null" json:"amount"`
	Currency  string     `gorm:"size:16;not null" json:"currency"`
	Remark    string     `gorm:"size:512" json:"remark,omitempty"`
	CreatedBy *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

// TableName maps ShopMonthlyExpense to finance_shop_monthly_expenses.
func (ShopMonthlyExpense) TableName() string { return "finance_shop_monthly_expenses" }
