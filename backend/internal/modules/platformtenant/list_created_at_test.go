package platformtenant_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// 平台租户 0 是隐式租户，列表创建时间取最早平台管理员的创建时间，不再空展示。
func TestList_platformTenantCreatedAtFilled(t *testing.T) {
	t.Parallel()
	db := openTenantTestDB(t)
	actor := seedActor(t, db, 0, "admin")
	r := newTenantRouter(db, actor, 0)

	w := doJSON(r, http.MethodGet, "/api/v1/platform/tenants", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			List []struct {
				ID        int64  `json:"id"`
				CreatedAt string `json:"createdAt"`
			} `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v (body=%s)", err, w.Body.String())
	}
	found := false
	for _, row := range body.Data.List {
		if row.ID == 0 {
			found = true
			if row.CreatedAt == "" {
				t.Fatal("platform tenant createdAt should be filled from earliest platform admin")
			}
		}
	}
	if !found {
		t.Fatal("platform tenant row missing from list")
	}
}
