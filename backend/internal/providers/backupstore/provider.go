// Package backupstore abstracts object storage for backup artifacts
// (S3-compatible endpoints: AWS S3 / MinIO / Aliyun OSS S3 API).
// It is intentionally separate from providers/storage: backup artifacts are
// large files streamed from disk and need listing for retention pruning.
package backupstore

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Object describes one stored backup artifact.
type Object struct {
	Key          string
	Size         int64
	LastModified time.Time
}

// Store is the provider abstraction for backup artifact object storage.
type Store interface {
	// Upload streams a local file to the object storage under key.
	Upload(ctx context.Context, key, localPath, contentType string) error
	// Download streams the object under key into localPath.
	Download(ctx context.Context, key, localPath string) error
	// List returns all objects under prefix.
	List(ctx context.Context, prefix string) ([]Object, error)
	// Delete removes the object under key.
	Delete(ctx context.Context, key string) error
	// Target returns a credential-free description of the storage target
	// (safe for logs and UI display).
	Target() string
}

// Config carries the S3-compatible connection settings. SecretAccessKey must
// never be logged or serialized; Target/String only expose masked info.
type Config struct {
	Provider        string // s3|oss|minio (informational)
	Endpoint        string // empty for AWS default endpoints
	Region          string
	Bucket          string
	Prefix          string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
}

// Configured reports whether the object storage upload is fully configured.
func (c Config) Configured() bool {
	return c.Bucket != "" && c.AccessKeyID != "" && c.SecretAccessKey != ""
}

// PartiallyConfigured reports incomplete credential/bucket combinations that
// should fail config validation instead of silently degrading to local-only.
func (c Config) PartiallyConfigured() bool {
	any := c.Bucket != "" || c.AccessKeyID != "" || c.SecretAccessKey != "" || c.Endpoint != ""
	return any && !c.Configured()
}

// Target returns a masked, credential-free target description like
// "s3://bucket/prefix (minio.example.com)".
func (c Config) Target() string {
	host := ""
	if c.Endpoint != "" {
		if u, err := url.Parse(c.Endpoint); err == nil && u.Host != "" {
			host = u.Host
		} else {
			host = c.Endpoint
		}
	}
	target := fmt.Sprintf("s3://%s", c.Bucket)
	if p := strings.Trim(c.Prefix, "/"); p != "" {
		target += "/" + p
	}
	if host != "" {
		target += " (" + host + ")"
	}
	return target
}

// New builds a Store from cfg. It returns (nil, nil) when object storage is
// not configured: callers keep the existing local-only path (degraded mode).
func New(cfg Config) (Store, error) {
	if !cfg.Configured() {
		return nil, nil
	}
	return newS3Store(cfg)
}
