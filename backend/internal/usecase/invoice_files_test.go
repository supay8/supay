package usecase_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/storage"
	"github.com/brandsrx/supay/internal/usecase"
)

type consumeOnConflictStorage struct{ inner *storage.MemoryObjectStorage }

func (s *consumeOnConflictStorage) Put(ctx context.Context, key string, reader io.Reader, opts domain.PutOptions) (domain.ObjectInfo, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return domain.ObjectInfo{}, err
	}
	if _, err := s.inner.Stat(ctx, key); err == nil {
		return domain.ObjectInfo{}, domain.ErrAlreadyExists
	}
	return s.inner.Put(ctx, key, bytes.NewReader(data), opts)
}
func (s *consumeOnConflictStorage) Get(ctx context.Context, key string) (io.ReadCloser, domain.ObjectInfo, error) {
	return s.inner.Get(ctx, key)
}
func (s *consumeOnConflictStorage) Stat(ctx context.Context, key string) (domain.ObjectInfo, error) {
	return s.inner.Stat(ctx, key)
}
func (s *consumeOnConflictStorage) Delete(ctx context.Context, key string) error {
	return s.inner.Delete(ctx, key)
}
func (s *consumeOnConflictStorage) PresignGet(ctx context.Context, key string, ttl time.Duration, filename string) (string, error) {
	return s.inner.PresignGet(ctx, key, ttl, filename)
}

type fileRepoFake struct {
	rows       map[string]*domain.InvoiceFile
	failCreate bool
}

func (r *fileRepoFake) BelongsToCompany(_ context.Context, companyID, invoiceID string) (bool, error) {
	return companyID == "company1" && invoiceID == "invoice1", nil
}
func (r *fileRepoFake) CreateFile(_ context.Context, f *domain.InvoiceFile) error {
	if r.failCreate {
		return errors.New("db unavailable")
	}
	r.rows[f.Kind] = f
	return nil
}
func (r *fileRepoFake) FindFile(_ context.Context, companyID, invoiceID, kind string) (*domain.InvoiceFile, error) {
	if f, ok := r.rows[kind]; ok {
		return f, nil
	}
	return nil, domain.ErrNotFound
}
func (r *fileRepoFake) DeleteFile(_ context.Context, _, _, kind, storageKey string) error {
	if file, ok := r.rows[kind]; ok && file.StorageKey == storageKey {
		delete(r.rows, kind)
	}
	return nil
}

func TestInvoiceFileServiceTenantAndCompensation(t *testing.T) {
	ctx := context.Background()
	object := storage.NewMemoryObjectStorage()
	repo := &fileRepoFake{rows: map[string]*domain.InvoiceFile{}}
	svc := usecase.NewInvoiceFileService(object, repo, 5*time.Minute)
	if _, err := svc.Save(ctx, "other", "invoice1", "CUF", "xml", strings.NewReader("signed"), 6); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign save: %v", err)
	}
	if _, _, err := svc.Open(ctx, "other", "invoice1", "xml"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign read: %v", err)
	}
	repo.failCreate = true
	if _, err := svc.Save(ctx, "company1", "invoice1", "CUF", "xml", strings.NewReader("signed"), 6); err == nil {
		t.Fatal("expected metadata failure")
	}
	key, _ := domain.InvoiceObjectKey("company1", "invoice1", "CUF", "xml")
	if _, err := object.Stat(ctx, key); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("orphan after DB failure: %v", err)
	}
	repo.failCreate = false
	file, err := svc.Save(ctx, "company1", "invoice1", "CUF", "xml", strings.NewReader("signed"), 6)
	if err != nil {
		t.Fatal(err)
	}
	if file.SHA256 == "" || file.Size != 6 {
		t.Fatalf("metadata: %+v", file)
	}
	body, _, err := svc.Open(ctx, "company1", "invoice1", "xml")
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(body)
	body.Close()
	if err != nil || string(got) != "signed" {
		t.Fatalf("read: %q %v", got, err)
	}
	if _, err := svc.Save(ctx, "company1", "invoice1", "CUF", "xml", strings.NewReader("signed"), 6); err != nil {
		t.Fatalf("idempotent save: %v", err)
	}
	if _, err := svc.Save(ctx, "company1", "invoice1", "CUF", "xml", strings.NewReader("other"), 5); !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("changed fiscal file: %v", err)
	}
}

func TestInvoiceFileServiceReconcilesExistingObjectAfterReaderWasConsumed(t *testing.T) {
	ctx := context.Background()
	objects := &consumeOnConflictStorage{inner: storage.NewMemoryObjectStorage()}
	repo := &fileRepoFake{rows: map[string]*domain.InvoiceFile{}}
	svc := usecase.NewInvoiceFileService(objects, repo, time.Minute)

	key, err := domain.InvoiceObjectKey("company1", "invoice1", "CUF", "xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := objects.inner.Put(ctx, key, strings.NewReader("signed"), domain.PutOptions{ContentType: "application/xml", Size: 6}); err != nil {
		t.Fatal(err)
	}
	file, err := svc.Save(ctx, "company1", "invoice1", "CUF", "xml", strings.NewReader("signed"), 6)
	if err != nil {
		t.Fatalf("reconciliar objeto huérfano: %v", err)
	}
	if file == nil || repo.rows["xml"] == nil || file.SHA256 != repo.rows["xml"].SHA256 {
		t.Fatalf("metadata no reconciliada: file=%+v rows=%+v", file, repo.rows)
	}
	if _, err := svc.Save(ctx, "company1", "invoice1", "CUF", "xml", strings.NewReader("other!"), 6); !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("contenido distinto no fue rechazado: %v", err)
	}
}
