package postgres

import (
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostgresInvoiceDocumentRepository struct {
	db *gorm.DB
}

func NewPostgresInvoiceDocumentRepository(db *gorm.DB) domain.InvoiceDocumentRepository {
	return &PostgresInvoiceDocumentRepository{db: db}
}

func (r *PostgresInvoiceDocumentRepository) Create(document *domain.InvoiceDocument) error {
	if err := r.db.Transaction(func(tx *gorm.DB) error {
		model, err := rotateInvoiceDocument(tx, document)
		if err != nil {
			return err
		}
		document.ID = model.ID
		document.Version = model.Version
		document.CreatedAt = model.CreatedAt
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func rotateInvoiceDocument(tx *gorm.DB, document *domain.InvoiceDocument) (models.InvoiceDocument, error) {
	var invoice models.Invoice
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", document.InvoiceID).First(&invoice).Error; err != nil {
		return models.InvoiceDocument{}, err
	}
	var currentVersion int
	if err := tx.Model(&models.InvoiceDocument{}).
		Where("invoice_id = ? AND document_type = ?", document.InvoiceID, string(document.DocumentType)).
		Select("COALESCE(MAX(version), 0)").Scan(&currentVersion).Error; err != nil {
		return models.InvoiceDocument{}, err
	}
	if document.Version <= currentVersion {
		document.Version = currentVersion + 1
	}
	if document.IsCurrent {
		if err := tx.Model(&models.InvoiceDocument{}).
			Where("invoice_id = ? AND document_type = ? AND is_current = true", document.InvoiceID, string(document.DocumentType)).
			Update("is_current", false).Error; err != nil {
			return models.InvoiceDocument{}, err
		}
	}
	model := models.InvoiceDocument{
		ID: document.ID, InvoiceID: document.InvoiceID,
		DocumentType: string(document.DocumentType), Version: document.Version,
		Content: document.Content, StorageRef: document.StorageRef,
		MIMEType: document.MIMEType, SHA256: document.SHA256,
		IsCurrent: document.IsCurrent,
	}
	if model.ID == "" {
		model.ID = uuid.NewString()
	}
	if err := tx.Create(&model).Error; err != nil {
		return models.InvoiceDocument{}, err
	}
	return model, nil
}

func (r *PostgresInvoiceDocumentRepository) List(invoiceID string) ([]*domain.InvoiceDocument, error) {
	var rows []models.InvoiceDocument
	if err := r.db.Where("invoice_id = ?", invoiceID).
		Order("document_type ASC, version DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return toDomainInvoiceDocuments(rows), nil
}

func (r *PostgresInvoiceDocumentRepository) ListCurrent(invoiceID string) ([]*domain.InvoiceDocument, error) {
	var rows []models.InvoiceDocument
	if err := r.db.Where("invoice_id = ? AND is_current = true", invoiceID).
		Order("document_type ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return toDomainInvoiceDocuments(rows), nil
}

func toDomainInvoiceDocuments(rows []models.InvoiceDocument) []*domain.InvoiceDocument {
	result := make([]*domain.InvoiceDocument, 0, len(rows))
	for i := range rows {
		row := rows[i]
		result = append(result, &domain.InvoiceDocument{
			ID: row.ID, InvoiceID: row.InvoiceID,
			DocumentType: domain.InvoiceDocumentType(row.DocumentType), Version: row.Version,
			Content: row.Content, StorageRef: row.StorageRef, MIMEType: row.MIMEType,
			SHA256: row.SHA256, IsCurrent: row.IsCurrent, CreatedAt: row.CreatedAt,
		})
	}
	return result
}
