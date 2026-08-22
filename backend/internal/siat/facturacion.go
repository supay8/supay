package siat

import (
	"context"
	"encoding/xml"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	goSiat "github.com/ron86i/go-siat/v2"
	"github.com/ron86i/go-siat/v2/pkg/models"
	"github.com/ron86i/go-siat/v2/pkg/models/invoices"
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
	// CodigoTipoFactura es el tipo de documento factura (1 = factura).
	CodigoTipoFactura int `json:"codigoTipoFactura"`
	// NombreEstudiante y PeriodoFacturado son obligatorios solo para el
	// documento-sector 11 (FACTURA SECTORES EDUCATIVOS).
	NombreEstudiante string `json:"nombreEstudiante,omitempty"`
	PeriodoFacturado string `json:"periodoFacturado,omitempty"`

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
	CodigoDocumentoSector int `json:"codigoDocumentoSector"`
	CodigoTipoFactura     int `json:"codigoTipoFactura"`
}

// SectorEducativo es el código de documento-sector del SIAT para la factura de
// sectores educativos (FSEDU), asociado a actividades de enseñanza (p.ej. 8549100).
const SectorEducativo = 11

// SectorCompraVenta es el documento-sector de la factura de compra y venta.
const SectorCompraVenta = 1

// SectorTasaCero es el código de documento-sector del SIAT para facturas con
// tasa cero (productos exentos de IVA). La cabecera es idéntica a CompraVenta
// pero el XML usa la raíz facturaElectronicaTasaCero y MontoTotalSujetoIva=0.
const SectorTasaCero = 8

// SectorNotaCreditoDebito es el código de documento-sector del SIAT para notas
// de crédito y débito (documento de ajuste, sector 24).
const SectorNotaCreditoDebito = 24

// ResultadoDocumento es la respuesta procesada del SIAT para una operación
// sobre un documento ya emitido (verificar estado, anular o revertir).
type ResultadoDocumento struct {
	Transaccion     bool      `json:"transaccion"`
	CodigoEstado    int       `json:"codigoEstado"`
	CodigoRecepcion string    `json:"codigoRecepcion,omitempty"`
	Mensajes        []Mensaje `json:"mensajes,omitempty"`
}

// SolicitudNotaCreditoDebito contiene los datos para emitir una nota de crédito
// o débito (documento de ajuste, sector 24) ante el SIAT. Extiende los datos
// base de facturación con los campos específicos del XSD de notas.
type SolicitudNotaCreditoDebito struct {
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

	// Datos del emisor
	RazonSocialEmisor string  `json:"razonSocialEmisor"`
	Municipio         string  `json:"municipio"`
	Direccion         string  `json:"direccion"`
	Telefono          *string `json:"telefono,omitempty"`

	// Datos del receptor
	Cliente ClienteFactura `json:"cliente"`

	// Datos de pago
	CodigoMetodoPago int     `json:"codigoMetodoPago"`
	CodigoMoneda     int     `json:"codigoMoneda"`
	TipoCambio       float64 `json:"tipoCambio"`
	Leyenda          string  `json:"leyenda"`

	// Referencia a la factura original que se está ajustando
	CufFacturaOriginal  string    `json:"cufFacturaOriginal"`
	FechaEmisionFactura time.Time `json:"fechaEmisionFactura"`
	MontoTotalOriginal  float64   `json:"montoTotalOriginal"`
	MontoTotalDevuelto  float64   `json:"montoTotalDevuelto"`
	MontoDescuento      *float64  `json:"montoDescuento,omitempty"`
	MontoEfectivoNota   float64   `json:"montoEfectivoNota"`

	// Tipo de nota: 1 = crédito, 2 = débito
	TipoNota TipoNota `json:"tipoNota"`

	// CodigoDocumentoSector y CodigoTipoFactura del documento original
	CodigoDocumentoSector int `json:"codigoDocumentoSector"`
	CodigoTipoFactura     int `json:"codigoTipoFactura"`

	Items []ItemFactura `json:"items"`
}

