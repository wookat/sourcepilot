// Command seedperf seeds / cleans the PERF- prefixed load-test dataset
// (万级订单/采购/库存流水/自动化日志/回款费用) used by performance audits.
//
// Usage:
//
//	go run ./cmd/seedperf -mode seed   [-tenant 0]
//	go run ./cmd/seedperf -mode clean  [-tenant 0]
//	go run ./cmd/seedperf -mode verify [-tenant 0]
//
// seed is idempotent: it removes prior PERF- rows before inserting. clean /
// verify target the PERF- prefix only and never touch DEMO- demo data.
// Refuses to run when APP_ENV=production.
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
	"github.com/trademind-ai/trademind/backend/internal/modules/perfseed"
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
		fatal(fmt.Errorf("seedperf refuses to run in production"))
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
		fmt.Fprintf(os.Stderr, "seedperf: auto-resolved tenant id %d\n", tenantID)
		if tenantID <= 0 && *mode == "seed" {
			tenantID = 1
			fmt.Fprintln(os.Stderr, "seedperf: resolved tenant is not positive; falling back to tenant 1 (override with -tenant)")
		}
	}

	seeder := &perfseed.Seeder{DB: db, TenantID: tenantID, AppEnv: cfg.AppEnv}
	ctx := context.Background()

	var out any
	switch *mode {
	case "seed":
		out, err = seeder.Seed(ctx)
	case "clean":
		out, err = seeder.Cleanup(ctx)
	case "verify":
		res, verr := seeder.VerifyClean(ctx)
		err = verr
		out = res
		if verr == nil {
			for table, n := range res.Counts {
				if n > 0 {
					err = fmt.Errorf("residual %s rows in %s: %d", perfseed.PerfPrefix, table, n)
					break
				}
			}
		}
	default:
		err = fmt.Errorf("unknown mode %q (want seed|clean|verify)", *mode)
	}
	if b, jerr := json.MarshalIndent(out, "", "  "); jerr == nil && out != nil {
		fmt.Println(string(b))
	}
	if err != nil {
		fatal(err)
	}
	if *mode == "verify" {
		fmt.Printf("verify: zero %s residual rows\n", perfseed.PerfPrefix)
	}
}

// resolveTenant picks the earliest admin user's tenant, falling back to
// ADMIN_BOOTSTRAP_TENANT_ID.
func resolveTenant(db *gorm.DB, cfg *config.Config) int64 {
	var row admin.AdminUser
	if err := db.Order("created_at ASC").First(&row).Error; err == nil {
		return row.TenantID
	}
	return cfg.BootstrapAdminTenantID
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "seedperf:", err)
	os.Exit(1)
}
