package migrationimport_test

import (
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/migrationimport"
)

// TestMappingPresetColumnBounds is the round117 regression for the R116 audit
// P2 item: mapping presets must reject column indices outside the supported
// range instead of storing arbitrary values.
func TestMappingPresetColumnBounds(t *testing.T) {
	db := openTestDB(t)
	migrateRound115(t, db)
	svc := newSvc115(db)
	c := testCtx(1)

	base := migrationimport.MappingPresetBody{
		Kind:    migrationimport.KindInventory,
		Name:    "越界方案",
		Columns: []string{"SKU", "仓库", "数量"},
	}

	cases := []map[string]int{
		{"skuCode": -1, "quantity": 2},
		{"skuCode": migrationimport.MaxMappingColumns, "quantity": 2},
		{"skuCode": 1 << 30, "quantity": 2},
	}
	for _, mapping := range cases {
		body := base
		body.Mapping = mapping
		if _, err := svc.SaveMappingPreset(c, body, nil); err == nil {
			t.Fatalf("mapping %v: expected out-of-range rejection", mapping)
		}
	}

	// In-range boundary values still save.
	body := base
	body.Mapping = map[string]int{"skuCode": 0, "quantity": migrationimport.MaxMappingColumns - 1}
	if _, err := svc.SaveMappingPreset(c, body, nil); err != nil {
		t.Fatalf("boundary mapping rejected: %v", err)
	}
}
