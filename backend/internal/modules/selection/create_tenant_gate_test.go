package selection

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 回归：R84 前 tenant 0（平台管理员上下文）创建选品任务会落库成 pending，
// 但 worker gate 拒绝 tenant_id<=0，任务永远停在 pending。
// 现在创建入口必须直接拒绝，不留悬挂任务。
func TestSelectionCreateRejectsNonPositiveTenant(t *testing.T) {
	db := newGuardDB(t)
	uid := seedAdmin(t, db, "admin")
	r := newSelectionRouter(db, uid, 0)

	body := `{"targetPlatform":"tiktok","items":[{"title":"cand"}]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/selection/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for tenant 0 create, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "TENANT_CONTEXT_MISSING") {
		t.Fatalf("expected TENANT_CONTEXT_MISSING, got %s", w.Body.String())
	}

	var n int64
	if err := db.Model(&SelectionTask{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected no residual task rows, got %d", n)
	}

	// tenant >0 通过租户闸门（测试环境无 redis，允许 503 queue unavailable）
	r2 := newSelectionRouter(db, uid, 1)
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/selection/tasks", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	r2.ServeHTTP(w2, req2)
	if strings.Contains(w2.Body.String(), "TENANT_CONTEXT_MISSING") {
		t.Fatalf("tenant 1 create must pass tenant gate, got %d body=%s", w2.Code, w2.Body.String())
	}
}
