package migrationimport

import (
	"encoding/csv"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/trademind-ai/trademind/backend/internal/pkg/csvsafe"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// templateSample returns one example data row per import kind (values follow
// the canonical field order of FieldsForKind).
func templateSample(kind string) []string {
	switch kind {
	case KindOrder:
		return []string{"SO-20260101-0001", "PLT-88001", "王小明", "13800000000", "buyer@example.com",
			"中国", "广东省", "深圳市", "南山区科技园路 1 号", "518000",
			"无线蓝牙耳机 X100", "SKU-X100-BLK", "黑色", "2", "89.00", "178.00", "CNY", "已发货",
			"2026-01-01 10:30:00", "2026-01-01 10:35:00", "SF1234567890"}
	case KindInventory:
		return []string{"SKU-X100-BLK", "WH-MAIN", "120", "45.00"}
	case KindSource:
		return []string{"深圳华强北电子", "SKU-X100-BLK", "https://detail.1688.com/offer/123456789.html", "38.50", "3216549870"}
	default:
		return []string{"无线蓝牙耳机 X100", "SKU-X100-BLK", "黑色", "89.00", "45.00", "120", "CNY",
			"https://example.com/img/x100.jpg", "高续航无线蓝牙耳机", "https://detail.1688.com/offer/123456789.html"}
	}
}

// TemplateCSV GET /imports/templates/:kind — standard import template
// (header = canonical field labels + one example row). The CSV opens in
// Excel directly, so one template serves both CSV and Excel flows.
func (h *Handler) TemplateCSV(c *gin.Context) {
	kind, err := normalizeKind(c.Param("kind"))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	fields := FieldsForKind(kind)
	header := make([]string, 0, len(fields))
	for _, f := range fields {
		header = append(header, f.Label)
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=trademind-import-template-%s.csv", kind))
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM for Excel
	w := csv.NewWriter(c.Writer)
	_ = w.Write(csvsafe.Row(header))
	_ = w.Write(csvsafe.Row(templateSample(kind)))
	w.Flush()
}
