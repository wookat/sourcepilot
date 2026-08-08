package orderexception

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// sourceScope is the trusted tenant/store binding of one exception source row.
type sourceScope struct {
	TenantID int64
	ShopID   *uuid.UUID
}

// resolveSourceScope resolves the tenant and store a workbench source row
// belongs to. Unknown / unparsable sources resolve to gorm.ErrRecordNotFound so
// callers answer 404 without leaking existence.
func (s *Service) resolveSourceScope(ctx context.Context, sourceType, sourceID string) (*sourceScope, error) {
	if s == nil || s.DB == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.resolveSourceScopeDB(ctx, s.DB, sourceType, sourceID)
}

// resolveSourceScopeDB is resolveSourceScope against an explicit *gorm.DB
// (the MCP write pipeline passes its own transaction).
func (s *Service) resolveSourceScopeDB(ctx context.Context, conn *gorm.DB, sourceType, sourceID string) (*sourceScope, error) {
	if conn == nil {
		return nil, gorm.ErrRecordNotFound
	}
	sid, err := uuid.Parse(strings.TrimSpace(sourceID))
	if err != nil {
		return nil, gorm.ErrRecordNotFound
	}
	db := conn.WithContext(ctx)
	orderScope := func(orderID uuid.UUID) (*sourceScope, error) {
		var o order.Order
		if err := db.Select("id", "tenant_id", "shop_id").
			First(&o, "id = ? AND deleted_at IS NULL", orderID).Error; err != nil {
			return nil, err
		}
		return &sourceScope{TenantID: o.TenantID, ShopID: o.ShopID}, nil
	}

	switch strings.TrimSpace(sourceType) {
	case SourceOrder:
		return orderScope(sid)
	case SourceOrderItem:
		var oi order.OrderItem
		if err := db.Select("id", "order_id").First(&oi, "id = ?", sid).Error; err != nil {
			return nil, err
		}
		return orderScope(oi.OrderID)
	case SourceOrderItemSKUMatch:
		var m order.OrderItemSKUMatch
		if err := db.Select("id", "order_id").First(&m, "id = ?", sid).Error; err != nil {
			return nil, err
		}
		return orderScope(m.OrderID)
	case SourceOrderInventoryEffect:
		var e inventory.OrderInventoryEffect
		if err := db.Select("id", "order_id").First(&e, "id = ?", sid).Error; err != nil {
			return nil, err
		}
		return orderScope(e.OrderID)
	case SourceInventorySyncTask:
		var t inventory.InventorySyncTask
		if err := db.Select("id", "tenant_id", "shop_id").First(&t, "id = ?", sid).Error; err != nil {
			return nil, err
		}
		shopID := t.ShopID
		return &sourceScope{TenantID: t.TenantID, ShopID: &shopID}, nil
	case SourceOrderSyncTask:
		var t ordersync.OrderSyncTask
		if err := db.Select("id", "tenant_id", "shop_id").First(&t, "id = ?", sid).Error; err != nil {
			return nil, err
		}
		shopID := t.ShopID
		return &sourceScope{TenantID: t.TenantID, ShopID: &shopID}, nil
	default:
		return nil, gorm.ErrRecordNotFound
	}
}

// EnsureSourceOperable gates every mutating workbench route on the source row's
// own tenant and store: another tenant's row or a store outside the caller's
// grants resolves to gorm.ErrRecordNotFound (404, no existence leak), and a
// store the caller may only view resolves to adminperm.ErrStoreNotOperable
// (403 / 40303).
func (s *Service) EnsureSourceOperable(c *gin.Context, sourceType, sourceID string) error {
	if s == nil || s.DB == nil {
		return gorm.ErrRecordNotFound
	}
	scope, err := s.resolveSourceScope(c.Request.Context(), sourceType, sourceID)
	if err != nil {
		return err
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	if scope.TenantID != tid {
		return gorm.ErrRecordNotFound
	}
	return adminperm.EnsureStoreOperable(c, s.DB, scope.ShopID)
}
