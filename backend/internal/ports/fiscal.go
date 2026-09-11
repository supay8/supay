// Package ports define los contratos que el dominio expone hacia los
// adaptadores externos (SIAT, sandbox, etc.). Ningún tipo de este paquete
// depende del SDK go-siat ni de implementaciones concretas.
package ports

import (
	"context"
	"encoding/json"
	"time"
)

// FiscalMessage es un mensaje devuelto por el servicio fiscal (SIAT o sandbox).
type FiscalMessage struct {
	Codigo      int    `json:"codigo"`
	Descripcion string `json:"descripcion"`
}

// FiscalCustomer representa los datos del receptor de un documento fiscal.
type FiscalCustomer struct {
	NombreRazonSocial            string  `json:"razonSocial"`
	CodigoTipoDocumentoIdentidad int     `json:"codigoTipoDocumentoIdentidad"`
	NumeroDocumento              string  `json:"numeroDocumento"`
	Complemento                  *string `json:"complemento,omitempty"`
	CodigoCliente                *string `json:"codigoCliente,omitempty"`
}

// FiscalItem representa una línea de detalle de un documento fiscal.
type FiscalItem struct {
	ActividadEconomica string          `json:"actividadEconomica"`
	CodigoProductoSin  int64           `json:"codigoProductoSin"`
	CodigoProducto     string          `json:"codigoProducto"`
	Descripcion        string          `json:"descripcion"`
	Cantidad           float64         `json:"cantidad"`
	UnidadMedida       int             `json:"unidadMedida"`
	PrecioUnitario     float64         `json:"precioUnitario"`
	MontoDescuento     *float64        `json:"montoDescuento,omitempty"`
	SubTotal           float64         `json:"subTotal"`
	DatosSector        json.RawMessage `json:"datosSector,omitempty"`
}

// FiscalDocument agrupa los datos necesarios para emitir un documento fiscal.
type FiscalDocument struct {
	// XML conserva el documento fiscal persistido para enviarlo sin reconstruirlo.
	XML                   string          `json:"-"`
	CodigoAmbiente        int             `json:"codigoAmbiente"`
	CodigoSistema         string          `json:"codigoSistema"`
	Nit                   string          `json:"nit"`
	Modalidad             int             `json:"modalidad"`
	NumeroFactura         int64           `json:"numeroFactura"`
	NumeroFacturaOriginal int64           `json:"numeroFacturaOriginal,omitempty"`
	CodigoSucursal        int             `json:"codigoSucursal"`
	CodigoPuntoVenta      int             `json:"codigoPuntoVenta"`
	Cuis                  string          `json:"cuis"`
	Cufd                  string          `json:"cufd"`
	CodigoControl         string          `json:"codigoControl"`
	FechaEmision          time.Time       `json:"fechaEmision"`
	Usuario               string          `json:"usuario"`
	Leyenda               string          `json:"leyenda"`
	RazonSocialEmisor     string          `json:"razonSocialEmisor"`
	Municipio             string          `json:"municipio"`
	Direccion             string          `json:"direccion"`
	Telefono              *string         `json:"telefono,omitempty"`
	CodigoMetodoPago      int             `json:"codigoMetodoPago"`
	CodigoMoneda          int             `json:"codigoMoneda"`
	TipoCambio            float64         `json:"tipoCambio"`
	MontoTotal            float64         `json:"montoTotal"`
	CodigoDocumentoSector int             `json:"codigoDocumentoSector"`
	Layout                string          `json:"layout,omitempty"`
	CodigoTipoFactura     int             `json:"codigoTipoFactura"`
	Cafc                  *string         `json:"cafc,omitempty"`
	DatosSector           json.RawMessage `json:"datosSector,omitempty"`
	NombreEstudiante      string          `json:"nombreEstudiante,omitempty"`
	PeriodoFacturado      string          `json:"periodoFacturado,omitempty"`
	Archivo               string          `json:"archivo,omitempty"`
	HashArchivo           string          `json:"hashArchivo,omitempty"`
	Cuf                   string          `json:"cuf,omitempty"`
	Cliente               FiscalCustomer  `json:"cliente"`
	Items                 []FiscalItem    `json:"items"`
	OriginalItems         []FiscalItem    `json:"original_items,omitempty"`
}

