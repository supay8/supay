package postgres

import (
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostgresSentPackageRepository struct {
	db *gorm.DB
}

func NewPostgresSentPackageRepository(db *gorm.DB) domain.SentPackageRepository {
	return &PostgresSentPackageRepository{db: db}
}

func (r *PostgresSentPackageRepository) Create(pkg *domain.SentPackage) error {
	dbModel := models.SentPackage{
		ID:                    uuid.NewString(),
		CompanyId:             pkg.CompanyId,
		PointOfSaleId:         pkg.PointOfSaleId,
		Type:                  string(pkg.Type),
		CodigoRecepcion:       pkg.CodigoRecepcion,
		HashArchivo:           pkg.HashArchivo,
		CantidadFacturas:      pkg.CantidadFacturas,
		CodigoDocumentoSector: pkg.CodigoDocumentoSector,
		CodigoTipoFactura:     pkg.CodigoTipoFactura,
		CodigoEmision:         pkg.CodigoEmision,
		CodigoEvento:          pkg.CodigoEvento,
		Status:                string(pkg.Status),
		Mensajes:              pkg.Mensajes,
		XmlHash:               pkg.XmlHash,
		SentAt:                pkg.SentAt,
	}

	if err := r.db.Create(&dbModel).Error; err != nil {
		return err
	}

	pkg.ID = dbModel.ID
	pkg.CreatedAt = dbModel.CreatedAt
	return nil
}

func (r *PostgresSentPackageRepository) GetByID(id string) (*domain.SentPackage, error) {
	var dbModel models.SentPackage
	if err := r.db.Where("id = ?", id).First(&dbModel).Error; err != nil {
		return nil, err
	}
	return toDomainSentPackage(&dbModel), nil
}

func (r *PostgresSentPackageRepository) GetByCodigoRecepcion(codigoRecepcion string) (*domain.SentPackage, error) {
	var dbModel models.SentPackage
	if err := r.db.Where("codigo_recepcion = ?", codigoRecepcion).First(&dbModel).Error; err != nil {
		return nil, err
	}
	return toDomainSentPackage(&dbModel), nil
}

func (r *PostgresSentPackageRepository) ListByPointOfSale(pointOfSaleID string) ([]*domain.SentPackage, error) {
	var dbModels []models.SentPackage
	if err := r.db.Where("point_of_sale_id = ?", pointOfSaleID).
		Order("created_at DESC").
		Find(&dbModels).Error; err != nil {
		return nil, err
	}
	return toDomainSentPackages(dbModels), nil
}

func (r *PostgresSentPackageRepository) ListByCompany(companyID string) ([]*domain.SentPackage, error) {
	var dbModels []models.SentPackage
	if err := r.db.Where("company_id = ?", companyID).
		Order("created_at DESC").
		Find(&dbModels).Error; err != nil {
		return nil, err
	}
	return toDomainSentPackages(dbModels), nil
}

func (r *PostgresSentPackageRepository) Update(pkg *domain.SentPackage) error {
	var dbModel models.SentPackage
	if err := r.db.Where("id = ?", pkg.ID).First(&dbModel).Error; err != nil {
		return err
	}

	dbModel.Status = string(pkg.Status)
	dbModel.Mensajes = pkg.Mensajes
	dbModel.ValidatedAt = pkg.ValidatedAt

	return r.db.Save(&dbModel).Error
}

func toDomainSentPackage(dbModel *models.SentPackage) *domain.SentPackage {
	return &domain.SentPackage{
		ID:                    dbModel.ID,
		CompanyId:             dbModel.CompanyId,
		PointOfSaleId:         dbModel.PointOfSaleId,
		Type:                  domain.SentPackageType(dbModel.Type),
		CodigoRecepcion:       dbModel.CodigoRecepcion,
		HashArchivo:           dbModel.HashArchivo,
		CantidadFacturas:      dbModel.CantidadFacturas,
		CodigoDocumentoSector: dbModel.CodigoDocumentoSector,
		CodigoTipoFactura:     dbModel.CodigoTipoFactura,
		CodigoEmision:         dbModel.CodigoEmision,
		CodigoEvento:          dbModel.CodigoEvento,
		Status:                domain.SentPackageStatus(dbModel.Status),
		Mensajes:              dbModel.Mensajes,
		XmlHash:               dbModel.XmlHash,
		SentAt:                dbModel.SentAt,
		ValidatedAt:           dbModel.ValidatedAt,
		CreatedAt:             dbModel.CreatedAt,
	}
}

func toDomainSentPackages(dbModels []models.SentPackage) []*domain.SentPackage {
	out := make([]*domain.SentPackage, 0, len(dbModels))
	for i := range dbModels {
		out = append(out, toDomainSentPackage(&dbModels[i]))
	}
	return out
}
