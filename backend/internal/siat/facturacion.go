package siat

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	goSiat "github.com/ron86i/go-siat/v2"
	"github.com/ron86i/go-siat/v2/pkg/models"
	"github.com/ron86i/go-siat/v2/pkg/utils"
)

// ClienteFactura son los datos del comprador que requiere la cabecera de la
// factura (códigos de catálogo SIN ya mapeados).
type ClienteFactura struct {
	NombreRazonSocial            string  `json:"razonSocial"`
	CodigoTipoDocumentoIdentidad int     `json:"codigoTipoDocumentoIdentidad"`
	NumeroDocumento              string  `json:"numeroDocumento"`
	Complemento                  *string `json:"complemento,omitempty"`
	CodigoCliente                string  `json:"codigoCliente"`
}

// ItemFactura es una línea de detalle ya mapeada a los catálogos del SIN.
type ItemFactura struct {
	ActividadEconomica string   `json:"actividadEconomica"`
	CodigoProductoSin  int64    `json:"codigoProductoSin"`
	CodigoProducto     string   `json:"codigoProducto"`
	Descripcion        string   `json:"descripcion"`
	Cantidad           float64  `json:"cantidad"`
	UnidadMedida       int      `json:"unidadMedida"`
	PrecioUnitario     float64  `json:"precioUnitario"`
	MontoDescuento     *float64 `json:"montoDescuento,omitempty"`
	SubTotal           float64  `json:"subTotal"`
}

// SolicitudFactura agrupa los prerrequisitos para emitir una factura de
// compraventa: CUIS/CUFD vigentes, identidad del emisor y cliente/ítems
// mapeados a los catálogos sincronizados del SIN.
type SolicitudFactura struct {
	CodigoAmbiente   int       `json:"codigoAmbiente"`
	CodigoSistema    string    `json:"codigoSistema"`
	Nit              string    `json:"nit"`
	Modalidad        int       `json:"modalidad"`
	NumeroFactura    int64     `json:"numeroFactura"`
	CodigoSucursal   int       `json:"codigoSucursal"`
	CodigoPuntoVenta int       `json:"codigoPuntoVenta"`
	Cuis             string    `json:"cuis"`
	Cufd             string    `json:"cufd"`
	CodigoControl    string    `json:"codigoControl"`
	FechaEmision     time.Time `json:"fechaEmision"`
	Usuario          string    `json:"usuario"`
	Leyenda          string    `json:"leyenda"`

	RazonSocialEmisor string  `json:"razonSocialEmisor"`
	Municipio         string  `json:"municipio"`
	Direccion         string  `json:"direccion"`
	Telefono          *string `json:"telefono,omitempty"`

	CodigoMetodoPago int     `json:"codigoMetodoPago"`
	CodigoMoneda     int     `json:"codigoMoneda"`
	TipoCambio       float64 `json:"tipoCambio"`
	MontoTotal       float64 `json:"montoTotal"`

	// CodigoDocumentoSector identifica el diseño de factura del SIAT (1 =
	// compraventa, 11 = sector educativo). 0 se interpreta como compraventa.
	CodigoDocumentoSector int `json:"codigoDocumentoSector"`
	// Layout selecciona una variante cuando un mismo código tiene más de un
	// builder, como ocurre con el sector 24.
	Layout string `json:"layout,omitempty"`
	// CodigoTipoFactura es el override opcional del tipoFacturaDocumento
	// (catálogo del SIN: 1 = con crédito fiscal, 2 = sin derecho, 3 = nota de
	// ajuste). Si es 0 se deriva del perfil del documento-sector.
	CodigoTipoFactura int `json:"codigoTipoFactura"`
	// DatosSector contiene los campos específicos del documento-sector
	// (nombreEstudiante/periodoFacturado para el 11, montos y referencia para
	// notas, pasajero para boletos aéreos, etc.), validados contra el registro
	// de perfiles antes de construir el XML.
	DatosSector json.RawMessage `json:"datosSector,omitempty"`
	// NombreEstudiante y PeriodoFacturado son la forma legada de los campos
	// específicos del documento-sector 11 (FACTURA SECTORES EDUCATIVOS);
	// siguen aceptándose pero se mapean a datosSector.
	NombreEstudiante string `json:"nombreEstudiante,omitempty"`
	PeriodoFacturado string `json:"periodoFacturado,omitempty"`
	// Archivo y HashArchivo se usan directamente para sectores sin builder,
	// actualmente el sector 33.
	Archivo     string `json:"archivo,omitempty"`
	HashArchivo string `json:"hashArchivo,omitempty"`
	Cuf         string `json:"cuf,omitempty"`

	Cliente ClienteFactura `json:"cliente"`
	Items   []ItemFactura  `json:"items"`
}

