package database

import (
	"fmt"

	"gorm.io/gorm"
)

// migrateRound188McpAuditIndex adds the composite index the round187
// performance audit found missing on the MCP audit table:
//
//   - mcp_tool_call_logs (tenant_id, created_at DESC): the audit list is
//     always tenant-scoped and paginated by created_at DESC; with only the
//     single-column created_at index, deep pagination walks the index
//     backwards and discards roughly half the rows through the tenant
//     filter, degrading linearly with the cross-tenant total.
//
// Safe to re-run (IF NOT EXISTS). Rollback: DROP INDEX IF EXISTS <name>.
func migrateRound188McpAuditIndex(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate round188: db is nil")
	}
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	sql := `CREATE INDEX IF NOT EXISTS idx_mcp_tool_call_logs_tenant_created
		 ON mcp_tool_call_logs (tenant_id, created_at DESC)`
	if err := db.Exec(sql).Error; err != nil {
		return fmt.Errorf("round188 index: %w", err)
	}
	return nil
}
