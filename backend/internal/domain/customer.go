package domain

import "time"

// Customer es una dimensión histórica/analítica del receptor. Solo el flujo de
// facturación puede insertar registros; nunca se usa para reconstruir una
// factura, que conserva su propio snapshot fiscal.
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

// CustomerRepository es append-only: no expone Update ni Delete.
type CustomerRepository interface {
	Create(c *Customer) error
	GetByID(id string) (*Customer, error)
	GetByCompanyAndFiscalIdentity(companyID string, documentType, documentNumber string, complement *string, name string, email string) (*Customer, error)
	List(companyID string) ([]*Customer, error)
}
