package pdf

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/brandsrx/supay/internal/config"
)

// NewStorageFromConfig crea el Storage según config.StorageDriver.
func NewStorageFromConfig(cfg config.Config) (Storage, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.StorageDriver))
	if driver == "" {
		driver = "none"
	}
	switch driver {
	case "none":
		return NewNoopStorage(), nil
	case "local":
		st, err := NewLocalStorage(cfg.StoragePath)
		if err != nil {
			return nil, err
		}
		slog.Info("pdf storage: local", "path", cfg.StoragePath)
		return st, nil
	case "r2":
		if strings.TrimSpace(cfg.R2.Bucket) == "" ||
			strings.TrimSpace(cfg.R2.AccessKeyID) == "" ||
			strings.TrimSpace(cfg.R2.SecretAccessKey) == "" ||
			(strings.TrimSpace(cfg.R2.AccountID) == "" && strings.TrimSpace(cfg.R2.Endpoint) == "") {
			return nil, fmt.Errorf("pdf storage: STORAGE_DRIVER=r2 requiere R2_BUCKET, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY y R2_ACCOUNT_ID (o R2_ENDPOINT)")
		}
		r2Cfg := R2StorageConfig{
			AccountID:       cfg.R2.AccountID,
			AccessKeyID:     cfg.R2.AccessKeyID,
			SecretAccessKey: cfg.R2.SecretAccessKey,
			Bucket:          cfg.R2.Bucket,
			Endpoint:        cfg.R2.Endpoint,
			PublicURL:       cfg.R2.PublicURL,
		}
		st, err := NewR2Storage(r2Cfg)
		if err != nil {
			return nil, fmt.Errorf("pdf storage r2: %w", err)
		}
		slog.Info("pdf storage: r2", "bucket", cfg.R2.Bucket, "endpoint", cfg.R2.Endpoint)
		return st, nil
	default:
		return nil, fmt.Errorf("pdf storage: driver desconocido %q", driver)
	}
}
