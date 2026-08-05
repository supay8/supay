package domain

import "time"

type InvoiceStatus string

const (
	InvoicePending   InvoiceStatus = "PENDING"
	InvoiceSent      InvoiceStatus = "SENT"
	InvoiceAccepted  InvoiceStatus = "ACCEPTED"
	InvoiceRejected  InvoiceStatus = "REJECTED"
	InvoiceObserved  InvoiceStatus = "OBSERVED"
	InvoiceOffline   InvoiceStatus = "OFFLINE"
	InvoiceCancelled InvoiceStatus = "CANCELLED"
)

type InvoiceItem struct {
	ID                string   `json:"id"`
	InvoiceId         string   `json:"invoice_id"`
	Code              string   `json:"code"`
	Description       string   `json:"description"`
	CodigoActividad   *string  `json:"codigo_actividad,omitempty"`
	CodigoProductoSin *string  `json:"codigo_producto_sin,omitempty"`
	UnitCode          *int     `json:"unit_code,omitempty"`
	Quantity          float64  `json:"quantity"`
	UnitPrice         float64  `json:"unit_price"`
	Discount          float64  `json:"discount"`
	Subtotal          float64  `json:"subtotal"`
}

type Invoice struct {
	ID                 string         `json:"id"`
	CompanyId          string         `json:"company_id"`
	CustomerId         string         `json:"customer_id"`
	PointOfSaleId      string         `json:"point_of_sale_id"`
	CufdId             string         `json:"cufd_id"`
	InvoiceNumber      int            `json:"invoice_number"`
	Cuf                *string        `json:"cuf,omitempty"`
	EmissionType       string         `json:"emission_type"`
	CodigoMetodoPago   int            `json:"codigo_metodo_pago"`
	CodigoMoneda       int            `json:"codigo_moneda"`
	TipoCambio         float64        `json:"tipo_cambio"`
	IssueDate          time.Time      `json:"issue_date"`
	Subtotal           float64        `json:"subtotal"`
	Discount           float64        `json:"discount"`
	Total              float64        `json:"total"`
	Xml                *string        `json:"xml,omitempty"`
	XmlHash            *string        `json:"xml_hash,omitempty"`
	SiatReceptionCode  *string        `json:"siat_reception_code,omitempty"`
	Status             InvoiceStatus  `json:"status"`
	CreatedAt          time.Time      `json:"created_at"`

	Customer    Customer    `json:"customer"`
	PointOfSale PointOfSale `json:"point_of_sale"`
	Items       []InvoiceItem `json:"items"`
}

type InvoiceRepository interface {
	// Create persiste la factura (borrador PENDING) asignando el número
	// correlativo por point_of_sale_id bajo advisory lock (atómico).
	Create(inv *Invoice) error
	GetByID(id string) (*Invoice, error)
	ListByPointOfSale(pointOfSaleID string) ([]*Invoice, error)
	Update(inv *Invoice) error
	FindActiveCufdForPointOfSale(pointOfSaleID string, at time.Time) (*Cufd, error)
}
