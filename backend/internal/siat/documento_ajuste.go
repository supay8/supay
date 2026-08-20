package siat

import (
	"context"
	"fmt"
	"strings"
	"time"

	goSiat "github.com/ron86i/go-siat/v2"
	"github.com/ron86i/go-siat/v2/pkg/models"
)

// TipoNota define el tipo de nota de ajuste según el catálogo del SIAT.
type TipoNota int

const (
	TipoNotaCredito TipoNota = 1
	TipoNotaDebito  TipoNota = 2
)

// SolicitudDocumentoAjuste contiene los datos para emitir una nota de crédito
// o débito (documento de ajuste) ante el SIAT.
type SolicitudDocumentoAjuste struct {
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
	TipoNota         TipoNota  `json:"tipoNota"`

	// CUF de la factura original que se está ajustando.
	CufFacturaOriginal string `json:"cufFacturaOriginal"`

	// CodigoDocumentoSector y CodigoTipoFactura del documento ajustado.
	CodigoDocumentoSector int `json:"codigoDocumentoSector"`
	CodigoTipoFactura     int `json:"codigoTipoFactura"`

	// Datos del emisor
	RazonSocialEmisor string  `json:"razonSocialEmisor"`
	Municipio         string  `json:"municipio"`
	Direccion         string  `json:"direccion"`
	Telefono          *string `json:"telefono,omitempty"`

	// Datos del receptor
	Cliente ClienteFactura `json:"cliente"`

	// Datos de la nota
	CodigoMetodoPago int     `json:"codigoMetodoPago"`
	CodigoMoneda     int     `json:"codigoMoneda"`
	TipoCambio       float64 `json:"tipoCambio"`
	MontoTotal       float64 `json:"montoTotal"`
	Leyenda          string  `json:"leyenda"`

	// Motivo de la nota de crédito/débito
	Motivo string `json:"motivo"`

	// Items de la nota (pueden ser diferentes a los de la factura original)
	Items []ItemFactura `json:"items"`
}

// ResultadoDocumentoAjuste es la respuesta procesada de recepcionDocumentoAjuste.
type ResultadoDocumentoAjuste struct {
	Transaccion     bool      `json:"transaccion"`
	CodigoEstado    int       `json:"codigoEstado"`
	CodigoRecepcion string    `json:"codigoRecepcion,omitempty"`
	Mensajes        []Mensaje `json:"mensajes,omitempty"`
	Cuf             string    `json:"cuf,omitempty"`
	Xml             string    `json:"xml,omitempty"`
	XmlHash         string    `json:"xmlHash,omitempty"`
}

// SolicitudAnulacionDocumentoAjuste identifica un documento de ajuste ya emitido
// para anular o revertir.
type SolicitudAnulacionDocumentoAjuste struct {
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
}

// EmitirDocumentoAjuste construye, firma y envía un documento de ajuste
// (nota de crédito o nota de débito) al SIAT.
func (s *Service) EmitirDocumentoAjuste(ctx context.Context, req SolicitudDocumentoAjuste) (*ResultadoDocumentoAjuste, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}
	if s.sdk == nil {
		return nil, fmt.Errorf("siat documento ajuste: servicio SIAT no inicializado")
	}

	sector := req.CodigoDocumentoSector
	if sector <= 0 {
		sector = 1
	}
	tipoFactura := req.CodigoTipoFactura
	if tipoFactura <= 0 {
		tipoFactura = 1
	}

	// Determinar tipo de documento (NC=3, ND=4)
	tipoDoc := 3 // Nota de Crédito
	if req.TipoNota == TipoNotaDebito {
		tipoDoc = 4 // Nota de Débito
	}

	// Serializar y firmar el documento
	xmlData, err := buildDocumentoAjusteXML(req, sector, tipoFactura, tipoDoc)
	if err != nil {
		return nil, fmt.Errorf("siat documento ajuste: %w", err)
	}

	xmlToSend := xmlData
	if req.Modalidad == ModalidadElectronica {
		xmlToSend, err = s.sdk.Config().SignXML(xmlData)
		if err != nil {
			return nil, fmt.Errorf("siat documento ajuste: no se pudo firmar el XML: %w", err)
		}
	}

	archivo, hash, err := empaquetaArchivo(xmlToSend)
	if err != nil {
		return nil, fmt.Errorf("siat documento ajuste: %w", err)
	}

	rcpBuilder := models.NewRecepcionDocumentoAjusteBuilder().
		WithCodigoModalidad(req.Modalidad).
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoDocumentoSector(sector).
		WithCodigoEmision(goSiat.EmisionOnline).
		WithTipoFacturaDocumento(tipoDoc).
		WithCuis(req.Cuis).
		WithCufd(req.Cufd).
		WithFechaEnvio(time.Now().In(LaPaz)).
		WithArchivo(archivo).
		WithHashArchivo(hash)

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.sdk.DocumentoAjuste().RecepcionDocumentoAjuste(ctx, rcpBuilder.Build())
	if err != nil {
		return nil, fmt.Errorf("siat documento ajuste: %w", err)
	}

	transaccion, codigoEstado, codigoRecepcion, mensajes, err := extraerResultadoFacturacion(resp)
	if err != nil {
		return nil, fmt.Errorf("siat documento ajuste: %w", err)
	}

	return &ResultadoDocumentoAjuste{
		Transaccion:     transaccion,
		CodigoEstado:    codigoEstado,
		CodigoRecepcion: codigoRecepcion,
		Mensajes:        mensajes,
		Xml:             string(xmlToSend),
		XmlHash:         hash,
	}, nil
}

