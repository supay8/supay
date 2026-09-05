package domain

import (
	"encoding/json"
	"time"
)

type InvoiceStatus string

const (
	InvoicePending   InvoiceStatus = "PENDING"
	InvoiceSending   InvoiceStatus = "SENDING"
	InvoiceSent      InvoiceStatus = "SENT"
	InvoiceAccepted  InvoiceStatus = "ACCEPTED"
	InvoiceRejected  InvoiceStatus = "REJECTED"
	InvoiceObserved  InvoiceStatus = "OBSERVED"
	InvoiceOffline   InvoiceStatus = "OFFLINE"
	InvoiceCancelled InvoiceStatus = "CANCELLED"
)

// Valid indica si el estado es uno de los valores conocidos.
func (s InvoiceStatus) Valid() bool {
	switch s {
	case InvoicePending, InvoiceSending, InvoiceSent, InvoiceAccepted,
		InvoiceRejected, InvoiceObserved, InvoiceOffline, InvoiceCancelled:
		return true
	}
	return false
}

// InvoiceStatuses lista los estados válidos para mensajes de validación.
func InvoiceStatuses() string {
	return "PENDING, SENDING, SENT, ACCEPTED, REJECTED, OBSERVED, OFFLINE, CANCELLED"
}

type InvoiceItem struct {
	ID                string  `json:"id"`
	InvoiceId         string  `json:"invoice_id"`
	ProductID         *string `json:"product_id,omitempty"`
	Code              string  `json:"code"`
	Description       string  `json:"description"`
	CodigoActividad   *string `json:"codigo_actividad,omitempty"`
	CodigoProductoSin *string `json:"codigo_producto_sin,omitempty"`
	UnitCode          *int    `json:"unit_code,omitempty"`
	Quantity          float64 `json:"quantity"`
	UnitPrice         float64 `json:"unit_price"`
	Discount          float64 `json:"discount"`
	Subtotal          float64 `json:"subtotal"`
	// SectorData contiene los campos sectoriales del detalle (ítem) validados
	// contra CamposDetalle del perfil del documento-sector. Opcional: un ítem
	// sin SectorData mantiene exactamente el comportamiento anterior.
	SectorData json.RawMessage `json:"sector_data,omitempty"`
}

type Invoice struct {
	ID                    string          `json:"id"`
	CompanyId             string          `json:"company_id"`
	CustomerId            string          `json:"customer_id"`
	PointOfSaleId         string          `json:"point_of_sale_id"`
	IdempotencyKey        *string         `json:"idempotency_key,omitempty"`
	CufdId                string          `json:"cufd_id"`
	ContingencyEventId    *string         `json:"contingency_event_id,omitempty"`
	InvoiceNumber         int             `json:"invoice_number"`
	Cuf                   *string         `json:"cuf,omitempty"`
	EmissionType          string          `json:"emission_type"`
	CodigoMetodoPago      int             `json:"codigo_metodo_pago"`
	CodigoMoneda          int             `json:"codigo_moneda"`
	TipoCambio            float64         `json:"tipo_cambio"`
	CodigoDocumentoSector int             `json:"codigo_documento_sector"`
	Layout                string          `json:"layout,omitempty"`
	Modalidad             int             `json:"modalidad"`
	CodigoTipoFactura     int             `json:"codigo_tipo_factura"`
	Archivo               string          `json:"archivo,omitempty"`
	HashArchivo           string          `json:"hash_archivo,omitempty"`
	NombreEstudiante      *string         `json:"nombre_estudiante,omitempty"`
	PeriodoFacturado      *string         `json:"periodo_facturado,omitempty"`
	SectorData            json.RawMessage `json:"sector_data,omitempty"`
	AjustaFacturaId       *string         `json:"ajusta_factura_id,omitempty"`
	IssueDate             time.Time       `json:"issue_date"`
	Subtotal              float64         `json:"subtotal"`
	Discount              float64         `json:"discount"`
	Total                 float64         `json:"total"`
	Xml                   *string         `json:"xml,omitempty"`
	XmlHash               *string         `json:"xml_hash,omitempty"`
	SiatReceptionCode     *string         `json:"siat_reception_code,omitempty"`
	SiatMensajes          *string         `json:"siat_mensajes,omitempty"`
	MotivoAnulacion       *int            `json:"motivo_anulacion,omitempty"`
	FechaAnulacion        *time.Time      `json:"fecha_anulacion,omitempty"`
	Status                InvoiceStatus   `json:"status"`
	CreatedAt             time.Time       `json:"created_at"`

	Company     Company       `json:"company"`
	Customer    Customer      `json:"customer"`
	PointOfSale PointOfSale   `json:"point_of_sale"`
	CufdRecord  Cufd          `json:"cufd_record"`
	Items       []InvoiceItem `json:"items"`
}

// InvoiceListFilter acota el listado de facturas por punto de venta, estado y
// rango de fecha de emisión, con paginación. El listado nunca devuelve los
// campos pesados (xml/archivo).
type InvoiceListFilter struct {
	PointOfSaleID string
	Status        *InvoiceStatus
	From          *time.Time
	To            *time.Time
	Limit         int
	Offset        int
}

type InvoiceRepository interface {
	// Create persiste la factura (borrador PENDING) asignando el número
	// correlativo por point_of_sale_id bajo advisory lock (atómico).
	Create(inv *Invoice) error
	GetByID(id string) (*Invoice, error)
	GetByIDs(ids []string) ([]*Invoice, error)
	ListByPointOfSale(pointOfSaleID string) ([]*Invoice, error)
	// ListFiltered devuelve el listado paginado según el filtro, sin los
	// campos pesados (xml/archivo), junto con el total de coincidencias.
	ListFiltered(filter InvoiceListFilter) ([]*Invoice, int64, error)
	Update(inv *Invoice) error
	TransitionStatus(id string, from, to InvoiceStatus, reason InvoiceTransitionReason, fields map[string]any, event *InvoiceEvent) (bool, error)
	// ClaimForEmission marca la factura como SENDING si está PENDING
	// (transición atómica), retornando false si el estado ya no es PENDING.
	ClaimForEmission(id string) (bool, error)
	// ReleaseStaleSending revierte a PENDING las facturas atascadas en SENDING
	// durante más de olderThan (crash del proceso, fallo del update final),
	// devolviendo cuántas fueron liberadas.
	ReleaseStaleSending(olderThan time.Duration) (int64, error)
	FindActiveCufdForPointOfSale(pointOfSaleID string, at time.Time) (*Cufd, error)
	// GetByIdempotencyKey devuelve la factura asociada a una clave de
	// idempotencia dentro de un punto de venta. nil si no existe.
	GetByIdempotencyKey(pointOfSaleID, key string) (*Invoice, error)
}
