package models

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
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
	EmissionMasiva       EmissionType = "MASIVA"
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
	Ambiente        SiatEnvironment `gorm:"-"`
	Modalidad       int             `gorm:"-"`
	Municipio       string          `gorm:"type:varchar(100);not null;default:''"`
	Direccion       string          `gorm:"type:text;not null;default:''"`
	Telefono        string          `gorm:"type:varchar(50);not null;default:''"`
	CodigoActividad *string         `gorm:"type:varchar(20)"`
	PiePagina       string          `gorm:"type:text;not null;default:''"`
	UsuarioSiat     string          `gorm:"type:varchar(50);not null;default:'SUPAY'"`
	IsActive        bool            `gorm:"column:is_active;not null;default:true"`
	CreatedAt       time.Time
	UpdatedAt       time.Time

	PointsOfSale []PointOfSale `gorm:"foreignKey:CompanyId"`
	Customers    []Customer    `gorm:"foreignKey:CompanyId"`
	Invoices     []Invoice     `gorm:"foreignKey:CompanyId"`
	Config       TenantConfig  `gorm:"foreignKey:TenantID;references:ID"`
}

func (Company) TableName() string { return "tenants" }

type TenantConfig struct {
	TenantID           string          `gorm:"column:tenant_id;type:uuid;primaryKey"`
	Ambiente           SiatEnvironment `gorm:"type:varchar(20);not null;default:'PILOTO'"`
	CodigoModalidad    int             `gorm:"not null;default:1"`
	TokenDelegado      string          `gorm:"column:token_delegado;type:text;not null;default:''"`
	MaxInvoicesMonthly int             `gorm:"not null;default:1000"`
	MaxPointsOfSale    int             `gorm:"not null;default:5"`
	MaxAPIKeys         int             `gorm:"column:max_api_keys;not null;default:10"`
	Settings           datatypes.JSON  `gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type PointOfSale struct {
	ID               string  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId        string  `gorm:"column:tenant_id;type:uuid;uniqueIndex:idx_points_of_sale_tenant_codes,priority:1;not null"`
	BranchId         *string `gorm:"type:uuid;index"`
	CodigoSucursal   int     `gorm:"uniqueIndex:idx_points_of_sale_tenant_codes,priority:2;default:0;not null"`
	CodigoPuntoVenta int     `gorm:"uniqueIndex:idx_points_of_sale_tenant_codes,priority:3;not null"`
	Name             string  `gorm:"type:varchar(100);not null;default:''"`
	Description      string  `gorm:"type:varchar(150);not null"`
	Cuis             *string `gorm:"type:varchar(100)"`
	CuisCreatedAt    *time.Time
	CuisExpiresAt    *time.Time
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
	Branch            *Branch            `gorm:"foreignKey:CompanyId,BranchId;references:CompanyId,ID"`
	Cufds             []Cufd             `gorm:"foreignKey:PointOfSaleId"`
	Invoices          []Invoice          `gorm:"foreignKey:PointOfSaleId"`
	ContingencyEvents []ContingencyEvent `gorm:"foreignKey:PointOfSaleId"`
}

func (PointOfSale) TableName() string { return "points_of_sale" }

type Branch struct {
	ID             string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId      string `gorm:"column:tenant_id;type:uuid;uniqueIndex:idx_branches_tenant_code,priority:1;not null"`
	CodigoSucursal int    `gorm:"uniqueIndex:idx_branches_tenant_code,priority:2;not null"`
	Name           string `gorm:"type:varchar(150);not null"`
	Address        string `gorm:"type:text"`
	Active         bool   `gorm:"column:is_active;default:true;not null"`
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

// CatalogVersion/CatalogItem reemplazan las tablas de catálogo por tipo. Cada
// sincronización crea una versión inmutable y las lecturas toman la más nueva.
type CatalogVersion struct {
	ID        string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID  string    `gorm:"column:tenant_id;type:uuid;not null"`
	Tipo      string    `gorm:"type:varchar(50);not null"`
	Version   int       `gorm:"not null"`
	SyncedAt  time.Time `gorm:"not null"`
	Source    string    `gorm:"type:varchar(50);not null;default:'SIAT'"`
	CreatedAt time.Time
}

type CatalogItem struct {
	ID          string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	VersionID   string         `gorm:"column:version_id;type:uuid;not null"`
	Codigo      string         `gorm:"type:varchar(100);not null"`
	Descripcion string         `gorm:"type:text;not null"`
	Metadata    datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'"`

	Version CatalogVersion `gorm:"foreignKey:VersionID;constraint:OnDelete:CASCADE"`
}

