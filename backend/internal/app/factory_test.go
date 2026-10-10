package app

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// No live database or SIAT account is involved: startup composes dependencies
// eagerly, but the injected SQL pool does not dial until an operation needs it.
func hermeticDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open("host=127.0.0.1 port=1 user=test dbname=test sslmode=disable"), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	sql, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sql.Close() })
	return db
}
func selfHostedConfig(t *testing.T) config.Config {
	return config.Config{RunMode: config.RunModeWeb, Port: "8080", JWTSecret: strings.Repeat("s", 32), JWTIssuer: "test", JWTAccessTTL: time.Hour, BackendSecret: "test-backend", StorageDriver: "local", StoragePath: t.TempDir(), StorageSigningSecret: strings.Repeat("s", 32), StoragePresignTTL: time.Minute, SiatSandbox: true}
}
func TestSelfHostedFactoryComposesWithoutExternalCalls(t *testing.T) {
	application, err := NewSelfHostedApp(selfHostedConfig(t), WithDatabase(hermeticDatabase(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	if application.Server() == nil || application.Server().Handler == nil || application.authUsecase == nil || application.invoiceUsecase == nil {
		t.Fatal("incomplete application")
	}
	if application.cipher != nil {
		t.Fatal("an absent cipher must not become a typed-nil interface")
	}
	if application.certStorage == nil {
		t.Fatal("sandbox must still support certificate storage")
	}
	if err := application.Close(); err != nil {
		t.Fatal(err)
	}
}
func TestFactoryPropagatesInfrastructureErrors(t *testing.T) {
	cfg := selfHostedConfig(t)
	path := filepath.Join(cfg.StoragePath, "file")
	if err := os.WriteFile(path, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg.StoragePath = path
	if application, err := NewSelfHostedApp(cfg, WithDatabase(hermeticDatabase(t))); err == nil || application != nil {
		t.Fatalf("expected constructor error, app=%v err=%v", application, err)
	}
	cfg = selfHostedConfig(t)
	cfg.EncryptionKey = "invalid"
	if application, err := NewSelfHostedApp(cfg, WithDatabase(hermeticDatabase(t))); err == nil || application != nil {
		t.Fatalf("expected encryption error, app=%v err=%v", application, err)
	}
}
func TestAppCloseReversesOwnershipAndJoinsErrors(t *testing.T) {
	sentinel := errors.New("close failed")
	var calls []int
	app := &App{closers: []func() error{func() error { calls = append(calls, 1); return nil }, func() error { calls = append(calls, 2); return sentinel }}}
	if err := app.Close(); !errors.Is(err, sentinel) || !slices.Equal(calls, []int{2, 1}) {
		t.Fatalf("calls=%v err=%v", calls, err)
	}
	if err := app.Close(); err != nil || len(calls) != 2 {
		t.Fatal("Close must be idempotent")
	}
}
func TestFactoryRejectsInvalidModesAndMissingDatabase(t *testing.T) {
	cfg := selfHostedConfig(t)
	cfg.DeploymentMode = "invalid"
	if _, err := NewApp(cfg); err == nil {
		t.Fatal("invalid deployment mode accepted")
	}
	cfg.DeploymentMode = "selfhosted"
	if _, err := NewApp(cfg, WithDatabase(nil)); err == nil {
		t.Fatal("nil database accepted")
	}
}

func TestCloudFactoryComposesWithoutExternalCalls(t *testing.T) {
	cfg := selfHostedConfig(t)
	cfg.JWTSecret = ""
	cfg.StorageDriver = "r2"
	cfg.R2 = config.R2Config{Bucket: "test", AccessKeyID: "test", SecretAccessKey: "test", Endpoint: "https://storage.example.invalid"}
	cfg.BetterAuth = config.BetterAuthConfig{JWKSURL: "https://auth.example.invalid/jwks", Issuer: "https://auth.example.invalid", Audience: "supay", JWKSCacheTTL: time.Hour, HTTPTimeout: time.Second}
	application, err := NewCloudApp(cfg, WithDatabase(hermeticDatabase(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	if application.Server() == nil || application.authUsecase != nil || application.authOptions.Tokens == nil {
		t.Fatal("cloud composition used local authentication")
	}
}
