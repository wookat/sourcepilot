// Package faildb builds a *gorm.DB whose every query fails, so tests can
// assert that permission and scope resolution fails closed when the database
// is unreachable (connection exhaustion, timeouts, degraded replicas).
package faildb

import (
	"database/sql"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Closed returns a gorm handle backed by an already-closed pool: no network
// traffic, every statement returns "sql: database is closed".
func Closed() (*gorm.DB, error) {
	sqlDB, err := sql.Open("pgx", "postgres://faildb:faildb@127.0.0.1:1/faildb")
	if err != nil {
		return nil, fmt.Errorf("faildb: open: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return nil, fmt.Errorf("faildb: close: %w", err)
	}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               logger.Discard,
	})
	if err != nil {
		return nil, fmt.Errorf("faildb: gorm: %w", err)
	}
	return db, nil
}
