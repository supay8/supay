package postgres

import (
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostgresCustomerRepository struct {
	db *gorm.DB
}

func NewPostgresCustomerRepository(db *gorm.DB) domain.CustomerRepository {
	return &PostgresCustomerRepository{db: db}
}

// Create inserta una fila histórica. Solo el flujo de facturación lo invoca.
func (r *PostgresCustomerRepository) Create(c *domain.Customer) error {
	dbModel := models.Customer{
		ID:             uuid.NewString(),
		CompanyId:      c.CompanyId,
		DocumentType:   models.DocumentType(c.DocumentType),
		DocumentNumber: c.DocumentNumber,
		Complement:     c.Complement,
		Email:          c.Email,
		Name:           c.Name,
		CodigoCliente:  c.CodigoCliente,
	}
	if err := r.db.Create(&dbModel).Error; err != nil {
		return err
	}
	c.ID = dbModel.ID
	c.CreatedAt = dbModel.CreatedAt
	return nil
}

func (r *PostgresCustomerRepository) GetByID(id string) (*domain.Customer, error) {
	var m models.Customer
	if err := r.db.Where("id = ? AND is_active = true", id).First(&m).Error; err != nil {
		return nil, err
	}
	return toDomainCustomer(&m), nil
}

func (r *PostgresCustomerRepository) GetByCompanyAndFiscalIdentity(companyID string, documentType, documentNumber string, complement *string, name string, email string) (*domain.Customer, error) {

	var m models.Customer

	query := r.db.Where(`tenant_id = ? AND document_type = ? AND document_number = ?
		AND COALESCE(complement, '') = COALESCE(?, '') AND name = ?
		AND COALESCE(email, '') = ? AND is_active = true`,
		companyID, models.DocumentType(documentType), documentNumber, complement, name, email)
	if err := query.First(&m).Error; err != nil {
		return nil, err
	}
	return toDomainCustomer(&m), nil
}

func (r *PostgresCustomerRepository) List(companyID string) ([]*domain.Customer, error) {
	if companyID == "" {
		return nil, domain.ErrMissingCompanyID
	}
	var modelsList []models.Customer
	if err := r.db.Where("tenant_id = ? AND is_active = true", companyID).Order("created_at ASC").Find(&modelsList).Error; err != nil {
		return nil, err
	}
	res := make([]*domain.Customer, 0, len(modelsList))
	for i := range modelsList {
		res = append(res, toDomainCustomer(&modelsList[i]))
	}
	return res, nil
}

func toDomainCustomer(m *models.Customer) *domain.Customer {
	return &domain.Customer{
		ID:             m.ID,
		CompanyId:      m.CompanyId,
		DocumentType:   string(m.DocumentType),
		DocumentNumber: m.DocumentNumber,
		Complement:     m.Complement,
		Email:          m.Email,
		Name:           m.Name,
		CodigoCliente:  m.CodigoCliente,
		CreatedAt:      m.CreatedAt,
	}
}
