package postgres

import (
	"strings"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostgresProductRepository struct {
	db *gorm.DB
}

func NewPostgresProductRepository(db *gorm.DB) domain.ProductRepository {
	return &PostgresProductRepository{db: db}
}

func (r *PostgresProductRepository) Create(product *domain.Product) error {
	m := models.Product{
		ID: product.ID, CompanyId: product.CompanyID, SKU: strings.TrimSpace(product.SKU),
		Name: product.Name, Active: product.Active,
	}
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	if err := r.db.Create(&m).Error; err != nil {
		return err
	}
	product.ID = m.ID
	product.CreatedAt = m.CreatedAt
	product.UpdatedAt = m.UpdatedAt
	return nil
}

func (r *PostgresProductRepository) GetByID(companyID, id string) (*domain.Product, error) {
	var m models.Product
	if err := r.db.Preload("Mappings").First(&m, "id = ? AND company_id = ? AND active = true", id, companyID).Error; err != nil {
		return nil, err
	}
	return toDomainProduct(&m), nil
}

func (r *PostgresProductRepository) GetBySKU(companyID, sku string) (*domain.Product, error) {
	var m models.Product
	if err := r.db.Preload("Mappings").First(&m, "company_id = ? AND sku = ? AND active = true", companyID, strings.TrimSpace(sku)).Error; err != nil {
		return nil, err
	}
	return toDomainProduct(&m), nil
}

func (r *PostgresProductRepository) List(companyID string) ([]*domain.Product, error) {
	var rows []models.Product
	if err := r.db.Preload("Mappings").Where("company_id = ? AND active = true", companyID).Order("sku ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*domain.Product, 0, len(rows))
	for i := range rows {
		out = append(out, toDomainProduct(&rows[i]))
	}
	return out, nil
}

func (r *PostgresProductRepository) UpsertMapping(companyID string, mapping domain.ProductMapping) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var product models.Product
		if err := tx.First(&product, "id = ? AND company_id = ? AND active = true", mapping.ProductID, companyID).Error; err != nil {
			return err
		}
		if mapping.IsDefault {
			if err := tx.Model(&models.ProductMapping{}).Where("product_id = ?", product.ID).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		m := models.ProductMapping{
			ID: mapping.ID, ProductId: product.ID, CodigoProductoSin: mapping.CodigoProductoSin,
			SinProductId:    mapping.SinProductID,
			CodigoActividad: mapping.CodigoActividad, CodigoDocumentoSector: mapping.CodigoDocumentoSector,
			UnidadMedida: mapping.UnidadMedida, IsDefault: mapping.IsDefault, Active: true,
			SyncedAt: mapping.SyncedAt,
		}
		if m.ID == "" {
			m.ID = uuid.NewString()
		}
		return tx.Where("id = ?", m.ID).Assign(m).FirstOrCreate(&m).Error
	})
}

func toDomainProduct(m *models.Product) *domain.Product {
	out := &domain.Product{ID: m.ID, CompanyID: m.CompanyId, SKU: m.SKU, Name: m.Name, Active: m.Active, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
	out.Mappings = make([]domain.ProductMapping, 0, len(m.Mappings))
	for _, mapping := range m.Mappings {
		out.Mappings = append(out.Mappings, domain.ProductMapping{ID: mapping.ID, ProductID: mapping.ProductId, SinProductID: mapping.SinProductId, CodigoProductoSin: mapping.CodigoProductoSin, CodigoActividad: mapping.CodigoActividad, CodigoDocumentoSector: mapping.CodigoDocumentoSector, UnidadMedida: mapping.UnidadMedida, IsDefault: mapping.IsDefault, Active: mapping.Active, SyncedAt: mapping.SyncedAt})
	}
	return out
}
