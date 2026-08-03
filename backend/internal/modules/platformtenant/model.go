// Package platformtenant manages platform-level tenants. Only platform
// administrators (tenant 0 admin accounts) can list or create tenants.
package platformtenant

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PlatformTenantID is the reserved tenant id of the platform tenant. Admin
// accounts in this tenant are platform administrators.
const PlatformTenantID int64 = 0

// Tenant status values.
const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

// Tenant is a platform-managed tenant. Tenant 0 is the implicit platform
// tenant and has no row in this table.
type Tenant struct {
	ID        int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string         `gorm:"size:128;not null;uniqueIndex" json:"name"`
	Status    string         `gorm:"size:16;not null;default:active" json:"status"`
	CreatedBy *uuid.UUID     `gorm:"type:char(36)" json:"createdBy,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName keeps a stable table name for migrations.
func (Tenant) TableName() string {
	return "tenants"
}
