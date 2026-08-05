package migrationimport

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

// commitChunkSize groups inventory-opening rows into one transaction per
// chunk: one commit fsync per 500 rows instead of one per row.
const commitChunkSize = 500

// sqlInChunkSize bounds the number of values bound into one IN (...) query.
const sqlInChunkSize = 1000

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func chunkStrings(values []string, size int) [][]string {
	if size < 1 {
		size = 1
	}
	var out [][]string
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		out = append(out, values[start:end])
	}
	return out
}

// findTenantSKUsBulk resolves many SKU codes in a handful of IN queries
// (instead of one query per row). Codes mapping to more than one SKU are
// reported in ambiguous; codes absent from both maps do not exist.
func (s *Service) findTenantSKUsBulk(c *gin.Context, tenantID int64, codes []string) (map[string]*product.ProductSKU, map[string]bool, error) {
	found := make(map[string]*product.ProductSKU, len(codes))
	ambiguous := map[string]bool{}
	for _, chunk := range chunkStrings(uniqueNonEmpty(codes), sqlInChunkSize) {
		var skus []product.ProductSKU
		if err := s.DB.WithContext(c.Request.Context()).
			Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
			Where("products.tenant_id = ? AND product_skus.sku_code IN ?", tenantID, chunk).
			Find(&skus).Error; err != nil {
			return nil, nil, err
		}
		for i := range skus {
			code := skus[i].SKUCode
			if _, dup := found[code]; dup || ambiguous[code] {
				ambiguous[code] = true
				delete(found, code)
				continue
			}
			sku := skus[i]
			found[code] = &sku
		}
	}
	return found, ambiguous, nil
}

// resolveBulkSKU translates the bulk lookup result for one code into the
// row-level error message used by the per-row commit paths.
func resolveBulkSKU(found map[string]*product.ProductSKU, ambiguous map[string]bool, code string) (*product.ProductSKU, error) {
	if ambiguous[code] {
		return nil, fmt.Errorf("SKU「%s」存在多条记录，无法确定目标", code)
	}
	sku, ok := found[code]
	if !ok {
		return nil, fmt.Errorf("SKU「%s」不存在", code)
	}
	return sku, nil
}

// existingSKUCodes reports which of the given codes already exist for the
// tenant (duplicate detection for the product import in bulk).
func (s *Service) existingSKUCodes(c *gin.Context, tenantID int64, codes []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, chunk := range chunkStrings(uniqueNonEmpty(codes), sqlInChunkSize) {
		var hit []string
		if err := s.DB.WithContext(c.Request.Context()).
			Table("product_skus").
			Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
			Where("products.tenant_id = ? AND product_skus.sku_code IN ?", tenantID, chunk).
			Pluck("product_skus.sku_code", &hit).Error; err != nil {
			return nil, err
		}
		for _, code := range hit {
			out[code] = true
		}
	}
	return out, nil
}

// existingOrderNos reports which of the given order numbers already exist for
// the tenant (duplicate detection for the order import in bulk).
func (s *Service) existingOrderNos(c *gin.Context, tenantID int64, nos []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, chunk := range chunkStrings(uniqueNonEmpty(nos), sqlInChunkSize) {
		var hit []string
		if err := s.DB.WithContext(c.Request.Context()).
			Table("orders").
			Where("tenant_id = ? AND order_no IN ? AND deleted_at IS NULL", tenantID, chunk).
			Pluck("order_no", &hit).Error; err != nil {
			return nil, err
		}
		for _, no := range hit {
			out[no] = true
		}
	}
	return out, nil
}

// chunkTx runs fn once per chunk of n rows inside one transaction each.
func chunkTx(db *gorm.DB, total, size int, fn func(tx *gorm.DB, start, end int) error) error {
	if size < 1 {
		size = 1
	}
	for start := 0; start < total; start += size {
		end := start + size
		if end > total {
			end = total
		}
		if err := db.Transaction(func(tx *gorm.DB) error { return fn(tx, start, end) }); err != nil {
			return err
		}
	}
	return nil
}
