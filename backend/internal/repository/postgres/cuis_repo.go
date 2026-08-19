package postgres

import (
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/brandsrx/supay/internal/siat"
	"gorm.io/gorm"
)

type PostgresCuisRepository struct {
	db *gorm.DB
}

func NewPostgresCuisRepository(db *gorm.DB) domain.CuisRepository {
	return &PostgresCuisRepository{db: db}
}

func (r *PostgresCuisRepository) Create(c *domain.Cuis) error {
	model := models.Cuis{
		PointOfSaleId: c.PointOfSaleID,
		Cuis:          c.Cuis,
		ValidFrom:     c.ValidFrom,
		ValidTo:       c.ValidTo,
		Active:        c.Active,
	}
	if err := r.db.Create(&model).Error; err != nil {
		return err
	}
	c.ID = model.ID
	c.CreatedAt = model.CreatedAt
	return nil
}

func (r *PostgresCuisRepository) GetActiveByPos(pointOfSaleID string) (*domain.Cuis, error) {
	var m models.Cuis
	now := time.Now().In(siat.LaPaz)
	if err := r.db.Where("point_of_sale_id = ? AND valid_from <= ? AND valid_to >= ? AND active = true", pointOfSaleID, now, now).
		Order("created_at DESC").First(&m).Error; err != nil {
		return nil, err
	}
	return &domain.Cuis{
		ID:            m.ID,
		PointOfSaleID: m.PointOfSaleId,
		Cuis:          m.Cuis,
		ValidFrom:     m.ValidFrom,
		ValidTo:       m.ValidTo,
		Active:        m.Active,
		CreatedAt:     m.CreatedAt,
	}, nil
}

func (r *PostgresCuisRepository) DeactivateExpired() error {
	// mark old cuis inactive
	return r.db.Model(&models.Cuis{}).Where("valid_to < ? AND active = true", time.Now().In(siat.LaPaz)).Update("active", false).Error
}