// ResultadoEmision es la respuesta procesada de recepcionFactura junto con el
// CUF generado y el XML (firmado en modalidad electrónica) que se envió.
type ResultadoEmision struct {
	Cuf             string    `json:"cuf"`
	Transaccion     bool      `json:"transaccion"`
	CodigoEstado    int       `json:"codigoEstado"`
	CodigoRecepcion string    `json:"codigoRecepcion,omitempty"`
	Mensajes        []Mensaje `json:"mensajes,omitempty"`
	Xml             string    `json:"xml,omitempty"`
	XmlHash         string    `json:"xmlHash,omitempty"`
	// Archivo es la cadena Base64 del XML de la factura comprimido en GZip tal
	// como viajó en el campo archivo de recepcionFactura (auditoría).
	Archivo string `json:"archivo,omitempty"`
}

// SolicitudDocumento identifica un documento ya emitido ante el SIAT para las
// operaciones de consulta de estado, anulación y reversión de anulación.
type SolicitudDocumento struct {
	CodigoAmbiente   int    `json:"codigoAmbiente"`
	CodigoSistema    string `json:"codigoSistema"`
	Nit              string `json:"nit"`
	Modalidad        int    `json:"modalidad"`
	Cuf              string `json:"cuf"`
	CodigoSucursal   int    `json:"codigoSucursal"`
	CodigoPuntoVenta int    `json:"codigoPuntoVenta"`
	Cuis             string `json:"cuis"`
	Cufd             string `json:"cufd"`
	// CodigoDocumentoSector debe coincidir con el usado al emitir (1 =
	// compraventa, 11 = sector educativo).
	CodigoDocumentoSector int    `json:"codigoDocumentoSector"`
	CodigoTipoFactura     int    `json:"codigoTipoFactura"`
	Layout                string `json:"layout,omitempty"`
}

// ResultadoDocumento es la respuesta procesada del SIAT para una operación
// sobre un documento ya emitido (verificar estado, anular o revertir).
type ResultadoDocumento struct {
	Transaccion     bool      `json:"transaccion"`
	CodigoEstado    int       `json:"codigoEstado"`
	CodigoRecepcion string    `json:"codigoRecepcion,omitempty"`
	Mensajes        []Mensaje `json:"mensajes,omitempty"`
}

// EmitirFactura construye, firma (si corresponde) y envía un documento al SIAT
// usando el SDK go-siat, para cualquier documento-sector registrado en el
// catálogo de perfiles (sectores.go). Las facturas van por recepcionFactura de
// su fachada; los documentos de ajuste (24/29/47/48) por
// recepcionDocumentoAjuste. El SDK serializa el XML, lo firma con XMLDSig cuando
// la modalidad es electrónica, lo comprime en gzip y calcula el hash SHA-256.
func (s *Service) EmitirFactura(ctx context.Context, req SolicitudFactura) (*ResultadoEmision, error) {
	if s.sdk == nil {
		return nil, fmt.Errorf("siat emision: servicio SIAT no inicializado")
	}
	if err := s.applyIdentity(&req); err != nil {
		return nil, err
	}
	if err := req.validate(); err != nil {
		return nil, err
	}

	perfil, err := PerfilSectorLayout(req.CodigoDocumentoSector, req.Layout)
	if err != nil {
		return nil, fmt.Errorf("siat emision: %w", err)
	}

	// CUF: debe usar el MISMO timestamp de la cabecera y el MISMO correlativo,
	// y el tipoFacturaDocumento derivado del perfil (mismo valor que viajará en
	// la solicitud de recepción). En la emisión individual el CUF se compone
	// siempre con emisión en línea.
	var factura any
	var cuf string
	var tipoDoc int
	var archivo, hash string
	var xmlSent []byte
	tipoDoc = perfil.TipoDocumentoResuelto(req.CodigoTipoFactura)
	if perfil.HasBuilder() {
		factura, cuf, tipoDoc, err = buildFacturaSDK(req, goSiat.EmisionOnline)
		if err != nil {
			return nil, err
		}
		xmlData, marshalErr := xml.Marshal(factura)
		if marshalErr != nil {
			return nil, fmt.Errorf("siat emision: no se pudo serializar la factura: %w", marshalErr)
		}
		xmlData = removeEmptyOptionalFacturaFields(xmlData)
		xmlToSend := xmlData
		if req.Modalidad == ModalidadElectronica {
			xmlToSend, err = s.sdk.Config().SignXML(xmlData)
			if err != nil {
				return nil, fmt.Errorf("siat emision: no se pudo firmar el XML: %w", err)
			}
		}
		archivo, hash, err = empaquetaArchivo(xmlToSend)
		if err != nil {
			return nil, fmt.Errorf("siat emision: %w", err)
		}
		xmlSent = xmlToSend
	} else {
		archivo = strings.TrimSpace(req.Archivo)
		hash = strings.TrimSpace(req.HashArchivo)
		cuf = strings.TrimSpace(req.Cuf)
	}

	ctx = withDynamicConfig(ctx, s.sdk.Config(), 0, "", "")

	var resp any
	if perfil.EsAjuste() {
		rcp := models.NewRecepcionDocumentoAjusteBuilder().
			WithCodigoModalidad(req.Modalidad).
			WithCodigoSucursal(req.CodigoSucursal).
			WithCodigoPuntoVenta(req.CodigoPuntoVenta).
			WithCodigoDocumentoSector(perfil.Codigo).
			WithCodigoEmision(goSiat.EmisionOnline).
			WithTipoFacturaDocumento(tipoDoc).
			WithCuis(req.Cuis).
			WithCufd(req.Cufd).
			WithFechaEnvio(time.Now().In(LaPaz)).
			WithArchivo(archivo).
			WithHashArchivo(hash)
		resp, err = s.recepcionDocumentoAjusteEnvio(ctx, rcp.Build())
	} else {
		rcp := models.NewRecepcionFacturaBuilder().
			WithCodigoModalidad(req.Modalidad).
			WithCodigoSucursal(req.CodigoSucursal).
			WithCodigoPuntoVenta(req.CodigoPuntoVenta).
			WithCodigoDocumentoSector(perfil.Codigo).
			WithCodigoEmision(goSiat.EmisionOnline).
			WithTipoFacturaDocumento(tipoDoc).
			WithCuis(req.Cuis).
			WithCufd(req.Cufd).
			WithFechaEnvio(time.Now().In(LaPaz)).
			WithArchivo(archivo).
			WithHashArchivo(hash)
		resp, err = s.recepcionFacturaParaPerfil(ctx, perfil, req.Modalidad, rcp.Build())
	}
	if err != nil {
		return nil, fmt.Errorf("siat emision: %w", err)
	}
	// Nota: RespuestaRecepcion no implementa common.Result, por lo que
	// goSiat.Verify no aplica; la verificación es manual (Transaccion/CodigoEstado).
	transaccion, codigoEstado, codigoRecepcion, mensajes, err := extraerResultadoFacturacion(resp)
	if err != nil {
		return nil, fmt.Errorf("siat emision: %w", err)
	}
	return &ResultadoEmision{
		Cuf:             cuf,
		Transaccion:     transaccion,
		CodigoEstado:    codigoEstado,
		CodigoRecepcion: codigoRecepcion,
		Mensajes:        mensajes,
		Xml:             string(xmlSent),
		XmlHash:         hash,
		Archivo:         archivo,
	}, nil
}

