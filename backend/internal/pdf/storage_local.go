package pdf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// LocalStorage persiste PDFs en el filesystem local.
// Requiere que el caller mapee un volumen si quiere persistencia entre reinicios.
type LocalStorage struct {
	basePath string
}

func NewLocalStorage(basePath string) (*LocalStorage, error) {
	if basePath == "" {
		basePath = "./storage/pdfs"
	}
	// Crear directorio si no existe (permite docker run sin volumen).
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("pdf local storage: no se pudo crear directorio %q: %w", basePath, err)
	}
	return &LocalStorage{basePath: basePath}, nil
}

func (s *LocalStorage) filePath(invoiceID string) string {
	// Sanitizar invoiceID para evitar path traversal.
	name := filepath.Base(invoiceID)
	if name == "" || name == "." || name == "/" {
		name = "unknown"
	}
	return filepath.Join(s.basePath, name+".pdf")
}

func (s *LocalStorage) Save(_ context.Context, invoiceID string, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("pdf local storage: datos vacíos para %s", invoiceID)
	}
	path := s.filePath(invoiceID)
	// Escritura atómica: tmp + rename.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("pdf local storage: write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("pdf local storage: rename: %w", err)
	}
	return nil
}

func (s *LocalStorage) Get(_ context.Context, invoiceID string) ([]byte, error) {
	path := s.filePath(invoiceID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("pdf local storage: read %q: %w", path, err)
	}
	return data, nil
}

func (s *LocalStorage) Exists(_ context.Context, invoiceID string) bool {
	path := s.filePath(invoiceID)
	_, err := os.Stat(path)
	return err == nil
}

var _ Storage = (*LocalStorage)(nil)
