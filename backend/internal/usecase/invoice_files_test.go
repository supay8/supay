package usecase_test

import (
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
