package domain

import (
	"errors"
	"time"
)

// ErrCustomerDocumentConflict indica que ya existe un cliente con el mismo
// documento para la empresa.
var ErrCustomerDocumentConflict = errors.New("ya existe un cliente con este documento para la empresa")

type Customer struct {
	ID             string  `json:"id"`
	CompanyId      string  `json:"company_id"`
	DocumentType   string  `json:"document_type"`
	DocumentNumber string  `json:"document_number"`
	Complement     *string `json:"complement,omitempty"`
	Name           string  `json:"name"`
	CreatedAt      time.Time `json:"created_at"`
}

type CustomerRepository interface {
	Create(c *Customer) error
	GetByID(id string) (*Customer, error)
	GetByCompanyAndDocument(companyID, documentType, documentNumber string) (*Customer, error)
	List(companyID string) ([]*Customer, error)
}
