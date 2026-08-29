package pdf

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/brandsrx/supay/internal/config"
)

// NewStorageFromConfig crea el Storage según config.StorageDriver.
// selfhosted + r2 => ya normalizado a none en config, pero se defiende de nuevo.
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
		if strings.ToLower(cfg.DeploymentMode) == "selfhosted" {
			slog.Warn("pdf storage: R2 solicitado en modo selfhosted, forzando local (disco)")
			st, err := NewLocalStorage(cfg.StoragePath)
			if err != nil {
				return nil, err
			}
			slog.Info("pdf storage: local (fallback selfhosted)", "path", cfg.StoragePath)
			return st, nil
		}
		// Si R2 está incompleto (sin bucket), no hacer fatal: warn + fallback a noop para no bloquear arranque en dev.
		if strings.TrimSpace(cfg.R2.Bucket) == "" {
			slog.Warn("pdf storage: R2_BUCKET vacío, usando none (on-demand) hasta configurar R2")
			return NewNoopStorage(), nil
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
		slog.Warn("pdf storage: driver desconocido, usando none", "driver", driver)
		return NewNoopStorage(), nil
	}
}
