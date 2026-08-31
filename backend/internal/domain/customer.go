package domain

import (
	"errors"
	"time"
)

// ErrCustomerDocumentConflict indica que ya existe un cliente con el mismo
// documento para la empresa.
var ErrCustomerDocumentConflict = errors.New("ya existe un cliente con este documento para la empresa")

// ErrCustomerImmutable indica que el cliente tiene facturas asociadas y por
// lo tanto sus campos fiscales son inmutables (regla de negocio reforzada
// además por el trigger trg_customers_immutability en la base de datos).
var ErrCustomerImmutable = errors.New("el cliente tiene facturas emitidas; sus datos fiscales son inmutables, cree un nuevo cliente")

// Customer es el registro fiscal del receptor de las facturas. Es create-only:
// después de que tenga facturas asociadas, los campos fiscales (Name,
// DocumentType, DocumentNumber, Complement) no se modifican; si cambian, se
// crea un nuevo Customer.
type Customer struct {
	ID             string    `json:"id"`
	CompanyId      string    `json:"company_id"`
	DocumentType   string    `json:"document_type"`
	DocumentNumber string    `json:"document_number"`
	Complement     *string   `json:"complement,omitempty"`
	Email          *string   `json:"email,omitempty"`
	Name           string    `json:"name"`
	CodigoCliente  string    `json:"codigo_cliente"`
	CreatedAt      time.Time `json:"created_at"`
}

// CustomerRepository es append-only: no expone Update ni Delete. La
// modificación de un cliente con facturas está bloqueada en la BD mediante
// trg_customers_immutability.
type CustomerRepository interface {
	Create(c *Customer) error
	GetByID(id string) (*Customer, error)
	GetByCompanyAndDocument(companyID, documentType, documentNumber string) (*Customer, error)
	GetByCompanyAndFiscalIdentity(companyID string, customer *Customer) (*Customer, error)
	List(companyID string) ([]*Customer, error)
}
