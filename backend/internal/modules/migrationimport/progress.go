package migrationimport

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// commitProgress tracks one in-flight commit so the frontend can poll a
// progress bar and a duplicate submit of the same batch is rejected while
// the first one is still running. In-memory state matches the single-process
// backend deployment; a restart simply clears in-flight entries (the batch
// idempotency key in import_jobs still prevents double writes).
type commitProgress struct {
	Processed int       `json:"processed"`
	Total     int       `json:"total"`
	StartedAt time.Time `json:"startedAt"`
}

type progressTracker struct {
	mu      sync.Mutex
	entries map[string]*commitProgress
}

var commits = &progressTracker{entries: map[string]*commitProgress{}}

func progressKey(tenantID int64, kind, fileHash string) string {
	return fmt.Sprintf("%d:%s:%s", tenantID, kind, strings.TrimSpace(fileHash))
}

// begin registers an in-flight commit; it reports false when the same batch
// is already being committed.
func (t *progressTracker) begin(key string, total int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.entries[key]; ok {
		return false
	}
	t.entries[key] = &commitProgress{Total: total, StartedAt: time.Now()}
	return true
}

func (t *progressTracker) advance(key string, done int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if p, ok := t.entries[key]; ok {
		p.Processed += done
		if p.Processed > p.Total {
			p.Processed = p.Total
		}
	}
}

func (t *progressTracker) finish(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, key)
}

func (t *progressTracker) snapshot(key string) (commitProgress, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	p, ok := t.entries[key]
	if !ok {
		return commitProgress{}, false
	}
	return *p, true
}

// Progress GET /imports/progress?kind=&fileHash= — in-flight commit progress
// for the caller's tenant. active=false means no commit is running (finished
// or never started); the frontend then reads the commit response instead.
func (h *Handler) Progress(c *gin.Context) {
	kind, err := normalizeKind(c.Query("kind"))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	fileHash := strings.TrimSpace(c.Query("fileHash"))
	if fileHash == "" {
		response.Fail(c, 400, response.CodeBadRequest, "文件指纹（fileHash）不能为空")
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	p, ok := commits.snapshot(progressKey(tid, kind, fileHash))
	if !ok {
		response.OK(c, gin.H{"active": false, "processed": 0, "total": 0})
		return
	}
	response.OK(c, gin.H{"active": true, "processed": p.Processed, "total": p.Total})
}
