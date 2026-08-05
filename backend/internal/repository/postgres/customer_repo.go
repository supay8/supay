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

func (r *PostgresCustomerRepository) Create(c *domain.Customer) error {
	dbModel := models.Customer{
		ID:             uuid.NewString(),
		CompanyId:      c.CompanyId,
		DocumentType:   models.DocumentType(c.DocumentType),
		DocumentNumber: c.DocumentNumber,
		Complement:     c.Complement,
		Name:           c.Name,
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
	if err := r.db.Where("id = ?", id).First(&m).Error; err != nil {
		return nil, err
	}
	return toDomainCustomer(&m), nil
}

func (r *PostgresCustomerRepository) GetByCompanyAndDocument(companyID, documentType, documentNumber string) (*domain.Customer, error) {
	var m models.Customer
	if err := r.db.Where("company_id = ? AND document_type = ? AND document_number = ?",
		companyID, models.DocumentType(documentType), documentNumber).First(&m).Error; err != nil {
		return nil, err
	}
	return toDomainCustomer(&m), nil
}

func (r *PostgresCustomerRepository) List(companyID string) ([]*domain.Customer, error) {
	var modelsList []models.Customer
	query := r.db.Order("created_at ASC")
	if companyID != "" {
		query = query.Where("company_id = ?", companyID)
	}
	if err := query.Find(&modelsList).Error; err != nil {
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
		Name:           m.Name,
		CreatedAt:      m.CreatedAt,
	}
}
