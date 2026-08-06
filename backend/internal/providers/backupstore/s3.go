package backupstore

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const opTimeout = 10 * time.Minute

type s3Store struct {
	cfg    Config
	client *s3.Client
}

func newS3Store(cfg Config) (*s3Store, error) {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, "")),
	}
	if cfg.CABundlePath != "" {
		httpClient, err := httpClientWithCABundle(cfg.CABundlePath)
		if err != nil {
			return nil, err
		}
		opts = append(opts, awsconfig.WithHTTPClient(httpClient))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("backup object storage: aws config load: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.UsePathStyle
	})
	return &s3Store{cfg: cfg, client: client}, nil
}

func (s *s3Store) Target() string { return s.cfg.Target() }

// httpClientWithCABundle builds an HTTP client that trusts the system roots
// plus the certificates in the given PEM bundle.
func httpClientWithCABundle(path string) (*awshttp.BuildableClient, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("backup object storage: read CA bundle: %w", err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("backup object storage: CA bundle %s contains no valid PEM certificate", path)
	}
	return awshttp.NewBuildableClient().WithTransportOptions(func(tr *http.Transport) {
		if tr.TLSClientConfig == nil {
			tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		tr.TLSClientConfig.RootCAs = pool
	}), nil
}

func normKey(key string) string {
	return strings.TrimLeft(strings.ReplaceAll(key, "\\", "/"), "/")
}

func (s *s3Store) Upload(ctx context.Context, key, localPath, contentType string) error {
	key = normKey(key)
	if key == "" {
		return fmt.Errorf("backup object storage: empty object key")
	}
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("backup object storage: open artifact: %w", err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("backup object storage: stat artifact: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	in := &s3.PutObjectInput{
		Bucket:        aws.String(s.cfg.Bucket),
		Key:           aws.String(key),
		Body:          f,
		ContentLength: aws.Int64(info.Size()),
	}
	if ct := strings.TrimSpace(contentType); ct != "" {
		in.ContentType = aws.String(ct)
	}
	if _, err := s.client.PutObject(ctx, in); err != nil {
		return fmt.Errorf("backup object storage: put object: %w", err)
	}
	return nil
}

func (s *s3Store) Download(ctx context.Context, key, localPath string) error {
	key = normKey(key)
	if key == "" {
		return fmt.Errorf("backup object storage: empty object key")
	}
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("backup object storage: get object: %w", err)
	}
	defer func() { _ = out.Body.Close() }()
	if err := os.MkdirAll(filepath.Dir(localPath), 0o700); err != nil {
		return fmt.Errorf("backup object storage: prepare local dir: %w", err)
	}
	f, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("backup object storage: create local file: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, out.Body); err != nil {
		return fmt.Errorf("backup object storage: write local file: %w", err)
	}
	return nil
}

func (s *s3Store) List(ctx context.Context, prefix string) ([]Object, error) {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	var objects []Object
	var continuation *string
	for {
		out, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.cfg.Bucket),
			Prefix:            aws.String(normKey(prefix)),
			ContinuationToken: continuation,
		})
		if err != nil {
			return nil, fmt.Errorf("backup object storage: list objects: %w", err)
		}
		for _, obj := range out.Contents {
			o := Object{Key: aws.ToString(obj.Key), Size: aws.ToInt64(obj.Size)}
			if obj.LastModified != nil {
				o.LastModified = *obj.LastModified
			}
			objects = append(objects, o)
		}
		if out.IsTruncated == nil || !*out.IsTruncated {
			break
		}
		continuation = out.NextContinuationToken
	}
	return objects, nil
}

func (s *s3Store) Delete(ctx context.Context, key string) error {
	key = normKey(key)
	if key == "" {
		return fmt.Errorf("backup object storage: empty object key")
	}
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("backup object storage: delete object: %w", err)
	}
	return nil
}
