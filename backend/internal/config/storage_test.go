package config

import (
	"strings"
	"testing"
)

func TestStorageConfigurationFailsFast(t *testing.T) {
	t.Setenv("STORAGE_DRIVER", "")
	t.Setenv("STORAGE_SIGNING_SECRET", strings.Repeat("s", 32))
	cfg := Load()
	if cfg.StorageDriver != "local" || cfg.ValidateStorage() != nil {
		t.Fatalf("default local: %+v, %v", cfg.StorageDriver, cfg.ValidateStorage())
	}
	t.Setenv("STORAGE_PRESIGN_TTL", "not-a-duration")
	if err := Load().ValidateStorage(); err == nil {
		t.Fatal("invalid TTL accepted")
	}
	t.Setenv("STORAGE_PRESIGN_TTL", "5m")
	t.Setenv("STORAGE_DRIVER", "unknown")
	if err := Load().ValidateStorage(); err == nil {
		t.Fatal("unknown driver accepted")
	}
	t.Setenv("STORAGE_DRIVER", "r2")
	t.Setenv("R2_ACCOUNT_ID", "")
	t.Setenv("R2_ACCESS_KEY_ID", "")
	t.Setenv("R2_SECRET_ACCESS_KEY", "")
	t.Setenv("R2_BUCKET", "")
	t.Setenv("R2_ENDPOINT", "")
	if err := Load().ValidateStorage(); err == nil {
		t.Fatal("incomplete R2 config accepted")
	}
}
