// Package migrationimport implements the seller migration channel: file-based
// product / order imports (Dianxiaomi / Mabang style exports) with column
// mapping, per-row validation, batch idempotency and import history.
package migrationimport

import (
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// Import kinds.
const (
	KindProduct = "product"
	KindOrder   = "order"
)

// Source formats (detected from headers or chosen by the user).
const (
	SourceDianxiaomi = "dianxiaomi"
	SourceMabang     = "mabang"
	SourceCustom     = "custom"
)

// Job statuses.
const (
	JobStatusSuccess        = "success"
	JobStatusPartialSuccess = "partial_success"
	JobStatusFailed         = "failed"
)

// Row statuses (only failed / duplicate rows are persisted).
const (
	RowStatusFailed    = "failed"
	RowStatusDuplicate = "duplicate"
)

// MaxImportRows caps a single import batch.
const MaxImportRows = 1000

// ImportJob is one committed import batch (task-style history record).
type ImportJob struct {
	model.Base
	TenantID      int64      `gorm:"default:0;index;uniqueIndex:idx_import_jobs_batch" json:"tenantId"`
	Kind          string     `gorm:"size:16;index;not null;uniqueIndex:idx_import_jobs_batch" json:"kind"`
	BatchKey      string     `gorm:"size:128;not null;uniqueIndex:idx_import_jobs_batch" json:"batchKey"`
	ShopID        *uuid.UUID `gorm:"type:char(36);index" json:"shopId,omitempty"`
	SourceFormat  string     `gorm:"size:32;not null" json:"sourceFormat"`
	FileName      string     `gorm:"size:512" json:"fileName"`
	Status        string     `gorm:"size:32;index;not null" json:"status"`
	TotalRows     int        `gorm:"not null" json:"totalRows"`
	SuccessRows   int        `gorm:"not null" json:"successRows"`
	FailedRows    int        `gorm:"not null" json:"failedRows"`
	DuplicateRows int        `gorm:"not null" json:"duplicateRows"`
	CreatedBy     *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

// TableName maps ImportJob to import_jobs.
func (ImportJob) TableName() string { return "import_jobs" }

// ImportJobRow persists one failed or duplicate source row for the error report.
type ImportJobRow struct {
	model.HardDeleteBase
	JobID     uuid.UUID      `gorm:"type:char(36);index;not null" json:"jobId"`
	RowNumber int            `gorm:"not null" json:"rowNumber"`
	Status    string         `gorm:"size:16;index;not null" json:"status"`
	Field     string         `gorm:"size:64" json:"field,omitempty"`
	Message   string         `gorm:"size:512" json:"message"`
	RawValues datatypes.JSON `gorm:"type:jsonb" json:"rawValues,omitempty"`
}

// TableName maps ImportJobRow to import_job_rows.
func (ImportJobRow) TableName() string { return "import_job_rows" }
