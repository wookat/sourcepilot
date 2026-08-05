package migrationimport_test

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/migrationimport"
)

// TestImportPerf10k measures validate + commit wall time and heap growth for a
// 10k-row inventory opening import. It runs only with IMPORT_PERF=1 so the
// regular suite stays fast; numbers print to the test log for evidence runs.
func TestImportPerf10k(t *testing.T) {
	if os.Getenv("IMPORT_PERF") != "1" {
		t.Skip("set IMPORT_PERF=1 to run the 10k-row performance measurement")
	}
	db := openTestDB(t)
	migrateRound115(t, db)
	svc := newSvc115(db)
	c := testCtx(1)

	const n = 10000
	rows := make([][]string, 0, n)
	for i := 0; i < n; i++ {
		code := fmt.Sprintf("PERF-SKU-%05d", i)
		seedSKU(t, db, 1, code)
		rows = append(rows, []string{code, "", "10", "1.50"})
	}
	body := migrationimport.WizardBody{
		Kind:         migrationimport.KindInventory,
		Columns:      []string{"SKU编码", "仓库编码", "期初数量", "参考进价"},
		Rows:         rows,
		Mapping:      map[string]int{"skuCode": 0, "warehouseCode": 1, "quantity": 2, "costPrice": 3},
		FileName:     "perf-10k.csv",
		FileHash:     "hash-perf-10k",
		SourceFormat: migrationimport.SourceCustom,
	}

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	start := time.Now()
	out, err := svc.Validate(c, body)
	if err != nil {
		t.Fatal(err)
	}
	validateDur := time.Since(start)
	if out.ErrorRows != 0 {
		t.Fatalf("validate errors: %+v", out)
	}

	start = time.Now()
	res, err := svc.Commit(c, body, nil)
	if err != nil {
		t.Fatal(err)
	}
	commitDur := time.Since(start)
	runtime.ReadMemStats(&after)

	if res.SuccessRows != n {
		t.Fatalf("commit: %+v", res)
	}
	t.Logf("rows=%d validate=%s commit=%s heapDelta=%.1fMB heapInUse=%.1fMB",
		n, validateDur, commitDur,
		float64(after.HeapAlloc-before.HeapAlloc)/(1<<20),
		float64(after.HeapInuse)/(1<<20))
}
