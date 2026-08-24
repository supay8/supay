package models

import (
	"encoding/json"
	"time"

	"gorm.io/datatypes" // Útil para el tipo Json de GORM
)

// --- Enums representados como strings en Go ---

type SiatEnvironment string

const (
	EnvironmentPiloto     SiatEnvironment = "PILOTO"
	EnvironmentProduccion SiatEnvironment = "PRODUCCION"
)

// CodigoAmbiente devuelve el código numérico que el SIAT espera: 1 = producción, 2 = piloto.
func (e SiatEnvironment) CodigoAmbiente() int {
	if e == EnvironmentProduccion {
		return 1
	}
	return 2
}

type InvoiceStatus string

const (
	StatusPending   InvoiceStatus = "PENDING"
	StatusSending   InvoiceStatus = "SENDING"
	StatusSent      InvoiceStatus = "SENT"
	StatusAccepted  InvoiceStatus = "ACCEPTED"
	StatusRejected  InvoiceStatus = "REJECTED"
	StatusObserved  InvoiceStatus = "OBSERVED"
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
	ID              string          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Nit             string          `gorm:"type:varchar(20);uniqueIndex;not null"`
	BusinessName    string          `gorm:"type:varchar(150);not null"`
	CodigoSistema   string          `gorm:"type:varchar(100);not null"`
	Ambiente        SiatEnvironment `gorm:"type:varchar(20);default:'PILOTO'"`
	Municipio       string          `gorm:"type:varchar(100);not null;default:''"`
	Direccion       string          `gorm:"type:text;not null;default:''"`
	Telefono        string          `gorm:"type:varchar(50);not null;default:''"`
	CodigoActividad *string         `gorm:"type:varchar(20)"`
	PiePagina       string          `gorm:"type:text;not null;default:''"`
	UsuarioSiat     string          `gorm:"type:varchar(50);not null;default:'SUPAY'"`
	CreatedAt       time.Time
	UpdatedAt       time.Time

	PointsOfSale []PointOfSale `gorm:"foreignKey:CompanyId"`
	Customers    []Customer    `gorm:"foreignKey:CompanyId"`
	Invoices     []Invoice     `gorm:"foreignKey:CompanyId"`
}

type PointOfSale struct {
	ID               string  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId        string  `gorm:"type:uuid;uniqueIndex:idx_company_sucursal_pv,priority:1;not null"`
	BranchId         *string `gorm:"type:uuid;index"`
	CodigoSucursal   int     `gorm:"uniqueIndex:idx_company_sucursal_pv,priority:2;default:0;not null"`
	CodigoPuntoVenta int     `gorm:"uniqueIndex:idx_company_sucursal_pv,priority:3;not null"`
	Description      string  `gorm:"type:varchar(150);not null"`
	Cuis             *string `gorm:"type:varchar(100)"`
	CuisCreatedAt    *time.Time
	IsActive         bool   `gorm:"default:true;not null"`
	SiatCode         *int   `gorm:"type:int"`
	Status           string `gorm:"type:varchar(50);default:'CREATING'"`
	CreatedAt        time.Time

	// Datos del registro oficial ante el SIAT (operación registroPuntoVenta)
	TipoPuntoVenta   *int `gorm:"type:int"`
	SiatTransaccion  bool `gorm:"default:false;not null"`
	SiatRegisteredAt *time.Time
	SiatResponse     json.RawMessage `gorm:"type:jsonb"`
	SiatError        *string         `gorm:"type:text"`

	Company           Company            `gorm:"foreignKey:CompanyId"`
	Cufds             []Cufd             `gorm:"foreignKey:PointOfSaleId"`
	Invoices          []Invoice          `gorm:"foreignKey:PointOfSaleId"`
	ContingencyEvents []ContingencyEvent `gorm:"foreignKey:PointOfSaleId"`
}

type Branch struct {
	ID             string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId      string `gorm:"type:uuid;uniqueIndex:idx_company_sucursal,priority:1;not null"`
	CodigoSucursal int    `gorm:"uniqueIndex:idx_company_sucursal,priority:2;not null"`
	Name           string `gorm:"type:varchar(150);not null"`
	Address        string `gorm:"type:text"`
	Active         bool   `gorm:"default:true;not null"`
	CreatedAt      time.Time

	Company Company `gorm:"foreignKey:CompanyId"`
}

