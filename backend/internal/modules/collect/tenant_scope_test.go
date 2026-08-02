package collect

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Cross-tenant reads must return 404 (no existence leak); same-tenant reads
// keep working, matching the order/selection scope semantics.
func TestCollectReadsAreTenantScoped(t *testing.T) {
	db := newCollectGuardDB(t)
	uid := seedCollectAdmin(t, db, "admin")

	mine := CollectTask{TenantID: 1, Source: "1688", SourceURL: "https://detail.1688.com/offer/1.html", Status: StatusFailed}
	other := CollectTask{TenantID: 2, Source: "1688", SourceURL: "https://detail.1688.com/offer/2.html", Status: StatusFailed}
	if err := db.Create(&mine).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	myBatch := CollectBatch{TenantID: 1, Source: "1688", Status: BatchStatusRunning}
	otherBatch := CollectBatch{TenantID: 2, Source: "1688", Status: BatchStatusRunning}
	if err := db.Create(&myBatch).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&otherBatch).Error; err != nil {
		t.Fatal(err)
	}

	r := newCollectGuardRouter(db, uid, 1)

	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}

	// same-tenant reads succeed
	for _, path := range []string{
		"/api/v1/collect/tasks/" + mine.ID.String(),
		"/api/v1/collect/tasks/" + mine.ID.String() + "/events",
		"/api/v1/collect/batches/" + myBatch.ID.String(),
		"/api/v1/collect/batches/" + myBatch.ID.String() + "/tasks",
	} {
		if w := get(path); w.Code != http.StatusOK {
			t.Fatalf("GET %s (same tenant): got %d, want 200", path, w.Code)
		}
	}

	// cross-tenant reads return 404 without leaking existence
	for _, path := range []string{
		"/api/v1/collect/tasks/" + other.ID.String(),
		"/api/v1/collect/tasks/" + other.ID.String() + "/events",
		"/api/v1/collect/batches/" + otherBatch.ID.String(),
		"/api/v1/collect/batches/" + otherBatch.ID.String() + "/tasks",
	} {
		if w := get(path); w.Code != http.StatusNotFound {
			t.Fatalf("GET %s (cross tenant): got %d, want 404", path, w.Code)
		}
	}

	// list only returns current tenant's rows
	w := get("/api/v1/collect/tasks")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /collect/tasks: got %d, want 200", w.Code)
	}
	var body struct {
		Data struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	for _, it := range body.Data.Items {
		if it.ID == other.ID.String() {
			t.Fatalf("list leaked cross-tenant task %s", it.ID)
		}
	}
	found := false
	for _, it := range body.Data.Items {
		if it.ID == mine.ID.String() {
			found = true
		}
	}
	if !found {
		t.Fatalf("list missing same-tenant task %s", mine.ID)
	}

	// cross-tenant manual retry must also be a 404, not a mutation
	w2 := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/collect/tasks/"+other.ID.String()+"/retry", nil)
	r.ServeHTTP(w2, req)
	if w2.Code == http.StatusOK {
		t.Fatalf("POST retry (cross tenant): got 200, want failure")
	}
	var after CollectTask
	if err := db.First(&after, "id = ?", other.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.Status != StatusFailed {
		t.Fatalf("cross-tenant retry mutated task status to %s", after.Status)
	}
}
