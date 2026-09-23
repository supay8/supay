package domain

import (
	"context"
	"time"
)

type InvoiceFile struct {
	ID          string
	CompanyID   string
	InvoiceID   string
	Kind        string
	StorageKey  string
	SHA256      string
	Size        int64
	ContentType string
	CreatedAt   time.Time
}

type InvoiceFileRepository interface {
	BelongsToCompany(ctx context.Context, companyID, invoiceID string) (bool, error)
	CreateFile(ctx context.Context, file *InvoiceFile) error
	FindFile(ctx context.Context, companyID, invoiceID, kind string) (*InvoiceFile, error)
	DeleteFile(ctx context.Context, companyID, invoiceID, kind, storageKey string) error
}
