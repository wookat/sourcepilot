package orderexception

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// MCP write adapter for exception marks (R180 W2 白名单动作 异常标记). Methods
// are tenant-scoped by explicit tenant id (MCP tokens carry no admin
// principal / store grants), reuse the same idempotent upsert/delete
// semantics as the workbench routes, and accept an external *gorm.DB so the
// caller can run the mutation and its audit row in one transaction.
// Cross-tenant / missing / unparsable sources surface as
// ErrSourceNotFoundInTenant (404 semantics, no existence oracle).

// ErrSourceNotFoundInTenant is returned when an exception source row does not
// resolve inside the tenant (missing or cross-tenant — indistinguishable on
// purpose).
var ErrSourceNotFoundInTenant = errors.New("记录不存在")

// EnsureSourceInTenant verifies one exception source row belongs to tenantID.
func (s *Service) EnsureSourceInTenant(ctx context.Context, db *gorm.DB, tenantID int64, sourceType, sourceID string) error {
	if s == nil || db == nil {
		return errors.New("orderexception: no db")
	}
	scope, err := s.resolveSourceScopeDB(ctx, db, sourceType, sourceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSourceNotFoundInTenant
		}
		return err
	}
	if scope.TenantID != tenantID {
		return ErrSourceNotFoundInTenant
	}
	return nil
}

// MarkStateInTenant reads the current mark of one source (for write
// previews): markType is "" (open) / handled / ignored.
func (s *Service) MarkStateInTenant(ctx context.Context, db *gorm.DB, tenantID int64, sourceType, sourceID string) (string, error) {
	if err := s.EnsureSourceInTenant(ctx, db, tenantID, sourceType, sourceID); err != nil {
		return "", err
	}
	var row OrderExceptionMark
	err := db.WithContext(ctx).
		Where("source_type = ? AND source_id = ?", strings.TrimSpace(sourceType), strings.TrimSpace(sourceID)).
		Order("updated_at DESC").
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return row.MarkType, nil
}

// UpsertMarkInTenant marks one exception source handled / ignored inside the
// caller's transaction (idempotent upsert; the opposite mark is removed).
func (s *Service) UpsertMarkInTenant(ctx context.Context, db *gorm.DB, tenantID int64, exceptionType, sourceType, sourceID, markType, remark string) error {
	if err := s.EnsureSourceInTenant(ctx, db, tenantID, sourceType, sourceID); err != nil {
		return err
	}
	oid, oiid, err := s.resolveOrderPointersDB(ctx, db, sourceType, sourceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSourceNotFoundInTenant
		}
		return err
	}
	now := time.Now().UTC()
	row := OrderExceptionMark{
		ExceptionType: strings.TrimSpace(exceptionType),
		SourceType:    strings.TrimSpace(sourceType),
		SourceID:      strings.TrimSpace(sourceID),
		MarkType:      strings.TrimSpace(markType),
		OrderID:       oid,
		OrderItemID:   oiid,
		Remark:        strings.TrimSpace(remark),
	}
	row.UpdatedAt = now
	row.CreatedAt = now

	opposite := MarkIgnored
	if row.MarkType == MarkIgnored {
		opposite = MarkHandled
	}
	if err := db.WithContext(ctx).
		Where("exception_type = ? AND source_type = ? AND source_id = ? AND mark_type = ?", row.ExceptionType, row.SourceType, row.SourceID, opposite).
		Delete(&OrderExceptionMark{}).Error; err != nil {
		return err
	}

	var existing OrderExceptionMark
	err = db.WithContext(ctx).
		Where("exception_type = ? AND source_type = ? AND source_id = ? AND mark_type = ?", row.ExceptionType, row.SourceType, row.SourceID, row.MarkType).
		First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.WithContext(ctx).Create(&row).Error
	}
	if err != nil {
		return err
	}
	return db.WithContext(ctx).Model(&existing).Updates(map[string]any{
		"remark":        row.Remark,
		"updated_at":    now,
		"created_by":    nil,
		"order_id":      oid,
		"order_item_id": oiid,
	}).Error
}

// DeleteMarksInTenant removes every mark of one source inside the caller's
// transaction (idempotent; reports how many rows were removed).
func (s *Service) DeleteMarksInTenant(ctx context.Context, db *gorm.DB, tenantID int64, sourceType, sourceID string) (int64, error) {
	if err := s.EnsureSourceInTenant(ctx, db, tenantID, sourceType, sourceID); err != nil {
		return 0, err
	}
	res := db.WithContext(ctx).
		Where("source_type = ? AND source_id = ?", strings.TrimSpace(sourceType), strings.TrimSpace(sourceID)).
		Delete(&OrderExceptionMark{})
	return res.RowsAffected, res.Error
}
