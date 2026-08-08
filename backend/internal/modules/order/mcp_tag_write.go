package order

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MCP write adapter for order tags (R179 W1 首个白名单写动作). These methods are
// tenant-scoped by explicit tenant id (MCP tokens carry no admin principal /
// store grants), reuse the same idempotent link semantics as the admin API,
// and accept an external *gorm.DB so the caller can run the mutation and its
// audit row in one transaction. Cross-tenant / missing targets surface as
// ErrOrderNotFoundInTenant / ErrOrderTagNotFound (404 semantics, no
// existence oracle).

// ErrOrderNotFoundInTenant is returned when an order no does not resolve
// inside the tenant (missing or cross-tenant — indistinguishable on purpose).
var ErrOrderNotFoundInTenant = errors.New("订单不存在")

// FindOrderByNoInTenant resolves one order by its business order no within
// the tenant.
func (s *Service) FindOrderByNoInTenant(ctx context.Context, db *gorm.DB, tenantID int64, orderNo string) (*Order, error) {
	if s == nil || db == nil {
		return nil, fmt.Errorf("order: no db")
	}
	no := strings.TrimSpace(orderNo)
	if no == "" {
		return nil, fmt.Errorf("orderNo 不能为空")
	}
	var row Order
	if err := db.WithContext(ctx).
		First(&row, "tenant_id = ? AND order_no = ?", tenantID, no).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrderNotFoundInTenant
		}
		return nil, err
	}
	return &row, nil
}

// FindOrderTagByNameInTenant resolves one tag by name within the tenant.
func (s *Service) FindOrderTagByNameInTenant(ctx context.Context, db *gorm.DB, tenantID int64, name string) (*OrderTag, error) {
	if s == nil || db == nil {
		return nil, fmt.Errorf("order: no db")
	}
	n := strings.TrimSpace(name)
	if n == "" {
		return nil, fmt.Errorf("tagName 不能为空")
	}
	var row OrderTag
	if err := db.WithContext(ctx).
		First(&row, "tenant_id = ? AND name = ?", tenantID, n).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrderTagNotFound
		}
		return nil, err
	}
	return &row, nil
}

// AttachOrderTagInTenant links one tag to one order (idempotent: an existing
// link is skipped via ON CONFLICT DO NOTHING). Returns applied=1 when a new
// link was written, 0 when the order already carried the tag.
func (s *Service) AttachOrderTagInTenant(ctx context.Context, db *gorm.DB, tenantID int64, order *Order, tag *OrderTag) (int64, error) {
	if s == nil || db == nil {
		return 0, fmt.Errorf("order: no db")
	}
	if order == nil || tag == nil || order.TenantID != tenantID || tag.TenantID != tenantID {
		return 0, ErrOrderNotFoundInTenant
	}
	link := OrderTagLink{TenantID: tenantID, OrderID: order.ID, TagID: tag.ID, Source: TagLinkSourceManual}
	res := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "order_id"}, {Name: "tag_id"}},
		DoNothing: true,
	}).Create(&link)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// DetachOrderTagInTenant removes one tag link from one order (idempotent:
// removing an absent link is a no-op). Returns removed=1 when a link was
// deleted, 0 when the order did not carry the tag.
func (s *Service) DetachOrderTagInTenant(ctx context.Context, db *gorm.DB, tenantID int64, order *Order, tag *OrderTag) (int64, error) {
	if s == nil || db == nil {
		return 0, fmt.Errorf("order: no db")
	}
	if order == nil || tag == nil || order.TenantID != tenantID || tag.TenantID != tenantID {
		return 0, ErrOrderNotFoundInTenant
	}
	res := db.WithContext(ctx).
		Where("tenant_id = ? AND order_id = ? AND tag_id = ?", tenantID, order.ID, tag.ID).
		Delete(&OrderTagLink{})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// OrderTagNamesInTenant returns the order's current tag names (for write
// previews / results).
func (s *Service) OrderTagNamesInTenant(ctx context.Context, db *gorm.DB, tenantID int64, orderID uuid.UUID) ([]string, error) {
	if s == nil || db == nil {
		return nil, fmt.Errorf("order: no db")
	}
	var names []string
	if err := db.WithContext(ctx).Raw(`
		SELECT t.name
		FROM order_tag_links l
		JOIN order_tags t ON t.id = l.tag_id
		WHERE l.tenant_id = ? AND l.order_id = ?
		ORDER BY t.name ASC
	`, tenantID, orderID).Scan(&names).Error; err != nil {
		return nil, err
	}
	if names == nil {
		names = []string{}
	}
	return names, nil
}
