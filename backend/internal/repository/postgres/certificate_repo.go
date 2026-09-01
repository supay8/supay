package postgres

import (
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostgresCertificateRepository struct {
	db *gorm.DB
}

func NewPostgresCertificateRepository(db *gorm.DB) domain.CertificateRepository {
	return &PostgresCertificateRepository{db: db}
}

func (r *PostgresCertificateRepository) Create(c *domain.Certificate) error {
	dbModel := models.Certificate{
		ID:                   uuid.NewString(),
		CompanyId:            c.CompanyId,
		Name:                 c.Name,
		Type:                 c.Type,
		Status:               string(c.Status),
		NotBefore:            c.NotBefore,
		NotAfter:             c.NotAfter,
		Issuer:               c.Issuer,
		Subject:              c.Subject,
		Thumbprint:           c.Thumbprint,
		SiatUserCode:         c.SiatUserCode,
		ConfigPath:           c.ConfigPath,
		RenewedFrom:          c.RenewedFrom,
		EncryptedToken:       c.EncryptedToken,
		EncryptedP12Password: c.EncryptedP12Password,
		P12StorageRef:        c.P12StorageRef,
		Modalidad:            c.Modalidad,
		Ambiente:             c.Ambiente,
		Nit:                  c.Nit,
	}

	if err := r.db.Create(&dbModel).Error; err != nil {
		return err
	}

	c.ID = dbModel.ID
	c.CreatedAt = dbModel.CreatedAt
	c.UpdatedAt = dbModel.UpdatedAt
	return nil
}

func (r *PostgresCertificateRepository) GetByID(id string) (*domain.Certificate, error) {
	var dbModel models.Certificate
	if err := r.db.Where("id = ?", id).First(&dbModel).Error; err != nil {
		return nil, err
	}
	return toDomainCertificate(&dbModel), nil
}

func (r *PostgresCertificateRepository) GetActiveByCompany(companyID string) (*domain.Certificate, error) {
	var dbModel models.Certificate
	if err := r.db.Where("company_id = ? AND status = ?", companyID, domain.CertificateActive).
		Order("not_after DESC").
		First(&dbModel).Error; err != nil {
		return nil, err
	}
	return toDomainCertificate(&dbModel), nil
}

func (r *PostgresCertificateRepository) ListByCompany(companyID string) ([]*domain.Certificate, error) {
	var dbModels []models.Certificate
	if err := r.db.Where("company_id = ?", companyID).Order("created_at DESC").Find(&dbModels).Error; err != nil {
		return nil, err
	}
	result := make([]*domain.Certificate, len(dbModels))
	for i := range dbModels {
		result[i] = toDomainCertificate(&dbModels[i])
	}
	return result, nil
}

func (r *PostgresCertificateRepository) Update(c *domain.Certificate) error {
	dbModel := models.Certificate{
		ID:                   c.ID,
		CompanyId:            c.CompanyId,
		Name:                 c.Name,
		Type:                 c.Type,
		Status:               string(c.Status),
		NotBefore:            c.NotBefore,
		NotAfter:             c.NotAfter,
		Issuer:               c.Issuer,
		Subject:              c.Subject,
		Thumbprint:           c.Thumbprint,
		SiatUserCode:         c.SiatUserCode,
		ConfigPath:           c.ConfigPath,
		RenewedFrom:          c.RenewedFrom,
		EncryptedToken:       c.EncryptedToken,
		EncryptedP12Password: c.EncryptedP12Password,
		P12StorageRef:        c.P12StorageRef,
		Modalidad:            c.Modalidad,
		Ambiente:             c.Ambiente,
		Nit:                  c.Nit,
	}
	return r.db.Save(&dbModel).Error
}

func (r *PostgresCertificateRepository) Delete(id string) error {
	return r.db.Delete(&models.Certificate{}, "id = ?", id).Error
}

func toDomainCertificate(m *models.Certificate) *domain.Certificate {
	c := &domain.Certificate{
		ID:                   m.ID,
		CompanyId:            m.CompanyId,
		Name:                 m.Name,
		Type:                 m.Type,
		Status:               domain.CertificateStatus(m.Status),
		NotBefore:            m.NotBefore,
		NotAfter:             m.NotAfter,
		Issuer:               m.Issuer,
		Subject:              m.Subject,
		Thumbprint:           m.Thumbprint,
		SiatUserCode:         m.SiatUserCode,
		ConfigPath:           m.ConfigPath,
		RenewedFrom:          m.RenewedFrom,
		CreatedAt:            m.CreatedAt,
		UpdatedAt:            m.UpdatedAt,
		EncryptedToken:       m.EncryptedToken,
		EncryptedP12Password: m.EncryptedP12Password,
		P12StorageRef:        m.P12StorageRef,
		Modalidad:            m.Modalidad,
		Ambiente:             m.Ambiente,
		Nit:                  m.Nit,
	}
	return c
}