// FiscalResult es la respuesta de una emisión fiscal.
type FiscalResult struct {
	Cuf             string          `json:"cuf"`
	Transaccion     bool            `json:"transaccion"`
	CodigoEstado    int             `json:"codigoEstado"`
	CodigoRecepcion string          `json:"codigoRecepcion,omitempty"`
	Mensajes        []FiscalMessage `json:"mensajes,omitempty"`
	Xml             string          `json:"xml,omitempty"`
	XmlHash         string          `json:"xmlHash,omitempty"`
	Archivo         string          `json:"archivo,omitempty"`
}

// FiscalDocumentQuery identifica un documento ya emitido para consulta,
// anulación o reversión de anulación.
type FiscalDocumentQuery struct {
	CodigoAmbiente        int    `json:"codigoAmbiente"`
	CodigoSistema         string `json:"codigoSistema"`
	Nit                   string `json:"nit"`
	Modalidad             int    `json:"modalidad"`
	Cuf                   string `json:"cuf"`
	CodigoSucursal        int    `json:"codigoSucursal"`
	CodigoPuntoVenta      int    `json:"codigoPuntoVenta"`
	Cuis                  string `json:"cuis"`
	Cufd                  string `json:"cufd"`
	CodigoDocumentoSector int    `json:"codigoDocumentoSector"`
	CodigoTipoFactura     int    `json:"codigoTipoFactura"`
	Layout                string `json:"layout,omitempty"`
}

// FiscalDocumentResult es la respuesta sobre un documento ya emitido.
type FiscalDocumentResult struct {
	Transaccion     bool            `json:"transaccion"`
	CodigoEstado    int             `json:"codigoEstado"`
	CodigoRecepcion string          `json:"codigoRecepcion,omitempty"`
	Mensajes        []FiscalMessage `json:"mensajes,omitempty"`
}

// CredentialRequest contiene la identidad de un punto de venta para solicitar
// CUIS o CUFD.
type CredentialRequest struct {
	CodigoAmbiente   int
	CodigoSistema    string
	Nit              string
	CodigoSucursal   int
	CodigoModalidad  int
	CodigoPuntoVenta int
	Cuis             string // usado para CUFD
}

// CuisResult es la respuesta de una solicitud de CUIS.
type CuisResult struct {
	Codigo        string          `json:"codigo"`
	FechaVigencia time.Time       `json:"fecha_vigencia"`
	Transaccion   bool            `json:"transaccion"`
	Mensajes      []FiscalMessage `json:"mensajes,omitempty"`
}

// CufdResult es la respuesta de una solicitud de CUFD.
type CufdResult struct {
	Codigo        string          `json:"codigo"`
	CodigoControl string          `json:"codigo_control"`
	CodigoQR      *string         `json:"codigo_qr,omitempty"`
	Direccion     string          `json:"direccion"`
	FechaVigencia time.Time       `json:"fecha_vigencia"`
	Transaccion   bool            `json:"transaccion"`
	Mensajes      []FiscalMessage `json:"mensajes,omitempty"`
}

// FiscalEvent representa un evento significativo ante el servicio fiscal.
type FiscalEvent struct {
	CodigoAmbiente        int       `json:"codigoAmbiente"`
	CodigoSistema         string    `json:"codigoSistema"`
	Nit                   string    `json:"nit"`
	CodigoSucursal        int       `json:"codigoSucursal"`
	CodigoPuntoVenta      int       `json:"codigoPuntoVenta"`
	Cuis                  string    `json:"cuis"`
	Cufd                  string    `json:"cufd"`
	CufdEvento            string    `json:"cufdEvento"`
	CodigoMotivoEvento    int       `json:"codigoMotivoEvento"`
	Descripcion           string    `json:"descripcion"`
	FechaHoraInicioEvento time.Time `json:"fechaHoraInicioEvento"`
	FechaHoraFinEvento    time.Time `json:"fechaHoraFinEvento"`
}

