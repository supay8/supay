package domain

import "time"

// SentPackageType identifica el tipo de envío al SIAT.
type SentPackageType string

const (
	PackageTypePaquete SentPackageType = "PAQUETE"
	PackageTypeMasiva  SentPackageType = "MASIVA"
	PackageTypeCompras SentPackageType = "COMPRAS"
)

// SentPackageStatus identifica el estado de un envío.
type SentPackageStatus string

const (
	PackageStatusSent     SentPackageStatus = "SENT"
	PackageStatusAccepted SentPackageStatus = "ACCEPTED"
	PackageStatusRejected SentPackageStatus = "REJECTED"
	PackageStatusPending  SentPackageStatus = "PENDING_VALIDATION"
)

// SentPackage representa un envío de facturas al SIAT (paquete, lote masivo
// o paquete de compras) con su código de recepción, hash y estado de validación.
type SentPackage struct {
	ID                    string            `json:"id"`
	CompanyId             string            `json:"company_id"`
	PointOfSaleId         string            `json:"point_of_sale_id"`
	Type                  SentPackageType   `json:"type"`
	CodigoRecepcion       string            `json:"codigo_recepcion"`
	HashArchivo           string            `json:"hash_archivo"`
	CantidadFacturas      int               `json:"cantidad_facturas"`
	CodigoDocumentoSector int               `json:"codigo_documento_sector"`
	CodigoTipoFactura     int               `json:"codigo_tipo_factura"`
	CodigoEmision         int               `json:"codigo_emision"`
	CodigoEvento          *int64            `json:"codigo_evento,omitempty"`
	ContingencyEventId    *string           `json:"contingency_event_id,omitempty"`
	Status                SentPackageStatus `json:"status"`
	Mensajes              *string           `json:"mensajes,omitempty"`
	XmlHash               string            `json:"xml_hash"`
	SentAt                time.Time         `json:"sent_at"`
	ValidatedAt           *time.Time        `json:"validated_at,omitempty"`
	CreatedAt             time.Time         `json:"created_at"`

	Company     Company     `json:"company"`
	PointOfSale PointOfSale `json:"point_of_sale"`
}

// SentPackageRepository define el contrato para la persistencia de envíos.
type SentPackageRepository interface {
	Create(pkg *SentPackage) error
	GetByID(id string) (*SentPackage, error)
	GetByCodigoRecepcion(codigoRecepcion string) (*SentPackage, error)
	ListByPointOfSale(pointOfSaleID string) ([]*SentPackage, error)
	ListByCompany(companyID string) ([]*SentPackage, error)
	Update(pkg *SentPackage) error
}