// EmitirFactura construye, firma (si corresponde) y envía una factura al SIAT
// usando el SDK go-siat. Soporta el documento-sector 1 (compraventa) y el 11
// (sector educativo/FSEDU). El SDK serializa el XML, lo firma con XMLDSig cuando
// la modalidad es electrónica, lo comprime en gzip y calcula el hash SHA-256
// automáticamente (WithFactura).
func (s *Service) EmitirFactura(ctx context.Context, req SolicitudFactura) (*ResultadoEmision, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}
	if s.sdk == nil {
		return nil, fmt.Errorf("siat emision: servicio SIAT no inicializado")
	}

	sector := req.CodigoDocumentoSector
	if sector <= 0 {
		sector = 1
	}
	tipoFactura := req.CodigoTipoFactura
	if tipoFactura <= 0 {
		tipoFactura = 1
	}

	// CUF: debe usar el MISMO timestamp de la cabecera y el MISMO correlativo.
	// En la emisión individual el CUF se compone siempre con emisión en línea.
	factura, cuf, err := buildFacturaSDK(req, goSiat.EmisionOnline)
	if err != nil {
		return nil, err
	}

	// La recepción (recepcionFactura) es el mismo método web del SIAT para todos
	// los sectores; el documento-sector se envía en la cabecera y en la solicitud.
	rcpBuilder := models.NewRecepcionFacturaBuilder().
		WithCodigoModalidad(req.Modalidad).
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoDocumentoSector(sector).
		WithCodigoEmision(goSiat.EmisionOnline).
		WithTipoFacturaDocumento(tipoFactura).
		WithCuis(req.Cuis).
		WithCufd(req.Cufd).
		WithFechaEnvio(time.Now().In(LaPaz))

	// Firma única: serializamos, firmamos (si modalidad electrónica) y
	// empaquetamos una sola vez. El mismo XML firmado se usa tanto para el
	// envío al SIAT como para persistencia en base de datos (auditoría).
	xmlData, err := xml.Marshal(factura)
	if err != nil {
		return nil, fmt.Errorf("siat emision: no se pudo serializar la factura: %w", err)
	}
	// go-siat representa campos opcionales sin valor con xsi:nil. El XSD de
	// facturas del SIAT no acepta esos nodos para complemento/cafc, por lo que
	// se eliminan antes de firmar y empaquetar el XML.
	xmlData = removeEmptyOptionalFacturaFields(xmlData)
	xmlToSend := xmlData
	if req.Modalidad == ModalidadElectronica {
		xmlToSend, err = s.sdk.Config().SignXML(xmlData)
		if err != nil {
			return nil, fmt.Errorf("siat emision: no se pudo firmar el XML: %w", err)
		}
	}
	archivo, hash, err := empaquetaArchivo(xmlToSend)
	if err != nil {
		return nil, fmt.Errorf("siat emision: %w", err)
	}

	rcpBuilder.WithArchivo(archivo).WithHashArchivo(hash)

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.recepcionFacturaEnvio(ctx, sector, req.Modalidad, rcpBuilder.Build())
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
		Xml:             string(xmlToSend),
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
	for _, field := range []string{"complemento", "cafc"} {
		data = regexp.MustCompile(`<`+field+`(?:\\s+[^>]*)?>\\s*</`+field+`>`).ReplaceAll(data, nil)
		data = regexp.MustCompile(`<`+field+`(?:\\s+[^>]*)?\\s*/>`).ReplaceAll(data, nil)
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

// buildFacturaSDK construye el struct de factura del SDK (compraventa o sector
// educativo) para una SolicitudFactura, junto con el CUF generado con el mismo
// timestamp y correlativo de la cabecera. codigoEmision define cómo se compone
// el CUF: EmisionOnline para la emisión individual y EmisionOffline para las
// facturas emitidas fuera de línea que luego viajan en un paquete.
func buildFacturaSDK(req SolicitudFactura, codigoEmision int) (factura any, cuf string, err error) {
	nit := parseNit(req.Nit)
	sector := req.CodigoDocumentoSector
	if sector <= 0 {
		sector = 1
	}
	tipoFactura := req.CodigoTipoFactura
	if tipoFactura <= 0 {
		tipoFactura = 1
	}

	cuf, err = utils.NewCUF().
		WithNit(nit).
		WithFechaHora(req.FechaEmision).
		WithSucursal(req.CodigoSucursal).
		WithModalidad(req.Modalidad).
		WithTipoEmision(codigoEmision).
		WithTipoFactura(tipoFactura).
		WithTipoDocumentoSector(sector).
		WithNumeroFactura(req.NumeroFactura).
		WithPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoControl(req.CodigoControl).
		Generate()
	if err != nil {
		return nil, "", fmt.Errorf("siat emision cuf: %w", err)
	}
	puntoVenta := req.CodigoPuntoVenta
	nombreCliente := req.Cliente.NombreRazonSocial
	complementoPtr := optionalStringPtr(req.Cliente.Complemento)
	telefonoPtr := optionalStringPtr(req.Telefono)

	if sector == SectorEducativo {
		// FACTURA SECTORES EDUCATIVOS (documento-sector 11): estructura XSD
		// facturaElectronicaSectorEducativo con campos específicos del estudiante.
		cabecera := invoices.NewSectorEducativoCabeceraBuilder().
			WithNitEmisor(nit).
			WithRazonSocialEmisor(req.RazonSocialEmisor).
			WithMunicipio(req.Municipio).
			WithTelefono(telefonoPtr).
			WithNumeroFactura(req.NumeroFactura).
			WithCuf(cuf).
			WithCufd(req.Cufd).
			WithCodigoSucursal(req.CodigoSucursal).
			WithDireccion(req.Direccion).
			WithCodigoPuntoVenta(&puntoVenta).
			WithFechaEmision(req.FechaEmision).
			WithNombreRazonSocial(&nombreCliente).
			WithCodigoTipoDocumentoIdentidad(req.Cliente.CodigoTipoDocumentoIdentidad).
			WithNumeroDocumento(req.Cliente.NumeroDocumento).
			WithComplemento(complementoPtr).
			WithCodigoCliente(req.Cliente.CodigoCliente).
			WithNumeroTarjeta(&zeroInt64).
			WithMontoGiftCard(&zeroFloat).
			WithDescuentoAdicional(&zeroFloat).
			WithCodigoExcepcion(&zeroInt).
			WithCafc(nil).
			WithNombreEstudiante(req.NombreEstudiante).
			WithPeriodoFacturado(req.PeriodoFacturado).
			WithCodigoMetodoPago(req.CodigoMetodoPago).
			WithMontoTotal(req.MontoTotal).
			WithMontoTotalSujetoIva(req.MontoTotal).
			WithCodigoMoneda(req.CodigoMoneda).
			WithTipoCambio(req.TipoCambio).
			WithMontoTotalMoneda(req.MontoTotal).
			WithLeyenda(req.Leyenda).
			WithUsuario(req.Usuario).
			WithCodigoDocumentoSector(sector).
			Build()

		facturaBuilder := invoices.NewSectorEducativoBuilder().
			WithModalidad(req.Modalidad).
			WithCabecera(cabecera)
		for i := range req.Items {
			item := req.Items[i]
			detalle := invoices.NewSectorEducativoDetalleBuilder().
				WithActividadEconomica(item.ActividadEconomica).
				WithCodigoProductoSin(item.CodigoProductoSin).
				WithCodigoProducto(item.CodigoProducto).
				WithDescripcion(item.Descripcion).
				WithCantidad(item.Cantidad).
				WithUnidadMedida(item.UnidadMedida).
				WithPrecioUnitario(item.PrecioUnitario).
				WithMontoDescuento(descuentoPtr(item.MontoDescuento)).
				WithSubTotal(item.SubTotal).
				Build()
			facturaBuilder.AddDetalle(detalle)
		}
		return facturaBuilder.Build(), cuf, nil
	}

	if sector == SectorTasaCero {
		// FACTURA TASA CERO (documento-sector 8): misma estructura que CompraVenta
		// pero con raíz XML facturaElectronicaTasaCero y MontoTotalSujetoIva=0.
		cabecera := invoices.NewTasaCeroCabeceraBuilder().
			WithNitEmisor(nit).
			WithRazonSocialEmisor(req.RazonSocialEmisor).
			WithMunicipio(req.Municipio).
			WithTelefono(telefonoPtr).
			WithNumeroFactura(req.NumeroFactura).
			WithCuf(cuf).
			WithCufd(req.Cufd).
			WithCodigoSucursal(req.CodigoSucursal).
			WithDireccion(req.Direccion).
			WithCodigoPuntoVenta(&puntoVenta).
			WithFechaEmision(req.FechaEmision).
			WithNombreRazonSocial(&nombreCliente).
			WithCodigoTipoDocumentoIdentidad(req.Cliente.CodigoTipoDocumentoIdentidad).
			WithNumeroDocumento(req.Cliente.NumeroDocumento).
			WithComplemento(complementoPtr).
			WithCodigoCliente(req.Cliente.CodigoCliente).
			WithNumeroTarjeta(&zeroInt64).
			WithMontoGiftCard(&zeroFloat).
			WithDescuentoAdicional(&zeroFloat).
			WithCodigoExcepcion(&zeroInt).
			WithCafc(nil).
			WithCodigoMetodoPago(req.CodigoMetodoPago).
			WithMontoTotal(req.MontoTotal).
			WithMontoTotalSujetoIva(0).
			WithCodigoMoneda(req.CodigoMoneda).
			WithTipoCambio(req.TipoCambio).
			WithMontoTotalMoneda(req.MontoTotal).
			WithLeyenda(req.Leyenda).
			WithUsuario(req.Usuario).
			WithCodigoDocumentoSector(sector).
			Build()

		facturaBuilder := invoices.NewTasaCeroBuilder().
			WithModalidad(req.Modalidad).
			WithCabecera(cabecera)
		for i := range req.Items {
			item := req.Items[i]
			detalle := invoices.NewTasaCeroDetalleBuilder().
				WithActividadEconomica(item.ActividadEconomica).
				WithCodigoProductoSin(item.CodigoProductoSin).
				WithCodigoProducto(item.CodigoProducto).
				WithDescripcion(item.Descripcion).
				WithCantidad(item.Cantidad).
				WithUnidadMedida(item.UnidadMedida).
				WithPrecioUnitario(item.PrecioUnitario).
				WithMontoDescuento(descuentoPtr(item.MontoDescuento)).
				WithSubTotal(item.SubTotal).
				Build()
			facturaBuilder.AddDetalle(detalle)
		}
		return facturaBuilder.Build(), cuf, nil
	}

	// FACTURA COMPRAVENTA (documento-sector 1).
	cabecera := invoices.NewCompraVentaCabeceraBuilder().
		WithNitEmisor(nit).
		WithRazonSocialEmisor(req.RazonSocialEmisor).
		WithMunicipio(req.Municipio).
		WithTelefono(telefonoPtr).
		WithNumeroFactura(req.NumeroFactura).
		WithCuf(cuf).
		WithCufd(req.Cufd).
		WithCodigoSucursal(req.CodigoSucursal).
		WithDireccion(req.Direccion).
		WithCodigoPuntoVenta(&puntoVenta).
		WithFechaEmision(req.FechaEmision).
		WithNombreRazonSocial(&nombreCliente).
		WithCodigoTipoDocumentoIdentidad(req.Cliente.CodigoTipoDocumentoIdentidad).
		WithNumeroDocumento(req.Cliente.NumeroDocumento).
		WithComplemento(complementoPtr).
		WithCodigoCliente(req.Cliente.CodigoCliente).
		WithNumeroTarjeta(&zeroInt64).
		WithMontoGiftCard(&zeroFloat).
		WithDescuentoAdicional(&zeroFloat).
		WithCodigoExcepcion(&zeroInt64).
		WithCafc(nil).
		WithCodigoMetodoPago(req.CodigoMetodoPago).
		WithMontoTotal(req.MontoTotal).
		WithMontoTotalSujetoIva(req.MontoTotal).
		WithCodigoMoneda(req.CodigoMoneda).
		WithTipoCambio(req.TipoCambio).
		WithMontoTotalMoneda(req.MontoTotal).
		WithLeyenda(req.Leyenda).
		WithUsuario(req.Usuario).
		WithCodigoDocumentoSector(sector).
		Build()

	facturaBuilder := invoices.NewCompraVentaBuilder().
		WithModalidad(req.Modalidad).
		WithCabecera(cabecera)
	for i := range req.Items {
		item := req.Items[i]
		detalle := invoices.NewCompraVentaDetalleBuilder().
			WithActividadEconomica(item.ActividadEconomica).
			WithCodigoProductoSin(item.CodigoProductoSin).
			WithCodigoProducto(item.CodigoProducto).
			WithDescripcion(item.Descripcion).
			WithCantidad(item.Cantidad).
			WithUnidadMedida(item.UnidadMedida).
			WithPrecioUnitario(item.PrecioUnitario).
			WithMontoDescuento(descuentoPtr(item.MontoDescuento)).
			WithSubTotal(item.SubTotal).
			Build()
		facturaBuilder.AddDetalle(detalle)
	}
	return facturaBuilder.Build(), cuf, nil
}

// buildNotaCreditoDebito construye el struct de nota de crédito/débito del SDK
// (sector 24) para una SolicitudNotaCreditoDebito, junto con el CUF generado.
// El CUF usa TipoFactura=3 para NC y TipoFactura=4 para ND.
func buildNotaCreditoDebito(req SolicitudNotaCreditoDebito, codigoEmision int) (factura any, cuf string, err error) {
	nit := parseNit(req.Nit)
	sector := SectorNotaCreditoDebito
	if req.CodigoDocumentoSector > 0 {
		sector = req.CodigoDocumentoSector
	}
	tipoFactura := req.CodigoTipoFactura
	if tipoFactura <= 0 {
		tipoFactura = 1
	}

	// Tipo de documento para el CUF: 3 = Nota de Crédito, 4 = Nota de Débito
	tipoDoc := 3
	if req.TipoNota == TipoNotaDebito {
		tipoDoc = 4
	}

	cuf, err = utils.NewCUF().
		WithNit(nit).
		WithFechaHora(req.FechaEmision).
		WithSucursal(req.CodigoSucursal).
		WithModalidad(req.Modalidad).
		WithTipoEmision(codigoEmision).
		WithTipoFactura(tipoDoc).
		WithTipoDocumentoSector(sector).
		WithNumeroFactura(req.NumeroFactura).
		WithPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoControl(req.CodigoControl).
		Generate()
	if err != nil {
		return nil, "", fmt.Errorf("siat nota credito/debito cuf: %w", err)
	}

	puntoVenta := req.CodigoPuntoVenta
	nombreCliente := req.Cliente.NombreRazonSocial
	complementoPtr := optionalStringPtr(req.Cliente.Complemento)
	telefonoPtr := optionalStringPtr(req.Telefono)

	cabecera := invoices.NewNotaCreditoDebitoCabeceraBuilder().
		WithNitEmisor(nit).
		WithRazonSocialEmisor(req.RazonSocialEmisor).
		WithMunicipio(req.Municipio).
		WithTelefono(telefonoPtr).
		WithNumeroNotaCreditoDebito(req.NumeroFactura).
		WithCuf(cuf).
		WithCufd(req.Cufd).
		WithCodigoSucursal(req.CodigoSucursal).
		WithDireccion(req.Direccion).
		WithCodigoPuntoVenta(&puntoVenta).
		WithFechaEmision(req.FechaEmision).
		WithNombreRazonSocial(&nombreCliente).
		WithCodigoTipoDocumentoIdentidad(req.Cliente.CodigoTipoDocumentoIdentidad).
		WithNumeroDocumento(req.Cliente.NumeroDocumento).
		WithComplemento(complementoPtr).
		WithCodigoCliente(req.Cliente.CodigoCliente).
		WithNumeroFactura(req.NumeroFactura).
		WithNumeroAutorizacionCuf(req.CufFacturaOriginal).
		WithFechaEmisionFactura(req.FechaEmisionFactura).
		WithMontoTotalOriginal(req.MontoTotalOriginal).
		WithMontoTotalDevuelto(req.MontoTotalDevuelto).
		WithMontoDescuentoCreditoDebito(req.MontoDescuento).
		WithMontoEfectivoCreditoDebito(req.MontoEfectivoNota).
		WithCodigoExcepcion(&zeroInt).
		WithLeyenda(req.Leyenda).
		WithUsuario(req.Usuario).
		Build()

	notaBuilder := invoices.NewNotaCreditoDebitoBuilder().
		WithModalidad(req.Modalidad).
		WithCabecera(cabecera)
	for i := range req.Items {
		item := req.Items[i]
		detalle := invoices.NewNotaDetalleCreditoDebitoBuilder().
			WithActividadEconomica(item.ActividadEconomica).
			WithCodigoProductoSin(item.CodigoProductoSin).
			WithCodigoProducto(item.CodigoProducto).
			WithDescripcion(item.Descripcion).
			WithCantidad(item.Cantidad).
			WithUnidadMedida(item.UnidadMedida).
			WithPrecioUnitario(item.PrecioUnitario).
			WithMontoDescuento(descuentoPtr(item.MontoDescuento)).
			WithSubTotal(item.SubTotal).
			WithCodigoDetalleTransaccion(i + 1).
			Build()
		notaBuilder.AddDetalle(detalle)
	}
	return notaBuilder.Build(), cuf, nil
}

// VerificarEstado consulta al SIAT el estado real de un documento ya emitido
// (verificacionEstadoFactura) usando la fachada compraventa del SDK.
func (s *Service) VerificarEstado(ctx context.Context, req SolicitudDocumento) (*ResultadoDocumento, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}
	if s.sdk == nil {
		return nil, fmt.Errorf("siat verificacion: servicio SIAT no inicializado")
	}

	request := models.NewVerificacionEstadoFacturaBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoDocumentoSector(req.sector()).
		WithCodigoEmision(goSiat.EmisionOnline).
		WithTipoFacturaDocumento(req.tipoFactura()).
		WithCuf(req.Cuf).
		WithCuis(req.Cuis).
		WithCufd(req.Cufd).
		WithCodigoModalidad(req.Modalidad).
		WithNit(parseNit(req.Nit)).
		Build()

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.verificacionEstadoEnvio(ctx, req.sector(), req.Modalidad, request)
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

// AnularFactura anula un documento ya emitido ante el SIAT (anulacionFactura)
// indicando el motivo del catálogo sincronizado motivoAnulacion.
func (s *Service) AnularFactura(ctx context.Context, req SolicitudDocumento, codigoMotivo int) (*ResultadoDocumento, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}
	if s.sdk == nil {
		return nil, fmt.Errorf("siat anulacion: servicio SIAT no inicializado")
	}

	request := models.NewAnulacionFacturaBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoDocumentoSector(req.sector()).
		WithCodigoEmision(goSiat.EmisionOnline).
		WithTipoFacturaDocumento(req.tipoFactura()).
		WithCuf(req.Cuf).
		WithCuis(req.Cuis).
		WithCufd(req.Cufd).
		WithCodigoMotivo(codigoMotivo).
		WithCodigoModalidad(req.Modalidad).
		WithCodigoAmbiente(req.CodigoAmbiente).
		WithCodigoSistema(req.CodigoSistema).
		WithNit(parseNit(req.Nit)).
		Build()

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.anulacionFacturaEnvio(ctx, req.sector(), req.Modalidad, request)
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
// (reversionAnulacionFactura), devolviendo el documento a su estado anterior.
func (s *Service) RevertirAnulacion(ctx context.Context, req SolicitudDocumento) (*ResultadoDocumento, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}
	if s.sdk == nil {
		return nil, fmt.Errorf("siat reversion anulacion: servicio SIAT no inicializado")
	}

	request := models.NewReversionAnulacionFacturaBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoDocumentoSector(req.sector()).
		WithCodigoEmision(goSiat.EmisionOnline).
		WithTipoFacturaDocumento(req.tipoFactura()).
		WithCuf(req.Cuf).
		WithCuis(req.Cuis).
		WithCufd(req.Cufd).
		WithCodigoModalidad(req.Modalidad).
		WithCodigoAmbiente(req.CodigoAmbiente).
		WithCodigoSistema(req.CodigoSistema).
		WithNit(parseNit(req.Nit)).
		Build()

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.reversionAnulacionEnvio(ctx, req.sector(), req.Modalidad, request)
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

// envioFactura ejecuta una operación de facturación en el servicio del SDK
// adecuado para el documento-sector y la modalidad: CompraVenta() atiende los
// sectores 1, 35 y 41; el resto (p.ej. sector 11 educativo) se enruta por
// modalidad a Electronica() o Computarizada().
func (s *Service) recepcionFacturaEnvio(ctx context.Context, sector, modalidad int, req models.RecepcionFactura) (any, error) {
	switch sector {
	case SectorCompraVenta, 35, 41:
		return s.sdk.CompraVenta().RecepcionFactura(ctx, req)
	}
	envio := s.sdk.Electronica()
	if modalidad == ModalidadComputarizada {
		envio = s.sdk.Computarizada()
	}
	return envio.RecepcionFactura(ctx, req)
}

func (s *Service) verificacionEstadoEnvio(ctx context.Context, sector, modalidad int, req models.VerificacionEstadoFactura) (any, error) {
	switch sector {
	case SectorCompraVenta, 35, 41:
		return s.sdk.CompraVenta().VerificacionEstadoFactura(ctx, req)
	}
	envio := s.sdk.Electronica()
	if modalidad == ModalidadComputarizada {
		envio = s.sdk.Computarizada()
	}
	return envio.VerificacionEstadoFactura(ctx, req)
}

func (s *Service) anulacionFacturaEnvio(ctx context.Context, sector, modalidad int, req models.AnulacionFactura) (any, error) {
	switch sector {
	case SectorCompraVenta, 35, 41:
		return s.sdk.CompraVenta().AnulacionFactura(ctx, req)
	}
	envio := s.sdk.Electronica()
	if modalidad == ModalidadComputarizada {
		envio = s.sdk.Computarizada()
	}
	return envio.AnulacionFactura(ctx, req)
}

func (s *Service) reversionAnulacionEnvio(ctx context.Context, sector, modalidad int, req models.ReversionAnulacionFactura) (any, error) {
	switch sector {
	case SectorCompraVenta, 35, 41:
		return s.sdk.CompraVenta().ReversionAnulacionFactura(ctx, req)
	}
	envio := s.sdk.Electronica()
	if modalidad == ModalidadComputarizada {
		envio = s.sdk.Computarizada()
	}
	return envio.ReversionAnulacionFactura(ctx, req)
}

// extraerResultadoFacturacion lee Transaccion, CodigoEstado, CodigoRecepcion y
// MensajesList de la RespuestaServicioFacturacion común a las respuestas de
// facturación del SDK (recepción, verificación, anulación y reversión). Se usa
// reflexión porque los tipos de respuesta viven en paquetes internos del SDK.
func extraerResultadoFacturacion(resp any) (transaccion bool, codigoEstado int, codigoRecepcion string, mensajes []Mensaje, err error) {
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
	if s.CodigoAmbiente != AmbienteProduccion && s.CodigoAmbiente != AmbientePruebas {
		return fmt.Errorf("siat emision: codigoAmbiente inválido")
	}
	if strings.TrimSpace(s.CodigoSistema) == "" {
		return fmt.Errorf("siat emision: codigoSistema es obligatorio")
	}
	if strings.TrimSpace(s.Nit) == "" {
		return fmt.Errorf("siat emision: nit es obligatorio")
	}
	if s.Modalidad != ModalidadElectronica && s.Modalidad != ModalidadComputarizada {
		return fmt.Errorf("siat emision: modalidad inválida (%d)", s.Modalidad)
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
	if s.CodigoDocumentoSector == SectorEducativo {
		if strings.TrimSpace(s.NombreEstudiante) == "" {
			return fmt.Errorf("siat emision: nombreEstudiante es obligatorio para el documento-sector educativo")
		}
		if strings.TrimSpace(s.PeriodoFacturado) == "" {
			return fmt.Errorf("siat emision: periodoFacturado es obligatorio para el documento-sector educativo")
		}
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
	if s.CodigoAmbiente != AmbienteProduccion && s.CodigoAmbiente != AmbientePruebas {
		return fmt.Errorf("siat documento: codigoAmbiente inválido")
	}
	if strings.TrimSpace(s.CodigoSistema) == "" {
		return fmt.Errorf("siat documento: codigoSistema es obligatorio")
	}
	if strings.TrimSpace(s.Nit) == "" {
		return fmt.Errorf("siat documento: nit es obligatorio")
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

func (s SolicitudNotaCreditoDebito) validate() error {
	if s.CodigoAmbiente != AmbienteProduccion && s.CodigoAmbiente != AmbientePruebas {
		return fmt.Errorf("siat nota credito/debito: codigoAmbiente inválido")
	}
	if strings.TrimSpace(s.CodigoSistema) == "" {
		return fmt.Errorf("siat nota credito/debito: codigoSistema es obligatorio")
	}
	if strings.TrimSpace(s.Nit) == "" {
		return fmt.Errorf("siat nota credito/debito: nit es obligatorio")
	}
	if s.Modalidad != ModalidadElectronica && s.Modalidad != ModalidadComputarizada {
		return fmt.Errorf("siat nota credito/debito: modalidad inválida (%d)", s.Modalidad)
	}
	if s.NumeroFactura <= 0 {
		return fmt.Errorf("siat nota credito/debito: numeroFactura debe ser mayor a cero")
	}
	if s.CodigoSucursal < 0 || s.CodigoPuntoVenta < 0 {
		return fmt.Errorf("siat nota credito/debito: codigoSucursal y codigoPuntoVenta deben ser >= 0")
	}
	if strings.TrimSpace(s.Cuis) == "" || strings.TrimSpace(s.Cufd) == "" || strings.TrimSpace(s.CodigoControl) == "" {
		return fmt.Errorf("siat nota credito/debito: cuis, cufd y codigoControl son obligatorios")
	}
	if s.FechaEmision.IsZero() {
		return fmt.Errorf("siat nota credito/debito: fechaEmision es obligatoria")
	}
	if strings.TrimSpace(s.CufFacturaOriginal) == "" {
		return fmt.Errorf("siat nota credito/debito: cufFacturaOriginal es obligatorio")
	}
	if s.FechaEmisionFactura.IsZero() {
		return fmt.Errorf("siat nota credito/debito: fechaEmisionFactura es obligatoria")
	}
	if s.MontoTotalOriginal < 0 {
		return fmt.Errorf("siat nota credito/debito: montoTotalOriginal no puede ser negativo")
	}
	if s.MontoEfectivoNota < 0 {
		return fmt.Errorf("siat nota credito/debito: montoEfectivoNota no puede ser negativo")
	}
	if s.TipoNota != TipoNotaCredito && s.TipoNota != TipoNotaDebito {
		return fmt.Errorf("siat nota credito/debito: tipoNota debe ser 1 (crédito) o 2 (débito)")
	}
	if strings.TrimSpace(s.Leyenda) == "" {
		return fmt.Errorf("siat nota credito/debito: leyenda es obligatoria")
	}
	if strings.TrimSpace(s.RazonSocialEmisor) == "" {
		return fmt.Errorf("siat nota credito/debito: razonSocialEmisor es obligatoria")
	}
	if strings.TrimSpace(s.Municipio) == "" {
		return fmt.Errorf("siat nota credito/debito: municipio es obligatorio")
	}
	if strings.TrimSpace(s.Direccion) == "" {
		return fmt.Errorf("siat nota credito/debito: direccion es obligatoria")
	}
	if s.CodigoMetodoPago <= 0 || s.CodigoMoneda <= 0 || s.TipoCambio <= 0 {
		return fmt.Errorf("siat nota credito/debito: codigoMetodoPago, codigoMoneda y tipoCambio deben ser mayores a cero")
	}
	if strings.TrimSpace(s.Cliente.NombreRazonSocial) == "" {
		return fmt.Errorf("siat nota credito/debito: nombre del cliente es obligatorio")
	}
	if s.Cliente.CodigoTipoDocumentoIdentidad <= 0 {
		return fmt.Errorf("siat nota credito/debito: codigoTipoDocumentoIdentidad debe ser mayor a cero")
	}
	if strings.TrimSpace(s.Cliente.NumeroDocumento) == "" {
		return fmt.Errorf("siat nota credito/debito: numeroDocumento del cliente es obligatorio")
	}
	if len(s.Items) == 0 {
		return fmt.Errorf("siat nota credito/debito: la nota debe tener al menos un ítem")
	}
	for i := range s.Items {
		item := s.Items[i]
		if strings.TrimSpace(item.ActividadEconomica) == "" {
			return fmt.Errorf("siat nota credito/debito: actividadEconomica del ítem %d es obligatoria", i+1)
		}
		if item.CodigoProductoSin <= 0 {
			return fmt.Errorf("siat nota credito/debito: codigoProductoSin del ítem %d debe ser mayor a cero", i+1)
		}
		if strings.TrimSpace(item.Descripcion) == "" {
			return fmt.Errorf("siat nota credito/debito: descripcion del ítem %d es obligatoria", i+1)
		}
		if item.Cantidad <= 0 {
			return fmt.Errorf("siat nota credito/debito: cantidad del ítem %d debe ser mayor a cero", i+1)
		}
		if item.UnidadMedida <= 0 {
			return fmt.Errorf("siat nota credito/debito: unidadMedida del ítem %d debe ser mayor a cero", i+1)
		}
		if item.PrecioUnitario < 0 || item.SubTotal < 0 {
			return fmt.Errorf("siat nota credito/debito: precio y subtotal del ítem %d no pueden ser negativos", i+1)
		}
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