// FiscalEventResult es la respuesta de registro de un evento significativo.
type FiscalEventResult struct {
	Transaccion     bool            `json:"transaccion"`
	CodigoRecepcion string          `json:"codigo_recepcion,omitempty"`
	Mensajes        []FiscalMessage `json:"mensajes,omitempty"`
}

// FiscalPackage agrupa un paquete de facturas para envío en contingencia.
type FiscalPackage struct {
	// Prepared conserva el envío opaco construido por FiscalBatchPreparer.
	// Debe entregarse al mismo adaptador sin modificar la solicitud preparada.
	Prepared              any              `json:"-"`
	CodigoAmbiente        int              `json:"codigoAmbiente"`
	CodigoSistema         string           `json:"codigoSistema"`
	Nit                   string           `json:"nit"`
	Modalidad             int              `json:"modalidad"`
	CodigoSucursal        int              `json:"codigoSucursal"`
	CodigoPuntoVenta      int              `json:"codigoPuntoVenta"`
	Cuis                  string           `json:"cuis"`
	Cufd                  string           `json:"cufd"`
	CodigoControl         string           `json:"codigoControl"`
	CodigoDocumentoSector int              `json:"codigoDocumentoSector"`
	Layout                string           `json:"layout,omitempty"`
	CodigoTipoFactura     int              `json:"codigoTipoFactura"`
	CodigoEmision         int              `json:"codigoEmision"`
	CodigoEvento          int64            `json:"codigoEvento"`
	Archivo               string           `json:"archivo,omitempty"`
	HashArchivo           string           `json:"hashArchivo,omitempty"`
	Descripcion           string           `json:"descripcion,omitempty"`
	Facturas              []FiscalDocument `json:"facturas"`
}

// FiscalPackageResult es la respuesta de envío o validación de paquete/masiva.
type FiscalPackageResult struct {
	Transaccion      bool            `json:"transaccion"`
	CodigoEstado     int             `json:"codigo_estado"`
	CodigoRecepcion  string          `json:"codigo_recepcion,omitempty"`
	Mensajes         []FiscalMessage `json:"mensajes,omitempty"`
	Archivo          string          `json:"archivo,omitempty"`
	HashArchivo      string          `json:"hash_archivo,omitempty"`
	CantidadFacturas int             `json:"cantidad_facturas"`
	Cufs             []string        `json:"cufs,omitempty"`
}

// FiscalBulk agrupa un lote de facturas para emisión masiva.
type FiscalBulk struct {
	// Prepared conserva el envío opaco construido por FiscalBatchPreparer.
	Prepared              any              `json:"-"`
	CodigoAmbiente        int              `json:"codigoAmbiente"`
	CodigoSistema         string           `json:"codigoSistema"`
	Nit                   string           `json:"nit"`
	Modalidad             int              `json:"modalidad"`
	CodigoSucursal        int              `json:"codigoSucursal"`
	CodigoPuntoVenta      int              `json:"codigoPuntoVenta"`
	Cuis                  string           `json:"cuis"`
	Cufd                  string           `json:"cufd"`
	CodigoControl         string           `json:"codigoControl"`
	CodigoDocumentoSector int              `json:"codigoDocumentoSector"`
	Layout                string           `json:"layout,omitempty"`
	CodigoTipoFactura     int              `json:"codigoTipoFactura"`
	CodigoEmision         int              `json:"codigoEmision"`
	Archivo               string           `json:"archivo,omitempty"`
	HashArchivo           string           `json:"hashArchivo,omitempty"`
	Facturas              []FiscalDocument `json:"facturas"`
}

