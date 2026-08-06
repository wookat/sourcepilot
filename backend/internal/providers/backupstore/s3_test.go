package backupstore

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeS3 is a minimal in-memory S3-compatible endpoint (path-style).
type fakeS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
	failPut int // fail this many PUTs before succeeding
}

type listEntry struct {
	Key          string    `xml:"Key"`
	Size         int64     `xml:"Size"`
	LastModified time.Time `xml:"LastModified"`
}

type listResult struct {
	XMLName     xml.Name    `xml:"ListBucketResult"`
	IsTruncated bool        `xml:"IsTruncated"`
	Contents    []listEntry `xml:"Contents"`
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := strings.TrimPrefix(r.URL.Path, "/test-bucket/")
	switch {
	case r.Method == http.MethodPut:
		if f.failPut > 0 {
			f.failPut--
			http.Error(w, "injected failure", http.StatusInternalServerError)
			return
		}
		body := make([]byte, 0)
		buf := make([]byte, 4096)
		for {
			n, err := r.Body.Read(buf)
			body = append(body, buf[:n]...)
			if err != nil {
				break
			}
		}
		f.objects[key] = body
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2":
		prefix := r.URL.Query().Get("prefix")
		res := listResult{IsTruncated: false}
		for k, v := range f.objects {
			if strings.HasPrefix(k, prefix) {
				res.Contents = append(res.Contents, listEntry{Key: k, Size: int64(len(v)), LastModified: time.Now().UTC()})
			}
		}
		w.Header().Set("Content-Type", "application/xml")
		_ = xml.NewEncoder(w).Encode(res)
	case r.Method == http.MethodGet:
		v, ok := f.objects[key]
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		_, _ = w.Write(v)
	case r.Method == http.MethodDelete:
		delete(f.objects, key)
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodHead:
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "unsupported", http.StatusMethodNotAllowed)
	}
}

func newFakeStore(t *testing.T, fake *fakeS3) Store {
	t.Helper()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	store, err := New(Config{
		Provider: "minio", Endpoint: srv.URL, Region: "us-east-1",
		Bucket: "test-bucket", Prefix: "backups/test",
		AccessKeyID: "test-ak", SecretAccessKey: "test-sk", UsePathStyle: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if store == nil {
		t.Fatal("expected configured store")
	}
	return store
}

func TestNewReturnsNilWhenUnconfigured(t *testing.T) {
	store, err := New(Config{})
	if err != nil || store != nil {
		t.Fatalf("expected nil store for unconfigured, got %v %v", store, err)
	}
}

func TestTargetMasksCredentials(t *testing.T) {
	cfg := Config{Endpoint: "http://minio.internal:9000", Bucket: "b", Prefix: "backups/prod", AccessKeyID: "AKIA123", SecretAccessKey: "super-secret"}
	target := cfg.Target()
	if strings.Contains(target, "super-secret") || strings.Contains(target, "AKIA123") {
		t.Fatalf("target must not contain credentials: %s", target)
	}
	if target != "s3://b/backups/prod (minio.internal:9000)" {
		t.Fatalf("unexpected target: %s", target)
	}
}

func TestPartiallyConfigured(t *testing.T) {
	if (Config{}).PartiallyConfigured() {
		t.Fatal("empty config is not partially configured")
	}
	if !(Config{Bucket: "b"}).PartiallyConfigured() {
		t.Fatal("bucket-only config is partially configured")
	}
	if (Config{Bucket: "b", AccessKeyID: "a", SecretAccessKey: "s"}).PartiallyConfigured() {
		t.Fatal("complete config is not partially configured")
	}
}

func TestUploadDownloadListDelete(t *testing.T) {
	fake := &fakeS3{objects: map[string][]byte{}}
	store := newFakeStore(t, fake)
	ctx := context.Background()

	dir := t.TempDir()
	src := filepath.Join(dir, "artifact.dump")
	if err := os.WriteFile(src, []byte("backup-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Upload(ctx, "backups/test/artifact.dump", src, "application/octet-stream"); err != nil {
		t.Fatal(err)
	}

	objects, err := store.List(ctx, "backups/test")
	if err != nil || len(objects) != 1 || objects[0].Key != "backups/test/artifact.dump" {
		t.Fatalf("unexpected list result: %v %v", objects, err)
	}

	dst := filepath.Join(dir, "restored", "artifact.dump")
	if err := store.Download(ctx, "backups/test/artifact.dump", dst); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != "backup-bytes" {
		t.Fatalf("unexpected downloaded content: %q %v", got, err)
	}

	if err := store.Delete(ctx, "backups/test/artifact.dump"); err != nil {
		t.Fatal(err)
	}
	objects, err = store.List(ctx, "backups/test")
	if err != nil || len(objects) != 0 {
		t.Fatalf("expected empty list after delete: %v %v", objects, err)
	}
}

func TestDownloadMissingObjectFails(t *testing.T) {
	fake := &fakeS3{objects: map[string][]byte{}}
	store := newFakeStore(t, fake)
	dst := filepath.Join(t.TempDir(), "missing.dump")
	if err := store.Download(context.Background(), "backups/test/missing.dump", dst); err == nil {
		t.Fatal("expected error for missing object")
	}
}
