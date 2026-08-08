package mcpaudit

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service persists and lists MCP tool-call audit logs.
type Service struct {
	DB *gorm.DB

	mu        sync.Mutex
	lastWrite map[string]time.Time
}

// WriteOpts describes one tool call to audit. Mode / ParamsSummary /
// ResultSummary / ConfirmHash are only set for whitelisted write tools.
type WriteOpts struct {
	TenantID      int64
	TokenID       uuid.UUID
	TokenName     string
	TokenMasked   string
	Tool          string
	Status        string
	Mode          string
	ParamsSummary string
	ResultSummary string
	ConfirmHash   string
	// Amount is the money amount of an amount-bearing write tool call
	// (procurement_mark_paid); zero for every other tool.
	Amount     float64
	DurationMs int64
}

// Write appends one audit row. Best effort: the caller treats a failed write
// as non-fatal for the tool call itself, so the error is only returned for
// logging.
func (s *Service) Write(ctx context.Context, opts WriteOpts) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("mcpaudit: no db")
	}
	return s.WriteTx(s.DB.WithContext(ctx), opts)
}

// WriteTx appends one audit row on the given handle, which may be a
// transaction. The MCP write path runs the business mutation and its audit
// row in one transaction so a failed audit write rolls the mutation back
// (fail-closed with no orphan writes).
func (s *Service) WriteTx(db *gorm.DB, opts WriteOpts) error {
	if s == nil || db == nil {
		return fmt.Errorf("mcpaudit: no db")
	}
	status := strings.TrimSpace(opts.Status)
	switch status {
	case StatusSuccess, StatusError, StatusAuthFailed, StatusRateLimited:
	default:
		status = StatusError
	}
	mode := strings.TrimSpace(opts.Mode)
	switch mode {
	case "", ModeDryRun, ModeExecute:
	default:
		mode = ""
	}
	row := ToolCallLog{
		TenantID:      opts.TenantID,
		TokenID:       opts.TokenID,
		TokenName:     truncate(opts.TokenName, 128),
		TokenMasked:   truncate(opts.TokenMasked, 40),
		Tool:          truncate(strings.TrimSpace(opts.Tool), 64),
		Status:        status,
		Mode:          mode,
		ParamsSummary: truncate(opts.ParamsSummary, 512),
		ResultSummary: truncate(opts.ResultSummary, 512),
		ConfirmHash:   truncate(opts.ConfirmHash, 64),
		Amount:        opts.Amount,
		DurationMs:    opts.DurationMs,
	}
	if err := db.Create(&row).Error; err != nil {
		return fmt.Errorf("mcpaudit: write: %w", err)
	}
	return nil
}

// CountExecutesByToken counts successful execute-mode rows for one token
// since a cutoff (per-token hourly write quota). Errors must be treated as
// quota exhausted by callers (fail closed).
func (s *Service) CountExecutesByToken(db *gorm.DB, tokenID uuid.UUID, since time.Time) (int64, error) {
	if s == nil || db == nil {
		return 0, fmt.Errorf("mcpaudit: no db")
	}
	var n int64
	err := db.Model(&ToolCallLog{}).
		Where("token_id = ? AND mode = ? AND status = ? AND created_at >= ?",
			tokenID, ModeExecute, StatusSuccess, since).
		Count(&n).Error
	return n, err
}

// CountExecutesByTenant counts successful execute-mode rows for one tenant
// since a cutoff (tenant daily write quota). Errors must be treated as quota
// exhausted by callers (fail closed).
func (s *Service) CountExecutesByTenant(db *gorm.DB, tenantID int64, since time.Time) (int64, error) {
	if s == nil || db == nil {
		return 0, fmt.Errorf("mcpaudit: no db")
	}
	var n int64
	err := db.Model(&ToolCallLog{}).
		Where("tenant_id = ? AND mode = ? AND status = ? AND created_at >= ?",
			tenantID, ModeExecute, StatusSuccess, since).
		Count(&n).Error
	return n, err
}

// SumExecuteAmountByTenantTool sums the Amount column of successful
// execute-mode rows of one tool for one tenant since a cutoff (tenant daily
// amount ceiling for procurement_mark_paid). Errors must be treated as
// ceiling exhausted by callers (fail closed).
func (s *Service) SumExecuteAmountByTenantTool(db *gorm.DB, tenantID int64, tool string, since time.Time) (float64, error) {
	if s == nil || db == nil {
		return 0, fmt.Errorf("mcpaudit: no db")
	}
	var total float64
	err := db.Model(&ToolCallLog{}).
		Where("tenant_id = ? AND tool = ? AND mode = ? AND status = ? AND created_at >= ?",
			tenantID, tool, ModeExecute, StatusSuccess, since).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error
	return total, err
}

// throttleInterval bounds WriteThrottled to one row per key per interval, so
// per-request events (rejected credentials, rate-limited calls) cannot grow
// the audit table at the caller's request rate.
const throttleInterval = time.Minute

// WriteThrottled appends one audit row unless the same key already produced a
// row within the last minute. The throttle is in-process (per replica), which
// bounds table growth while keeping at least one visible row per source and
// minute. Used for auth-failure and rate-limit events where identity may be
// unknown and volume is attacker-controlled.
func (s *Service) WriteThrottled(ctx context.Context, key string, opts WriteOpts) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("mcpaudit: no db")
	}
	full := opts.Tool + "|" + opts.Status + "|" + key
	now := time.Now()
	s.mu.Lock()
	if s.lastWrite == nil {
		s.lastWrite = make(map[string]time.Time)
	}
	if last, ok := s.lastWrite[full]; ok && now.Sub(last) < throttleInterval {
		s.mu.Unlock()
		return nil
	}
	s.lastWrite[full] = now
	if len(s.lastWrite) > 4096 {
		for k, v := range s.lastWrite {
			if now.Sub(v) >= throttleInterval {
				delete(s.lastWrite, k)
			}
		}
	}
	s.mu.Unlock()
	return s.Write(ctx, opts)
}

// ListFilter narrows List within the tenant scope.
type ListFilter struct {
	Tool     string
	Status   string
	Mode     string
	Page     int
	PageSize int
}

// ListResult carries one page of audit rows.
type ListResult struct {
	Total int64
	Items []ToolCallLog
}

const maxPageSize = 100

// List returns the tenant's audit rows, newest first.
func (s *Service) List(ctx context.Context, tenantID int64, f ListFilter) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("mcpaudit: no db")
	}
	page := f.Page
	if page < 1 {
		page = 1
	}
	ps := f.PageSize
	if ps < 1 {
		ps = 20
	}
	if ps > maxPageSize {
		ps = maxPageSize
	}
	tx := s.DB.WithContext(ctx).Model(&ToolCallLog{}).Where("tenant_id = ?", tenantID)
	if v := strings.TrimSpace(f.Tool); v != "" {
		tx = tx.Where("tool = ?", v)
	}
	if v := strings.TrimSpace(f.Status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	if v := strings.TrimSpace(f.Mode); v != "" {
		tx = tx.Where("mode = ?", v)
	}
	var res ListResult
	if err := tx.Count(&res.Total).Error; err != nil {
		return nil, fmt.Errorf("mcpaudit: count: %w", err)
	}
	if err := tx.Order("created_at DESC").Offset((page - 1) * ps).Limit(ps).Find(&res.Items).Error; err != nil {
		return nil, fmt.Errorf("mcpaudit: list: %w", err)
	}
	return &res, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
