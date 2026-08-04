package postgres

import (
	"strconv"
	"strings"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostgresPointOfSaleRepository struct {
	db *gorm.DB
}

func NewPostgresPointOfSaleRepository(db *gorm.DB) domain.PointOfSaleRepository {
	return &PostgresPointOfSaleRepository{db: db}
}

func (r *PostgresPointOfSaleRepository) Create(pos *domain.PointOfSale) error {
	// Serializa la creación por (empresa, sucursal) con un advisory lock de
	// PostgreSQL para que el cálculo de codigoPuntoVenta (MAX+1) y el insert
	// sean atómicos. El índice único compuesto es la red de seguridad final.
	return r.db.Transaction(func(tx *gorm.DB) error {
		lockKey := pos.CompanyId + "|" + strconv.Itoa(pos.CodigoSucursal)
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", lockKey).Error; err != nil {
			return err
		}

		var next int
		if err := tx.Model(&models.PointOfSale{}).
			Where("company_id = ? AND codigo_sucursal = ?", pos.CompanyId, pos.CodigoSucursal).
			Select("COALESCE(MAX(codigo_punto_venta), 0) + 1").
			Scan(&next).Error; err != nil {
			return err
		}
		pos.CodigoPuntoVenta = next

		dbModel := models.PointOfSale{
			ID:               uuid.NewString(),
			CompanyId:        pos.CompanyId,
			CodigoSucursal:   pos.CodigoSucursal,
			CodigoPuntoVenta: pos.CodigoPuntoVenta,
			Description:      pos.Description,
			Cuis:             pos.Cuis,
			CuisCreatedAt:    pos.CuisCreatedAt,
			IsActive:         pos.IsActive,
		}

		if err := tx.Create(&dbModel).Error; err != nil {
			if isUniqueViolation(err) {
				return domain.ErrPointOfSaleCodeConflict
			}
			return err
		}

		pos.ID = dbModel.ID
		pos.CreatedAt = dbModel.CreatedAt
		return nil
	})
}

func (r *PostgresPointOfSaleRepository) GetByID(id string) (*domain.PointOfSale, error) {
	var dbModel models.PointOfSale
	if err := r.db.Where("id = ?", id).First(&dbModel).Error; err != nil {
		return nil, err
	}

	return toDomainPointOfSale(&dbModel), nil
}

func (r *PostgresPointOfSaleRepository) List(companyID string) ([]*domain.PointOfSale, error) {
	var dbModels []models.PointOfSale
	query := r.db.Order("created_at ASC")
	if companyID != "" {
		query = query.Where("company_id = ?", companyID)
	}

	if err := query.Find(&dbModels).Error; err != nil {
		return nil, err
	}

	pointsOfSale := make([]*domain.PointOfSale, 0, len(dbModels))
	for i := range dbModels {
		pointsOfSale = append(pointsOfSale, toDomainPointOfSale(&dbModels[i]))
	}
	return pointsOfSale, nil
}

func (r *PostgresPointOfSaleRepository) Update(pos *domain.PointOfSale) error {
	var dbModel models.PointOfSale
	if err := r.db.Where("id = ?", pos.ID).First(&dbModel).Error; err != nil {
		return err
	}

	dbModel.CompanyId = pos.CompanyId
	dbModel.CodigoSucursal = pos.CodigoSucursal
	dbModel.CodigoPuntoVenta = pos.CodigoPuntoVenta
	dbModel.Description = pos.Description
	dbModel.Cuis = pos.Cuis
	dbModel.CuisCreatedAt = pos.CuisCreatedAt
	dbModel.IsActive = pos.IsActive

	if err := r.db.Save(&dbModel).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ErrPointOfSaleCodeConflict
		}
		return err
	}
	return nil
}

func (r *PostgresPointOfSaleRepository) Delete(id string) error {
	if err := r.db.Where("id = ?", id).Delete(&models.PointOfSale{}).Error; err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrPointOfSaleHasDependencies
		}
		return err
	}
	return nil
}

func toDomainPointOfSale(dbModel *models.PointOfSale) *domain.PointOfSale {
	return &domain.PointOfSale{
		ID:               dbModel.ID,
		CompanyId:        dbModel.CompanyId,
		CodigoSucursal:   dbModel.CodigoSucursal,
		CodigoPuntoVenta: dbModel.CodigoPuntoVenta,
		Description:      dbModel.Description,
		Cuis:             dbModel.Cuis,
		CuisCreatedAt:    dbModel.CuisCreatedAt,
		IsActive:         dbModel.IsActive,
		CreatedAt:        dbModel.CreatedAt,
	}
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}

func isForeignKeyViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23503")
}
