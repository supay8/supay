package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/brandsrx/supay/internal/domain"
)

// CertStorage abstrae el almacenamiento de .p12 cifrados.
// Implementaciones: local (self-hosted), r2 (cloud SaaS, bucket privado supay-certs), memory (tests).
type CertStorage interface {
	// Put almacena bytes cifrados en key (ej. certs/<companyId>/<certId>.p12.enc) y retorna ref.
	// ref es r2://<bucket>/<key> para R2 o path local para local.
	Put(ctx context.Context, key string, data []byte) (string, error)
	Get(ctx context.Context, ref string) ([]byte, error)
	Delete(ctx context.Context, ref string) error
}

// MemoryCertStorage solo para tests (go test).
type MemoryCertStorage struct {
	data map[string][]byte
}

func NewMemoryCertStorage() *MemoryCertStorage {
	return &MemoryCertStorage{data: make(map[string][]byte)}
}

func (m *MemoryCertStorage) Put(_ context.Context, key string, data []byte) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("cert memory storage: key vacío")
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	m.data[key] = cp
	return "memory://" + key, nil
}

func (m *MemoryCertStorage) Get(_ context.Context, ref string) ([]byte, error) {
	key := strings.TrimPrefix(ref, "memory://")
	if v, ok := m.data[key]; ok {
		cp := make([]byte, len(v))
		copy(cp, v)
		return cp, nil
	}
	// también intentar con ref tal cual si es key directa
	if v, ok := m.data[ref]; ok {
		cp := make([]byte, len(v))
		copy(cp, v)
		return cp, nil
	}
	return nil, fmt.Errorf("cert memory storage: no encontrado %q", ref)
}

func (m *MemoryCertStorage) Delete(_ context.Context, ref string) error {
	key := strings.TrimPrefix(ref, "memory://")
	delete(m.data, key)
	delete(m.data, ref)
	return nil
}

// LocalCertStorage persiste cifrados en disco local self-hosted.
type LocalCertStorage struct {
	basePath string
}

func NewLocalCertStorage(basePath string) (*LocalCertStorage, error) {
	if basePath == "" {
		basePath = "./storage/certs"
	}
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("cert local storage: mkdir %q: %w", basePath, err)
	}
	return &LocalCertStorage{basePath: basePath}, nil
}

func (s *LocalCertStorage) fullPath(key string) string {
	// Evitar traversal: limpiar y forzar dentro de basePath
	clean := filepath.Clean(strings.TrimSpace(key))
	clean = strings.TrimPrefix(clean, "/")
	return filepath.Join(s.basePath, clean)
}

func (s *LocalCertStorage) Put(_ context.Context, key string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("cert local storage: datos vacíos para %q", key)
	}
	path := s.fullPath(key)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return path, nil
}

func (s *LocalCertStorage) Get(_ context.Context, ref string) ([]byte, error) {
	// ref puede ser key o path absoluto
	candidates := []string{ref, s.fullPath(ref), s.fullPath(strings.TrimPrefix(ref, "r2://"))}
	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil {
			return data, nil
		}
	}
	// intentar leer ref tal cual si es path absoluto
	if data, err := os.ReadFile(ref); err == nil {
		return data, nil
	}
	return nil, fmt.Errorf("cert local storage: no encontrado %q", ref)
}

func (s *LocalCertStorage) Delete(_ context.Context, ref string) error {
	candidates := []string{ref, s.fullPath(ref)}
	for _, p := range candidates {
		_ = os.Remove(p)
	}
	_ = os.Remove(ref)
	return nil
}

// R2CertStorage persiste cifrados en bucket privado supay-certs (SSE-S3 automático).
// El bucket debe crearse manualmente vía dashboard/CLI/Terraform, sin auto-creación.
type R2CertStorage struct {
	cfg     R2Config
	objects *R2ObjectStorage
}

type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	Endpoint        string
}

func NewR2CertStorage(cfg R2Config) (*R2CertStorage, error) {
	objects, err := NewR2ObjectStorage(context.Background(), ObjectR2Config{AccountID: cfg.AccountID, AccessKeyID: cfg.AccessKeyID, SecretAccessKey: cfg.SecretAccessKey, Bucket: cfg.Bucket, Endpoint: cfg.Endpoint})
	if err != nil {
		return nil, err
	}
	return &R2CertStorage{cfg: cfg, objects: objects}, nil
}

func (s *R2CertStorage) refToKey(ref string) string {
	// ref formato r2://<bucket>/<key> o solo key
	if strings.HasPrefix(ref, "r2://") {
		trim := strings.TrimPrefix(ref, "r2://")
		// remover bucket prefix si viene
		if idx := strings.Index(trim, "/"); idx >= 0 {
			return trim[idx+1:]
		}
		return trim
	}
	return strings.TrimPrefix(ref, "/")
}

func (s *R2CertStorage) Put(ctx context.Context, key string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("cert r2 storage: datos vacíos para %q", key)
	}
	if _, err := s.objects.Put(ctx, key, bytes.NewReader(data), domain.PutOptions{ContentType: "application/octet-stream", Size: int64(len(data))}); err != nil {
		return "", err
	}
	return "r2://" + s.cfg.Bucket + "/" + key, nil
}

func (s *R2CertStorage) Get(ctx context.Context, ref string) ([]byte, error) {
	key := s.refToKey(ref)
	body, _, err := s.objects.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	return io.ReadAll(body)
}

func (s *R2CertStorage) Delete(ctx context.Context, ref string) error {
	return s.objects.Delete(ctx, s.refToKey(ref))
}

var _ CertStorage = (*MemoryCertStorage)(nil)
var _ CertStorage = (*LocalCertStorage)(nil)
var _ CertStorage = (*R2CertStorage)(nil)
