package productpublish

import (
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/productcheck"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"gorm.io/gorm"
)

// Service wires DB + outbound provider execution for product_publish_tasks.
type Service struct {
	DB          *gorm.DB
	Redis       *rdb.Client
	Shops       *shop.Service
	Settings    *settings.Service
	OpLog       *operationlog.Service
	Readiness   *productcheck.Service
	Idempotency *idempotency.Service

	QueueEnabled bool
	QueueName    string
	TaskTimeout  time.Duration

	// AllowTenantZeroTasks lets queue workers process platform-tenant
	// (tenant 0) demo seed tasks, mirroring the API-side demo gate
	// (EnableDemoSeed && !production). Publish capability limits such as
	// local_draft_only still apply, so no real platform call is made.
	AllowTenantZeroTasks bool

	BatchMaxProducts int
	BatchMaxTargets  int
	BatchMaxTasks    int
}

func (s *Service) normalizedQueueName() string {
	q := strings.TrimSpace(s.QueueName)
	if q == "" {
		return "product:publish:tasks"
	}
	return q
}
