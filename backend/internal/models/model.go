package models

import (
	"time"

	"gorm.io/datatypes" // Útil para el tipo Json de GORM
)

// --- Enums representados como strings en Go ---

type SiatEnvironment string

const (
	EnvironmentPiloto     SiatEnvironment = "PILOTO"
	EnvironmentProduccion SiatEnvironment = "PRODUCCION"
)

type InvoiceStatus string

const (
	StatusPending   InvoiceStatus = "PENDING"
	StatusSent      InvoiceStatus = "SENT"
	StatusAccepted  InvoiceStatus = "ACCEPTED"
	StatusRejected  InvoiceStatus = "REJECTED"
	StatusOffline   InvoiceStatus = "OFFLINE"
	StatusCancelled InvoiceStatus = "CANCELLED"
)

type DocumentType string

const (
	DocCI  DocumentType = "CI"
	DocCEX DocumentType = "CEX"
	DocPAS DocumentType = "PAS"
	DocNIT DocumentType = "NIT"
	DocOD  DocumentType = "OD"
)

type EmissionType string

const (
	EmissionEnLinea      EmissionType = "EN_LINEA"
	EmissionOffline      EmissionType = "OFFLINE"
	EmissionContingencia EmissionType = "CONTINGENCIA"
)

type ContingencyReason string

const (
	ReasonFaltaEnergia  ContingencyReason = "FALTA_ENERGIA_ELECTRICA"
	ReasonFallaInternet ContingencyReason = "FALLA_CONEXION_INTERNET"
	ReasonFallaSin      ContingencyReason = "FALLA_SERVIDOR_SIN"
	ReasonFallaSistema  ContingencyReason = "FALLA_SISTEMA_FACTURACION"
	ReasonOtro          ContingencyReason = "OTRO"
)

// --- Modelos de Base de Datos ---

type Company struct {
	ID            string          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Nit           string          `gorm:"type:varchar(20);uniqueIndex;not null"`
	BusinessName  string          `gorm:"type:varchar(150);not null"`
	CodigoSistema string          `gorm:"type:varchar(100);not null"`
	Ambiente      SiatEnvironment `gorm:"type:varchar(20);default:'PILOTO'"`
	CreatedAt     time.Time
	UpdatedAt     time.Time

	PointsOfSale []PointOfSale `gorm:"foreignKey:CompanyId"`
	Customers    []Customer    `gorm:"foreignKey:CompanyId"`
	Invoices     []Invoice     `gorm:"foreignKey:CompanyId"`
}

type PointOfSale struct {
	ID               string  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId        string  `gorm:"type:uuid;uniqueIndex:idx_company_sucursal_pv,priority:1;not null"`
	CodigoSucursal   int     `gorm:"uniqueIndex:idx_company_sucursal_pv,priority:2;default:0;not null"`
	CodigoPuntoVenta int     `gorm:"uniqueIndex:idx_company_sucursal_pv,priority:3;not null"`
	Description      string  `gorm:"type:varchar(150);not null"`
	Cuis             *string `gorm:"type:varchar(100)"`
	CuisCreatedAt    *time.Time
	IsActive         bool `gorm:"default:true;not null"`
	CreatedAt        time.Time

	Company           Company            `gorm:"foreignKey:CompanyId"`
	Cufds             []Cufd             `gorm:"foreignKey:PointOfSaleId"`
	Invoices          []Invoice          `gorm:"foreignKey:PointOfSaleId"`
	ContingencyEvents []ContingencyEvent `gorm:"foreignKey:PointOfSaleId"`
}

type Cufd struct {
	ID            string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	PointOfSaleId string    `gorm:"type:uuid;index:idx_cufd_pos_valid;not null"`
	Cufd          string    `gorm:"type:text;not null"`
	Direccion     string    `gorm:"type:text;not null"`
	CodigoControl string    `gorm:"type:varchar(100);not null"`
	ValidFrom     time.Time `gorm:"index:idx_cufd_pos_valid;not null"`
	ValidTo       time.Time `gorm:"not null"`

	PointOfSale PointOfSale `gorm:"foreignKey:PointOfSaleId"`
	Invoices    []Invoice   `gorm:"foreignKey:CufdId"`
}

