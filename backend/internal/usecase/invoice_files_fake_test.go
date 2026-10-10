package usecase

import (
	"context"
	"github.com/brandsrx/supay/internal/domain"
)

type fakePDFGenerator struct{ calls int }

func (f *fakePDFGenerator) GenerateAndPersist(context.Context, string) { f.calls++ }

type fakeInvoiceFileRepo struct {
	invoices *fakeInvoiceRepo
	rows     map[string]*domain.InvoiceFile
}

func (r *fakeInvoiceFileRepo) key(companyID, invoiceID, kind string) string {
	return companyID + "/" + invoiceID + "/" + kind
}

func (r *fakeInvoiceFileRepo) BelongsToCompany(_ context.Context, companyID, invoiceID string) (bool, error) {
	inv, ok := r.invoices.invoices[invoiceID]
	return ok && inv.CompanyId == companyID, nil
}

func (r *fakeInvoiceFileRepo) CreateFile(_ context.Context, file *domain.InvoiceFile) error {
	r.rows[r.key(file.CompanyID, file.InvoiceID, file.Kind)] = file
	return nil
}

func (r *fakeInvoiceFileRepo) FindFile(_ context.Context, companyID, invoiceID, kind string) (*domain.InvoiceFile, error) {
	file, ok := r.rows[r.key(companyID, invoiceID, kind)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return file, nil
}

func (r *fakeInvoiceFileRepo) DeleteFile(_ context.Context, companyID, invoiceID, kind, storageKey string) error {
	key := r.key(companyID, invoiceID, kind)
	if file, ok := r.rows[key]; ok && file.StorageKey == storageKey {
		delete(r.rows, key)
	}
	return nil
}