type SinProduct struct {
	ID                string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId         string    `gorm:"column:tenant_id;type:uuid;uniqueIndex:idx_sin_products_tenant_code,priority:1;not null"`
	CodigoActividad   int64     `gorm:"uniqueIndex:idx_sin_products_tenant_code,priority:2;not null;default:0"`
	CodigoProductoSin int64     `gorm:"uniqueIndex:idx_sin_products_tenant_code,priority:3;not null"`
	Descripcion       string    `gorm:"type:text;not null"`
	Active            bool      `gorm:"column:is_active;default:true;not null"`
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
	CompanyId     string `gorm:"column:tenant_id;type:uuid;uniqueIndex:idx_sync_state,priority:1;not null"`
	PointOfSaleId string `gorm:"type:uuid;uniqueIndex:idx_sync_state,priority:2;not null"`
	Operation     string `gorm:"type:varchar(80);uniqueIndex:idx_sync_state,priority:3;not null"`
	Status        string `gorm:"type:varchar(20);not null"`
	RowsSaved     int    `gorm:"not null;default:0"`
	SyncedAt      *time.Time
	Error         string `gorm:"type:text;not null;default:''"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Cufd struct {
	ID            string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantId      string    `gorm:"column:tenant_id;type:uuid;not null"`
	PointOfSaleId string    `gorm:"type:uuid;index:idx_cufd_pos_valid;not null"`
	Cufd          string    `gorm:"type:text;not null"`
	Direccion     string    `gorm:"type:text;not null"`
	CodigoControl string    `gorm:"type:varchar(100);not null"`
	CodigoQR      *string   `gorm:"type:text"`
	ValidFrom     time.Time `gorm:"index:idx_cufd_pos_valid;not null"`
	ValidTo       time.Time `gorm:"not null"`
	Active        bool      `gorm:"column:is_active;default:true;not null"`
	CreatedAt     time.Time

	PointOfSale PointOfSale `gorm:"foreignKey:PointOfSaleId"`
	Invoices    []Invoice   `gorm:"foreignKey:CufdId"`
}

func (Cufd) TableName() string { return "cufd_history" }

type Cuis struct {
	ID            string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantId      string    `gorm:"column:tenant_id;type:uuid;not null"`
	PointOfSaleId string    `gorm:"column:point_of_sale_id;type:uuid;not null"`
	Cuis          string    `gorm:"type:varchar(100);not null"`
	ValidFrom     time.Time `gorm:"not null"`
	ValidTo       *time.Time
	Active        bool `gorm:"column:is_active;default:true;not null"`
	CreatedAt     time.Time

	PointOfSale PointOfSale `gorm:"foreignKey:TenantId,PointOfSaleId;references:CompanyId,ID"`
}

func (Cuis) TableName() string { return "cuis_history" }

type ContingencyEvent struct {
	ID                string            `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantId          string            `gorm:"column:tenant_id;type:uuid;not null"`
	PointOfSaleId     string            `gorm:"type:uuid;index:idx_contingency_pos_start;not null"`
	Reason            ContingencyReason `gorm:"type:varchar(50);not null"`
	Description       *string           `gorm:"type:text"`
	StartDate         time.Time         `gorm:"index:idx_contingency_pos_start;not null"`
	EndDate           *time.Time
	SiatEventCode     *string `gorm:"type:varchar(50)"`
	SiatReceptionCode *string `gorm:"type:varchar(100)"`
	CufdId            *string `gorm:"column:cufd_id;type:uuid"`
	IsSynced          bool    `gorm:"default:false;not null"`
	SyncedAt          *time.Time
	CreatedAt         time.Time

	PointOfSale PointOfSale `gorm:"foreignKey:PointOfSaleId"`
	CufdRecord  *Cufd       `gorm:"foreignKey:TenantId,CufdId;references:TenantId,ID"`
	Invoices    []Invoice   `gorm:"foreignKey:ContingencyEventId"`
}

