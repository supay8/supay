package storage

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brandsrx/supay/internal/config"
)

func isTestBinary() bool {
	if flag.Lookup("test.v") != nil {
		return true
	}
	if strings.Contains(strings.Join(os.Args, " "), "test") {
		return true
	}
	// Go test binary contiene ".test"
	if strings.Contains(os.Args[0], ".test") {
		return true
	}
	return false
}

func isTestEnv() bool {
	if strings.EqualFold(os.Getenv("GO_ENV"), "test") || strings.EqualFold(os.Getenv("ENV"), "test") || strings.EqualFold(os.Getenv("APP_ENV"), "test") {
		return true
	}
	return isTestBinary()
}

// certBasePath deriva el path local para certs a partir de StoragePath.
// Retorna el directorio raíz de storage (ej. ./storage) para que la key "certs/<companyId>/..." resulte en ./storage/certs/...
// Si StoragePath es .../pdfs o .../certs, retorna su directorio padre.
func certBasePath(storagePath string) string {
	if strings.TrimSpace(storagePath) == "" {
		return "./storage"
	}
	clean := filepath.Clean(storagePath)
	if strings.HasSuffix(clean, "pdfs") || strings.HasSuffix(clean, "certs") {
		return filepath.Dir(clean)
	}
	return clean
}

// NewCertStorageFromConfig crea el CertStorage según STORAGE_DRIVER.
// - local: self-hosted open-source, disco ./storage/certs 0700/0600
// - r2: SaaS cloud, bucket privado supay-certs (creación manual, SSE-S3)
// - none/memory: solo tests (go test), error estricto en prod/dev
func NewCertStorageFromConfig(cfg config.Config) (CertStorage, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.StorageDriver))
	if driver == "" {
		driver = "none"
	}
	switch driver {
	case "none", "memory":
		if !isTestEnv() {
			return nil, fmt.Errorf("cert storage: STORAGE_DRIVER=none/memory solo permitido en tests (GO_ENV=test); en producción use local o r2 (driver actual %q)", driver)
		}
		return NewMemoryCertStorage(), nil
	case "local":
		base := certBasePath(cfg.StoragePath)
		// Permitir override explícito via CERT_STORAGE_PATH
		if v := strings.TrimSpace(os.Getenv("CERT_STORAGE_PATH")); v != "" {
			base = v
		}
		return NewLocalCertStorage(base)
	case "r2":
		if strings.TrimSpace(cfg.R2.Bucket) == "" {
			return nil, fmt.Errorf("cert storage: R2_BUCKET es obligatorio para STORAGE_DRIVER=r2 (bucket supay-certs debe crearse manual en dashboard R2, privado, SSE-S3)")
		}
		r2cfg := R2Config{
			AccountID:       cfg.R2.AccountID,
			AccessKeyID:     cfg.R2.AccessKeyID,
			SecretAccessKey: cfg.R2.SecretAccessKey,
			Bucket:          cfg.R2.Bucket,
			Endpoint:        cfg.R2.Endpoint,
		}
		return NewR2CertStorage(r2cfg)
	default:
		return nil, fmt.Errorf("cert storage: driver desconocido %q (use local, r2, o none solo en tests)", driver)
	}
}
