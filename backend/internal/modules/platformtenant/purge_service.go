package platformtenant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/logger"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
	"gorm.io/gorm"
)

var (
	ErrPlatformTenantNoPurge = errors.New("平台租户（tenant 0）不可清退")
	ErrTenantNotDisabled     = errors.New("租户未停用，请先停用后再清退")
	ErrPurgeConfirmMismatch  = errors.New("确认名称与租户名称不一致")
	ErrPurgeAlreadyRunning   = errors.New("该租户已有清退任务在执行中")
)

// purgeTimeout bounds one background purge run.
const purgeTimeout = 30 * time.Minute

// purgeSweepExcludedTables are tables carrying a tenant_id column that must
// survive a purge: the purge task history itself is platform-side audit data.
var purgeSweepExcludedTables = map[string]bool{
	"tenant_purge_tasks": true,
}

// PurgeReport is the per-table residual report attached to a purge task.
type PurgeReport struct {
	Tables     map[string]int64 `json:"tables"`
	Total      int64            `json:"total"`
	VerifiedAt string           `json:"verifiedAt"`
}

// PurgeTaskDTO is the API shape of a purge task.
type PurgeTaskDTO struct {
	ID         string       `json:"id"`
	TenantID   int64        `json:"tenantId"`
	TenantName string       `json:"tenantName"`
	Status     string       `json:"status"`
	Error      string       `json:"error,omitempty"`
	Report     *PurgeReport `json:"report,omitempty"`
	StartedAt  string       `json:"startedAt,omitempty"`
	FinishedAt string       `json:"finishedAt,omitempty"`
	CreatedAt  string       `json:"createdAt"`
}

func purgeTaskDTO(t *TenantPurgeTask) *PurgeTaskDTO {
	if t == nil {
		return nil
	}
	dto := &PurgeTaskDTO{
		ID:         t.ID.String(),
		TenantID:   t.TenantID,
		TenantName: t.TenantName,
		Status:     t.Status,
		Error:      t.Error,
		CreatedAt:  t.CreatedAt.UTC().Format(time.RFC3339),
	}
	if t.StartedAt != nil {
		dto.StartedAt = t.StartedAt.UTC().Format(time.RFC3339)
	}
	if t.FinishedAt != nil {
		dto.FinishedAt = t.FinishedAt.UTC().Format(time.RFC3339)
	}
	if strings.TrimSpace(t.Report) != "" {
		var rep PurgeReport
		if err := json.Unmarshal([]byte(t.Report), &rep); err == nil {
			dto.Report = &rep
		}
	}
	return dto
}

// StartPurge validates the safety gates and enqueues a background purge of a
// disabled tenant. Tenant 0 can never be purged.
func (s *Service) StartPurge(c *gin.Context, id int64, confirmName string, actorID *uuid.UUID) (*PurgeTaskDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("platformtenant: no db")
	}
	if id == PlatformTenantID {
		return nil, ErrPlatformTenantNoPurge
	}
	ctx := c.Request.Context()
	var t Tenant
	if err := s.DB.WithContext(ctx).First(&t, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}
	if normalizeStatus(t.Status) != StatusDisabled {
		return nil, ErrTenantNotDisabled
	}
	if strings.TrimSpace(confirmName) != t.Name {
		return nil, ErrPurgeConfirmMismatch
	}
	var running int64
	if err := s.DB.WithContext(ctx).Model(&TenantPurgeTask{}).
		Where("tenant_id = ? AND status IN ?", id, []string{PurgeStatusPending, PurgeStatusRunning}).
		Count(&running).Error; err != nil {
		return nil, err
	}
	if running > 0 {
		return nil, ErrPurgeAlreadyRunning
	}
	task := &TenantPurgeTask{
		ID:         uuid.New(),
		TenantID:   t.ID,
		TenantName: t.Name,
		Status:     PurgeStatusPending,
		CreatedBy:  actorID,
	}
	if err := s.DB.WithContext(ctx).Create(task).Error; err != nil {
		return nil, err
	}
	s.writeOpLog(c, actorID, "tenant.purge.start", t.ID,
		fmt.Sprintf("tenantId=%d name=%s purgeTaskId=%s", t.ID, t.Name, task.ID))
	if s.PurgeSync {
		s.runPurge(task.ID)
	} else {
		go s.runPurge(task.ID)
	}
	return purgeTaskDTO(task), nil
}