// Customer es el registro fiscal del receptor. Create-only: la inmutabilidad
// de los campos fiscales una vez facturado se refuerza con el trigger
// trg_customers_immutability (ver internal/repository/database/db.go).
type Customer struct {
	ID             string       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId      string       `gorm:"column:tenant_id;type:uuid;not null"`
	DocumentType   DocumentType `gorm:"type:varchar(20);not null"`
	DocumentNumber string       `gorm:"type:varchar(30);not null"`
	Complement     *string      `gorm:"type:varchar(10)"`
	Name           string       `gorm:"type:varchar(150);not null"`
	Email          *string      `gorm:"type:varchar(150)"`
	CodigoCliente  string       `gorm:"type:varchar(50);not null;default:''"`
	IsActive       bool         `gorm:"column:is_active;not null;default:true"`
	CreatedAt      time.Time

	Company  Company   `gorm:"foreignKey:CompanyId"`
	Invoices []Invoice `gorm:"foreignKey:CustomerId"`
}

type Invoice struct {
	ID                     string          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId              string          `gorm:"column:tenant_id;type:uuid;not null"`
	CustomerId             string          `gorm:"type:uuid;index;not null"`
	CustomerDocumentType   DocumentType    `gorm:"column:customer_document_type;type:varchar(20);not null"`
	CustomerDocumentNumber string          `gorm:"column:customer_document_number;type:varchar(30);not null"`
	CustomerComplement     *string         `gorm:"column:customer_complement;type:varchar(10)"`
	CustomerName           string          `gorm:"column:customer_name;type:varchar(150);not null"`
	CustomerEmail          *string         `gorm:"column:customer_email;type:varchar(150)"`
	CustomerCode           string          `gorm:"column:customer_code;type:varchar(50);not null"`
	PointOfSaleId          string          `gorm:"type:uuid;uniqueIndex:idx_pos_invoice_num,priority:1;uniqueIndex:idx_invoice_idem_key,priority:1;not null"`
	IdempotencyKey         *string         `gorm:"type:varchar(100);uniqueIndex:idx_invoice_idem_key,priority:2"`
	CufdId                 string          `gorm:"type:uuid;not null"`
	ContingencyEventId     *string         `gorm:"type:uuid"`
	InvoiceNumber          int             `gorm:"uniqueIndex:idx_pos_invoice_num,priority:2;not null"`
	Cuf                    *string         `gorm:"type:varchar(150);uniqueIndex"`
	EmissionType           EmissionType    `gorm:"type:varchar(30);default:'EN_LINEA';not null"`
	CodigoMetodoPago       int             `gorm:"default:1;not null"`
	CodigoMoneda           int             `gorm:"default:1;not null"`
	TipoCambio             decimal.Decimal `gorm:"type:numeric(18,5);default:1;not null"`
	CodigoDocumentoSector  int             `gorm:"default:1;not null"`
	Layout                 string          `gorm:"type:varchar(80)"`
	Modalidad              int             `gorm:"default:1;not null"`
	CodigoTipoFactura      int             `gorm:"default:1;not null"`
	Archivo                string          `gorm:"type:text"`
	HashArchivo            string          `gorm:"type:varchar(100)"`
	NombreEstudiante       *string         `gorm:"type:varchar(150)"`
	PeriodoFacturado       *string         `gorm:"type:varchar(30)"`
	SectorData             datatypes.JSON  `gorm:"type:jsonb"`
	AjustaFacturaId        *string         `gorm:"type:uuid;index"`
	IssueDate              time.Time       `gorm:"index;not null"`
	Subtotal               decimal.Decimal `gorm:"type:numeric(18,2);not null"`
	Discount               decimal.Decimal `gorm:"type:numeric(18,2);default:0;not null"`
	Total                  decimal.Decimal `gorm:"type:numeric(18,2);not null"`
	XmlHash                *string         `gorm:"type:varchar(100)"`
	SiatReceptionCode      *string         `gorm:"type:varchar(100)"`
	SiatMensajes           *string         `gorm:"type:text"`
	Status                 InvoiceStatus   `gorm:"type:varchar(30);default:'PENDING';index;not null"`
	MotivoAnulacion        *int
	FechaAnulacion         *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time

	Company          Company           `gorm:"foreignKey:CompanyId"`
	PointOfSale      PointOfSale       `gorm:"foreignKey:PointOfSaleId"`
	CufdRecord       Cufd              `gorm:"foreignKey:CufdId"`
	ContingencyEvent *ContingencyEvent `gorm:"foreignKey:ContingencyEventId"`
	AjustaFactura    *Invoice          `gorm:"foreignKey:AjustaFacturaId"`

	Items  []InvoiceItem  `gorm:"foreignKey:InvoiceId"`
	Events []InvoiceEvent `gorm:"foreignKey:InvoiceId"`
}