// FiscalPurchase agrupa los datos de un paquete de facturas de compras.
type FiscalPurchase struct {
	Descripcion      string    `json:"descripcion"`
	TipoCompra       int       `json:"tipoCompra"`
	CodigoAmbiente   int       `json:"codigoAmbiente"`
	CodigoSistema    string    `json:"codigoSistema"`
	Nit              string    `json:"nit"`
	CodigoSucursal   int       `json:"codigoSucursal"`
	CodigoPuntoVenta int       `json:"codigoPuntoVenta"`
	Cuis             string    `json:"cuis"`
	Cufd             string    `json:"cufd"`
	Archivo          string    `json:"archivo"`
	HashArchivo      string    `json:"hashArchivo"`
	CantidadFacturas int       `json:"cantidadFacturas"`
	Gestion          int       `json:"gestion"`
	Periodo          int       `json:"periodo"`
	FechaEnvio       time.Time `json:"fechaEnvio"`
}

// FiscalPurchaseResult es la respuesta de recepción de compras.
type FiscalPurchaseResult struct {
	Transaccion     bool            `json:"transaccion"`
	CodigoEstado    int             `json:"codigo_estado"`
	CodigoRecepcion string          `json:"codigo_recepcion"`
	Mensajes        []FiscalMessage `json:"mensajes,omitempty"`
}

// FiscalSignRequest contiene el XML a firmar digitalmente.
type FiscalSignRequest struct {
	Xml string `json:"xml"`
}

// FiscalSignResult es el resultado de la firma digital.
type FiscalSignResult struct {
	XmlFirmado  string `json:"xmlFirmado"`
	Archivo     string `json:"archivo"`
	HashArchivo string `json:"hashArchivo"`
	Firma       string `json:"firma"`
}

// FiscalAdjustment representa un documento de ajuste (nota de crédito/débito).
type FiscalAdjustment struct {
	CodigoAmbiente        int            `json:"codigoAmbiente"`
	CodigoSistema         string         `json:"codigoSistema"`
	Nit                   string         `json:"nit"`
	Modalidad             int            `json:"modalidad"`
	NumeroFactura         int64          `json:"numeroFactura"`
	CodigoSucursal        int            `json:"codigoSucursal"`
	CodigoPuntoVenta      int            `json:"codigoPuntoVenta"`
	Cuis                  string         `json:"cuis"`
	Cufd                  string         `json:"cufd"`
	CodigoControl         string         `json:"codigoControl"`
	FechaEmision          time.Time      `json:"fechaEmision"`
	Usuario               string         `json:"usuario"`
	TipoNota              int            `json:"tipoNota"`
	CufFacturaOriginal    string         `json:"cufFacturaOriginal"`
	CodigoDocumentoSector int            `json:"codigoDocumentoSector"`
	Layout                string         `json:"layout,omitempty"`
	CodigoTipoFactura     int            `json:"codigoTipoFactura"`
	RazonSocialEmisor     string         `json:"razonSocialEmisor"`
	Municipio             string         `json:"municipio"`
	Direccion             string         `json:"direccion"`
	Telefono              *string        `json:"telefono,omitempty"`
	Cliente               FiscalCustomer `json:"cliente"`
	CodigoMetodoPago      int            `json:"codigoMetodoPago"`
	CodigoMoneda          int            `json:"codigoMoneda"`
	TipoCambio            float64        `json:"tipoCambio"`
	MontoTotal            float64        `json:"montoTotal"`
	Leyenda               string         `json:"leyenda"`
	Motivo                string         `json:"motivo"`
	Items                 []FiscalItem   `json:"items"`
}

// FiscalAdjustmentResult es la respuesta de emisión de un documento de ajuste.
type FiscalAdjustmentResult struct {
	Transaccion     bool            `json:"transaccion"`
	CodigoEstado    int             `json:"codigoEstado"`
	CodigoRecepcion string          `json:"codigoRecepcion,omitempty"`
	Mensajes        []FiscalMessage `json:"mensajes,omitempty"`
	Cuf             string          `json:"cuf,omitempty"`
	Xml             string          `json:"xml,omitempty"`
	XmlHash         string          `json:"xmlHash,omitempty"`
}