// ResultadoFirma es el resultado de la firma digital (Etapa VIII): el XML
// firmado con el certificado (XAdES-BES) junto con el par archivo/hashArchivo
// tal como se enviaría en recepcionFactura.
type ResultadoFirma struct {
	XmlFirmado  string `json:"xmlFirmado"`
	Archivo     string `json:"archivo"`
	HashArchivo string `json:"hashArchivo"`
	Firma       string `json:"firma"`
}

// FirmarFacturaXML firma digitalmente el XML de una factura con el certificado
// configurado (P12 o PEM) usando el SDK go-siat (utils.SignWithP12Bytes /
// SignXMLBytes, firma XAdES-BES envolvente) y devuelve el XML firmado junto
// con el archivo gzip+Base64 y su hash SHA-256 (archivo/hashArchivo del SIAT).
func (s *Service) FirmarFacturaXML(ctx context.Context, xml string) (*ResultadoFirma, error) {
	if s.sdk == nil {
		return nil, fmt.Errorf("siat firma: servicio SIAT no inicializado")
	}
	if strings.TrimSpace(xml) == "" {
		return nil, fmt.Errorf("siat firma: el XML a firmar es obligatorio")
	}
	if s.sdk.Config().CredentialSign.GetType() == "UNKNOWN" {
		return nil, fmt.Errorf("siat firma: no se configuraron credenciales de firma digital (P12 o PEM)")
	}

	signed, err := s.sdk.Config().SignXML([]byte(xml))
	if err != nil {
		return nil, fmt.Errorf("siat firma: %w", err)
	}

	archivo, hash, err := empaquetaArchivo(signed)
	if err != nil {
		return nil, fmt.Errorf("siat firma: %w", err)
	}

	return &ResultadoFirma{
		XmlFirmado:  string(signed),
		Archivo:     archivo,
		HashArchivo: hash,
		Firma:       "XAdES-BES",
	}, nil
}

// Valores por defecto seguros para los campos opcionales de la cabecera/detalle:
// el SIAT rechaza el atributo xsi:nil="true" que el SDK emite cuando un puntero
// es nil, por lo que siempre se envían estos valores neutros en su lugar.
var (
	emptyStr  = ""
	zeroFloat = 0.0
	zeroInt   = 0
	zeroInt64 = int64(0)
)

func optionalStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	clean := strings.TrimSpace(*value)
	if clean == "" {
		return nil
	}
	return &clean
}

func removeEmptyOptionalFacturaFields(data []byte) []byte {
	for _, field := range []string{"telefono", "complemento", "cafc", "montoDescuentoCreditoDebito"} {
		data = regexp.MustCompile(`<`+field+`(?:\s[^>]*)?></`+field+`>`).ReplaceAll(data, nil)
		data = regexp.MustCompile(`<`+field+`(?:\s[^>]*)?/>`).ReplaceAll(data, nil)
		data = regexp.MustCompile(`(?s)<`+field+`(?:\s[^>]*)?>\s*</`+field+`>`).ReplaceAll(data, nil)
	}
	return data
}

// descuentoPtr devuelve un puntero seguro para el MontoDescuento de un ítem.
func descuentoPtr(v *float64) *float64 {
	if v == nil {
		return &zeroFloat
	}
	return v
}

// buildFacturaSDK construye el documento del SDK para una SolicitudFactura de
// CUALQUIER documento-sector registrado, junto con el CUF generado (con el
// timestamp y correlativo de la cabecera) y el tipoFacturaDocumento derivado
// del perfil. codigoEmision define cómo se compone el CUF: EmisionOnline para
// la emisión individual y EmisionOffline para facturas que viajan en paquete o
// emisión masiva.
func buildFacturaSDK(req SolicitudFactura, codigoEmision int) (factura any, cuf string, tipoDoc int, err error) {
	defer func() {
		// La capa reflexiva (builder_reflex.go) reporta incompatibilidades de
		// campos con panics: se convierten en errores normales aquí.
		if r := recover(); r != nil {
			factura, cuf, tipoDoc, err = nil, "", 0, fmt.Errorf("siat sectores %d: %v", req.CodigoDocumentoSector, r)
		}
	}()
	perfil, err := PerfilSectorLayout(req.CodigoDocumentoSector, req.Layout)
	if err != nil {
		return nil, "", 0, err
	}
	if err := perfil.ValidarModalidad(req.Modalidad); err != nil {
		return nil, "", 0, err
	}
	if req.CodigoDocumentoSector != perfil.Codigo {
		return nil, "", 0, fmt.Errorf("siat sectores: código de solicitud %d no coincide con cabecera %d", req.CodigoDocumentoSector, perfil.Codigo)
	}
	if !perfil.HasBuilder() {
		return nil, "", 0, fmt.Errorf("siat sectores %d: no tiene builder; use Archivo y HashArchivo", perfil.Codigo)
	}
	tipoDoc = perfil.TipoDocumentoResuelto(req.CodigoTipoFactura)

	doc, err := perfil.adapter.Prepare(perfil, req)
	if err != nil {
		return nil, "", 0, err
	}

	nit := parseNit(req.Nit)
	cuf, err = utils.NewCUF().
		WithNit(nit).
		WithFechaHora(req.FechaEmision).
		WithSucursal(req.CodigoSucursal).
		WithModalidad(req.Modalidad).
		WithTipoEmision(codigoEmision).
		WithTipoFactura(tipoDoc).
		WithTipoDocumentoSector(perfil.Codigo).
		WithNumeroFactura(req.NumeroFactura).
		WithPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoControl(req.CodigoControl).
		Generate()
	if err != nil {
		return nil, "", 0, fmt.Errorf("siat sectores %d: cuf: %w", perfil.Codigo, err)
	}

	factura = perfil.adapter.Build(perfil, doc, cuf)
	if err := validarCodigoDocumentoSectorFactura(req.CodigoDocumentoSector, perfil.Codigo, factura); err != nil {
		return nil, "", 0, err
	}
	return factura, cuf, tipoDoc, nil
}

// validarCodigoDocumentoSectorFactura protege la correspondencia entre el
// código solicitado, el perfil seleccionado y el código que terminó en la
// cabecera XML. El SIAT rechaza cualquier discrepancia con el código 932.
func validarCodigoDocumentoSectorFactura(codigoSolicitud, codigoPerfil int, factura any) error {
	if codigoSolicitud != codigoPerfil {
		return fmt.Errorf("siat sectores: código de solicitud %d no coincide con el perfil %d", codigoSolicitud, codigoPerfil)
	}
	codigoCabecera, err := codigoDocumentoSectorXML(factura)
	if err != nil {
		return fmt.Errorf("siat sectores %d: no se pudo verificar codigoDocumentoSector de la cabecera: %w", codigoPerfil, err)
	}
	if codigoCabecera != codigoPerfil {
		return fmt.Errorf("siat sectores: código de cabecera %d no coincide con el perfil %d", codigoCabecera, codigoPerfil)
	}
	return nil
}