type InvoiceSequence struct {
	TenantID      string `gorm:"column:tenant_id;type:uuid;primaryKey"`
	PointOfSaleID string `gorm:"column:point_of_sale_id;type:uuid;primaryKey"`
	NextNumber    int    `gorm:"not null"`
	UpdatedAt     time.Time
}

type InvoiceItem struct {
	ID                string  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID          string  `gorm:"column:tenant_id;type:uuid;not null"`
	InvoiceId         string  `gorm:"type:uuid;index;not null"`
	Code              string  `gorm:"type:varchar(50);not null"`
	Description       string  `gorm:"type:text;not null"`
	CodigoActividad   *string `gorm:"type:varchar(20)"`
	CodigoProductoSin *string `gorm:"type:varchar(20)"`
	UnitCode          *int
	Quantity          decimal.Decimal `gorm:"type:numeric(18,5);not null"`
	UnitPrice         decimal.Decimal `gorm:"type:numeric(18,5);not null"`
	Discount          decimal.Decimal `gorm:"type:numeric(18,2);default:0;not null"`
	Subtotal          decimal.Decimal `gorm:"type:numeric(18,2);not null"`
	SectorData        datatypes.JSON  `gorm:"type:jsonb"`

	Invoice Invoice `gorm:"foreignKey:InvoiceId"`
}

type InvoiceEvent struct {
	ID        string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	InvoiceId string         `gorm:"type:uuid;index;not null"`
	TenantID  string         `gorm:"column:tenant_id;type:uuid;index;not null"`
	EventKey  *string        `gorm:"column:event_key;type:varchar(100)"`
	Type      string         `gorm:"type:varchar(50);not null"`
	Message   string         `gorm:"type:text;not null"`
	Payload   datatypes.JSON `gorm:"type:jsonb"`
	CreatedAt time.Time

	Invoice Invoice `gorm:"foreignKey:InvoiceId"`
}