// TipoPuntoVenta es el catálogo sincronizado de tipos de punto de venta
// (operación sincronizarParametricaTipoPuntoVenta del SIAT).
type TipoPuntoVenta struct {
	ID                 string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId          string    `gorm:"type:uuid;uniqueIndex:idx_company_tipo_pv,priority:1;not null"`
	CodigoClasificador int       `gorm:"uniqueIndex:idx_company_tipo_pv,priority:2;not null"`
	Descripcion        string    `gorm:"type:varchar(200);not null"`
	SyncedAt           time.Time `gorm:"not null"`
	CreatedAt          time.Time

	Company Company `gorm:"foreignKey:CompanyId"`
}

// Catalog es un elemento de un catálogo sincronizado del SIAT (operaciones
// sincronizarParametrica* y sincronizar* del servicio FacturacionSincronizacion).
type Catalog struct {
	ID          string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId   string    `gorm:"type:uuid;index:idx_catalog_company_tipo,priority:1;not null"`
	Tipo        string    `gorm:"type:varchar(50);index:idx_catalog_company_tipo,priority:2;not null"`
	Codigo      int       `gorm:"not null"`
	Descripcion string    `gorm:"type:text;not null"`
	SyncedAt    time.Time `gorm:"not null"`
	CreatedAt   time.Time

	Company Company `gorm:"foreignKey:CompanyId"`
}

type SinProduct struct {
	ID                string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId         string    `gorm:"type:uuid;uniqueIndex:idx_company_sin_product,priority:1;not null"`
	CodigoActividad   int64     `gorm:"uniqueIndex:idx_company_sin_product,priority:2;not null;default:0"`
	CodigoProductoSin int64     `gorm:"uniqueIndex:idx_company_sin_product,priority:3;not null"`
	Descripcion       string    `gorm:"type:text;not null"`
	Active            bool      `gorm:"default:true;not null"`
	SyncedAt          time.Time `gorm:"not null"`
	CreatedAt         time.Time
	UpdatedAt         time.Time

	Company Company `gorm:"foreignKey:CompanyId"`
}

// SiatActividad es el catálogo de actividades económicas sincronizado del SIAT
// (operación sincronizarActividades). CodigoCaeb se guarda como string porque
// el SIAT lo transmite como texto.
type SiatActividad struct {
	ID            string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId     string    `gorm:"type:uuid;uniqueIndex:idx_company_caeb,priority:1;not null"`
	CodigoCaeb    string    `gorm:"type:varchar(20);uniqueIndex:idx_company_caeb,priority:2;not null"`
	Descripcion   string    `gorm:"type:text;not null"`
	TipoActividad string    `gorm:"type:varchar(10);not null;default:''"`
	SyncedAt      time.Time `gorm:"not null"`
	CreatedAt     time.Time

	Company Company `gorm:"foreignKey:CompanyId"`
}

func (SiatActividad) TableName() string { return "siat_actividades" }

// SiatLeyendaFactura es el catálogo de leyendas de factura sincronizado del
// SIAT (operación sincronizarListaLeyendasFactura), asociado por actividad.
type SiatLeyendaFactura struct {
	ID                 string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId          string    `gorm:"type:uuid;index:idx_company_leyenda_act,priority:1;not null"`
	CodigoActividad    string    `gorm:"type:varchar(20);index:idx_company_leyenda_act,priority:2;not null"`
	DescripcionLeyenda string    `gorm:"type:text;not null"`
	SyncedAt           time.Time `gorm:"not null"`
	CreatedAt          time.Time

	Company Company `gorm:"foreignKey:CompanyId"`
}

func (SiatLeyendaFactura) TableName() string { return "siat_leyendas_factura" }

// SiatActividadDocSector es la relación actividad ↔ documento-sector
// sincronizada del SIAT (operación sincronizarListaActividadesDocumentoSector).
type SiatActividadDocSector struct {
	ID                    string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId             string    `gorm:"type:uuid;uniqueIndex:idx_company_act_sector,priority:1;not null"`
	CodigoActividad       string    `gorm:"type:varchar(20);uniqueIndex:idx_company_act_sector,priority:2;not null"`
	CodigoDocumentoSector int       `gorm:"uniqueIndex:idx_company_act_sector,priority:3;not null"`
	TipoDocumentoSector   string    `gorm:"type:varchar(20);not null;default:''"`
	SyncedAt              time.Time `gorm:"not null"`
	CreatedAt             time.Time

	Company Company `gorm:"foreignKey:CompanyId"`
}

func (SiatActividadDocSector) TableName() string { return "siat_actividades_doc_sector" }

