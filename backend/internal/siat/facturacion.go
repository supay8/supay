package siat

import (
	"context"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
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
	NombreRazonSocial            string  `json:"nombreRazonSocial"`
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
}

// EmitirFactura construye, firma (si corresponde) y envía una factura de
// compraventa al SIAT usando el SDK go-siat. El SDK serializa el XML, lo firma
// con XMLDSig cuando la modalidad es electrónica, lo comprime en gzip y calcula
// el hash SHA-256 automáticamente (WithFactura).
func (s *Service) EmitirFactura(ctx context.Context, req SolicitudFactura) (*ResultadoEmision, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}
	if s.sdk == nil {
		return nil, fmt.Errorf("siat emision: servicio SIAT no inicializado")
	}

	nit := parseNit(req.Nit)

	// CUF: debe usar el MISMO timestamp de la cabecera y el MISMO correlativo.
	cuf, err := utils.NewCUF().
		WithNit(nit).
		WithFechaHora(req.FechaEmision).
		WithSucursal(req.CodigoSucursal).
		WithModalidad(req.Modalidad).
		WithTipoEmision(goSiat.EmisionOnline).
		WithTipoFactura(1).
		WithTipoDocumentoSector(1).
		WithNumeroFactura(req.NumeroFactura).
		WithPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoControl(req.CodigoControl).
		Generate()
	if err != nil {
		return nil, fmt.Errorf("siat emision cuf: %w", err)
	}

	puntoVenta := req.CodigoPuntoVenta
	nombreCliente := req.Cliente.NombreRazonSocial

	cabecera := invoices.NewCompraVentaCabeceraBuilder().
		WithNitEmisor(nit).
		WithRazonSocialEmisor(req.RazonSocialEmisor).
		WithMunicipio(req.Municipio).
		WithTelefono(req.Telefono).
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
		WithComplemento(req.Cliente.Complemento).
		WithCodigoCliente(req.Cliente.CodigoCliente).
		WithCodigoMetodoPago(req.CodigoMetodoPago).
		WithMontoTotal(req.MontoTotal).
		WithMontoTotalSujetoIva(req.MontoTotal).
		WithCodigoMoneda(req.CodigoMoneda).
		WithTipoCambio(req.TipoCambio).
		WithMontoTotalMoneda(req.MontoTotal).
		WithLeyenda(req.Leyenda).
		WithUsuario(req.Usuario).
		WithCodigoDocumentoSector(1).
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
			WithMontoDescuento(item.MontoDescuento).
			WithSubTotal(item.SubTotal).
			Build()
		facturaBuilder.AddDetalle(detalle)
	}
	factura := facturaBuilder.Build()

	// La modalidad debe configurarse ANTES de WithFactura: es la que decide si
	// el XML se firma digitalmente.
	rcpBuilder := models.NewRecepcionFacturaBuilder().
		WithCodigoModalidad(req.Modalidad).
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoDocumentoSector(1).
		WithCodigoEmision(goSiat.EmisionOnline).
		WithTipoFacturaDocumento(1).
		WithCuis(req.Cuis).
		WithCufd(req.Cufd).
		WithFechaEnvio(time.Now())
	if err := rcpBuilder.WithFactura(factura, s.sdk.Config()); err != nil {
		return nil, fmt.Errorf("siat emision: no se pudo empaquetar la factura: %w", err)
	}

	// XML firmado (o no) y hash tal como se envían, para persistencia/auditoría.
	xmlToSend, hash, err := signedXMLAndHash(factura, s.sdk.Config(), req.Modalidad)
	if err != nil {
		return nil, fmt.Errorf("siat emision: %w", err)
	}

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.sdk.CompraVenta().RecepcionFactura(ctx, rcpBuilder.Build())
	if err != nil {
		return nil, fmt.Errorf("siat emision: %w", err)
	}

	result := resp.Body.Content.RespuestaServicioFacturacion
	// Nota: RespuestaRecepcion no implementa common.Result, por lo que
	// goSiat.Verify no aplica; la verificación es manual (Transaccion/CodigoEstado).
	return &ResultadoEmision{
		Cuf:             cuf,
		Transaccion:     result.Transaccion,
		CodigoEstado:    result.CodigoEstado,
		CodigoRecepcion: result.CodigoRecepcion,
		Mensajes:        toMensajes(result.MensajesList),
		Xml:             xmlToSend,
		XmlHash:         hash,
	}, nil
}

// signedXMLAndHash serializa la factura, la firma si la modalidad lo exige y
// calcula el hash SHA-256 (hex) del XML gzipeado, replicando exactamente lo que
// el SDK envía en Archivo/HashArchivo.
func signedXMLAndHash(factura any, signer models.XMLSigner, modalidad int) (string, string, error) {
	xmlData, err := xml.Marshal(factura)
	if err != nil {
		return "", "", fmt.Errorf("no se pudo serializar el XML: %w", err)
	}

	xmlToSend := xmlData
	if modalidad == ModalidadElectronica && signer != nil {
		xmlToSend, err = signer.SignXML(xmlData)
		if err != nil {
			return "", "", fmt.Errorf("no se pudo firmar el XML: %w", err)
		}
	}

	compressed, err := utils.Gzip(xmlToSend)
	if err != nil {
		return "", "", fmt.Errorf("no se pudo comprimir el XML: %w", err)
	}
	sum := sha256.Sum256(compressed)
	return string(xmlToSend), fmt.Sprintf("%x", sum), nil
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

// parseNit convierte el NIT (string) a int64 para los builders del SDK.
func parseNit(nit string) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(nit), 10, 64)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}
