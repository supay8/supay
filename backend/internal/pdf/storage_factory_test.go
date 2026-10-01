package pdf

import (
	"strings"
	"testing"

	"github.com/brandsrx/supay/internal/config"
)

func TestNewStorageFromConfigR2FailsClosed(t *testing.T) {
	_, err := NewStorageFromConfig(config.Config{StorageDriver: "r2", DeploymentMode: "cloud"})
	if err == nil || !strings.Contains(err.Error(), "R2_BUCKET") {
		t.Fatalf("R2 incompleto debe fallar, err=%v", err)
	}
}

func TestNewStorageFromConfigRejectsUnknownDriver(t *testing.T) {
	if _, err := NewStorageFromConfig(config.Config{StorageDriver: "mystery"}); err == nil {
		t.Fatal("driver desconocido no debe degradarse a noop")
	}
}