func codigoDocumentoSectorXML(factura any) (int, error) {
	data, err := xml.Marshal(factura)
	if err != nil {
		return 0, err
	}
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		token, err := decoder.Token()
		if err != nil {
			return 0, fmt.Errorf("campo ausente: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "codigoDocumentoSector" {
			continue
		}
		var codigo int
		if err := decoder.DecodeElement(&codigo, &start); err != nil {
			return 0, err
		}
		return codigo, nil
	}
}

// fusionarCamposLegados mapea los campos legados nombreEstudiante/periodoFacturado
// a datos_sector cuando el perfil es educativo (11/46) y no vinieron en él. Se
// ejecuta ANTES de ValidarDatosSector para que la validación vea los campos ya
// fusionados.
func fusionarCamposLegados(perfil *SectorProfile, req SolicitudFactura) []byte {
	if (perfil.Codigo != SectorEducativo && perfil.Codigo != 46) ||
		(req.NombreEstudiante == "" && req.PeriodoFacturado == "") {
		return req.DatosSector
	}
	base := map[string]any{}
	if len(req.DatosSector) > 0 {
		if err := json.Unmarshal(req.DatosSector, &base); err != nil {
			return req.DatosSector // el error real lo reportará ValidarDatosSector
		}
	}
	if _, ok := base["nombre_estudiante"]; !ok && req.NombreEstudiante != "" {
		base["nombre_estudiante"] = strings.TrimSpace(req.NombreEstudiante)
	}
	if _, ok := base["periodo_facturado"]; !ok && req.PeriodoFacturado != "" {
		base["periodo_facturado"] = strings.TrimSpace(req.PeriodoFacturado)
	}
	fusionado, err := json.Marshal(base)
	if err != nil {
		return req.DatosSector
	}
	return fusionado
}

// VerificarEstado consulta al SIAT el estado real de un documento ya emitido
// (verificacionEstadoFactura) en la fachada que atiende a su documento-sector;
// los documentos de ajuste van por el servicio DocumentoAjuste.
func (s *Service) VerificarEstado(ctx context.Context, req SolicitudDocumento) (*ResultadoDocumento, error) {
	if s.sdk == nil {
		return nil, fmt.Errorf("siat verificacion: servicio SIAT no inicializado")
	}
	if err := s.applyDocumentIdentity(&req); err != nil {
		return nil, err
	}
	if err := req.validate(); err != nil {
		return nil, err
	}
	perfil, err := PerfilSectorLayout(req.sector(), req.Layout)
	if err != nil {
		return nil, fmt.Errorf("siat verificacion: %w", err)
	}
	tipoDoc := perfil.TipoDocumentoResuelto(req.CodigoTipoFactura)

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	if perfil.EsAjuste() {
		request := models.NewVerificacionEstadoDocumentoAjusteBuilder().
			WithCodigoSucursal(req.CodigoSucursal).
			WithCodigoPuntoVenta(req.CodigoPuntoVenta).
			WithCodigoDocumentoSector(perfil.Codigo).
			WithCodigoEmision(goSiat.EmisionOnline).
			WithTipoFacturaDocumento(tipoDoc).
			WithCuf(req.Cuf).
			WithCuis(req.Cuis).
			WithCufd(req.Cufd).
			Build()
		resp, err := s.sdk.DocumentoAjuste().VerificacionEstadoDocumentoAjuste(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("siat verificacion: %w", err)
		}
		return resultadoDocumento(resp, "siat verificacion")
	}

	request := models.NewVerificacionEstadoFacturaBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoDocumentoSector(perfil.Codigo).
		WithCodigoEmision(goSiat.EmisionOnline).
		WithTipoFacturaDocumento(tipoDoc).
		WithCuf(req.Cuf).
		WithCuis(req.Cuis).
		WithCufd(req.Cufd).
		WithCodigoModalidad(req.Modalidad).
		WithNit(parseNit(req.Nit)).
		Build()

	resp, err := s.verificacionEstadoParaPerfil(ctx, perfil, req.Modalidad, request)
	if err != nil {
		return nil, fmt.Errorf("siat verificacion: %w", err)
	}

	transaccion, codigoEstado, codigoRecepcion, mensajes, err := extraerResultadoFacturacion(resp)
	if err != nil {
		return nil, fmt.Errorf("siat verificacion: %w", err)
	}
	return &ResultadoDocumento{
		Transaccion:     transaccion,
		CodigoEstado:    codigoEstado,
		CodigoRecepcion: codigoRecepcion,
		Mensajes:        mensajes,
	}, nil
}

