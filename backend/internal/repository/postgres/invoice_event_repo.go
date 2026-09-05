package postgres

import (
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type PostgresInvoiceEventRepository struct {
	db *gorm.DB
}

func NewPostgresInvoiceEventRepository(db *gorm.DB) domain.InvoiceEventRepository {
	return &PostgresInvoiceEventRepository{db: db}
}

func (r *PostgresInvoiceEventRepository) Create(event *domain.InvoiceEvent) error {
	model := models.InvoiceEvent{
		ID: event.ID, InvoiceId: event.InvoiceID, TenantID: event.TenantID,
		EventKey: event.EventKey, Type: event.Type, Message: event.Message,
		Payload: datatypes.JSON(event.Payload),
	}
	if model.ID == "" {
		model.ID = uuid.NewString()
	}
	if err := r.db.Create(&model).Error; err != nil {
		return err
	}
	event.ID = model.ID
	event.CreatedAt = model.CreatedAt
	return nil
}

func (r *PostgresInvoiceEventRepository) List(invoiceID string) ([]*domain.InvoiceEvent, error) {
	var rows []models.InvoiceEvent
	if err := r.db.Where("invoice_id = ?", invoiceID).
		Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]*domain.InvoiceEvent, 0, len(rows))
	for i := range rows {
		row := rows[i]
		result = append(result, &domain.InvoiceEvent{
			ID: row.ID, InvoiceID: row.InvoiceId, TenantID: row.TenantID,
			EventKey: row.EventKey, Type: row.Type, Message: row.Message,
			Payload: []byte(row.Payload), CreatedAt: row.CreatedAt,
		})
	}
	return result, nil
}
