package postgres

import (
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostgresTipoPuntoVentaRepository struct {
	db *gorm.DB
}

func NewPostgresTipoPuntoVentaRepository(db *gorm.DB) domain.TipoPuntoVentaRepository {
	return &PostgresTipoPuntoVentaRepository{db: db}
}

func (r *PostgresTipoPuntoVentaRepository) Replace(companyID string, tipos []domain.TipoPuntoVenta, syncedAt time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("company_id = ?", companyID).Delete(&models.TipoPuntoVenta{}).Error; err != nil {
			return err
		}
		if len(tipos) == 0 {
			return nil
		}
		rows := make([]models.TipoPuntoVenta, 0, len(tipos))
		for _, t := range tipos {
			rows = append(rows, models.TipoPuntoVenta{
				ID:                 uuid.NewString(),
				CompanyId:          companyID,
				CodigoClasificador: t.CodigoClasificador,
				Descripcion:        t.Descripcion,
				SyncedAt:           syncedAt,
			})
		}
		return tx.Create(&rows).Error
	})
}

func (r *PostgresTipoPuntoVentaRepository) List(companyID string) ([]*domain.TipoPuntoVenta, error) {
	var rows []models.TipoPuntoVenta
	if err := r.db.Where("company_id = ?", companyID).
		Order("codigo_clasificador ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]*domain.TipoPuntoVenta, 0, len(rows))
	for i := range rows {
		result = append(result, toDomainTipoPuntoVenta(&rows[i]))
	}
	return result, nil
}

func (r *PostgresTipoPuntoVentaRepository) FindByClasificador(companyID string, codigoClasificador int) (*domain.TipoPuntoVenta, error) {
	var row models.TipoPuntoVenta
	if err := r.db.Where("company_id = ? AND codigo_clasificador = ?", companyID, codigoClasificador).
		First(&row).Error; err != nil {
		return nil, err
	}
	return toDomainTipoPuntoVenta(&row), nil
}

func toDomainTipoPuntoVenta(m *models.TipoPuntoVenta) *domain.TipoPuntoVenta {
	return &domain.TipoPuntoVenta{
		ID:                 m.ID,
		CompanyID:          m.CompanyId,
		CodigoClasificador: m.CodigoClasificador,
		Descripcion:        m.Descripcion,
		SyncedAt:           m.SyncedAt,
		CreatedAt:          m.CreatedAt,
	}
}
