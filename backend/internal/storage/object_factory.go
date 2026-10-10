package storage

import (
	"context"
	"fmt"

	"github.com/brandsrx/supay/internal/config"
	"github.com/brandsrx/supay/internal/ports"
)

func NewObjectStorageFromConfig(ctx context.Context, cfg config.Config) (ports.Storage, error) {
	if err := cfg.ValidateStorage(); err != nil {
		return nil, err
	}
	switch cfg.StorageDriver {
	case "local":
		return NewLocalObjectStorage(cfg.StoragePath, cfg.StorageSigningSecret, cfg.StoragePublicURL)
	case "r2":
		return NewR2ObjectStorage(ctx, ObjectR2Config{AccountID: cfg.R2.AccountID, AccessKeyID: cfg.R2.AccessKeyID, SecretAccessKey: cfg.R2.SecretAccessKey, Bucket: cfg.R2.Bucket, Endpoint: cfg.R2.Endpoint})
	default:
		return nil, fmt.Errorf("STORAGE_DRIVER inválido")
	}
}
