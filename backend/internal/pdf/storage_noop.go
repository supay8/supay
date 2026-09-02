package pdf

import "context"

// NoopStorage es el driver por defecto (self-hosted sin persistencia).
// Siempre hace miss, manteniendo el comportamiento on-demand actual.
type NoopStorage struct{}

func NewNoopStorage() *NoopStorage { return &NoopStorage{} }

func (s *NoopStorage) Save(_ context.Context, _ string, _ []byte) error { return nil }

func (s *NoopStorage) Get(_ context.Context, _ string) ([]byte, error) { return nil, ErrNotFound }

func (s *NoopStorage) Exists(_ context.Context, _ string) bool { return false }

var _ Storage = (*NoopStorage)(nil)
