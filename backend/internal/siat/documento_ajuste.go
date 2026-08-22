package siat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DEPRECATED: el flujo de notas de crédito/débito vive ahora en el pipeline
// general de emisión (/invoices con codigoDocumentoSector 24/29/47/48 y
// datos_sector); EmitirFactura enruta internamente por recepcionDocumentoAjuste.
// Estas funciones se mantienen por compatibilidad de API y delegan en él.

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

// EmitirDocumentoAjuste construye, firma y envía un documento de ajuste al SIAT.
// Es un wrapper sobre EmitirFactura con los datos específicos del sector 24
// mapeados a datos_sector.
func (s *Service) EmitirDocumentoAjuste(ctx context.Context, req SolicitudDocumentoAjuste) (*ResultadoDocumentoAjuste, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}
	if s.sdk == nil {
		return nil, fmt.Errorf("siat documento ajuste: servicio SIAT no inicializado")
	}

	datos, err := json.Marshal(map[string]any{
		"numero_autorizacion_cuf":       strings.TrimSpace(req.CufFacturaOriginal),
		"fecha_emision_factura":         req.FechaEmision.Format("2006-01-02"),
		"monto_total_original":          req.MontoTotal,
		"monto_total_devuelto":          req.MontoTotal,
		"monto_efectivo_credito_debito": req.MontoTotal,
	})
	if err != nil {
		return nil, fmt.Errorf("siat documento ajuste: %w", err)
	}

	solicitud := SolicitudFactura{
		CodigoAmbiente:        req.CodigoAmbiente,
		CodigoSistema:         req.CodigoSistema,
		Nit:                   req.Nit,
		Modalidad:             req.Modalidad,
		NumeroFactura:         req.NumeroFactura,
		CodigoSucursal:        req.CodigoSucursal,
		CodigoPuntoVenta:      req.CodigoPuntoVenta,
		Cuis:                  req.Cuis,
		Cufd:                  req.Cufd,
		CodigoControl:         req.CodigoControl,
		FechaEmision:          req.FechaEmision,
		Usuario:               req.Usuario,
		RazonSocialEmisor:     req.RazonSocialEmisor,
		Municipio:             req.Municipio,
		Direccion:             req.Direccion,
		Telefono:              req.Telefono,
		Cliente:               req.Cliente,
		CodigoMetodoPago:      req.CodigoMetodoPago,
		CodigoMoneda:          req.CodigoMoneda,
		TipoCambio:            req.TipoCambio,
		Leyenda:               req.Leyenda,
		CodigoDocumentoSector: req.CodigoDocumentoSector,
		CodigoTipoFactura:     req.CodigoTipoFactura,
		DatosSector:           datos,
		Items:                 req.Items,
	}
	if solicitud.CodigoDocumentoSector <= 0 {
		solicitud.CodigoDocumentoSector = SectorNotaCreditoDebito
	}

	res, err := s.EmitirFactura(ctx, solicitud)
	if err != nil {
		return nil, err
	}
	return &ResultadoDocumentoAjuste{
		Transaccion:     res.Transaccion,
		CodigoEstado:    res.CodigoEstado,
		CodigoRecepcion: res.CodigoRecepcion,
		Mensajes:        res.Mensajes,
		Cuf:             res.Cuf,
		Xml:             res.Xml,
		XmlHash:         res.XmlHash,
	}, nil
}

// AnularDocumentoAjuste anula un documento de ajuste ya emitido ante el SIAT.
// Wrapper sobre AnularFactura con routing por perfil.
func (s *Service) AnularDocumentoAjuste(ctx context.Context, req SolicitudAnulacionDocumentoAjuste, codigoMotivo int) (*ResultadoDocumentoAjuste, error) {
	res, err := s.AnularFactura(ctx, req.solicitudDocumento(), codigoMotivo)
	if err != nil {
		return nil, err
	}
	return &ResultadoDocumentoAjuste{
		Transaccion:     res.Transaccion,
		CodigoEstado:    res.CodigoEstado,
		CodigoRecepcion: res.CodigoRecepcion,
		Mensajes:        res.Mensajes,
	}, nil
}

// RevertirAnulacionDocumentoAjuste revierte una anulación de documento de ajuste.
// Wrapper sobre RevertirAnulacion con routing por perfil.
func (s *Service) RevertirAnulacionDocumentoAjuste(ctx context.Context, req SolicitudAnulacionDocumentoAjuste) (*ResultadoDocumentoAjuste, error) {
	res, err := s.RevertirAnulacion(ctx, req.solicitudDocumento())
	if err != nil {
		return nil, err
	}
	return &ResultadoDocumentoAjuste{
		Transaccion:     res.Transaccion,
		CodigoEstado:    res.CodigoEstado,
		CodigoRecepcion: res.CodigoRecepcion,
		Mensajes:        res.Mensajes,
	}, nil
}

// VerificarEstadoDocumentoAjuste consulta el estado de un documento de ajuste.
// Wrapper sobre VerificarEstado con routing por perfil.
func (s *Service) VerificarEstadoDocumentoAjuste(ctx context.Context, req SolicitudAnulacionDocumentoAjuste) (*ResultadoDocumentoAjuste, error) {
	res, err := s.VerificarEstado(ctx, req.solicitudDocumento())
	if err != nil {
		return nil, err
	}
	return &ResultadoDocumentoAjuste{
		Transaccion:     res.Transaccion,
		CodigoEstado:    res.CodigoEstado,
		CodigoRecepcion: res.CodigoRecepcion,
		Mensajes:        res.Mensajes,
	}, nil
}

// solicitudDocumento adapta la solicitud de anulación/verificación al tipo
// genérico usado por las operaciones por perfil.
func (s SolicitudAnulacionDocumentoAjuste) solicitudDocumento() SolicitudDocumento {
	sector := s.CodigoDocumentoSector
	if sector <= 0 {
		sector = SectorNotaCreditoDebito
	}
	tipo := s.CodigoTipoFactura
	if tipo <= 0 {
		tipo = TipoDocumentoNotaCreditoDebito
	}
	return SolicitudDocumento{
		CodigoAmbiente:        s.CodigoAmbiente,
		CodigoSistema:         s.CodigoSistema,
		Nit:                   s.Nit,
		Modalidad:             s.Modalidad,
		Cuf:                   s.Cuf,
		CodigoSucursal:        s.CodigoSucursal,
		CodigoPuntoVenta:      s.CodigoPuntoVenta,
		Cuis:                  s.Cuis,
		Cufd:                  s.Cufd,
		CodigoDocumentoSector: sector,
		CodigoTipoFactura:     tipo,
	}
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