type ContingencyEvent struct {
	ID            string            `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	PointOfSaleId string            `gorm:"type:uuid;index:idx_contingency_pos_start;not null"`
	Reason        ContingencyReason `gorm:"type:varchar(50);not null"`
	Description   *string           `gorm:"type:text"`
	StartDate     time.Time         `gorm:"index:idx_contingency_pos_start;not null"`
	EndDate       *time.Time
	SiatEventCode *string `gorm:"type:varchar(50)"`
	IsSynced      bool    `gorm:"default:false;not null"`
	CreatedAt     time.Time

	PointOfSale PointOfSale `gorm:"foreignKey:PointOfSaleId"`
	Invoices    []Invoice   `gorm:"foreignKey:ContingencyEventId"`
}

type Customer struct {
	ID             string       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId      string       `gorm:"type:uuid;index:idx_company_doc,priority:1;not null"`
	DocumentType   DocumentType `gorm:"type:varchar(20);index:idx_company_doc,priority:2;not null"`
	DocumentNumber string       `gorm:"type:varchar(30);index:idx_company_doc,priority:3;not null"`
	Complement     *string      `gorm:"type:varchar(10)"`
	Name           string       `gorm:"type:varchar(150);not null"`

	Company  Company   `gorm:"foreignKey:CompanyId"`
	Invoices []Invoice `gorm:"foreignKey:CustomerId"`
}

type Invoice struct {
	ID                 string        `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId          string        `gorm:"type:uuid;not null"`
	CustomerId         string        `gorm:"type:uuid;not null"`
	PointOfSaleId      string        `gorm:"type:uuid;uniqueIndex:idx_pos_invoice_num,priority:1;not null"`
	CufdId             string        `gorm:"type:uuid;not null"`
	ContingencyEventId *string       `gorm:"type:uuid"`
	InvoiceNumber      int           `gorm:"uniqueIndex:idx_pos_invoice_num,priority:2;not null"`
	Cuf                *string       `gorm:"type:varchar(150);uniqueIndex"`
	EmissionType       EmissionType  `gorm:"type:varchar(30);default:'EN_LINEA';not null"`
	CodigoMetodoPago   int           `gorm:"default:1;not null"`
	CodigoMoneda       int           `gorm:"default:1;not null"`
	TipoCambio         float64       `gorm:"type:decimal(18,5);default:1;not null"`
	IssueDate          time.Time     `gorm:"index;not null"`
	Subtotal           float64       `gorm:"type:decimal(18,2);not null"`
	Discount           float64       `gorm:"type:decimal(18,2);default:0;not null"`
	Total              float64       `gorm:"type:decimal(18,2);not null"`
	Xml                *string       `gorm:"type:text"`
	XmlHash            *string       `gorm:"type:varchar(100)"`
	SiatReceptionCode  *string       `gorm:"type:varchar(100)"`
	Status             InvoiceStatus `gorm:"type:varchar(30);default:'PENDING';index;not null"`
	MotivoAnulacion    *int
	FechaAnulacion     *time.Time
	CreatedAt          time.Time

	Company          Company           `gorm:"foreignKey:CompanyId"`
	Customer         Customer          `gorm:"foreignKey:CustomerId"`
	PointOfSale      PointOfSale       `gorm:"foreignKey:PointOfSaleId"`
	CufdRecord       Cufd              `gorm:"foreignKey:CufdId"`
	ContingencyEvent *ContingencyEvent `gorm:"foreignKey:ContingencyEventId"`

	Items  []InvoiceItem  `gorm:"foreignKey:InvoiceId"`
	Events []InvoiceEvent `gorm:"foreignKey:InvoiceId"`
}

type InvoiceItem struct {
	ID                string  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	InvoiceId         string  `gorm:"type:uuid;index;not null"`
	Code              string  `gorm:"type:varchar(50);not null"`
	Description       string  `gorm:"type:text;not null"`
	CodigoActividad   *string `gorm:"type:varchar(20)"`
	CodigoProductoSin *string `gorm:"type:varchar(20)"`
	UnitCode          *int
	Quantity          float64 `gorm:"type:decimal(18,3);not null"`
	UnitPrice         float64 `gorm:"type:decimal(18,2);not null"`
	Discount          float64 `gorm:"type:decimal(18,2);default:0;not null"`
	Subtotal          float64 `gorm:"type:decimal(18,2);not null"`

	Invoice Invoice `gorm:"foreignKey:InvoiceId"`
}

type InvoiceEvent struct {
	ID        string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	InvoiceId string         `gorm:"type:uuid;index;not null"`
	Type      string         `gorm:"type:varchar(50);not null"`
	Message   string         `gorm:"type:text;not null"`
	Payload   datatypes.JSON `gorm:"type:jsonb"`
	CreatedAt time.Time

	Invoice Invoice `gorm:"foreignKey:InvoiceId"`
}