type CatalogSyncState struct {
	ID            string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId     string `gorm:"type:uuid;uniqueIndex:idx_sync_state,priority:1;not null"`
	PointOfSaleId string `gorm:"type:uuid;uniqueIndex:idx_sync_state,priority:2;not null"`
	Operation     string `gorm:"type:varchar(80);uniqueIndex:idx_sync_state,priority:3;not null"`
	Status        string `gorm:"type:varchar(20);not null"`
	RowsSaved     int    `gorm:"not null;default:0"`
	SyncedAt      *time.Time
	Error         string `gorm:"type:text;not null;default:''"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Product struct {
	ID        string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId string `gorm:"type:uuid;uniqueIndex:idx_company_product_sku,priority:1;not null"`
	SKU       string `gorm:"type:varchar(100);uniqueIndex:idx_company_product_sku,priority:2;not null"`
	Name      string `gorm:"type:varchar(200);not null"`
	Active    bool   `gorm:"default:true;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time

	Company  Company          `gorm:"foreignKey:CompanyId"`
	Mappings []ProductMapping `gorm:"foreignKey:ProductId"`
}

type ProductMapping struct {
	ID                    string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ProductId             string    `gorm:"type:uuid;index:idx_product_mapping,priority:1;not null"`
	SinProductId          *string   `gorm:"type:uuid;index"`
	CodigoProductoSin     int64     `gorm:"not null"`
	CodigoActividad       string    `gorm:"type:varchar(20);not null"`
	CodigoDocumentoSector int       `gorm:"not null"`
	UnidadMedida          int       `gorm:"not null"`
	IsDefault             bool      `gorm:"default:false;not null"`
	Active                bool      `gorm:"default:true;not null"`
	SyncedAt              time.Time `gorm:"not null"`

	Product    Product     `gorm:"foreignKey:ProductId"`
	SinProduct *SinProduct `gorm:"foreignKey:SinProductId"`
}

type Cufd struct {
	ID            string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	PointOfSaleId string    `gorm:"type:uuid;index:idx_cufd_pos_valid;not null"`
	Cufd          string    `gorm:"type:text;not null"`
	Direccion     string    `gorm:"type:text;not null"`
	CodigoControl string    `gorm:"type:varchar(100);not null"`
	CodigoQR      *string   `gorm:"type:text"`
	ValidFrom     time.Time `gorm:"index:idx_cufd_pos_valid;not null"`
	ValidTo       time.Time `gorm:"not null"`
	Active        bool      `gorm:"default:true;not null"`
	CreatedAt     time.Time

	PointOfSale PointOfSale `gorm:"foreignKey:PointOfSaleId"`
	Invoices    []Invoice   `gorm:"foreignKey:CufdId"`
}

type Cuis struct {
	ID            string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	PointOfSaleId string    `gorm:"type:uuid;index;not null"`
	Cuis          string    `gorm:"type:varchar(200);not null"`
	ValidFrom     time.Time `gorm:"not null"`
	ValidTo       time.Time `gorm:"not null"`
	Active        bool      `gorm:"default:true;not null"`
	CreatedAt     time.Time

	PointOfSale PointOfSale `gorm:"foreignKey:PointOfSaleId"`
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
	CreatedAt      time.Time

	Company  Company   `gorm:"foreignKey:CompanyId"`
	Invoices []Invoice `gorm:"foreignKey:CustomerId"`
}

type Invoice struct {
	ID                    string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId             string         `gorm:"type:uuid;not null"`
	CustomerId            string         `gorm:"type:uuid;not null"`
	PointOfSaleId         string         `gorm:"type:uuid;uniqueIndex:idx_pos_invoice_num,priority:1;not null"`
	CufdId                string         `gorm:"type:uuid;not null"`
	ContingencyEventId    *string        `gorm:"type:uuid"`
	InvoiceNumber         int            `gorm:"uniqueIndex:idx_pos_invoice_num,priority:2;not null"`
	Cuf                   *string        `gorm:"type:varchar(150);uniqueIndex"`
	EmissionType          EmissionType   `gorm:"type:varchar(30);default:'EN_LINEA';not null"`
	CodigoMetodoPago      int            `gorm:"default:1;not null"`
	CodigoMoneda          int            `gorm:"default:1;not null"`
	TipoCambio            float64        `gorm:"type:decimal(18,5);default:1;not null"`
	CodigoDocumentoSector int            `gorm:"default:1;not null"`
	Layout                string         `gorm:"type:varchar(80)"`
	Modalidad             int            `gorm:"default:1;not null"`
	CodigoTipoFactura     int            `gorm:"default:1;not null"`
	Archivo               string         `gorm:"type:text"`
	HashArchivo           string         `gorm:"type:varchar(100)"`
	NombreEstudiante      *string        `gorm:"type:varchar(150)"`
	PeriodoFacturado      *string        `gorm:"type:varchar(30)"`
	SectorData            datatypes.JSON `gorm:"type:jsonb"`
	AjustaFacturaId       *string        `gorm:"type:uuid;index"`
	IssueDate             time.Time      `gorm:"index;not null"`
	Subtotal              float64        `gorm:"type:decimal(18,2);not null"`
	Discount              float64        `gorm:"type:decimal(18,2);default:0;not null"`
	Total                 float64        `gorm:"type:decimal(18,2);not null"`
	Xml                   *string        `gorm:"type:text"`
	XmlHash               *string        `gorm:"type:varchar(100)"`
	SiatReceptionCode     *string        `gorm:"type:varchar(100)"`
	SiatMensajes          *string        `gorm:"type:text"`
	Status                InvoiceStatus  `gorm:"type:varchar(30);default:'PENDING';index;not null"`
	MotivoAnulacion       *int
	FechaAnulacion        *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time

	Company          Company           `gorm:"foreignKey:CompanyId"`
	Customer         Customer          `gorm:"foreignKey:CustomerId"`
	PointOfSale      PointOfSale       `gorm:"foreignKey:PointOfSaleId"`
	CufdRecord       Cufd              `gorm:"foreignKey:CufdId"`
	ContingencyEvent *ContingencyEvent `gorm:"foreignKey:ContingencyEventId"`
	AjustaFactura    *Invoice          `gorm:"foreignKey:AjustaFacturaId"`

	Items  []InvoiceItem  `gorm:"foreignKey:InvoiceId"`
	Events []InvoiceEvent `gorm:"foreignKey:InvoiceId"`
}

