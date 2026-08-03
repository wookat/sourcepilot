package operationdashboard

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// Scope carries RBAC store filters for dashboard aggregation.
type Scope struct {
	AllowedShopIDs []uuid.UUID // nil = admin (all stores)
	IsAdmin        bool
	TenantID       *int64 // nil = no tenant restriction (legacy paths)
}

func scopeFromContext(c *gin.Context, db *gorm.DB) Scope {
	if c == nil {
		return Scope{IsAdmin: true}
	}
	var tenantID *int64
	if tid, err := adminperm.TenantIDFromGin(c); err == nil {
		tenantID = &tid
	}
	p, _ := adminperm.LoadPrincipal(c, db)
	if p == nil || p.IsAdmin() {
		return Scope{IsAdmin: true, TenantID: tenantID}
	}
	return Scope{AllowedShopIDs: p.AllowedStoreIDs(), TenantID: tenantID}
}

func (sc Scope) applyShopColumn(tx *gorm.DB, column string) *gorm.DB {
	if tx == nil || sc.IsAdmin {
		return tx
	}
	if len(sc.AllowedShopIDs) == 0 {
		return tx.Where("1 = 0")
	}
	col := column
	if col == "" {
		col = "shop_id"
	}
	return tx.Where(col+" IN ?", sc.AllowedShopIDs)
}

func (sc Scope) applyProductScope(tx *gorm.DB) *gorm.DB {
	if tx == nil || sc.IsAdmin {
		return tx
	}
	if len(sc.AllowedShopIDs) == 0 {
		return tx.Where("1 = 0")
	}
	return tx.Where(`products.id IN (
		SELECT DISTINCT product_id FROM product_platform_publish_configs WHERE shop_id IN ?
		UNION
		SELECT DISTINCT product_id FROM product_publications WHERE shop_id IN ? AND deleted_at IS NULL
	)`, sc.AllowedShopIDs, sc.AllowedShopIDs)
}

// applyTenantColumn restricts rows to the current tenant via a tenant_id column.
func (sc Scope) applyTenantColumn(tx *gorm.DB, column string) *gorm.DB {
	if tx == nil || sc.TenantID == nil {
		return tx
	}
	col := column
	if col == "" {
		col = "tenant_id"
	}
	return tx.Where(col+" = ?", *sc.TenantID)
}

// applyTenantViaProduct restricts rows whose tenant derives from a product FK
// (same convention as taskcenter: rows without product linkage stay visible).
func (sc Scope) applyTenantViaProduct(tx *gorm.DB, productIDColumn string) *gorm.DB {
	if tx == nil || sc.TenantID == nil {
		return tx
	}
	return tx.Where("("+productIDColumn+" IS NULL OR "+productIDColumn+" IN (SELECT id FROM products WHERE tenant_id = ?))", *sc.TenantID)
}

// applyTenantViaShop restricts rows whose tenant derives from a shop FK.
func (sc Scope) applyTenantViaShop(tx *gorm.DB, shopIDColumn string) *gorm.DB {
	if tx == nil || sc.TenantID == nil {
		return tx
	}
	return tx.Where(shopIDColumn+" IN (SELECT id FROM shops WHERE tenant_id = ?)", *sc.TenantID)
}

// tenantValue returns the trusted tenant id or 0 when context is missing.
func (sc Scope) tenantValue() int64 {
	if sc.TenantID == nil {
		return 0
	}
	return *sc.TenantID
}

func (sc Scope) shopIDStrings() []string {
	if len(sc.AllowedShopIDs) == 0 {
		return nil
	}
	out := make([]string, 0, len(sc.AllowedShopIDs))
	for _, id := range sc.AllowedShopIDs {
		out = append(out, id.String())
	}
	return out
}
