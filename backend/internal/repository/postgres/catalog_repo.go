package postgres

import (
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostgresCatalogRepository struct {
	db *gorm.DB
}

func NewPostgresCatalogRepository(db *gorm.DB) domain.CatalogRepository {
	return &PostgresCatalogRepository{db: db}
}

func (r *PostgresCatalogRepository) Replace(companyID, tipo string, items []domain.CatalogItem, syncedAt time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("company_id = ? AND tipo = ?", companyID, tipo).Delete(&models.Catalog{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		rows := make([]models.Catalog, 0, len(items))
		for _, it := range items {
			rows = append(rows, models.Catalog{
				ID:          uuid.NewString(),
				CompanyId:   companyID,
				Tipo:        tipo,
				Codigo:      it.Codigo,
				Descripcion: it.Descripcion,
				SyncedAt:    syncedAt,
			})
		}
		return tx.Create(&rows).Error
	})
}

func (r *PostgresCatalogRepository) List(companyID, tipo string) ([]*domain.CatalogItem, error) {
	var rows []models.Catalog
	if err := r.db.Where("company_id = ? AND tipo = ?", companyID, tipo).
		Order("codigo ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]*domain.CatalogItem, 0, len(rows))
	for i := range rows {
		result = append(result, &domain.CatalogItem{
			Codigo:      rows[i].Codigo,
			Descripcion: rows[i].Descripcion,
			Tipo:        tipo,
		})
	}
	return result, nil
}

func (r *PostgresCatalogRepository) ListAll(companyID string) (map[string][]*domain.CatalogItem, error) {
	var rows []models.Catalog
	if err := r.db.Where("company_id = ?", companyID).
		Order("tipo ASC, codigo ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string][]*domain.CatalogItem)
	for i := range rows {
		tipo := rows[i].Tipo
		result[tipo] = append(result[tipo], &domain.CatalogItem{
			Codigo:      rows[i].Codigo,
			Descripcion: rows[i].Descripcion,
			Tipo:        tipo,
		})
	}
	return result, nil
}