// AnularDocumentoAjuste anula un documento de ajuste ya emitido ante el SIAT.
func (s *Service) AnularDocumentoAjuste(ctx context.Context, req SolicitudAnulacionDocumentoAjuste, codigoMotivo int) (*ResultadoDocumentoAjuste, error) {
	if s.sdk == nil {
		return nil, fmt.Errorf("siat anulacion documento ajuste: servicio SIAT no inicializado")
	}

	sector := req.CodigoDocumentoSector
	if sector <= 0 {
		sector = 1
	}
	tipoFactura := req.CodigoTipoFactura
	if tipoFactura <= 0 {
		tipoFactura = 1
	}

	request := models.NewAnulacionDocumentoAjusteBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoDocumentoSector(sector).
		WithCodigoEmision(goSiat.EmisionOnline).
		WithTipoFacturaDocumento(tipoFactura).
		WithCuf(req.Cuf).
		WithCuis(req.Cuis).
		WithCufd(req.Cufd).
		WithCodigoMotivo(codigoMotivo).
		Build()

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.sdk.DocumentoAjuste().AnulacionDocumentoAjuste(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("siat anulacion documento ajuste: %w", err)
	}

	transaccion, codigoEstado, codigoRecepcion, mensajes, err := extraerResultadoFacturacion(resp)
	if err != nil {
		return nil, fmt.Errorf("siat anulacion documento ajuste: %w", err)
	}

	return &ResultadoDocumentoAjuste{
		Transaccion:     transaccion,
		CodigoEstado:    codigoEstado,
		CodigoRecepcion: codigoRecepcion,
		Mensajes:        mensajes,
	}, nil
}

// RevertirAnulacionDocumentoAjuste revierte una anulación de documento de ajuste.
func (s *Service) RevertirAnulacionDocumentoAjuste(ctx context.Context, req SolicitudAnulacionDocumentoAjuste) (*ResultadoDocumentoAjuste, error) {
	if s.sdk == nil {
		return nil, fmt.Errorf("siat reversion documento ajuste: servicio SIAT no inicializado")
	}

	sector := req.CodigoDocumentoSector
	if sector <= 0 {
		sector = 1
	}
	tipoFactura := req.CodigoTipoFactura
	if tipoFactura <= 0 {
		tipoFactura = 1
	}

	request := models.NewReversionAnulacionDocumentoAjusteBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoDocumentoSector(sector).
		WithCodigoEmision(goSiat.EmisionOnline).
		WithTipoFacturaDocumento(tipoFactura).
		WithCuf(req.Cuf).
		WithCuis(req.Cuis).
		WithCufd(req.Cufd).
		Build()

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.sdk.DocumentoAjuste().ReversionAnulacionDocumentoAjuste(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("siat reversion documento ajuste: %w", err)
	}

	transaccion, codigoEstado, codigoRecepcion, mensajes, err := extraerResultadoFacturacion(resp)
	if err != nil {
		return nil, fmt.Errorf("siat reversion documento ajuste: %w", err)
	}

	return &ResultadoDocumentoAjuste{
		Transaccion:     transaccion,
		CodigoEstado:    codigoEstado,
		CodigoRecepcion: codigoRecepcion,
		Mensajes:        mensajes,
	}, nil
}