// LatestPurge returns the most recent purge task of a tenant (running or
// finished). Works after the tenant row itself has been deleted.
func (s *Service) LatestPurge(c *gin.Context, id int64) (*PurgeTaskDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("platformtenant: no db")
	}
	if id == PlatformTenantID {
		return nil, ErrPlatformTenantNoPurge
	}
	var task TenantPurgeTask
	err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ?", id).Order("created_at DESC").First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}
	return purgeTaskDTO(&task), nil
}

// runPurge executes one purge task in the background: cascade-delete all
// business data of the tenant, then verify zero residual rows per table.
func (s *Service) runPurge(taskID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), purgeTimeout)
	defer cancel()
	log := logger.L().With("module", "platformtenant", "purgeTaskId", taskID.String())
	var task TenantPurgeTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		log.Error("purge task load failed", "err", err)
		return
	}
	now := time.Now().UTC()
	if err := s.DB.WithContext(ctx).Model(&task).
		Updates(map[string]any{"status": PurgeStatusRunning, "started_at": now}).Error; err != nil {
		log.Error("purge task start update failed", "err", err)
		return
	}
	report, err := s.purgeTenantData(ctx, task.TenantID)
	finish := map[string]any{"finished_at": time.Now().UTC()}
	action := "tenant.purge.done"
	status := "success"
	msg := fmt.Sprintf("tenantId=%d name=%s purgeTaskId=%s", task.TenantID, task.TenantName, task.ID)
	if err != nil {
		finish["status"] = PurgeStatusFailed
		finish["error"] = err.Error()
		action = "tenant.purge.failed"
		status = "failed"
		msg += " error=" + err.Error()
		log.Error("tenant purge failed", "tenantId", task.TenantID, "err", err)
	} else {
		finish["status"] = PurgeStatusSucceeded
		if b, jerr := json.Marshal(report); jerr == nil {
			finish["report"] = string(b)
		}
		msg += fmt.Sprintf(" residualTotal=%d tables=%d", report.Total, len(report.Tables))
		log.Info("tenant purge succeeded", "tenantId", task.TenantID, "residualTotal", report.Total)
	}
	if uerr := s.DB.WithContext(ctx).Model(&task).Updates(finish).Error; uerr != nil {
		log.Error("purge task finish update failed", "err", uerr)
	}
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(context.Background(), operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      action,
			Resource:    "tenant",
			ResourceID:  fmt.Sprintf("%d", task.TenantID),
			Status:      status,
			Message:     msg,
		})
	}
}

// tenantChildIDs are pre-collected parent ids used to cascade into child
// tables that carry no tenant_id column, and to verify them afterwards.
type tenantChildIDs struct {
	Users         []string
	Shops         []string
	Products      []string
	Orders        []string
	Conversations []string
	Alerts        []string
	CollectTasks  []string
	TextBatches   []string
	ImageBatches  []string
	ImageTasks    []string
	Publications  []string
}

