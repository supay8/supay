package siat

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	goSiat "github.com/ron86i/go-siat/v2"
	"github.com/ron86i/go-siat/v2/pkg/models"
)

// EmisionMasiva es el codigoEmision que exige el SIAT para la emisión masiva
// (recepcionMasivaFactura): emisión en línea de alto volumen.
const EmisionMasiva = goSiat.EmisionMasiva

// MaxFacturasMasiva es el límite de facturas por solicitud de emisión masiva
// definido por el SIN.
const MaxFacturasMasiva = 1000

// SolicitudMasivaFactura agrupa los prerrequisitos para enviar un lote de
// facturas al SIAT por emisión masiva (recepcionMasivaFactura). Todas las
// facturas deben pertenecer al mismo documento-sector y compartir el mismo
// CUIS/CUFD.
type SolicitudMasivaFactura struct {
	CodigoAmbiente   int    `json:"codigoAmbiente"`
	CodigoSistema    string `json:"codigoSistema"`
	Nit              string `json:"nit"`
	Modalidad        int    `json:"modalidad"`
	CodigoSucursal   int    `json:"codigoSucursal"`
	CodigoPuntoVenta int    `json:"codigoPuntoVenta"`
	Cuis             string `json:"cuis"`
	Cufd             string `json:"cufd"`
	CodigoControl    string `json:"codigoControl"`

	// CodigoDocumentoSector es el diseño de factura del lote (1 = compraventa,
	// 11 = sector educativo). Todas las facturas deben ser del mismo sector.
	CodigoDocumentoSector int `json:"codigoDocumentoSector"`
	// CodigoTipoFactura es el tipo de documento factura (1 = factura).
	CodigoTipoFactura int `json:"codigoTipoFactura"`
	// CodigoEmision es el tipo de emisión; para emisión masiva el SIAT exige 3
	// (EmisionMasiva). 0 se interpreta como masiva.
	CodigoEmision int `json:"codigoEmision"`

	// Facturas son las facturas del lote (máximo 1000). La identidad común
	// (ambiente, sistema, NIT, modalidad, sucursal, punto de venta, CUIS/CUFD,
	// código de control y documento-sector) se hereda a las facturas que no la
	// traigan.
	Facturas []SolicitudFactura `json:"facturas"`
}

// EnviarMasivaFacturas envía un lote de facturas al SIAT por emisión masiva
// (recepcionMasivaFactura). Cada factura se construye con su propio CUF usando
// el codigoEmision masiva (3); el SDK las firma (modalidad electrónica), las
// empaqueta en TAR.GZ comprimido y calcula el hash SHA-256 automáticamente
// (WithFacturas). El CodigoRecepcion devuelto se usa luego en
// ValidarMasivaFacturas.
func (s *Service) EnviarMasivaFacturas(ctx context.Context, req SolicitudMasivaFactura) (*ResultadoPaquete, error) {
	req = req.normalized()
	if err := req.validate(); err != nil {
		return nil, err
	}
	if s.sdk == nil {
		return nil, fmt.Errorf("siat masiva: servicio SIAT no inicializado")
	}

	sector := req.sector()
	tipoFactura := req.tipoFactura()
	codigoEmision := req.codigoEmision()

	facturas := make([]any, 0, len(req.Facturas))
	cufs := make([]string, 0, len(req.Facturas))
	for i := range req.Facturas {
		factura, cuf, err := buildFacturaSDK(req.Facturas[i], codigoEmision)
		if err != nil {
			return nil, fmt.Errorf("siat masiva factura %d: %w", i+1, err)
		}
		facturas = append(facturas, factura)
		cufs = append(cufs, cuf)
	}

	lote := models.NewRecepcionMasivaFacturaBuilder().
		WithCodigoModalidad(req.Modalidad).
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoDocumentoSector(sector).
		WithCodigoEmision(codigoEmision).
		WithTipoFacturaDocumento(tipoFactura).
		WithCuis(req.Cuis).
		WithCufd(req.Cufd).
		// El SIAT interpreta la hora de pared sin zona de fechaEnvio como hora
		// local de Bolivia (UTC-4); el SDK formatea la hora tal cual la recibe
		// (no convierte), por lo que se envía la hora de pared de La Paz.
		WithFechaEnvio(time.Now().In(LaPaz))

	if err := lote.WithFacturas(facturas, s.sdk.Config()); err != nil {
		return nil, fmt.Errorf("siat masiva: no se pudo empaquetar las facturas: %w", err)
	}

	built := lote.Build()
	archivo, hash, cantidad := extraerArchivoMasiva(built)

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.recepcionMasivaEnvio(ctx, sector, req.Modalidad, built)
	if err != nil {
		return nil, fmt.Errorf("siat masiva: %w", err)
	}

	// Nota: RespuestaRecepcion no implementa common.Result, por lo que
	// goSiat.Verify no aplica; la verificación es manual (Transaccion/CodigoEstado).
	transaccion, codigoEstado, codigoRecepcion, mensajes, err := extraerResultadoFacturacion(resp)
	if err != nil {
		return nil, fmt.Errorf("siat masiva: %w", err)
	}

	return &ResultadoPaquete{
		Transaccion:      transaccion,
		CodigoEstado:     codigoEstado,
		CodigoRecepcion:  codigoRecepcion,
		Mensajes:         mensajes,
		Archivo:          archivo,
		HashArchivo:      hash,
		CantidadFacturas: cantidad,
		Cufs:             cufs,
	}, nil
}

