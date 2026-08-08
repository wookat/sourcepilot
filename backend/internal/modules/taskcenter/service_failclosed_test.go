package taskcenter

import (
	"testing"

	"github.com/google/uuid"
)

// R189: ListFailureParams.AllowedShopIDs distinguishes nil (admin, every store)
// from an empty set (the caller holds no store grant). The empty set must hide
// every row instead of skipping the store filter altogether.
func TestUnifiedFiltersEmptyStoreScopeHidesEverything(t *testing.T) {
	shop := uuid.New()
	row := UnifiedTaskDTO{ShopID: shop.String(), Platform: "douyin", NormalizedStatus: NormFailed, Status: "failed"}

	if passesUnifiedFilters(row, ListFailureParams{AllowedShopIDs: []uuid.UUID{}}) {
		t.Fatal("empty store scope must hide store-scoped failures")
	}
	if !passesUnifiedFilters(row, ListFailureParams{AllowedShopIDs: []uuid.UUID{shop}}) {
		t.Fatal("granted store must stay visible")
	}
	if passesUnifiedFilters(row, ListFailureParams{AllowedShopIDs: []uuid.UUID{uuid.New()}}) {
		t.Fatal("other stores must stay hidden")
	}
	if !passesUnifiedFilters(row, ListFailureParams{}) {
		t.Fatal("nil store scope (admin) must stay unrestricted")
	}
}