func pluckIDs(tx *gorm.DB, query string, args ...any) ([]string, error) {
	var ids []string
	if err := tx.Raw(query, args...).Scan(&ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func collectTenantChildIDs(tx *gorm.DB, tenantID int64) (*tenantChildIDs, error) {
	ids := &tenantChildIDs{}
	steps := []struct {
		dst   *[]string
		table string
	}{
		{&ids.Users, "admin_users"},
		{&ids.Shops, "shops"},
		{&ids.Products, "products"},
		{&ids.Orders, "orders"},
		{&ids.Conversations, "customer_conversations"},
		{&ids.Alerts, "task_alerts"},
		{&ids.CollectTasks, "collect_tasks"},
		{&ids.TextBatches, "ai_product_text_batches"},
		{&ids.ImageBatches, "ai_product_image_batches"},
	}
	for _, st := range steps {
		if !tx.Migrator().HasTable(st.table) {
			continue
		}
		got, err := pluckIDs(tx, fmt.Sprintf("SELECT id FROM %s WHERE tenant_id = ?", quoteIdent(tx, st.table)), tenantID)
		if err != nil {
			return nil, err
		}
		*st.dst = got
	}
	// image_tasks / product_publications carry no tenant_id: owned via
	// product, shop or creator.
	var err error
	if tx.Migrator().HasTable("image_tasks") {
		ids.ImageTasks, err = inChunks2(ids.Products, ids.Users, func(prods, users []string) ([]string, error) {
			return pluckIDs(tx, "SELECT id FROM image_tasks WHERE product_id IN ? OR created_by IN ?", prods, users)
		})
		if err != nil {
			return nil, err
		}
	}
	if tx.Migrator().HasTable("product_publications") {
		ids.Publications, err = inChunks2(ids.Products, ids.Shops, func(prods, shops []string) ([]string, error) {
			return pluckIDs(tx, "SELECT id FROM product_publications WHERE product_id IN ? OR shop_id IN ?", prods, shops)
		})
		if err != nil {
			return nil, err
		}
	}
	return ids, nil
}

const purgeChunkSize = 500

func chunks(ids []string) [][]string {
	var out [][]string
	for len(ids) > purgeChunkSize {
		out = append(out, ids[:purgeChunkSize])
		ids = ids[purgeChunkSize:]
	}
	if len(ids) > 0 {
		out = append(out, ids)
	}
	return out
}

// inChunks2 runs fn over the cross product of chunked id lists (either list
// may be empty; fn is invoked with a single-element sentinel to keep the SQL
// shape valid while never matching real rows).
func inChunks2(a, b []string, fn func(a, b []string) ([]string, error)) ([]string, error) {
	sentinel := []string{"00000000-0000-0000-0000-000000000000"}
	ca, cb := chunks(a), chunks(b)
	if len(ca) == 0 && len(cb) == 0 {
		return nil, nil
	}
	if len(ca) == 0 {
		ca = [][]string{sentinel}
	}
	if len(cb) == 0 {
		cb = [][]string{sentinel}
	}
	seen := map[string]bool{}
	var out []string
	for _, xa := range ca {
		for _, xb := range cb {
			got, err := fn(xa, xb)
			if err != nil {
				return nil, err
			}
			for _, id := range got {
				if !seen[id] {
					seen[id] = true
					out = append(out, id)
				}
			}
		}
	}
	return out, nil
}

type childStep struct {
	table string
	where string
	keys  []string
}

// childSteps enumerate every child table without a tenant_id column that is
// tenant-owned through a parent, in delete-safe (children first) order. The
// same list drives both the DELETE phase and the residual verification.
func childSteps(ids *tenantChildIDs) []childStep {
	return []childStep{
		{"task_alert_notifications", "alert_id IN ?", ids.Alerts},
		{"ai_image_task_items", "task_id IN ?", ids.ImageTasks},
		{"ai_product_text_items", "batch_id IN ?", ids.TextBatches},
		{"ai_product_image_items", "batch_id IN ?", ids.ImageBatches},
		{"image_tasks", "id IN ?", ids.ImageTasks},
		{"ai_tasks#product", "product_id IN ?", ids.Products},
		{"ai_tasks#conversation", "conversation_id IN ?", ids.Conversations},
		{"ai_tasks#creator", "created_by IN ?", ids.Users},
		{"collect_task_events", "task_id IN ?", ids.CollectTasks},
		{"product_publication_skus", "publication_id IN ?", ids.Publications},
		{"product_publications", "id IN ?", ids.Publications},
		{"product_skus", "product_id IN ?", ids.Products},
		{"product_images", "product_id IN ?", ids.Products},
		{"product_ai_content_applications", "product_id IN ?", ids.Products},
		{"product_image_applications", "product_id IN ?", ids.Products},
		{"product_platform_publish_configs", "product_id IN ?", ids.Products},
		{"order_items", "order_id IN ?", ids.Orders},
		{"order_shipments", "order_id IN ?", ids.Orders},
		{"order_item_sku_matches", "order_id IN ?", ids.Orders},
		{"order_exception_marks", "order_id IN ?", ids.Orders},
		{"order_inventory_effects", "order_id IN ?", ids.Orders},
		{"customer_messages", "conversation_id IN ?", ids.Conversations},
		{"customer_reply_suggestions", "conversation_id IN ?", ids.Conversations},
		{"customer_failure_events", "conversation_id IN ?", ids.Conversations},
		{"shop_auth_tokens", "shop_id IN ?", ids.Shops},
		{"douyin_sync_cursors", "shop_id IN ?", ids.Shops},
		{"douyin_oauth_states", "user_id IN ?", ids.Users},
		{"user_store_permissions#user", "user_id IN ?", ids.Users},
		{"user_store_permissions#store", "store_id IN ?", ids.Shops},
	}
}

func childStepTable(name string) string {
	if i := strings.IndexByte(name, '#'); i > 0 {
		return name[:i]
	}
	return name
}

// tenantColumnTables lists every base table in the current schema carrying a
// tenant_id column (the generic sweep target), excluding platform audit
// tables that must survive a purge.
func tenantColumnTables(tx *gorm.DB) ([]string, error) {
	if isSQLite(tx) {
		return tenantColumnTablesSQLite(tx)
	}
	schemaExpr := "current_schema()"
	if isMySQL(tx) {
		schemaExpr = "DATABASE()"
	}
	var tables []string
	q := fmt.Sprintf(
		"SELECT c.table_name FROM information_schema.columns c "+
			"JOIN information_schema.tables t ON t.table_schema = c.table_schema AND t.table_name = c.table_name "+
			"WHERE c.table_schema = %s AND c.column_name = 'tenant_id' AND t.table_type = 'BASE TABLE' "+
			"ORDER BY c.table_name", schemaExpr)
	if err := tx.Raw(q).Scan(&tables).Error; err != nil {
		return nil, err
	}
	out := tables[:0]
	for _, t := range tables {
		if !purgeSweepExcludedTables[t] {
			out = append(out, t)
		}
	}
	return out, nil
}

// purgeTenantData hard-deletes all business data of one tenant and returns a
// per-table zero-residual report. It fails when residual rows remain.
func (s *Service) purgeTenantData(ctx context.Context, tenantID int64) (*PurgeReport, error) {
	if tenantID == PlatformTenantID {
		return nil, ErrPlatformTenantNoPurge
	}
	db := s.DB.WithContext(ctx)
	var childIDs *tenantChildIDs
	err := db.Transaction(func(outerTx *gorm.DB) error {
		// Immutable audit guards (approval_records / execution_errors /
		// operation_task_events; on Postgres the replica session role also
		// covers the p9 audit triggers) must be lifted for this trusted
		// maintenance flow, same as demoseed cleanup.
		return operationtask.WithImmutableGuardsDisabled(outerTx, func(tx *gorm.DB) error {
			return s.purgeTenantTx(tx, tenantID, &childIDs)
		})
	})
	if err != nil {
		return nil, err
	}
	report, err := s.verifyTenantPurged(ctx, tenantID, childIDs)
	if err != nil {
		return nil, err
	}
	if report.Total > 0 {
		return report, fmt.Errorf("清退后仍有 %d 行残留数据", report.Total)
	}
	return report, nil
}

func (s *Service) purgeTenantTx(tx *gorm.DB, tenantID int64, childIDsOut **tenantChildIDs) error {
	childIDs, err := collectTenantChildIDs(tx, tenantID)
	if err != nil {
		return fmt.Errorf("collect child ids: %w", err)
	}
	*childIDsOut = childIDs
	for _, st := range childSteps(childIDs) {
		table := childStepTable(st.table)
		if !tx.Migrator().HasTable(table) {
			continue
		}
		for _, chunk := range chunks(st.keys) {
			sql := fmt.Sprintf("DELETE FROM %s WHERE %s", quoteIdent(tx, table), st.where)
			if err := tx.Exec(sql, chunk).Error; err != nil {
				return fmt.Errorf("purge %s: %w", st.table, err)
			}
		}
	}
	tables, err := tenantColumnTables(tx)
	if err != nil {
		return fmt.Errorf("list tenant tables: %w", err)
	}
	// Sweep with FK-aware retries: tables blocked by a foreign key are
	// retried after their referencing tables have been emptied. Each
	// DELETE runs in a savepoint so a FK failure does not abort the
	// enclosing transaction (Postgres 25P02).
	pending := tables
	for len(pending) > 0 {
		var next []string
		var lastErr error
		for _, table := range pending {
			delErr := tx.Transaction(func(sp *gorm.DB) error {
				return sp.Exec(fmt.Sprintf("DELETE FROM %s WHERE tenant_id = ?", quoteIdent(sp, table)), tenantID).Error
			})
			if delErr != nil {
				next = append(next, table)
				lastErr = fmt.Errorf("purge %s: %w", table, delErr)
			}
		}
		if len(next) == len(pending) {
			return lastErr
		}
		pending = next
	}
	if err := tx.Exec("DELETE FROM tenants WHERE id = ?", tenantID).Error; err != nil {
		return fmt.Errorf("purge tenants: %w", err)
	}
	return nil
}

// verifyTenantPurged counts residual rows per table after a purge. All
// counts must be zero.
func (s *Service) verifyTenantPurged(ctx context.Context, tenantID int64, childIDs *tenantChildIDs) (*PurgeReport, error) {
	tx := s.DB.WithContext(ctx)
	report := &PurgeReport{Tables: map[string]int64{}, VerifiedAt: time.Now().UTC().Format(time.RFC3339)}
	tables, err := tenantColumnTables(tx)
	if err != nil {
		return nil, err
	}
	for _, table := range tables {
		var n int64
		if err := tx.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE tenant_id = ?", quoteIdent(tx, table)), tenantID).Scan(&n).Error; err != nil {
			return nil, fmt.Errorf("verify %s: %w", table, err)
		}
		report.Tables[table] = n
		report.Total += n
	}
	var tn int64
	if err := tx.Raw("SELECT COUNT(*) FROM tenants WHERE id = ?", tenantID).Scan(&tn).Error; err != nil {
		return nil, fmt.Errorf("verify tenants: %w", err)
	}
	report.Tables["tenants"] = tn
	report.Total += tn
	if childIDs != nil {
		for _, ck := range childSteps(childIDs) {
			table := childStepTable(ck.table)
			if !tx.Migrator().HasTable(table) {
				continue
			}
			var sum int64
			for _, chunk := range chunks(ck.keys) {
				var n int64
				sql := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", quoteIdent(tx, table), ck.where)
				if err := tx.Raw(sql, chunk).Scan(&n).Error; err != nil {
					return nil, fmt.Errorf("verify %s: %w", ck.table, err)
				}
				sum += n
			}
			report.Tables[ck.table] += sum
			report.Total += sum
		}
	}
	return report, nil
}

func isMySQL(tx *gorm.DB) bool {
	return tx != nil && tx.Dialector != nil && tx.Dialector.Name() == "mysql"
}

func isSQLite(tx *gorm.DB) bool {
	return tx != nil && tx.Dialector != nil && strings.Contains(tx.Dialector.Name(), "sqlite")
}

// tenantColumnTablesSQLite supports the in-memory unit-test dialect, which
// has no information_schema.
func tenantColumnTablesSQLite(tx *gorm.DB) ([]string, error) {
	var names []string
	if err := tx.Raw("SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name").Scan(&names).Error; err != nil {
		return nil, err
	}
	var out []string
	for _, name := range names {
		if purgeSweepExcludedTables[name] {
			continue
		}
		var n int64
		if err := tx.Raw("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = 'tenant_id'", name).Scan(&n).Error; err != nil {
			return nil, err
		}
		if n > 0 {
			out = append(out, name)
		}
	}
	return out, nil
}

// quoteIdent quotes a table identifier for the active dialect.
func quoteIdent(tx *gorm.DB, name string) string {
	if isMySQL(tx) {
		return "`" + strings.ReplaceAll(name, "`", "") + "`"
	}
	return `"` + strings.ReplaceAll(name, `"`, "") + `"`
}

// SortedReportTables returns the report table names in stable order (for
// tests and rendering).
func (r *PurgeReport) SortedReportTables() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.Tables))
	for t := range r.Tables {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
