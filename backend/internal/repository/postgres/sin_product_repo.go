package postgres

import (
	"fmt"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostgresSinProductRepository struct{ db *gorm.DB }

func NewPostgresSinProductRepository(db *gorm.DB) domain.SinProductRepository {
	return &PostgresSinProductRepository{db: db}
}

func (r *PostgresSinProductRepository) Replace(companyID string, products []domain.SinProduct, syncedAt time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ?", companyID).Delete(&models.SinProduct{}).Error; err != nil {
			return err
		}
		if len(products) == 0 {
			return nil
		}
		rows := make([]models.SinProduct, 0, len(products))
		seen := make(map[[2]int64]bool, len(products))
		for _, product := range products {
			if product.CodigoProductoSin <= 0 || strings.TrimSpace(product.Descripcion) == "" {
				return fmt.Errorf("producto SIN inválido: código=%d", product.CodigoProductoSin)
			}
			// El SIAT puede devolver el mismo par actividad-producto más de una
			// vez en la misma respuesta; se conserva la primera ocurrencia para
			// no violar la unicidad (company_id, codigo_actividad, codigo).
			key := [2]int64{product.CodigoActividad, product.CodigoProductoSin}
			if seen[key] {
				continue
			}
			seen[key] = true
			rows = append(rows, models.SinProduct{ID: uuid.NewString(), CompanyId: companyID, CodigoProductoSin: product.CodigoProductoSin, CodigoActividad: product.CodigoActividad, Descripcion: strings.TrimSpace(product.Descripcion), Active: true, SyncedAt: syncedAt})
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

func (r *PostgresSinProductRepository) List(companyID, query string, codigoActividad int64, limit, offset int) ([]*domain.SinProduct, int64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	base := r.db.Model(&models.SinProduct{}).Where("tenant_id = ? AND is_active = true", companyID)
	if codigoActividad > 0 {
		base = base.Where("codigo_actividad = ?", codigoActividad)
	}
	term := strings.TrimSpace(query)
	if term != "" {
		base = base.Where("CAST(codigo_producto_sin AS TEXT) ILIKE ? OR descripcion ILIKE ?", "%"+term+"%", "%"+term+"%")
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.SinProduct
	if err := base.Order("codigo_producto_sin ASC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*domain.SinProduct, 0, len(rows))
	for i := range rows {
		out = append(out, toDomainSinProduct(&rows[i]))
	}
	return out, total, nil
}

func (r *PostgresSinProductRepository) ListAll(companyID string) ([]*domain.SinProduct, error) {
	var rows []models.SinProduct
	if err := r.db.Where("tenant_id = ? AND is_active = true", companyID).
		Order("codigo_producto_sin ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*domain.SinProduct, 0, len(rows))
	for i := range rows {
		out = append(out, toDomainSinProduct(&rows[i]))
	}
	return out, nil
}

func (r *PostgresSinProductRepository) GetByCode(companyID string, code int64) (*domain.SinProduct, error) {
	var row models.SinProduct
	if err := r.db.First(&row, "tenant_id = ? AND codigo_producto_sin = ? AND is_active = true", companyID, code).Error; err != nil {
		return nil, err
	}
	return toDomainSinProduct(&row), nil
}

func toDomainSinProduct(row *models.SinProduct) *domain.SinProduct {
	return &domain.SinProduct{ID: row.ID, CompanyID: row.CompanyId, CodigoProductoSin: row.CodigoProductoSin, CodigoActividad: row.CodigoActividad, Descripcion: row.Descripcion, Active: row.Active, SyncedAt: row.SyncedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}
