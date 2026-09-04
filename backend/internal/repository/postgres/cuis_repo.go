package postgres

import (
	"time"

	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"gorm.io/gorm"
)

type PostgresCuisRepository struct{ db *gorm.DB }

func NewPostgresCuisRepository(db *gorm.DB) domain.CuisRepository {
	return &PostgresCuisRepository{db: db}
}

func (r *PostgresCuisRepository) Create(c *domain.Cuis) error {
	var tenantID string
	if err := r.db.Model(&models.PointOfSale{}).
		Select("tenant_id").Where("id = ?", c.PointOfSaleID).Scan(&tenantID).Error; err != nil {
		return err
	}
	if tenantID == "" {
		return gorm.ErrRecordNotFound
	}
	var validTo *time.Time
	if !c.ValidTo.IsZero() {
		validTo = &c.ValidTo
	}
	row := models.Cuis{
		TenantId: tenantID, PointOfSaleId: c.PointOfSaleID, Cuis: c.Cuis,
		ValidFrom: c.ValidFrom, ValidTo: validTo, Active: c.Active,
	}
	if err := r.db.Transaction(func(tx *gorm.DB) error {
		if row.Active {
			if err := tx.Model(&models.Cuis{}).
				Where("tenant_id = ? AND point_of_sale_id = ? AND is_active = true", tenantID, c.PointOfSaleID).
				Updates(map[string]any{"is_active": false, "valid_to": c.ValidFrom}).Error; err != nil {
				return err
			}
		}
		return tx.Create(&row).Error
	}); err != nil {
		return err
	}
	c.ID, c.CreatedAt = row.ID, row.CreatedAt
	return nil
}

func (r *PostgresCuisRepository) GetActiveByPos(pointOfSaleID string) (*domain.Cuis, error) {
	var row models.Cuis
	now := time.Now().In(siat.LaPaz)
	if err := r.db.Where("point_of_sale_id = ? AND valid_from <= ? AND (valid_to IS NULL OR valid_to >= ?) AND is_active = true", pointOfSaleID, now, now).
		Order("created_at DESC").First(&row).Error; err != nil {
		return nil, err
	}
	return toDomainCuis(&row), nil
}

func (r *PostgresCuisRepository) DeactivateExpired() error {
	return r.db.Model(&models.Cuis{}).
		Where("valid_to < ? AND is_active = true", time.Now().In(siat.LaPaz)).
		Update("is_active", false).Error
}

func toDomainCuis(row *models.Cuis) *domain.Cuis {
	result := &domain.Cuis{
		ID: row.ID, PointOfSaleID: row.PointOfSaleId, Cuis: row.Cuis,
		ValidFrom: row.ValidFrom, Active: row.Active, CreatedAt: row.CreatedAt,
	}
	if row.ValidTo != nil {
		result.ValidTo = *row.ValidTo
	}
	return result
}