// AnularFactura anula un documento ya emitido ante el SIAT (anulacionFactura o
// anulacionDocumentoAjuste) indicando el motivo del catálogo sincronizado
// motivoAnulacion.
func (s *Service) AnularFactura(ctx context.Context, req SolicitudDocumento, codigoMotivo int) (*ResultadoDocumento, error) {
	if s.sdk == nil {
		return nil, fmt.Errorf("siat anulacion: servicio SIAT no inicializado")
	}
	if err := s.applyDocumentIdentity(&req); err != nil {
		return nil, err
	}
	if err := req.validate(); err != nil {
		return nil, err
	}
	perfil, err := PerfilSectorLayout(req.sector(), req.Layout)
	if err != nil {
		return nil, fmt.Errorf("siat anulacion: %w", err)
	}
	tipoDoc := perfil.TipoDocumentoResuelto(req.CodigoTipoFactura)

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	if perfil.EsAjuste() {
		request := models.NewAnulacionDocumentoAjusteBuilder().
			WithCodigoSucursal(req.CodigoSucursal).
			WithCodigoPuntoVenta(req.CodigoPuntoVenta).
			WithCodigoDocumentoSector(perfil.Codigo).
			WithCodigoEmision(goSiat.EmisionOnline).
			WithTipoFacturaDocumento(tipoDoc).
			WithCuf(req.Cuf).
			WithCuis(req.Cuis).
			WithCufd(req.Cufd).
			WithCodigoMotivo(codigoMotivo).
			Build()
		resp, err := s.sdk.DocumentoAjuste().AnulacionDocumentoAjuste(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("siat anulacion: %w", err)
		}
		return resultadoDocumento(resp, "siat anulacion")
	}

	request := models.NewAnulacionFacturaBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoDocumentoSector(perfil.Codigo).
		WithCodigoEmision(goSiat.EmisionOnline).
		WithTipoFacturaDocumento(tipoDoc).
		WithCuf(req.Cuf).
		WithCuis(req.Cuis).
		WithCufd(req.Cufd).
		WithCodigoMotivo(codigoMotivo).
		WithCodigoModalidad(req.Modalidad).
		WithCodigoAmbiente(req.CodigoAmbiente).
		WithCodigoSistema(req.CodigoSistema).
		WithNit(parseNit(req.Nit)).
		Build()

	resp, err := s.anulacionParaPerfil(ctx, perfil, req.Modalidad, request)
	if err != nil {
		return nil, fmt.Errorf("siat anulacion: %w", err)
	}

	transaccion, codigoEstado, codigoRecepcion, mensajes, err := extraerResultadoFacturacion(resp)
	if err != nil {
		return nil, fmt.Errorf("siat anulacion: %w", err)
	}
	return &ResultadoDocumento{
		Transaccion:     transaccion,
		CodigoEstado:    codigoEstado,
		CodigoRecepcion: codigoRecepcion,
		Mensajes:        mensajes,
	}, nil
}

// RevertirAnulacion revierte una anulación previamente aceptada por el SIAT
// (reversionAnulacionFactura / reversionAnulacionDocumentoAjuste), devolviendo
// el documento a su estado anterior.
func (s *Service) RevertirAnulacion(ctx context.Context, req SolicitudDocumento) (*ResultadoDocumento, error) {
	if s.sdk == nil {
		return nil, fmt.Errorf("siat reversion anulacion: servicio SIAT no inicializado")
	}
	if err := s.applyDocumentIdentity(&req); err != nil {
		return nil, err
	}
	if err := req.validate(); err != nil {
		return nil, err
	}
	perfil, err := PerfilSectorLayout(req.sector(), req.Layout)
	if err != nil {
		return nil, fmt.Errorf("siat reversion anulacion: %w", err)
	}
	tipoDoc := perfil.TipoDocumentoResuelto(req.CodigoTipoFactura)

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	if perfil.EsAjuste() {
		request := models.NewReversionAnulacionDocumentoAjusteBuilder().
			WithCodigoSucursal(req.CodigoSucursal).
			WithCodigoPuntoVenta(req.CodigoPuntoVenta).
			WithCodigoDocumentoSector(perfil.Codigo).
			WithCodigoEmision(goSiat.EmisionOnline).
			WithTipoFacturaDocumento(tipoDoc).
			WithCuf(req.Cuf).
			WithCuis(req.Cuis).
			WithCufd(req.Cufd).
			Build()
		resp, err := s.sdk.DocumentoAjuste().ReversionAnulacionDocumentoAjuste(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("siat reversion anulacion: %w", err)
		}
		return resultadoDocumento(resp, "siat reversion anulacion")
	}

	request := models.NewReversionAnulacionFacturaBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoDocumentoSector(perfil.Codigo).
		WithCodigoEmision(goSiat.EmisionOnline).
		WithTipoFacturaDocumento(tipoDoc).
		WithCuf(req.Cuf).
		WithCuis(req.Cuis).
		WithCufd(req.Cufd).
		WithCodigoModalidad(req.Modalidad).
		WithCodigoAmbiente(req.CodigoAmbiente).
		WithCodigoSistema(req.CodigoSistema).
		WithNit(parseNit(req.Nit)).
		Build()

	resp, err := s.reversionParaPerfil(ctx, perfil, req.Modalidad, request)
	if err != nil {
		return nil, fmt.Errorf("siat reversion anulacion: %w", err)
	}

	transaccion, codigoEstado, codigoRecepcion, mensajes, err := extraerResultadoFacturacion(resp)
	if err != nil {
		return nil, fmt.Errorf("siat reversion anulacion: %w", err)
	}
	return &ResultadoDocumento{
		Transaccion:     transaccion,
		CodigoEstado:    codigoEstado,
		CodigoRecepcion: codigoRecepcion,
		Mensajes:        mensajes,
	}, nil
}

