package postgres

import (
	"strings"

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

// Create inserta el cliente. El repositorio es append-only (sin Update/Delete):
// la inmutabilidad tras facturación se refuerza con el trigger
// trg_customers_immutability en la base de datos.
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
		// 23505 (idx_company_doc): carrera entre dos creates del mismo
		// documento; el usecase lo resuelve re-asociando el existente.
		if strings.Contains(err.Error(), "23505") {
			return domain.ErrCustomerDocumentConflict
		}
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
	if err := r.db.Where("tenant_id = ? AND document_type = ? AND document_number = ?",
		companyID, models.DocumentType(documentType), documentNumber).First(&m).Error; err != nil {
		return nil, err
	}
	return toDomainCustomer(&m), nil
}

func (r *PostgresCustomerRepository) GetByCompanyAndFiscalIdentity(companyID string, documentType, documentNumber string, complement *string, name string, email string) (*domain.Customer, error) {

	var m models.Customer

	query := r.db.Where("tenant_id = ? AND document_type = ? AND document_number = ? AND COALESCE(complement, '') = COALESCE(?, '')",
		companyID, models.DocumentType(documentType), documentNumber, complement)
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
	if err := r.db.Where("tenant_id = ?", companyID).Order("created_at ASC").Find(&modelsList).Error; err != nil {
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
		CodigoCliente:  m.CodigoCliente,
		CreatedAt:      m.CreatedAt,
	}
}