// FiscalSyncOperation identifica una operación de sincronización de catálogos.
type FiscalSyncOperation string

const (
	OpTipoPuntoVenta             FiscalSyncOperation = "tipoPuntoVenta"
	OpTipoMoneda                 FiscalSyncOperation = "tipoMoneda"
	OpTipoMetodoPago             FiscalSyncOperation = "tipoMetodoPago"
	OpTipoDocumentoIdentidad     FiscalSyncOperation = "tipoDocumentoIdentidad"
	OpTipoEmision                FiscalSyncOperation = "tipoEmision"
	OpTiposFactura               FiscalSyncOperation = "tiposFactura"
	OpTipoHabitacion             FiscalSyncOperation = "tipoHabitacion"
	OpTipoDocumentoSector        FiscalSyncOperation = "tipoDocumentoSector"
	OpUnidadMedida               FiscalSyncOperation = "unidadMedida"
	OpMotivoAnulacion            FiscalSyncOperation = "motivoAnulacion"
	OpPaisOrigen                 FiscalSyncOperation = "paisOrigen"
	OpEventosSignificativos      FiscalSyncOperation = "eventosSignificativos"
	OpMensajesServicios          FiscalSyncOperation = "mensajesServicios"
	OpActividades                FiscalSyncOperation = "actividades"
	OpProductosServicios         FiscalSyncOperation = "productosServicios"
	OpLeyendasFactura            FiscalSyncOperation = "leyendasFactura"
	OpActividadesDocumentoSector FiscalSyncOperation = "actividadesDocumentoSector"
	OpFechaHora                  FiscalSyncOperation = "fechaHora"
	OpVerificarComunicacion      FiscalSyncOperation = "verificarComunicacion"
)

// FiscalSyncOperations es el listado oficial de operaciones de sincronización.
var FiscalSyncOperations = []FiscalSyncOperation{
	OpTipoPuntoVenta,
	OpTipoMoneda,
	OpTipoMetodoPago,
	OpTipoDocumentoIdentidad,
	OpTipoEmision,
	OpTiposFactura,
	OpTipoHabitacion,
	OpTipoDocumentoSector,
	OpUnidadMedida,
	OpMotivoAnulacion,
	OpPaisOrigen,
	OpEventosSignificativos,
	OpMensajesServicios,
	OpActividades,
	OpProductosServicios,
	OpLeyendasFactura,
	OpActividadesDocumentoSector,
	OpFechaHora,
	OpVerificarComunicacion,
}

// ParseFiscalSyncOperation convierte un nombre de operación en su constante.
func ParseFiscalSyncOperation(raw string) (FiscalSyncOperation, bool) {
	op := FiscalSyncOperation(raw)
	for _, known := range FiscalSyncOperations {
		if known == op {
			return op, true
		}
	}
	return "", false
}

// FiscalSyncRequest contiene la identidad para sincronizar catálogos.
type FiscalSyncRequest struct {
	CodigoAmbiente   int    `json:"codigoAmbiente"`
	CodigoSistema    string `json:"codigoSistema"`
	Nit              string `json:"nit"`
	CodigoSucursal   int    `json:"codigoSucursal"`
	CodigoPuntoVenta int    `json:"codigoPuntoVenta"`
	Cuis             string `json:"cuis"`
}

// FiscalParametricItem es un elemento de catálogo paramétrico simple.
type FiscalParametricItem struct {
	CodigoClasificador int    `json:"codigoClasificador"`
	Descripcion        string `json:"descripcion"`
}

// FiscalSinProduct es un producto/servicio homologado del SIAT.
type FiscalSinProduct struct {
	CodigoProductoSin int64  `json:"codigoProductoSin"`
	CodigoActividad   int64  `json:"codigoActividad"`
	Descripcion       string `json:"descripcion"`
}

// FiscalActivity es una actividad económica del catálogo CAEB.
type FiscalActivity struct {
	CodigoCaeb    string `json:"codigoCaeb"`
	Descripcion   string `json:"descripcion"`
	TipoActividad string `json:"tipoActividad"`
}

