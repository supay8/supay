package postgres

import (
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"gorm.io/gorm"
)

type PostgresContingencyEventRepository struct {
	db *gorm.DB
}

func NewPostgresContingencyEventRepository(db *gorm.DB) domain.ContingencyEventRepository {
	return &PostgresContingencyEventRepository{db: db}
}

func (r *PostgresContingencyEventRepository) Create(e *domain.ContingencyEvent) error {
	m := models.ContingencyEvent{
		PointOfSaleId: e.PointOfSaleID,
		Reason:        models.ContingencyReason(e.Reason),
		Description:   e.Description,
		StartDate:     e.StartDate,
		EndDate:       e.EndDate,
		SiatEventCode: e.SiatEventCode,
		IsSynced:      e.IsSynced,
	}
	if err := r.db.Create(&m).Error; err != nil {
		return err
	}
	e.ID = m.ID
	e.CreatedAt = m.CreatedAt
	return nil
}

// GetLatestByPointOfSale devuelve el evento significativo más reciente (por
// fecha de inicio) ya sincronizado con el SIAT para el punto de venta.
func (r *PostgresContingencyEventRepository) GetLatestByPointOfSale(pointOfSaleID string) (*domain.ContingencyEvent, error) {
	var m models.ContingencyEvent
	if err := r.db.Where("point_of_sale_id = ? AND is_synced = true AND siat_event_code IS NOT NULL", pointOfSaleID).
		Order("start_date DESC, created_at DESC").First(&m).Error; err != nil {
		return nil, err
	}
	return &domain.ContingencyEvent{
		ID:            m.ID,
		PointOfSaleID: m.PointOfSaleId,
		Reason:        string(m.Reason),
		Description:   m.Description,
		StartDate:     m.StartDate,
		EndDate:       m.EndDate,
		SiatEventCode: m.SiatEventCode,
		IsSynced:      m.IsSynced,
		CreatedAt:     m.CreatedAt,
	}, nil
}
