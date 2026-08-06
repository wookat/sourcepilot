package backupstore

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testCAPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "trademind-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func caConfig(caPath string) Config {
	return Config{
		Provider: "minio", Endpoint: "https://minio.internal:9000",
		Bucket: "trademind-backups", AccessKeyID: "ak", SecretAccessKey: "sk",
		UsePathStyle: true, CABundlePath: caPath,
	}
}

func TestNewS3StoreWithValidCABundle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, testCAPEM(t), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := New(caConfig(path))
	if err != nil {
		t.Fatalf("valid CA bundle rejected: %v", err)
	}
	if store == nil {
		t.Fatal("store must be constructed")
	}
}

func TestNewS3StoreRejectsMissingCABundle(t *testing.T) {
	_, err := New(caConfig(filepath.Join(t.TempDir(), "missing.pem")))
	if err == nil {
		t.Fatal("missing CA bundle file must fail store construction")
	}
}

func TestNewS3StoreRejectsInvalidCABundle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "junk.pem")
	if err := os.WriteFile(path, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := New(caConfig(path))
	if err == nil {
		t.Fatal("invalid CA bundle must fail store construction")
	}
}
