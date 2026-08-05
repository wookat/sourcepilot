package demoseed

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/migrationimport"
	"gorm.io/gorm"
)

// demoImportJobIDs selects migration import jobs that belong to the target
// test dataset: jobs whose file name or batch key carries the prefix, plus
// jobs committed against a prefixed (demo) shop. Real imports are never
// matched.
func demoImportJobIDs(tx *gorm.DB, like string, demoShopIDs []uuid.UUID) *gorm.DB {
	q := tx.Model(&migrationimport.ImportJob{}).Unscoped().Select("id").
		Where("file_name LIKE ? OR batch_key LIKE ?", like, like)
	if len(demoShopIDs) > 0 {
		q = tx.Model(&migrationimport.ImportJob{}).Unscoped().Select("id").
			Where("file_name LIKE ? OR batch_key LIKE ? OR shop_id IN ?", like, like, demoShopIDs)
	}
	return q
}

// cleanupMigrationImports removes migration import artifacts (import_jobs +
// import_job_rows) carrying the target prefix or attached to demo shops.
// Drafts and orders created by those imports carry prefixed titles / SKU
// codes / order numbers and are removed by the existing prefix cleanup.
func cleanupMigrationImports(tx *gorm.DB, res *FullDemoResult, like string, demoShopIDs []uuid.UUID) error {
	if !tx.Migrator().HasTable("import_jobs") {
		return nil
	}
	del := func(table string, q *gorm.DB) error {
		if q.Error != nil {
			return fmt.Errorf("demoseed cleanup %s: %w", table, q.Error)
		}
		res.Counts[table] += q.RowsAffected
		return nil
	}
	jobIDs := demoImportJobIDs(tx, like, demoShopIDs)
	if tx.Migrator().HasTable("import_job_rows") {
		if err := del("import_job_rows",
			tx.Unscoped().Where("job_id IN (?)", jobIDs).Delete(&migrationimport.ImportJobRow{})); err != nil {
			return err
		}
	}
	if err := del("import_jobs",
		tx.Unscoped().Where("id IN (?)", demoImportJobIDs(tx, like, demoShopIDs)).
			Delete(&migrationimport.ImportJob{})); err != nil {
		return err
	}
	return nil
}

// migrationImportVerifyChecks counts residual migration import artifacts for
// VerifyClean (demo-shop-attached jobs are matched through the prefixed shop
// subquery so a missed job still surfaces alongside its residual shop).
func migrationImportVerifyChecks(tx *gorm.DB, like string, demoShopIDs func() *gorm.DB) []verifyCheck {
	return []verifyCheck{
		{table: "import_jobs", count: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("import_jobs") {
				return 0, nil
			}
			return n, tx.Model(&migrationimport.ImportJob{}).Unscoped().
				Where("(file_name LIKE ? OR batch_key LIKE ? OR shop_id IN (?)) AND import_jobs.deleted_at IS NULL", like, like, demoShopIDs()).
				Count(&n).Error
		}, softDeleted: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("import_jobs") {
				return 0, nil
			}
			return n, tx.Model(&migrationimport.ImportJob{}).Unscoped().
				Where("(file_name LIKE ? OR batch_key LIKE ? OR shop_id IN (?)) AND import_jobs.deleted_at IS NOT NULL", like, like, demoShopIDs()).
				Count(&n).Error
		}},
		{table: "import_job_rows", count: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("import_jobs") || !tx.Migrator().HasTable("import_job_rows") {
				return 0, nil
			}
			jobIDs := tx.Model(&migrationimport.ImportJob{}).Unscoped().Select("id").
				Where("file_name LIKE ? OR batch_key LIKE ? OR shop_id IN (?)", like, like, demoShopIDs())
			return n, tx.Model(&migrationimport.ImportJobRow{}).
				Where("job_id IN (?)", jobIDs).Count(&n).Error
		}},
	}
}
