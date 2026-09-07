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
	var tenantID string
	if err := r.db.Model(&models.PointOfSale{}).
		Select("tenant_id").Where("id = ?", e.PointOfSaleID).Scan(&tenantID).Error; err != nil {
		return err
	}
	if tenantID == "" {
		return gorm.ErrRecordNotFound
	}
	m := models.ContingencyEvent{
		TenantId:      tenantID,
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

func (r *PostgresContingencyEventRepository) Update(e *domain.ContingencyEvent) error {
	if e == nil || e.ID == "" {
		return gorm.ErrRecordNotFound
	}
	result := r.db.Model(&models.ContingencyEvent{}).
		Where("id = ? AND point_of_sale_id = ?", e.ID, e.PointOfSaleID).
		Updates(map[string]any{
			"reason":          models.ContingencyReason(e.Reason),
			"description":     e.Description,
			"start_date":      e.StartDate,
			"end_date":        e.EndDate,
			"siat_event_code": e.SiatEventCode,
			"is_synced":       e.IsSynced,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// GetLatestByPointOfSale devuelve el evento significativo más reciente (por
// fecha de inicio) para el punto de venta, incluyendo uno local aún no
// sincronizado que esté agrupando facturas offline.
func (r *PostgresContingencyEventRepository) GetLatestByPointOfSale(pointOfSaleID string) (*domain.ContingencyEvent, error) {
	var m models.ContingencyEvent
	if err := r.db.Where("point_of_sale_id = ?", pointOfSaleID).
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

func (r *PostgresContingencyEventRepository) GetBySiatCode(siatCode string) (*domain.ContingencyEvent, error) {
	var m models.ContingencyEvent
	if err := r.db.Where("siat_event_code = ?", siatCode).First(&m).Error; err != nil {
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
