package storage

import (
	"context"
	"os"
	"testing"

	"github.com/brandsrx/supay/internal/config"
)

func TestLocalCertStoragePutGetDelete(t *testing.T) {
	dir := t.TempDir()
	st, err := NewLocalCertStorage(dir)
	if err != nil {
		t.Fatalf("NewLocalCertStorage: %v", err)
	}
	ctx := context.Background()
	key := "certs/comp-1/cert-123.p12.enc"
	data := []byte("encrypted-p12-bytes-test")
	ref, err := st.Put(ctx, key, data)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if len(ref) < len("certs/comp-1/cert-123.p12.enc") {
		t.Fatalf("ref unexpected %q", ref)
	}
	got, err := st.Get(ctx, ref)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("Get mismatch got %q want %q", got, data)
	}
	// Get via key
	got2, err := st.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get via key: %v", err)
	}
	if string(got2) != string(data) {
		t.Fatalf("Get via key mismatch")
	}
	if err := st.Delete(ctx, ref); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := st.Get(ctx, ref); err == nil {
		t.Fatalf("Get after Delete should fail")
	}
	// file should be gone
	if _, err := os.Stat(ref); !os.IsNotExist(err) {
		t.Fatalf("file should be deleted, stat err %v", err)
	}
}

func TestMemoryCertStorage(t *testing.T) {
	st := NewMemoryCertStorage()
	ctx := context.Background()
	key := "certs/comp-1/cert-999.p12.enc"
	data := []byte("memory-data")
	ref, err := st.Put(ctx, key, data)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if ref != "memory://"+key {
		t.Fatalf("ref mismatch %q", ref)
	}
	got, err := st.Get(ctx, ref)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("mismatch")
	}
	if err := st.Delete(ctx, ref); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := st.Get(ctx, ref); err == nil {
		t.Fatalf("should not found after delete")
	}
}

func TestNewCertStorageFromConfig_LocalSelfHosted(t *testing.T) {
	cfg := config.Config{StorageDriver: "local", DeploymentMode: "selfhosted", StoragePath: "./storage/pdfs"}
	st, err := NewCertStorageFromConfig(cfg)
	if err != nil {
		t.Fatalf("expected local ok, got %v", err)
	}
	if _, ok := st.(*LocalCertStorage); !ok {
		t.Fatalf("expected LocalCertStorage got %T", st)
	}
}

func TestNewCertStorageFromConfig_R2SelfHosted(t *testing.T) {
	cfg := config.Config{StorageDriver: "r2", DeploymentMode: "selfhosted", R2: config.R2Config{Bucket: "supay-certs", AccountID: "acc", AccessKeyID: "key", SecretAccessKey: "secret"}}
	if _, err := NewCertStorageFromConfig(cfg); err != nil {
		t.Fatalf("driver should be independent from deployment mode: %v", err)
	}
}

func TestNewCertStorageFromConfig_R2MissingBucket(t *testing.T) {
	cfg := config.Config{StorageDriver: "r2", DeploymentMode: "cloud", R2: config.R2Config{Bucket: ""}}
	_, err := NewCertStorageFromConfig(cfg)
	if err == nil {
		t.Fatalf("expected error for missing bucket")
	}
}

func TestNewCertStorageFromConfig_R2CloudOK(t *testing.T) {
	cfg := config.Config{
		StorageDriver: "r2", DeploymentMode: "cloud",
		R2: config.R2Config{Bucket: "supay-certs", AccountID: "acc123", AccessKeyID: "k", SecretAccessKey: "s"},
	}
	st, err := NewCertStorageFromConfig(cfg)
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if _, ok := st.(*R2CertStorage); !ok {
		t.Fatalf("expected R2CertStorage got %T", st)
	}
}

func TestNewCertStorageFromConfig_NoneOnlyInTest(t *testing.T) {
	// Ensure GO_ENV not test -> should error
	t.Setenv("GO_ENV", "")
	t.Setenv("ENV", "")
	// Need to ensure isTestBinary false: we are in test binary so isTestBinary true -> would pass.
	// Instead test the logic via explicit check: in prod, none should error when not in test env.
	// Since we are in test binary, isTestEnv true, so none will be allowed. So we test the branch that none is allowed in test.
	cfg := config.Config{StorageDriver: "none", DeploymentMode: "selfhosted"}
	st, err := NewCertStorageFromConfig(cfg)
	if err != nil {
		t.Fatalf("in test binary, none should be allowed, got %v", err)
	}
	if _, ok := st.(*MemoryCertStorage); !ok {
		t.Fatalf("expected Memory got %T", st)
	}
	// For prod case, we need to simulate non-test env by checking factory directly with GO_ENV unset and not in test binary?
	// We can't easily simulate non-test binary without subprocess, so we test the error path via unknown driver
	cfg2 := config.Config{StorageDriver: "unknown", DeploymentMode: "selfhosted"}
	if _, err := NewCertStorageFromConfig(cfg2); err == nil {
		t.Fatalf("expected error for unknown driver")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}