// extraerResultadoFacturacion lee Transaccion, CodigoEstado, CodigoRecepcion y
// MensajesList de la RespuestaServicioFacturacion común a las respuestas de
// facturación del SDK (recepción, verificación, anulación y reversión). Se usa
// reflexión porque los tipos de respuesta viven en paquetes internos del SDK.
func extraerResultadoFacturacion(resp any) (transaccion bool, codigoEstado int, codigoRecepcion string, mensajes []Mensaje, err error) {
	log.Println("[LOG] extraerResultadoFacturacion")
	log.Println(resp)
	v := reflect.ValueOf(resp)
	if !v.IsValid() || v.Kind() != reflect.Pointer {
		return false, 0, "", nil, fmt.Errorf("respuesta del SIAT inválida")
	}
	body := v.Elem().FieldByName("Body")
	if !body.IsValid() {
		return false, 0, "", nil, fmt.Errorf("respuesta del SIAT sin Body")
	}
	content := body.FieldByName("Content")
	if !content.IsValid() {
		return false, 0, "", nil, fmt.Errorf("respuesta del SIAT sin Content")
	}
	rs := content.FieldByName("RespuestaServicioFacturacion")
	if !rs.IsValid() {
		return false, 0, "", nil, fmt.Errorf("respuesta del SIAT sin RespuestaServicioFacturacion")
	}
	if f := rs.FieldByName("Transaccion"); f.IsValid() {
		transaccion = f.Bool()
	}
	if f := rs.FieldByName("CodigoEstado"); f.IsValid() {
		codigoEstado = int(f.Int())
	}
	if f := rs.FieldByName("CodigoRecepcion"); f.IsValid() {
		codigoRecepcion = f.String()
	}
	if f := rs.FieldByName("MensajesList"); f.IsValid() {
		mensajes = toMensajes(f.Interface())
	}
	return transaccion, codigoEstado, codigoRecepcion, mensajes, nil
}

// resultadoDocumento convierte una respuesta cruda del SIAT en ResultadoDocumento.
func resultadoDocumento(resp any, contexto string) (*ResultadoDocumento, error) {
	transaccion, codigoEstado, codigoRecepcion, mensajes, err := extraerResultadoFacturacion(resp)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", contexto, err)
	}
	return &ResultadoDocumento{
		Transaccion:     transaccion,
		CodigoEstado:    codigoEstado,
		CodigoRecepcion: codigoRecepcion,
		Mensajes:        mensajes,
	}, nil
}

// empaquetaArchivo comprime los datos en GZip, los codifica en Base64 y calcula
// el hash SHA-256 (hex) del archivo comprimido: el par archivo/hashArchivo que
// exige recepcionFactura del SIAT. Delega a utils.CompressAndHash del SDK.
func empaquetaArchivo(data []byte) (archivo, hash string, err error) {
	hash, encoded, err := utils.CompressAndHash(data)
	if err != nil {
		return "", "", fmt.Errorf("no se pudo comprimir el XML: %w", err)
	}
	return encoded, hash, nil
}

func (s SolicitudDocumento) sector() int {
	if s.CodigoDocumentoSector <= 0 {
		return 1
	}
	return s.CodigoDocumentoSector
}

func (s SolicitudDocumento) tipoFactura() int {
	if s.CodigoTipoFactura <= 0 {
		return 1
	}
	return s.CodigoTipoFactura
}

