// Command seeddemo seeds / cleans a one-click demo dataset (DEMO- prefixed).
//
// Usage:
//
//	go run ./cmd/seeddemo -mode seed   [-tenant 0]
//	go run ./cmd/seeddemo -mode clean  [-tenant 0]
//	go run ./cmd/seeddemo -mode verify [-tenant 0]
//
// seed is idempotent: it removes prior DEMO- rows before inserting. Refuses to
// run when APP_ENV=production.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/demoseed"
	"gorm.io/gorm"
)

func main() {
	mode := flag.String("mode", "seed", "seed | clean | verify")
	tenant := flag.Int64("tenant", -1, "tenant id to seed into (-1 = auto: first admin user's tenant)")
	flag.Parse()

	_ = godotenv.Load()
	_ = godotenv.Load("../.env")

	cfg, err := config.Load()
	if err != nil {
		fatal(fmt.Errorf("load config: %w", err))
	}
	if config.IsProduction(cfg.AppEnv) {
		fatal(fmt.Errorf("seeddemo refuses to run in production"))
	}
	db, err := database.Open(cfg)
	if err != nil {
		fatal(fmt.Errorf("open database: %w", err))
	}
	defer func() { _ = database.Close(db) }()
	if err := database.AutoMigrate(db); err != nil {
		fatal(fmt.Errorf("auto migrate: %w", err))
	}

	tenantID := *tenant
	if tenantID < 0 {
		tenantID = resolveTenant(db, cfg)
		fmt.Fprintf(os.Stderr, "seeddemo: auto-resolved tenant id %d\n", tenantID)
	}

	seeder := &demoseed.FullDemoSeeder{DB: db, TenantID: tenantID, AppEnv: cfg.AppEnv}
	ctx := context.Background()

	var res *demoseed.FullDemoResult
	switch *mode {
	case "seed":
		res, err = seeder.Seed(ctx)
	case "clean":
		res, err = seeder.Cleanup(ctx)
	case "verify":
		res, err = seeder.VerifyClean(ctx)
	default:
		err = fmt.Errorf("unknown mode %q (want seed|clean|verify)", *mode)
	}
	if err != nil {
		fatal(err)
	}
	out, _ := json.MarshalIndent(res, "", "  ")
	fmt.Println(string(out))
	if *mode == "verify" {
		for table, n := range res.Counts {
			if n > 0 {
				fatal(fmt.Errorf("residual demo rows in %s: %d", table, n))
			}
		}
		fmt.Println("verify: zero DEMO- residual rows")
	}
}

// resolveTenant picks the tenant the demo data should belong to so it is
// visible to the bootstrap admin: the earliest admin user's tenant, falling
// back to ADMIN_BOOTSTRAP_TENANT_ID, then 0.
func resolveTenant(db *gorm.DB, cfg *config.Config) int64 {
	var row admin.AdminUser
	if err := db.Order("created_at ASC").First(&row).Error; err == nil {
		return row.TenantID
	}
	return cfg.BootstrapAdminTenantID
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "seeddemo:", err)
	os.Exit(1)
}
