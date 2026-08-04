package product

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/csvsafe"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// MaxListingExportProducts caps how many drafts one merged listing CSV can cover.
const MaxListingExportProducts = 50

// listingCSVHeader is the manual listing checklist header: everything an
// operator needs to re-enter a draft on Temu/Shopee by hand.
var listingCSVHeader = []string{
	"商品ID", "标题", "副标题(AI标题)", "描述", "类目", "价格", "币种", "主图URL", "规格列表", "来源", "来源链接", "状态",
}

func preferredListingText(primary, fallback string) string {
	if v := strings.TrimSpace(primary); v != "" {
		return v
	}
	return strings.TrimSpace(fallback)
}

func listingImageURL(img *ProductImage) string {
	if img == nil {
		return ""
	}
	if u := strings.TrimSpace(img.PublicURL); u != "" {
		return u
	}
	return strings.TrimSpace(img.OriginURL)
}

func listingMainImageURL(images []ProductImage) string {
	var main *ProductImage
	for i := range images {
		img := &images[i]
		if img.IsBestMain && listingImageURL(img) != "" {
			return listingImageURL(img)
		}
		if img.ImageType != ImageTypeMain || listingImageURL(img) == "" {
			continue
		}
		if main == nil || img.SortOrder < main.SortOrder {
			main = img
		}
	}
	if main != nil {
		return listingImageURL(main)
	}
	for i := range images {
		if u := listingImageURL(&images[i]); u != "" {
			return u
		}
	}
	return ""
}

func listingPriceRange(skus []ProductSKU) string {
	var min, max float64
	found := false
	for _, sku := range skus {
		if sku.Price == nil || *sku.Price <= 0 {
			continue
		}
		if !found {
			min, max = *sku.Price, *sku.Price
			found = true
			continue
		}
		if *sku.Price < min {
			min = *sku.Price
		}
		if *sku.Price > max {
			max = *sku.Price
		}
	}
	if !found {
		return ""
	}
	if min == max {
		return fmt.Sprintf("%.2f", min)
	}
	return fmt.Sprintf("%.2f~%.2f", min, max)
}

func listingSpecList(skus []ProductSKU) string {
	parts := make([]string, 0, len(skus))
	for _, sku := range skus {
		name := strings.TrimSpace(sku.SKUName)
		if name == "" {
			name = strings.TrimSpace(sku.SKUCode)
		}
		if name == "" {
			name = "默认规格"
		}
		p := name
		if sku.Price != nil && *sku.Price > 0 {
			p += fmt.Sprintf(" @%.2f", *sku.Price)
		}
		if sku.Stock != nil {
			p += fmt.Sprintf(" x%d", *sku.Stock)
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, " ; ")
}

// ExportListingListCSV renders one merged manual listing checklist covering
// several drafts (one row per draft) using the same tenant + shop scope as
// the draft list.
func (s *Service) ExportListingListCSV(c *gin.Context, ids []uuid.UUID) ([]byte, string, error) {
	if s == nil || s.DB == nil {
		return nil, "", fmt.Errorf("product: no db")
	}
	tx := s.DB.WithContext(c.Request.Context()).Model(&Product{}).
		Preload("Images").Preload("SKUs").
		Where("products.id IN ?", ids)
	if scoped, _, err := adminperm.ApplyTenantScope(c, tx); err != nil {
		return nil, "", err
	} else {
		tx = scoped
	}
	if scoped, err := adminperm.ApplyProductScope(c, s.DB, tx); err != nil {
		return nil, "", err
	} else {
		tx = scoped
	}
	var rows []Product
	if err := tx.Find(&rows).Error; err != nil {
		return nil, "", err
	}
	if len(rows) != len(ids) {
		return nil, "", gorm.ErrRecordNotFound
	}
	byID := make(map[uuid.UUID]*Product, len(rows))
	for i := range rows {
		byID[rows[i].ID] = &rows[i]
	}

	type categoryRow struct {
		ProductID    uuid.UUID
		CategoryPath string
	}
	var cats []categoryRow
	if err := s.DB.WithContext(c.Request.Context()).
		Model(&ProductPlatformPublishConfig{}).
		Select("product_id", "category_path").
		Where("product_id IN ? AND TRIM(COALESCE(category_path, '')) <> ''", ids).
		Order("last_mapped_at DESC NULLS LAST").
		Find(&cats).Error; err != nil {
		return nil, "", err
	}
	catByID := make(map[uuid.UUID]string, len(cats))
	for _, cr := range cats {
		if _, ok := catByID[cr.ProductID]; !ok {
			catByID[cr.ProductID] = cr.CategoryPath
		}
	}

	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM for Excel
	w := csv.NewWriter(&buf)
	if err := w.Write(listingCSVHeader); err != nil {
		return nil, "", err
	}
	for _, id := range ids {
		p := byID[id]
		if p == nil {
			return nil, "", gorm.ErrRecordNotFound
		}
		row := []string{
			p.ID.String(),
			preferredListingText(p.Title, p.OriginalTitle),
			strings.TrimSpace(p.AITitle),
			preferredListingText(p.Description, p.AIDescription),
			catByID[p.ID],
			listingPriceRange(p.SKUs),
			strings.TrimSpace(p.Currency),
			listingMainImageURL(p.Images),
			listingSpecList(p.SKUs),
			strings.TrimSpace(p.Source),
			strings.TrimSpace(p.SourceURL),
			strings.TrimSpace(p.Status),
		}
		if err := w.Write(csvsafe.Row(row)); err != nil {
			return nil, "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", err
	}
	name := fmt.Sprintf("listing-list-%d.csv", len(ids))
	return buf.Bytes(), name, nil
}

// ExportListingListCSVHandler GET /products/listing-list/export.csv?ids=id1,id2
func (h *Handler) ExportListingListCSVHandler(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "products unavailable")
		return
	}
	raw := strings.Split(c.Query("ids"), ",")
	ids := make([]uuid.UUID, 0, len(raw))
	seen := map[uuid.UUID]bool{}
	for _, r := range raw {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		u, err := uuid.Parse(r)
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid product id")
			return
		}
		if seen[u] {
			continue
		}
		seen[u] = true
		ids = append(ids, u)
	}
	if len(ids) == 0 {
		response.Fail(c, 400, response.CodeBadRequest, "ids required")
		return
	}
	if len(ids) > MaxListingExportProducts {
		response.Fail(c, 400, response.CodeBadRequest, "too many ids")
		return
	}
	data, name, err := h.Svc.ExportListingListCSV(c, ids)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	c.Data(200, "text/csv; charset=utf-8", data)
}
