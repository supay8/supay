package postgres

import (
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/brandsrx/supay/internal/ports"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostgresBranchRepository struct {
	db *gorm.DB
}

func NewPostgresBranchRepository(db *gorm.DB) ports.BranchRepository {
	return &PostgresBranchRepository{db: db}
}

func (r *PostgresBranchRepository) Create(b *domain.Branch) error {
	model := models.Branch{
		ID:             uuid.NewString(),
		CompanyId:      b.CompanyID,
		CodigoSucursal: b.CodigoSucursal,
		Name:           b.Name,
		Address:        b.Address,
		Active:         b.Active,
	}
	if err := r.db.Create(&model).Error; err != nil {
		if isUniqueViolation(err) {
			return repositoryError(domain.ErrBranchSucursalConflict)
		}
		return repositoryError(err)
	}
	b.ID = model.ID
	b.CreatedAt = model.CreatedAt
	return nil
}

func (r *PostgresBranchRepository) GetByID(id string) (*domain.Branch, error) {
	var m models.Branch
	if err := r.db.Where("id = ? AND is_active = true", id).First(&m).Error; err != nil {
		return nil, repositoryError(err)
	}
	return toDomainBranch(&m), nil
}

func (r *PostgresBranchRepository) GetByCompanyAndSucursal(companyID string, codigoSucursal int) (*domain.Branch, error) {
	var m models.Branch
	if err := r.db.Where("tenant_id = ? AND codigo_sucursal = ? AND is_active = true", companyID, codigoSucursal).First(&m).Error; err != nil {
		return nil, repositoryError(err)
	}
	return toDomainBranch(&m), nil
}

func (r *PostgresBranchRepository) List(companyID string) ([]*domain.Branch, error) {
	if companyID == "" {
		return nil, repositoryError(domain.ErrMissingCompanyID)
	}
	var modelsList []models.Branch
	if err := r.db.Where("tenant_id = ? AND is_active = true", companyID).Order("created_at ASC").Find(&modelsList).Error; err != nil {
		return nil, repositoryError(err)
	}
	res := make([]*domain.Branch, 0, len(modelsList))
	for _, m := range modelsList {
		res = append(res, toDomainBranch(&m))
	}
	return res, nil
}

func (r *PostgresBranchRepository) Update(b *domain.Branch) error {
	var m models.Branch
	if err := r.db.Where("id = ?", b.ID).First(&m).Error; err != nil {
		return repositoryError(err)
	}
	m.CodigoSucursal = b.CodigoSucursal
	m.Name = b.Name
	m.Address = b.Address
	m.Active = b.Active
	if err := r.db.Save(&m).Error; err != nil {
		if isUniqueViolation(err) {
			return repositoryError(domain.ErrBranchSucursalConflict)
		}
		return repositoryError(err)
	}
	return nil
}

func (r *PostgresBranchRepository) Delete(id string) error {
	return repositoryError(r.db.Model(&models.Branch{}).Where("id = ? AND is_active = true", id).
		Update("is_active", false).Error)
}

func toDomainBranch(m *models.Branch) *domain.Branch {
	return &domain.Branch{
		ID:             m.ID,
		CompanyID:      m.CompanyId,
		CodigoSucursal: m.CodigoSucursal,
		Name:           m.Name,
		Address:        m.Address,
		Active:         m.Active,
		CreatedAt:      m.CreatedAt,
	}
}
