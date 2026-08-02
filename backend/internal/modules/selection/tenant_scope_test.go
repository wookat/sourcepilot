package selection

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func seedTask(t *testing.T, db *gorm.DB, tenantID int64) *SelectionTask {
	t.Helper()
	task := &SelectionTask{
		TenantID:       tenantID,
		Name:           "t",
		TargetPlatform: "tiktok",
		TargetCountry:  "US",
		Status:         StatusSuccess,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	cand := &SelectionCandidate{
		TenantID: tenantID,
		TaskID:   task.ID,
		Title:    "cand",
		Status:   CandidateScored,
	}
	if err := db.Create(cand).Error; err != nil {
		t.Fatal(err)
	}
	return task
}

// selection reads must only expose tasks belonging to the caller's tenant.
func TestSelectionReadsAreTenantScoped(t *testing.T) {
	db := newGuardDB(t)
	uid := seedAdmin(t, db, "admin")

	mine := seedTask(t, db, 1)
	other := seedTask(t, db, 2)

	r := newSelectionRouter(db, uid, 1)

	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}

	// list only returns tenant-1 tasks
	w := get("/api/v1/selection/tasks")
	if w.Code != http.StatusOK {
		t.Fatalf("list: got %d", w.Code)
	}
	var env struct {
		Data struct {
			Items []struct {
				ID uuid.UUID `json:"id"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.Total != 1 || len(env.Data.Items) != 1 || env.Data.Items[0].ID != mine.ID {
		t.Fatalf("list must only contain tenant-1 task, got %+v", env.Data)
	}

	// own task readable, foreign task 404
	if w := get("/api/v1/selection/tasks/" + mine.ID.String()); w.Code != http.StatusOK {
		t.Fatalf("own task: got %d, want 200", w.Code)
	}
	if w := get("/api/v1/selection/tasks/" + other.ID.String()); w.Code != http.StatusNotFound {
		t.Fatalf("foreign task: got %d, want 404", w.Code)
	}
	if w := get("/api/v1/selection/tasks/" + mine.ID.String() + "/candidates"); w.Code != http.StatusOK {
		t.Fatalf("own candidates: got %d, want 200", w.Code)
	}
	if w := get("/api/v1/selection/tasks/" + other.ID.String() + "/candidates"); w.Code != http.StatusNotFound {
		t.Fatalf("foreign candidates: got %d, want 404", w.Code)
	}
}
