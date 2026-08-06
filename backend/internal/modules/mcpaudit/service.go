package mcpaudit

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service persists and lists MCP tool-call audit logs.
type Service struct {
	DB *gorm.DB
}

// WriteOpts describes one tool call to audit.
type WriteOpts struct {
	TenantID    int64
	TokenID     uuid.UUID
	TokenName   string
	TokenMasked string
	Tool        string
	Status      string
	DurationMs  int64
}

// Write appends one audit row. Best effort: the caller treats a failed write
// as non-fatal for the tool call itself, so the error is only returned for
// logging.
func (s *Service) Write(ctx context.Context, opts WriteOpts) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("mcpaudit: no db")
	}
	status := strings.TrimSpace(opts.Status)
	if status != StatusSuccess && status != StatusError {
		status = StatusError
	}
	row := ToolCallLog{
		TenantID:    opts.TenantID,
		TokenID:     opts.TokenID,
		TokenName:   truncate(opts.TokenName, 128),
		TokenMasked: truncate(opts.TokenMasked, 40),
		Tool:        truncate(strings.TrimSpace(opts.Tool), 64),
		Status:      status,
		DurationMs:  opts.DurationMs,
	}
	if err := s.DB.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("mcpaudit: write: %w", err)
	}
	return nil
}

// ListFilter narrows List within the tenant scope.
type ListFilter struct {
	Tool     string
	Status   string
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
