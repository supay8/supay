package postgres

import (
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/brandsrx/supay/internal/adapters/siat"
	"gorm.io/gorm"
)

type PostgresCufdRepository struct {
	db *gorm.DB
}

func NewPostgresCufdRepository(db *gorm.DB) domain.CufdRepository {
	return &PostgresCufdRepository{db: db}
}

func (r *PostgresCufdRepository) Create(c *domain.Cufd) error {
	model := models.Cufd{
		PointOfSaleId: c.PointOfSaleID,
		Cufd:          c.Cufd,
		Direccion:     c.Direccion,
		CodigoControl: c.ControlCode,
		CodigoQR:      c.CodigoQR,
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

func (r *PostgresCufdRepository) GetActiveByPos(pointOfSaleID string) (*domain.Cufd, error) {
	var m models.Cufd
	now := time.Now().In(siat.LaPaz)
	if err := r.db.Where("point_of_sale_id = ? AND valid_from <= ? AND valid_to >= ? AND active = true", pointOfSaleID, now, now).
		Order("created_at DESC").First(&m).Error; err != nil {
		return nil, err
	}
	return toDomainCufd(&m), nil
}

// GetByPosAndWindow devuelve el CUFD (activo primero, luego histórico) cuya
// vigencia valid_from/valid_to contiene exactamente la ventana [from, to] del
// evento reportado.
func (r *PostgresCufdRepository) GetByPosAndWindow(pointOfSaleID string, from, to time.Time) (*domain.Cufd, error) {
	var m models.Cufd
	if err := r.db.Where("point_of_sale_id = ? AND valid_from <= ? AND valid_to >= ?", pointOfSaleID, from, to).
		Order("active DESC, created_at DESC").First(&m).Error; err != nil {
		return nil, err
	}
	return toDomainCufd(&m), nil
}

func (r *PostgresCufdRepository) DeactivateExpired() error {
	return r.db.Model(&models.Cufd{}).Where("valid_to < ? AND active = true", time.Now().In(siat.LaPaz)).Update("active", false).Error
}
