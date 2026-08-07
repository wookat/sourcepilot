package adminperm

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrStoreNotOperable marks a shop the caller can view but not operate (403).
var ErrStoreNotOperable = errors.New("店铺无操作权限")

// EnsureStoreOperable verifies one store-scoped row may be written by the
// caller: an invisible store resolves to gorm.ErrRecordNotFound (404, no
// existence leak) and a view-only store to ErrStoreNotOperable (403). A nil
// shop id (tenant-level row) passes.
func EnsureStoreOperable(c *gin.Context, db *gorm.DB, shopID *uuid.UUID) error {
	if shopID == nil || *shopID == uuid.Nil {
		return nil
	}
	p, err := LoadPrincipal(c, db)
	if err != nil {
		return err
	}
	if p.IsAdmin() {
		return nil
	}
	if !p.CanViewStore(*shopID) {
		return gorm.ErrRecordNotFound
	}
	if !p.CanOperateStore(*shopID) {
		return ErrStoreNotOperable
	}
	return nil
}

// ApplyStoreOperateScope restricts a write query to the stores the caller may
// operate (grant scope operate/manage); view-only grants are excluded.
func ApplyStoreOperateScope(c *gin.Context, db *gorm.DB, tx *gorm.DB, column string) (*gorm.DB, error) {
	if tx == nil {
		return tx, nil
	}
	p, err := LoadPrincipal(c, db)
	if err != nil {
		return nil, err
	}
	if p.IsAdmin() {
		return tx, nil
	}
	col := strings.TrimSpace(column)
	if col == "" {
		col = "shop_id"
	}
	ids := p.OperableStoreIDs()
	if len(ids) == 0 {
		return tx.Where("1 = 0"), nil
	}
	return tx.Where(col+" IN ?", ids), nil
}

// EnsureStoresOperable verifies every shop id in raw exists in the caller's
// tenant and is operable by the caller. Cross-tenant, deleted or invisible
// shops resolve to gorm.ErrRecordNotFound (404, no existence leak); a shop
// the caller can only view resolves to ErrStoreNotOperable (403).
func EnsureStoresOperable(c *gin.Context, db *gorm.DB, raw []string) error {
	seen := map[uuid.UUID]bool{}
	ids := make([]uuid.UUID, 0, len(raw))
	for _, v := range raw {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		id, err := uuid.Parse(v)
		if err != nil || id == uuid.Nil {
			return gorm.ErrRecordNotFound
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	tid, err := TenantIDFromGin(c)
	if err != nil {
		return err
	}
	var count int64
	if err := db.WithContext(c.Request.Context()).Table("shops").
		Where("tenant_id = ? AND id IN ? AND deleted_at IS NULL", tid, ids).
		Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(ids)) {
		return gorm.ErrRecordNotFound
	}
	p, err := LoadPrincipal(c, db)
	if err != nil {
		return err
	}
	if p.IsAdmin() {
		return nil
	}
	for _, id := range ids {
		if !p.CanViewStore(id) {
			return gorm.ErrRecordNotFound
		}
		if !p.CanOperateStore(id) {
			return ErrStoreNotOperable
		}
	}
	return nil
}
