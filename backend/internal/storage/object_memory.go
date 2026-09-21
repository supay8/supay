package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"sync"
	"time"

	"github.com/brandsrx/supay/internal/domain"
)

// MemoryObjectStorage is a deterministic fake for use case tests.
type MemoryObjectStorage struct {
	mu      sync.Mutex
	objects map[string][]byte
	infos   map[string]domain.ObjectInfo
}

func NewMemoryObjectStorage() *MemoryObjectStorage {
	return &MemoryObjectStorage{objects: map[string][]byte{}, infos: map[string]domain.ObjectInfo{}}
}
func (s *MemoryObjectStorage) Put(ctx context.Context, key string, r io.Reader, opts domain.PutOptions) (domain.ObjectInfo, error) {
	if err := domain.ValidateObjectKey(key); err != nil {
		return domain.ObjectInfo{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.objects[key]; exists {
		return domain.ObjectInfo{}, domain.ErrAlreadyExists
	}
	h := sha256.New()
	var buf bytes.Buffer
	n, err := io.Copy(&buf, io.TeeReader(&contextReader{ctx, r}, h))
	if err != nil {
		return domain.ObjectInfo{}, err
	}
	if opts.Size > 0 && opts.Size != n {
		return domain.ObjectInfo{}, domain.ErrInvalidKey
	}
	info := domain.ObjectInfo{Key: key, Size: n, ContentType: opts.ContentType, SHA256: hex.EncodeToString(h.Sum(nil)), CreatedAt: time.Now().UTC()}
	s.objects[key] = buf.Bytes()
	s.infos[key] = info
	return info, nil
}
func (s *MemoryObjectStorage) Get(ctx context.Context, key string) (io.ReadCloser, domain.ObjectInfo, error) {
	info, err := s.Stat(ctx, key)
	if err != nil {
		return nil, domain.ObjectInfo{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return io.NopCloser(bytes.NewReader(s.objects[key])), info, nil
}
func (s *MemoryObjectStorage) Stat(ctx context.Context, key string) (domain.ObjectInfo, error) {
	if err := domain.ValidateObjectKey(key); err != nil {
		return domain.ObjectInfo{}, err
	}
	if err := ctx.Err(); err != nil {
		return domain.ObjectInfo{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	info, ok := s.infos[key]
	if !ok {
		return domain.ObjectInfo{}, domain.ErrNotFound
	}
	return info, nil
}
func (s *MemoryObjectStorage) Delete(ctx context.Context, key string) error {
	if err := domain.ValidateObjectKey(key); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	delete(s.infos, key)
	return nil
}
func (s *MemoryObjectStorage) PresignGet(ctx context.Context, key string, ttl time.Duration, filename string) (string, error) {
	if _, err := s.Stat(ctx, key); err != nil {
		return "", err
	}
	return "memory://" + key, nil
}

var _ domain.Storage = (*MemoryObjectStorage)(nil)