// FiscalLegend es una leyenda oficial asociada a una actividad.
type FiscalLegend struct {
	CodigoActividad    string `json:"codigoActividad"`
	DescripcionLeyenda string `json:"descripcionLeyenda"`
}

// FiscalActivityDocSector relaciona actividad con documento-sector.
type FiscalActivityDocSector struct {
	CodigoActividad       string `json:"codigoActividad"`
	CodigoDocumentoSector int    `json:"codigoDocumentoSector"`
	TipoDocumentoSector   string `json:"tipoDocumentoSector"`
}

// FiscalSyncResult es la respuesta normalizada de una sincronización.
type FiscalSyncResult struct {
	Transaccion          bool                      `json:"transaccion"`
	FechaHora            time.Time                 `json:"fechaHora,omitempty"`
	Codigos              []FiscalParametricItem    `json:"codigos,omitempty"`
	Productos            []FiscalSinProduct        `json:"productos,omitempty"`
	Actividades          []FiscalActivity          `json:"actividades,omitempty"`
	Leyendas             []FiscalLegend            `json:"leyendas,omitempty"`
	ActividadesDocSector []FiscalActivityDocSector `json:"actividadesDocSector,omitempty"`
	Mensajes             []FiscalMessage           `json:"mensajes,omitempty"`
}

// FiscalService es el puerto que abstrae todas las operaciones fiscales
// (SIAT real o sandbox). El dominio/application depende de esta interfaz,
// nunca de implementaciones concretas ni del SDK go-siat.
// OfflineFiscalService is an optional capability implemented by adapters that
// can build and sign an offline invoice without contacting the SIAT.
type OfflineFiscalService interface {
	PrepareOffline(ctx context.Context, doc FiscalDocument) (FiscalResult, error)
}

// FiscalBatchPreparer valida, construye y firma los documentos de un lote sin
// contactar al SIAT. Devuelve los XML/CUF y archivos exactos para persistirlos
// antes de reservar el envío; SendBulk/SendPackage reutilizan ese mismo envío.
type FiscalBatchPreparer interface {
	PrepareBulk(ctx context.Context, bulk FiscalBulk) (FiscalBulk, error)
	PreparePackage(ctx context.Context, pkg FiscalPackage) (FiscalPackage, error)
}

type FiscalService interface {
	Emit(ctx context.Context, doc FiscalDocument) (FiscalResult, error)
	VerifyStatus(ctx context.Context, query FiscalDocumentQuery) (FiscalDocumentResult, error)
	Annul(ctx context.Context, query FiscalDocumentQuery, codigoMotivo int) (FiscalDocumentResult, error)
	RevertAnnul(ctx context.Context, query FiscalDocumentQuery) (FiscalDocumentResult, error)
	RequestCUIS(ctx context.Context, req CredentialRequest) (CuisResult, error)
	RequestCUFD(ctx context.Context, req CredentialRequest) (CufdResult, error)
	RegisterSignificantEvent(ctx context.Context, ev FiscalEvent) (FiscalEventResult, error)
	SendPackage(ctx context.Context, pkg FiscalPackage) (FiscalPackageResult, error)
	ValidatePackage(ctx context.Context, pkg FiscalPackage, codigoRecepcion string) (FiscalPackageResult, error)
	SendBulk(ctx context.Context, bulk FiscalBulk) (FiscalPackageResult, error)
	ValidateBulk(ctx context.Context, bulk FiscalBulk, codigoRecepcion string) (FiscalPackageResult, error)
	SendPurchases(ctx context.Context, p FiscalPurchase) (FiscalPurchaseResult, error)
	SignXML(ctx context.Context, req FiscalSignRequest) (FiscalSignResult, error)
	EmitAdjustment(ctx context.Context, adj FiscalAdjustment) (FiscalAdjustmentResult, error)
	Synchronize(ctx context.Context, req FiscalSyncRequest, op FiscalSyncOperation) (FiscalSyncResult, error)
}