// OutboxEvent is the durable hand-off between the HTTP transaction and River.
// Payload contains identifiers only; invoice data is always reloaded by the worker.
type OutboxEvent struct {
	ID            string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID      string         `gorm:"column:tenant_id;type:uuid;index;not null"`
	AggregateType string         `gorm:"column:aggregate_type;type:varchar(50);not null"`
	AggregateID   string         `gorm:"column:aggregate_id;type:uuid;not null"`
	EventType     string         `gorm:"column:event_type;type:varchar(100);not null"`
	Payload       datatypes.JSON `gorm:"type:jsonb;not null"`
	Status        string         `gorm:"type:varchar(20);index;not null"`
	Attempts      int            `gorm:"not null;default:0"`
	AvailableAt   time.Time      `gorm:"column:available_at;not null"`
	LockedAt      *time.Time     `gorm:"column:locked_at"`
	LockedBy      *string        `gorm:"column:locked_by;type:varchar(100)"`
	PublishedAt   *time.Time     `gorm:"column:published_at"`
	LastError     *string        `gorm:"column:last_error;type:text"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (OutboxEvent) TableName() string { return "outbox" }

type SentPackage struct {
	ID                    string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId             string    `gorm:"column:tenant_id;type:uuid;index:idx_sent_packages_tenant;not null"`
	PointOfSaleId         string    `gorm:"type:uuid;index:idx_sent_pkg_pos;not null"`
	Type                  string    `gorm:"type:varchar(20);not null"`
	CodigoRecepcion       string    `gorm:"type:varchar(100);uniqueIndex:idx_sent_packages_recepcion,where:codigo_recepcion <> '';not null"`
	Modalidad             int       `gorm:"not null;default:0"`
	Layout                string    `gorm:"type:varchar(80);not null;default:''"`
	Cufd                  string    `gorm:"type:text;not null;default:''"`
	CufdID                *string   `gorm:"type:uuid"`
	Cuis                  string    `gorm:"type:text;not null;default:''"`
	HashArchivo           string    `gorm:"type:varchar(100);not null"`
	CantidadFacturas      int       `gorm:"not null"`
	CodigoDocumentoSector int       `gorm:"not null"`
	CodigoTipoFactura     int       `gorm:"not null"`
	CodigoEmision         int       `gorm:"not null"`
	CodigoEvento          *int64    `gorm:"type:bigint"`
	ContingencyEventId    *string   `gorm:"type:uuid;index"`
	Status                string    `gorm:"type:varchar(30);default:'SENT';index;not null"`
	Mensajes              *string   `gorm:"type:text"`
	XmlHash               string    `gorm:"type:varchar(100);not null"`
	SentAt                time.Time `gorm:"not null"`
	ValidatedAt           *time.Time
	CreatedAt             time.Time

	Company          Company           `gorm:"foreignKey:CompanyId"`
	PointOfSale      PointOfSale       `gorm:"foreignKey:PointOfSaleId"`
	ContingencyEvent *ContingencyEvent `gorm:"foreignKey:ContingencyEventId"`
	CufdRecord       *Cufd             `gorm:"foreignKey:CompanyId,CufdID;references:TenantId,ID"`
}

// SentPackageInvoice conserva la pertenencia y el orden del envío original.
type SentPackageInvoice struct {
	TenantID      string `gorm:"column:tenant_id;type:uuid;not null"`
	InvoiceID     string `gorm:"type:uuid;primaryKey"`
	SentPackageID string `gorm:"type:uuid;not null;uniqueIndex:idx_sent_package_invoice_position,priority:1"`
	Position      int    `gorm:"not null;uniqueIndex:idx_sent_package_invoice_position,priority:2"`

	Invoice     Invoice     `gorm:"foreignKey:TenantID,InvoiceID;references:CompanyId,ID"`
	SentPackage SentPackage `gorm:"foreignKey:TenantID,SentPackageID;references:CompanyId,ID"`
}

type Certificate struct {
	ID            string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId     string    `gorm:"column:tenant_id;type:uuid;index:idx_certificates_tenant;not null"`
	PointOfSaleId *string   `gorm:"column:point_of_sale_id;type:uuid"`
	Name          string    `gorm:"column:name;type:varchar(150);not null"`
	Type          string    `gorm:"type:varchar(10);not null"`
	Status        string    `gorm:"type:varchar(20);default:'ACTIVE';index;not null"`
	NotBefore     time.Time `gorm:"not null"`
	NotAfter      time.Time `gorm:"not null"`
	Issuer        string    `gorm:"type:text"`
	Subject       string    `gorm:"type:text"`
	Thumbprint    string    `gorm:"type:varchar(100)"`
	SiatUserCode  string    `gorm:"type:varchar(50)"`
	ConfigPath    string    `gorm:"type:text"`
	RenewedFrom   *string   `gorm:"type:uuid"`
	CreatedAt     time.Time
	UpdatedAt     time.Time

	// Material de firma cifrado AES-GCM, nunca texto plano.
	EncryptedP12Password string  `gorm:"type:text;not null;default:''"`
	EncryptedBlob        []byte  `gorm:"column:encrypted_blob;type:bytea"`
	EncryptedPassword    *string `gorm:"column:encrypted_password;type:text"`
	SerialNumber         *string `gorm:"column:serial_number;type:varchar(100)"`
	P12StorageRef        string  `gorm:"type:text;not null;default:''"`
	IsActive             bool    `gorm:"column:is_active;not null;default:true"`
	UploadedAt           time.Time

	Company           Company      `gorm:"foreignKey:CompanyId"`
	RenewedFromRecord *Certificate `gorm:"foreignKey:CompanyId,RenewedFrom;references:CompanyId,ID"`
}

type CertificateNotification struct {
	ID            string  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID      string  `gorm:"column:tenant_id;type:uuid;not null;index"`
	CertificateID string  `gorm:"column:certificate_id;type:uuid;not null;uniqueIndex:idx_certificate_notification_once,priority:1"`
	ThresholdDays int     `gorm:"column:threshold_days;not null;uniqueIndex:idx_certificate_notification_once,priority:2"`
	Channel       string  `gorm:"type:varchar(20);not null;uniqueIndex:idx_certificate_notification_once,priority:3"`
	Status        string  `gorm:"type:varchar(20);not null"`
	Attempts      int     `gorm:"not null;default:1"`
	LastError     *string `gorm:"type:text"`
	DeliveredAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time

	Certificate Certificate `gorm:"foreignKey:TenantID,CertificateID;references:CompanyId,ID"`
}

func (CertificateNotification) TableName() string { return "certificate_notifications" }
