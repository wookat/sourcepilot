// Package selection implements the AI 比价选品引擎: candidates → overseas market
// price → 1688 source match → profit model → LLM scoring → 可上架清单.
package selection

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// Selection task status values (same vocabulary as collect tasks).
const (
	StatusPending = "pending"
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"
	StatusPartial = "partial"
)

// Candidate status values.
const (
	CandidatePending = "pending"
	CandidatePriced  = "priced"
	CandidateMatched = "matched"
	CandidateScored  = "scored"
	CandidateFailed  = "failed"
)

// Evaluation decision values (人工审核).
const (
	DecisionPending  = "pending"
	DecisionApproved = "approved"
	DecisionRejected = "rejected"
)

// SelectionTask is one batch selection run.
type SelectionTask struct {
	model.HardDeleteBase
	TenantID       int64          `gorm:"not null;default:0;index" json:"tenantId"`
	Name           string         `gorm:"size:200" json:"name"`
	TargetPlatform string         `gorm:"size:32;not null" json:"targetPlatform"`
	TargetCountry  string         `gorm:"size:8" json:"targetCountry"`
	Status         string         `gorm:"size:32;index;not null" json:"status"`
	Params         datatypes.JSON `gorm:"type:jsonb" json:"params,omitempty"`
	ErrorMessage   string         `gorm:"type:text" json:"errorMessage,omitempty"`
	RetryCount     int            `gorm:"not null;default:0" json:"retryCount"`
	MaxRetries     int            `gorm:"not null;default:3" json:"maxRetries"`
	CreatedBy      *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	StartedAt      *time.Time     `json:"startedAt,omitempty"`
	FinishedAt     *time.Time     `json:"finishedAt,omitempty"`
	LockedBy       *string        `gorm:"size:220;index" json:"lockedBy,omitempty"`
	LockedUntil    *time.Time     `gorm:"index" json:"lockedUntil,omitempty"`
	LockVersion    int            `gorm:"default:0;not null" json:"lockVersion"`
	HeartbeatAt    *time.Time     `gorm:"index" json:"heartbeatAt,omitempty"`
	ExecutionID    *string        `gorm:"size:36;index" json:"executionId,omitempty"`
}

// TableName implements gorm table naming.
func (SelectionTask) TableName() string { return "selection_tasks" }

// SelectionCandidate is one product under evaluation inside a task.
type SelectionCandidate struct {
	model.HardDeleteBase
	TenantID       int64          `gorm:"not null;default:0;index" json:"tenantId"`
	TaskID         uuid.UUID      `gorm:"type:char(36);index;not null" json:"taskId"`
	ProductID      *uuid.UUID     `gorm:"type:char(36);index" json:"productId,omitempty"`
	Title          string         `gorm:"type:text" json:"title"`
	ImageURL       string         `gorm:"type:text" json:"imageUrl,omitempty"`
	Category       string         `gorm:"size:128" json:"category,omitempty"`
	SourceURL      string         `gorm:"size:2048" json:"sourceUrl,omitempty"`
	MarketPlatform string         `gorm:"size:32" json:"marketPlatform,omitempty"`
	MarketPrice    *float64       `gorm:"type:numeric(12,2)" json:"marketPrice,omitempty"`
	MarketCurrency string         `gorm:"size:8" json:"marketCurrency,omitempty"`
	MarketSales30d *int           `json:"marketSales30d,omitempty"`
	MarketRaw      datatypes.JSON `gorm:"type:jsonb" json:"marketRaw,omitempty"`
	Status         string         `gorm:"size:32;index;not null" json:"status"`
	ErrorMessage   string         `gorm:"type:text" json:"errorMessage,omitempty"`
}

// TableName implements gorm table naming.
func (SelectionCandidate) TableName() string { return "selection_candidates" }

// SelectionSourceMatch is one matched 1688 offer for a candidate.
type SelectionSourceMatch struct {
	model.HardDeleteBase
	TenantID       int64          `gorm:"not null;default:0;index" json:"tenantId"`
	CandidateID    uuid.UUID      `gorm:"type:char(36);index;not null" json:"candidateId"`
	SourcePlatform string         `gorm:"size:32;not null;default:'1688'" json:"sourcePlatform"`
	SourceURL      string         `gorm:"size:2048" json:"sourceUrl,omitempty"`
	SourceOfferID  string         `gorm:"size:64" json:"sourceOfferId,omitempty"`
	MatchMethod    string         `gorm:"size:16" json:"matchMethod,omitempty"`
	Similarity     *float64       `gorm:"type:numeric(5,4)" json:"similarity,omitempty"`
	MinPrice       *float64       `gorm:"type:numeric(12,2)" json:"minPrice,omitempty"`
	MaxPrice       *float64       `gorm:"type:numeric(12,2)" json:"maxPrice,omitempty"`
	Currency       string         `gorm:"size:8;default:'CNY'" json:"currency"`
	MOQ            *int           `json:"moq,omitempty"`
	SupplierName   string         `gorm:"size:255" json:"supplierName,omitempty"`
	SupplierRating *float64       `gorm:"type:numeric(4,2)" json:"supplierRating,omitempty"`
	RawData        datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
}

// TableName implements gorm table naming.
func (SelectionSourceMatch) TableName() string { return "selection_source_matches" }

// SelectionEvaluation is the final profit + AI score row (one per candidate).
type SelectionEvaluation struct {
	model.HardDeleteBase
	TenantID         int64          `gorm:"not null;default:0;index" json:"tenantId"`
	CandidateID      uuid.UUID      `gorm:"type:char(36);uniqueIndex;not null" json:"candidateId"`
	BestMatchID      *uuid.UUID     `gorm:"type:char(36)" json:"bestMatchId,omitempty"`
	PurchaseCost     *float64       `gorm:"type:numeric(12,2)" json:"purchaseCost,omitempty"`
	ShippingCost     *float64       `gorm:"type:numeric(12,2)" json:"shippingCost,omitempty"`
	CommissionFee    *float64       `gorm:"type:numeric(12,2)" json:"commissionFee,omitempty"`
	ExchangeRate     *float64       `gorm:"type:numeric(12,6)" json:"exchangeRate,omitempty"`
	LandedCost       *float64       `gorm:"type:numeric(12,2)" json:"landedCost,omitempty"`
	EstProfit        *float64       `gorm:"type:numeric(12,2)" json:"estProfit,omitempty"`
	EstMarginPercent *float64       `gorm:"type:numeric(6,2)" json:"estMarginPercent,omitempty"`
	AIScore          *float64       `gorm:"type:numeric(5,2);index" json:"aiScore,omitempty"`
	AIReasons        datatypes.JSON `gorm:"type:jsonb" json:"aiReasons,omitempty"`
	AIModel          string         `gorm:"size:64" json:"aiModel,omitempty"`
	AITaskID         *uuid.UUID     `gorm:"type:char(36)" json:"aiTaskId,omitempty"`
	Decision         string         `gorm:"size:16;index;not null;default:'pending'" json:"decision"`
	DecidedBy        *uuid.UUID     `gorm:"type:char(36)" json:"decidedBy,omitempty"`
	DecidedAt        *time.Time     `json:"decidedAt,omitempty"`
	DraftProductID   *uuid.UUID     `gorm:"type:char(36);index" json:"draftProductId,omitempty"`
}

// TableName implements gorm table naming.
func (SelectionEvaluation) TableName() string { return "selection_evaluations" }