type InvoiceItem struct {
	ID                string  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	InvoiceId         string  `gorm:"type:uuid;index;not null"`
	ProductId         *string `gorm:"type:uuid;index"`
	Code              string  `gorm:"type:varchar(50);not null"`
	Description       string  `gorm:"type:text;not null"`
	CodigoActividad   *string `gorm:"type:varchar(20)"`
	CodigoProductoSin *string `gorm:"type:varchar(20)"`
	UnitCode          *int
	Quantity          float64 `gorm:"type:decimal(18,3);not null"`
	UnitPrice         float64 `gorm:"type:decimal(18,2);not null"`
	Discount          float64 `gorm:"type:decimal(18,2);default:0;not null"`
	Subtotal          float64 `gorm:"type:decimal(18,2);not null"`
	SectorData        datatypes.JSON `gorm:"type:jsonb"`

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

type SentPackage struct {
	ID                    string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId             string    `gorm:"type:uuid;index:idx_sent_pkg_company;not null"`
	PointOfSaleId         string    `gorm:"type:uuid;index:idx_sent_pkg_pos;not null"`
	Type                  string    `gorm:"type:varchar(20);not null"`
	CodigoRecepcion       string    `gorm:"type:varchar(100);uniqueIndex;not null"`
	HashArchivo           string    `gorm:"type:varchar(100);not null"`
	CantidadFacturas      int       `gorm:"not null"`
	CodigoDocumentoSector int       `gorm:"not null"`
	CodigoTipoFactura     int       `gorm:"not null"`
	CodigoEmision         int       `gorm:"not null"`
	CodigoEvento          *int64    `gorm:"type:bigint"`
	Status                string    `gorm:"type:varchar(30);default:'SENT';index;not null"`
	Mensajes              *string   `gorm:"type:text"`
	XmlHash               string    `gorm:"type:varchar(100);not null"`
	SentAt                time.Time `gorm:"not null"`
	ValidatedAt           *time.Time
	CreatedAt             time.Time

	Company     Company     `gorm:"foreignKey:CompanyId"`
	PointOfSale PointOfSale `gorm:"foreignKey:PointOfSaleId"`
}

type Certificate struct {
	ID           string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId    string    `gorm:"type:uuid;index:idx_cert_company;not null"`
	Name         string    `gorm:"type:varchar(150);not null"`
	Type         string    `gorm:"type:varchar(10);not null"`
	Status       string    `gorm:"type:varchar(20);default:'ACTIVE';index;not null"`
	NotBefore    time.Time `gorm:"not null"`
	NotAfter     time.Time `gorm:"not null"`
	Issuer       string    `gorm:"type:text"`
	Subject      string    `gorm:"type:text"`
	Thumbprint   string    `gorm:"type:varchar(100)"`
	SiatUserCode string    `gorm:"type:varchar(50)"`
	ConfigPath   string    `gorm:"type:text"`
	RenewedFrom  *string   `gorm:"type:uuid"`
	CreatedAt    time.Time
	UpdatedAt    time.Time

	Company Company `gorm:"foreignKey:CompanyId"`
}
