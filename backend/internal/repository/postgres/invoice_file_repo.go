package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type invoiceFileRow struct {
	ID          string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyID   string `gorm:"column:tenant_id;type:uuid;not null"`
	InvoiceID   string `gorm:"type:uuid;not null"`
	Kind        string `gorm:"not null"`
	StorageKey  string `gorm:"not null"`
	SHA256      string `gorm:"not null"`
	Size        int64  `gorm:"not null"`
	ContentType string `gorm:"not null"`
	CreatedAt   time.Time
}

func (invoiceFileRow) TableName() string { return "invoice_files" }

type PostgresInvoiceFileRepository struct {
	db           *gorm.DB
	tenantColumn string
}

func NewPostgresInvoiceFileRepository(db *gorm.DB) *PostgresInvoiceFileRepository {
	return &PostgresInvoiceFileRepository{db: db, tenantColumn: "tenant_id"}
}

// NewLegacyPostgresInvoiceFileRepository is only for the pre-000017 XML backfill.
func NewLegacyPostgresInvoiceFileRepository(db *gorm.DB) *PostgresInvoiceFileRepository {
	return &PostgresInvoiceFileRepository{db: db, tenantColumn: "company_id"}
}

func (r *PostgresInvoiceFileRepository) BelongsToCompany(ctx context.Context, companyID, invoiceID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("invoices").Where("id = ? AND tenant_id = ?", invoiceID, companyID).Count(&count).Error
	return count == 1, repositoryError(err)
}

func (r *PostgresInvoiceFileRepository) CreateFile(ctx context.Context, file *domain.InvoiceFile) error {
	id := uuid.NewString()
	createdAt := file.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	row := map[string]any{"id": id, r.tenantColumn: file.CompanyID, "invoice_id": file.InvoiceID,
		"kind": file.Kind, "storage_key": file.StorageKey, "sha256": file.SHA256,
		"size": file.Size, "content_type": file.ContentType, "created_at": createdAt}
	if err := r.db.WithContext(ctx).Table("invoice_files").Create(row).Error; err != nil {
		return repositoryError(err)
	}
	file.ID, file.CreatedAt = id, createdAt
	return nil
}

func (r *PostgresInvoiceFileRepository) FindFile(ctx context.Context, companyID, invoiceID, kind string) (*domain.InvoiceFile, error) {
	var row invoiceFileRow
	err := r.db.WithContext(ctx).Select("invoice_files.*, "+r.tenantColumn+" AS tenant_id").
		Where(r.tenantColumn+" = ? AND invoice_id = ? AND kind = ?", companyID, invoiceID, kind).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, repositoryError(domain.ErrNotFound)
	}
	if err != nil {
		return nil, repositoryError(err)
	}
	return &domain.InvoiceFile{ID: row.ID, CompanyID: row.CompanyID, InvoiceID: row.InvoiceID, Kind: row.Kind, StorageKey: row.StorageKey, SHA256: row.SHA256, Size: row.Size, ContentType: row.ContentType, CreatedAt: row.CreatedAt}, nil
}

func (r *PostgresInvoiceFileRepository) DeleteFile(ctx context.Context, companyID, invoiceID, kind, storageKey string) error {
	result := r.db.WithContext(ctx).
		Where(r.tenantColumn+" = ? AND invoice_id = ? AND kind = ? AND storage_key = ?", companyID, invoiceID, kind, storageKey).
		Delete(&invoiceFileRow{})
	return repositoryError(result.Error)
}

var _ ports.InvoiceFileRepository = (*PostgresInvoiceFileRepository)(nil)
