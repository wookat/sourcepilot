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