// ValidarMasivaFacturas consulta al SIAT la validación de un lote ya enviado
// (validacionRecepcionMasivaFactura) usando el CodigoRecepcion devuelto por
// EnviarMasivaFacturas.
func (s *Service) ValidarMasivaFacturas(ctx context.Context, req SolicitudMasivaFactura, codigoRecepcion string) (*ResultadoPaquete, error) {
	if err := req.validateBase(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(codigoRecepcion) == "" {
		return nil, fmt.Errorf("siat masiva: codigoRecepcion es obligatorio para validar el lote")
	}
	if s.sdk == nil {
		return nil, fmt.Errorf("siat masiva: servicio SIAT no inicializado")
	}

	request := models.NewValidacionRecepcionMasivaFacturaBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoDocumentoSector(req.sector()).
		WithCodigoEmision(req.codigoEmision()).
		WithTipoFacturaDocumento(req.tipoFactura()).
		WithCuis(req.Cuis).
		WithCufd(req.Cufd).
		WithCodigoRecepcion(codigoRecepcion).
		WithCodigoModalidad(req.Modalidad).
		Build()

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.validacionMasivaEnvio(ctx, req.sector(), req.Modalidad, request)
	if err != nil {
		return nil, fmt.Errorf("siat masiva: %w", err)
	}

	transaccion, codigoEstado, codigoRecepcionResp, mensajes, err := extraerResultadoFacturacion(resp)
	if err != nil {
		return nil, fmt.Errorf("siat masiva: %w", err)
	}

	return &ResultadoPaquete{
		Transaccion:     transaccion,
		CodigoEstado:    codigoEstado,
		CodigoRecepcion: codigoRecepcionResp,
		Mensajes:        mensajes,
	}, nil
}

// recepcionMasivaEnvio ejecuta recepcionMasivaFactura en el servicio del SDK
// adecuado para el documento-sector y la modalidad: CompraVenta() atiende los
// sectores 1, 35 y 41; el resto (p.ej. sector 11 educativo) se enruta por
// modalidad a Electronica() o Computarizada().
func (s *Service) recepcionMasivaEnvio(ctx context.Context, sector, modalidad int, req models.RecepcionMasivaFactura) (any, error) {
	switch sector {
	case SectorCompraVenta, 35, 41:
		return s.sdk.CompraVenta().RecepcionMasivaFactura(ctx, req)
	}
	envio := s.sdk.Electronica()
	if modalidad == ModalidadComputarizada {
		envio = s.sdk.Computarizada()
	}
	return envio.RecepcionMasivaFactura(ctx, req)
}

func (s *Service) validacionMasivaEnvio(ctx context.Context, sector, modalidad int, req models.ValidacionRecepcionMasivaFactura) (any, error) {
	switch sector {
	case SectorCompraVenta, 35, 41:
		return s.sdk.CompraVenta().ValidacionRecepcionMasivaFactura(ctx, req)
	}
	envio := s.sdk.Electronica()
	if modalidad == ModalidadComputarizada {
		envio = s.sdk.Computarizada()
	}
	return envio.ValidacionRecepcionMasivaFactura(ctx, req)
}

// normalized devuelve una copia de la solicitud en la que cada factura hereda
// la identidad común del lote (ambiente, sistema, NIT, modalidad, sucursal,
// punto de venta, CUIS, CUFD, código de control y documento-sector) cuando no
// la trae propia.
func (s SolicitudMasivaFactura) normalized() SolicitudMasivaFactura {
	out := s
	out.Facturas = make([]SolicitudFactura, len(s.Facturas))
	copy(out.Facturas, s.Facturas)
	for i := range out.Facturas {
		f := &out.Facturas[i]
		if f.CodigoAmbiente == 0 {
			f.CodigoAmbiente = s.CodigoAmbiente
		}
		if strings.TrimSpace(f.CodigoSistema) == "" {
			f.CodigoSistema = s.CodigoSistema
		}
		if strings.TrimSpace(f.Nit) == "" {
			f.Nit = s.Nit
		}
		if f.Modalidad == 0 {
			f.Modalidad = s.Modalidad
		}
		if f.CodigoSucursal == 0 {
			f.CodigoSucursal = s.CodigoSucursal
		}
		if f.CodigoPuntoVenta == 0 {
			f.CodigoPuntoVenta = s.CodigoPuntoVenta
		}
		if strings.TrimSpace(f.Cuis) == "" {
			f.Cuis = s.Cuis
		}
		if strings.TrimSpace(f.Cufd) == "" {
			f.Cufd = s.Cufd
		}
		if strings.TrimSpace(f.CodigoControl) == "" {
			f.CodigoControl = s.CodigoControl
		}
		if f.CodigoDocumentoSector == 0 {
			f.CodigoDocumentoSector = s.CodigoDocumentoSector
		}
		if f.CodigoTipoFactura == 0 {
			f.CodigoTipoFactura = s.CodigoTipoFactura
		}
	}
	return out
}

// extraerArchivoMasiva lee archivo, hashArchivo y cantidadFacturas del request
// interno del SDK ya construido, para auditoría. El campo request del wrapper es
// no exportado, por lo que se lee con reflexión (String()/Int()).
func extraerArchivoMasiva(masiva models.RecepcionMasivaFactura) (archivo, hash string, cantidad int) {
	v := reflect.ValueOf(masiva)
	wrapper := v.FieldByName("RequestWrapper")
	if !wrapper.IsValid() {
		return "", "", 0
	}
	request := wrapper.FieldByName("request")
	if !request.IsValid() || request.Kind() != reflect.Pointer {
		return "", "", 0
	}
	solicitud := request.Elem().FieldByName("SolicitudServicioRecepcionMasiva")
	if !solicitud.IsValid() {
		return "", "", 0
	}
	recep := solicitud.FieldByName("SolicitudRecepcionFactura")
	if !recep.IsValid() {
		return "", "", 0
	}
	if f := recep.FieldByName("Archivo"); f.IsValid() && f.Kind() == reflect.String {
		archivo = f.String()
	}
	if f := recep.FieldByName("HashArchivo"); f.IsValid() && f.Kind() == reflect.String {
		hash = f.String()
	}
	if f := solicitud.FieldByName("CantidadFacturas"); f.IsValid() && f.Kind() == reflect.Int {
		cantidad = int(f.Int())
	}
	return archivo, hash, cantidad
}

func (s SolicitudMasivaFactura) sector() int {
	if s.CodigoDocumentoSector <= 0 {
		return SectorCompraVenta
	}
	return s.CodigoDocumentoSector
}

func (s SolicitudMasivaFactura) tipoFactura() int {
	if s.CodigoTipoFactura <= 0 {
		return 1
	}
	return s.CodigoTipoFactura
}

func (s SolicitudMasivaFactura) codigoEmision() int {
	if s.CodigoEmision <= 0 {
		return EmisionMasiva
	}
	return s.CodigoEmision
}

// validateBase valida la identidad común del contribuyente que exigen tanto
// recepcionMasivaFactura como validacionRecepcionMasivaFactura.
func (s SolicitudMasivaFactura) validateBase() error {
	if s.CodigoAmbiente != AmbienteProduccion && s.CodigoAmbiente != AmbientePruebas {
		return fmt.Errorf("siat masiva: codigoAmbiente inválido")
	}
	if strings.TrimSpace(s.CodigoSistema) == "" {
		return fmt.Errorf("siat masiva: codigoSistema es obligatorio")
	}
	if strings.TrimSpace(s.Nit) == "" {
		return fmt.Errorf("siat masiva: nit es obligatorio")
	}
	if s.Modalidad != ModalidadElectronica && s.Modalidad != ModalidadComputarizada {
		return fmt.Errorf("siat masiva: modalidad inválida (%d)", s.Modalidad)
	}
	if s.CodigoSucursal < 0 || s.CodigoPuntoVenta < 0 {
		return fmt.Errorf("siat masiva: codigoSucursal y codigoPuntoVenta deben ser >= 0")
	}
	if strings.TrimSpace(s.Cuis) == "" || strings.TrimSpace(s.Cufd) == "" || strings.TrimSpace(s.CodigoControl) == "" {
		return fmt.Errorf("siat masiva: cuis, cufd y codigoControl son obligatorios")
	}
	return nil
}

func (s SolicitudMasivaFactura) validate() error {
	if err := s.validateBase(); err != nil {
		return err
	}
	if len(s.Facturas) == 0 {
		return fmt.Errorf("siat masiva: el lote debe contener al menos una factura")
	}
	if len(s.Facturas) > MaxFacturasMasiva {
		return fmt.Errorf("siat masiva: el lote supera el límite de %d facturas del SIN", MaxFacturasMasiva)
	}
	for i := range s.Facturas {
		if err := s.Facturas[i].validate(); err != nil {
			return fmt.Errorf("siat masiva factura %d: %w", i+1, err)
		}
	}
	return nil
}
