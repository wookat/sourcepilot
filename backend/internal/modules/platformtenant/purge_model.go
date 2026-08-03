package platformtenant

import (
	"time"

	"github.com/google/uuid"
)

// Purge task status values.
const (
	PurgeStatusPending   = "pending"
	PurgeStatusRunning   = "running"
	PurgeStatusSucceeded = "succeeded"
	PurgeStatusFailed    = "failed"
)

// TenantPurgeTask records one background purge of a disabled tenant. The
// task row lives in tenant 0 scope (platform audit) and survives the purge.
type TenantPurgeTask struct {
	ID         uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	TenantID   int64      `gorm:"not null;index:idx_tenant_purge_tasks_tenant_created" json:"tenantId"`
	TenantName string     `gorm:"size:128;not null" json:"tenantName"`
	Status     string     `gorm:"size:16;not null;default:pending" json:"status"`
	Report     string     `gorm:"type:text" json:"-"`
	Error      string     `gorm:"type:text" json:"error,omitempty"`
	CreatedBy  *uuid.UUID `gorm:"type:char(36)" json:"createdBy,omitempty"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	CreatedAt  time.Time  `gorm:"index:idx_tenant_purge_tasks_tenant_created" json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

// TableName keeps a stable table name for migrations.
func (TenantPurgeTask) TableName() string {
	return "tenant_purge_tasks"
}