// VerificarEstadoDocumentoAjuste consulta el estado de un documento de ajuste.
func (s *Service) VerificarEstadoDocumentoAjuste(ctx context.Context, req SolicitudAnulacionDocumentoAjuste) (*ResultadoDocumentoAjuste, error) {
	if s.sdk == nil {
		return nil, fmt.Errorf("siat verificacion documento ajuste: servicio SIAT no inicializado")
	}

	sector := req.CodigoDocumentoSector
	if sector <= 0 {
		sector = 1
	}
	tipoFactura := req.CodigoTipoFactura
	if tipoFactura <= 0 {
		tipoFactura = 1
	}

	request := models.NewVerificacionEstadoDocumentoAjusteBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoDocumentoSector(sector).
		WithCodigoEmision(goSiat.EmisionOnline).
		WithTipoFacturaDocumento(tipoFactura).
		WithCuf(req.Cuf).
		WithCuis(req.Cuis).
		WithCufd(req.Cufd).
		Build()

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.sdk.DocumentoAjuste().VerificacionEstadoDocumentoAjuste(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("siat verificacion documento ajuste: %w", err)
	}

	transaccion, codigoEstado, codigoRecepcion, mensajes, err := extraerResultadoFacturacion(resp)
	if err != nil {
		return nil, fmt.Errorf("siat verificacion documento ajuste: %w", err)
	}

	return &ResultadoDocumentoAjuste{
		Transaccion:     transaccion,
		CodigoEstado:    codigoEstado,
		CodigoRecepcion: codigoRecepcion,
		Mensajes:        mensajes,
	}, nil
}

// buildDocumentoAjusteXML genera el XML del documento de ajuste según el XSD del SIAT.
// Por ahora genera una estructura simplificada; se debe ajustar al XSD exacto
// cuando se implemente la homologación.
func buildDocumentoAjusteXML(req SolicitudDocumentoAjuste, sector, tipoFactura, tipoDoc int) ([]byte, error) {
	// Nota: La serialización XML real debe seguir el XSD del SIAT para
	// documento de ajuste. Por ahora se genera una estructura básica que
	// será validada contra el XSD durante la homologación.
	nit := parseNit(req.Nit)
	cuf := "" // El CUF del documento de ajuste se genera en la emisión

	// TODO: Implementar builders específicos de NC/ND del SDK cuando estén
	// disponibles para todos los sectores. Por ahora se usa una estructura
	// genérica que será reemplazada.
	_ = nit
	_ = cuf
	_ = sector
	_ = tipoFactura
	_ = tipoDoc

	return nil, fmt.Errorf("siat documento ajuste: la generación de XML para documentos de ajuste requiere implementación específica por sector; consulte al equipo de desarrollo")
}

func (s SolicitudDocumentoAjuste) validate() error {
	if s.CodigoAmbiente != AmbienteProduccion && s.CodigoAmbiente != AmbientePruebas {
		return fmt.Errorf("siat documento ajuste: codigoAmbiente inválido")
	}
	if strings.TrimSpace(s.CodigoSistema) == "" {
		return fmt.Errorf("siat documento ajuste: codigoSistema es obligatorio")
	}
	if strings.TrimSpace(s.Nit) == "" {
		return fmt.Errorf("siat documento ajuste: nit es obligatorio")
	}
	if s.Modalidad != ModalidadElectronica && s.Modalidad != ModalidadComputarizada {
		return fmt.Errorf("siat documento ajuste: modalidad inválida (%d)", s.Modalidad)
	}
	if strings.TrimSpace(s.CufFacturaOriginal) == "" {
		return fmt.Errorf("siat documento ajuste: cufFacturaOriginal es obligatorio")
	}
	if strings.TrimSpace(s.Motivo) == "" {
		return fmt.Errorf("siat documento ajuste: motivo es obligatorio")
	}
	if s.TipoNota != TipoNotaCredito && s.TipoNota != TipoNotaDebito {
		return fmt.Errorf("siat documento ajuste: tipoNota debe ser 1 (crédito) o 2 (débito)")
	}
	return nil
}
