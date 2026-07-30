package selection

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// TaskParamOverrides are optional per-task overrides persisted in
// selection_tasks.params and applied over settings.selection defaults.
type TaskParamOverrides struct {
	ExchangeRate        *float64 `json:"exchangeRate,omitempty"`
	CommissionPercent   *float64 `json:"commissionPercent,omitempty"`
	LogisticsBaseFee    *float64 `json:"logisticsBaseFee,omitempty"`
	LogisticsPerKGFee   *float64 `json:"logisticsPerKgFee,omitempty"`
	LastMileFee         *float64 `json:"lastMileFee,omitempty"`
	ReturnRatePercent   *float64 `json:"returnRatePercent,omitempty"`
	FixedCostPerOrder   *float64 `json:"fixedCostPerOrder,omitempty"`
	MinMarginPercent    *float64 `json:"minMarginPercent,omitempty"`
	SourceMatchProvider string   `json:"sourceMatchProvider,omitempty"`
	TargetCurrency      string   `json:"targetCurrency,omitempty"`
}

// CandidateItem is one manually imported / collected candidate row.
type CandidateItem struct {
	Title          string   `json:"title"`
	ImageURL       string   `json:"imageUrl,omitempty"`
	Category       string   `json:"category,omitempty"`
	SourceURL      string   `json:"sourceUrl,omitempty"` // known 1688 offer url (crawler直连)
	MarketPrice    *float64 `json:"marketPrice,omitempty"`
	MarketCurrency string   `json:"marketCurrency,omitempty"`
	MarketSales30d *int     `json:"marketSales30d,omitempty"`
	WeightKG       *float64 `json:"weightKg,omitempty"`
}

// CreateTaskBody is POST /selection/tasks input. Candidate sources:
// items (manual import), productIds (existing drafts), keywords.
type CreateTaskBody struct {
	Name           string              `json:"name"`
	TargetPlatform string              `json:"targetPlatform" binding:"required"`
	TargetCountry  string              `json:"targetCountry"`
	Items          []CandidateItem     `json:"items"`
	ProductIDs     []string            `json:"productIds"`
	Keywords       []string            `json:"keywords"`
	Params         *TaskParamOverrides `json:"params"`
}

// DecisionBody is POST /selection/candidates/:id/decision input.
type DecisionBody struct {
	Decision string `json:"decision" binding:"required"` // approved|rejected
}

// TaskDTO is the list/detail projection of a selection task.
type TaskDTO struct {
	ID             uuid.UUID      `json:"id"`
	Name           string         `json:"name"`
	TargetPlatform string         `json:"targetPlatform"`
	TargetCountry  string         `json:"targetCountry"`
	Status         string         `json:"status"`
	Params         datatypes.JSON `json:"params,omitempty"`
	ErrorMessage   string         `json:"errorMessage,omitempty"`
	CandidateCount int64          `json:"candidateCount"`
	ScoredCount    int64          `json:"scoredCount"`
	FailedCount    int64          `json:"failedCount"`
	CreatedAt      time.Time      `json:"createdAt"`
	StartedAt      *time.Time     `json:"startedAt,omitempty"`
	FinishedAt     *time.Time     `json:"finishedAt,omitempty"`
}

// CandidateDTO joins candidate + evaluation + best match for the可上架清单.
type CandidateDTO struct {
	Candidate  SelectionCandidate     `json:"candidate"`
	Evaluation *SelectionEvaluation   `json:"evaluation,omitempty"`
	BestMatch  *SelectionSourceMatch  `json:"bestMatch,omitempty"`
	Matches    []SelectionSourceMatch `json:"matches,omitempty"`
}

// ListResult is a paged task list.
type ListResult struct {
	Items    []TaskDTO `json:"items"`
	Total    int64     `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
}
