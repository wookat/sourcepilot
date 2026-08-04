package demoseed

import (
	"context"
	"strings"
	"testing"
)

// Task workers (e.g. product publish) reject rows with tenant_id<=0, so Seed
// must refuse to write demo data into a non-positive tenant.
func TestSeedRejectsNonPositiveTenant(t *testing.T) {
	db := openFullDemoTestDB(t)
	if db == nil {
		return
	}
	for _, tid := range []int64{0, -1} {
		s := &FullDemoSeeder{DB: db, TenantID: tid, AppEnv: "development"}
		_, err := s.Seed(context.Background())
		if err == nil {
			t.Fatalf("Seed(tenant=%d) succeeded, want positive-tenant error", tid)
		}
		if !strings.Contains(err.Error(), "positive tenant id required") {
			t.Fatalf("Seed(tenant=%d) error = %v, want positive-tenant error", tid, err)
		}
	}
}