func (s SolicitudFactura) validate() error {
	if s.CodigoAmbiente != 0 && s.CodigoAmbiente != AmbienteProduccion && s.CodigoAmbiente != AmbientePruebas {
		return fmt.Errorf("siat emision: codigoAmbiente inválido")
	}
	if s.Modalidad != ModalidadElectronica && s.Modalidad != ModalidadComputarizada {
		return fmt.Errorf("siat emision: modalidad inválida (%d)", s.Modalidad)
	}
	perfil, err := PerfilSectorLayout(s.CodigoDocumentoSector, s.Layout)
	if err != nil {
		return fmt.Errorf("siat emision: %w", err)
	}
	if err := perfil.ValidarModalidad(s.Modalidad); err != nil {
		return err
	}
	if !perfil.HasBuilder() {
		if strings.TrimSpace(s.Archivo) == "" || strings.TrimSpace(s.HashArchivo) == "" {
			return fmt.Errorf("siat emision sector %d: archivo y hashArchivo son obligatorios porque no existe builder", perfil.Codigo)
		}
		return nil
	}
	if s.NumeroFactura <= 0 {
		return fmt.Errorf("siat emision: numeroFactura debe ser mayor a cero")
	}
	if s.CodigoSucursal < 0 || s.CodigoPuntoVenta < 0 {
		return fmt.Errorf("siat emision: codigoSucursal y codigoPuntoVenta deben ser >= 0")
	}
	if strings.TrimSpace(s.Cuis) == "" || strings.TrimSpace(s.Cufd) == "" || strings.TrimSpace(s.CodigoControl) == "" {
		return fmt.Errorf("siat emision: cuis, cufd y codigoControl son obligatorios")
	}
	if s.FechaEmision.IsZero() {
		return fmt.Errorf("siat emision: fechaEmision es obligatoria")
	}
	if strings.TrimSpace(s.Leyenda) == "" {
		return fmt.Errorf("siat emision: leyenda es obligatoria")
	}
	if strings.TrimSpace(s.RazonSocialEmisor) == "" {
		return fmt.Errorf("siat emision: razonSocialEmisor es obligatoria")
	}
	if strings.TrimSpace(s.Municipio) == "" {
		return fmt.Errorf("siat emision: municipio es obligatorio")
	}
	if strings.TrimSpace(s.Direccion) == "" {
		return fmt.Errorf("siat emision: direccion es obligatoria")
	}
	if s.CodigoMetodoPago <= 0 || s.CodigoMoneda <= 0 || s.TipoCambio <= 0 {
		return fmt.Errorf("siat emision: codigoMetodoPago, codigoMoneda y tipoCambio deben ser mayores a cero")
	}
	if s.MontoTotal < 0 {
		return fmt.Errorf("siat emision: montoTotal no puede ser negativo")
	}
	if strings.TrimSpace(s.Cliente.NombreRazonSocial) == "" {
		return fmt.Errorf("siat emision: nombre del cliente es obligatorio")
	}
	if s.Cliente.CodigoTipoDocumentoIdentidad <= 0 {
		return fmt.Errorf("siat emision: codigoTipoDocumentoIdentidad debe ser mayor a cero")
	}
	if strings.TrimSpace(s.Cliente.NumeroDocumento) == "" {
		return fmt.Errorf("siat emision: numeroDocumento del cliente es obligatorio")
	}
	if len(s.Items) == 0 {
		return fmt.Errorf("siat emision: la factura debe tener al menos un ítem")
	}
	for i := range s.Items {
		item := s.Items[i]
		if strings.TrimSpace(item.ActividadEconomica) == "" {
			return fmt.Errorf("siat emision: actividadEconomica del ítem %d es obligatoria", i+1)
		}
		if item.CodigoProductoSin <= 0 {
			return fmt.Errorf("siat emision: codigoProductoSin del ítem %d debe ser mayor a cero", i+1)
		}
		if strings.TrimSpace(item.Descripcion) == "" {
			return fmt.Errorf("siat emision: descripcion del ítem %d es obligatoria", i+1)
		}
		if item.Cantidad <= 0 {
			return fmt.Errorf("siat emision: cantidad del ítem %d debe ser mayor a cero", i+1)
		}
		if item.UnidadMedida <= 0 {
			return fmt.Errorf("siat emision: unidadMedida del ítem %d debe ser mayor a cero", i+1)
		}
		if item.PrecioUnitario < 0 || item.SubTotal < 0 {
			return fmt.Errorf("siat emision: precio y subtotal del ítem %d no pueden ser negativos", i+1)
		}
	}
	return nil
}

func (s SolicitudDocumento) validate() error {
	if s.CodigoAmbiente != 0 && s.CodigoAmbiente != AmbienteProduccion && s.CodigoAmbiente != AmbientePruebas {
		return fmt.Errorf("siat documento: codigoAmbiente inválido")
	}
	if s.Modalidad != ModalidadElectronica && s.Modalidad != ModalidadComputarizada {
		return fmt.Errorf("siat documento: modalidad inválida (%d)", s.Modalidad)
	}
	if strings.TrimSpace(s.Cuf) == "" {
		return fmt.Errorf("siat documento: cuf es obligatorio")
	}
	if s.CodigoSucursal < 0 || s.CodigoPuntoVenta < 0 {
		return fmt.Errorf("siat documento: codigoSucursal y codigoPuntoVenta deben ser >= 0")
	}
	if strings.TrimSpace(s.Cuis) == "" || strings.TrimSpace(s.Cufd) == "" {
		return fmt.Errorf("siat documento: cuis y cufd son obligatorios")
	}
	return nil
}

// parseNit convierte el NIT (string) a int64 para los builders del SDK.
func parseNit(nit string) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(nit), 10, 64)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}
