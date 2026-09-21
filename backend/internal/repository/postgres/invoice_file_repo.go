package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"gorm.io/gorm"
)

type invoiceFileRow struct {
	ID          string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyID   string `gorm:"type:uuid;not null"`
	InvoiceID   string `gorm:"type:uuid;not null"`
	Kind        string `gorm:"not null"`
	StorageKey  string `gorm:"not null"`
	SHA256      string `gorm:"not null"`
	Size        int64  `gorm:"not null"`
	ContentType string `gorm:"not null"`
	CreatedAt   time.Time
}

func (invoiceFileRow) TableName() string { return "invoice_files" }

type PostgresInvoiceFileRepository struct{ db *gorm.DB }

func NewPostgresInvoiceFileRepository(db *gorm.DB) *PostgresInvoiceFileRepository {
	return &PostgresInvoiceFileRepository{db: db}
}

func (r *PostgresInvoiceFileRepository) BelongsToCompany(ctx context.Context, companyID, invoiceID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("invoices").Where("id = ? AND tenant_id = ?", invoiceID, companyID).Count(&count).Error
	return count == 1, err
}

func (r *PostgresInvoiceFileRepository) CreateFile(ctx context.Context, file *domain.InvoiceFile) error {
	row := invoiceFileRow{CompanyID: file.CompanyID, InvoiceID: file.InvoiceID, Kind: file.Kind, StorageKey: file.StorageKey, SHA256: file.SHA256, Size: file.Size, ContentType: file.ContentType, CreatedAt: file.CreatedAt}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return err
	}
	file.ID = row.ID
	return nil
}

func (r *PostgresInvoiceFileRepository) FindFile(ctx context.Context, companyID, invoiceID, kind string) (*domain.InvoiceFile, error) {
	var row invoiceFileRow
	err := r.db.WithContext(ctx).Where("company_id = ? AND invoice_id = ? AND kind = ?", companyID, invoiceID, kind).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &domain.InvoiceFile{ID: row.ID, CompanyID: row.CompanyID, InvoiceID: row.InvoiceID, Kind: row.Kind, StorageKey: row.StorageKey, SHA256: row.SHA256, Size: row.Size, ContentType: row.ContentType, CreatedAt: row.CreatedAt}, nil
}

var _ domain.InvoiceFileRepository = (*PostgresInvoiceFileRepository)(nil)
