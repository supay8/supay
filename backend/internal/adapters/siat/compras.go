package siat

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/ron86i/go-siat/v2/pkg/models"
)

// MaxFacturasCompras es el tope de facturas por paquete de compras que admite el
// SIN (cantidadFacturas menor a 10).
const MaxFacturasCompras = 9

// SolicitudCompras agrupa los datos de un paquete de facturas de compras para
// recepcionPaqueteCompras. codigoPuntoVenta no aplica para el servicio de
// compras del SIAT (debe ser 0); la identidad del contribuyente
// (codigoAmbiente, codigoSistema, nit) se inyecta con withDynamicConfig.
// Descripcion y TipoCompra son informativos del lote: el SIAT los recibe dentro
// de los registros del archivo, no en la solicitud SOAP.
type SolicitudCompras struct {
	Descripcion string `json:"descripcion"`
	TipoCompra  int    `json:"tipoCompra"`

	CodigoAmbiente int    `json:"codigoAmbiente"`
	CodigoSistema  string `json:"codigoSistema"`
	Nit            string `json:"nit"`
	CodigoSucursal int    `json:"codigoSucursal"`
	// CodigoPuntoVenta no aplica en compras; el SIAT exige 0.
	CodigoPuntoVenta int    `json:"codigoPuntoVenta"`
	Cuis             string `json:"cuis"`
	Cufd             string `json:"cufd"`

	Archivo          string    `json:"archivo"`
	HashArchivo      string    `json:"hashArchivo"`
	CantidadFacturas int       `json:"cantidadFacturas"`
	Gestion          int       `json:"gestion"`
	Periodo          int       `json:"periodo"`
	FechaEnvio       time.Time `json:"fechaEnvio"`
}

// ResultadoCompras es la respuesta normalizada de recepcionPaqueteCompras.
type ResultadoCompras struct {
	Transaccion     bool      `json:"transaccion"`
	CodigoEstado    int       `json:"codigo_estado"`
	CodigoRecepcion string    `json:"codigo_recepcion"`
	Mensajes        []Mensaje `json:"mensajes,omitempty"`
}

// EnviarCompras envía al SIAT un paquete de facturas de compras
// (recepcionPaqueteCompras) para la empresa y punto de venta indicados. El
// archivo (Base64 de un TAR.GZ con los registros de compra) y su hash SHA-256
// los genera el integrador; el SDK se encarga del transporte, la inyección de
// identidad y la firma/empaquetado si aplicara.
func (s *Service) EnviarCompras(ctx context.Context, req SolicitudCompras) (*ResultadoCompras, error) {
	if s.sdk == nil {
		return nil, fmt.Errorf("siat compras: servicio SIAT no inicializado")
	}
	if err := applyIdentityValues(s.sdk.Config(), &req.CodigoAmbiente, &req.CodigoSistema, &req.Nit); err != nil {
		return nil, err
	}
	if err := req.validate(); err != nil {
		return nil, err
	}

	fechaEnvio := req.FechaEnvio
	if fechaEnvio.IsZero() {
		fechaEnvio = time.Now()
	}

	request := models.NewRecepcionPaqueteComprasBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCuis(req.Cuis).
		WithCufd(req.Cufd).
		WithArchivo(req.Archivo).
		WithCantidadFacturas(req.CantidadFacturas).
		WithGestion(req.Gestion).
		WithHashArchivo(req.HashArchivo).
		WithPeriodo(req.Periodo).
		// El SIAT interpreta la hora de pared sin zona de fechaEnvio como hora
		// local de Bolivia (UTC-4), por lo que se envía la hora de pared de La Paz.
		WithFechaEnvio(fechaEnvio.In(LaPaz)).
		Build()

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.sdk.RecepcionCompras().RecepcionPaqueteCompras(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("siat compras: %w", err)
	}

	transaccion, codigoEstado, codigoRecepcion, mensajes, err := extraerResultadoCompras(resp)
	if err != nil {
		return nil, fmt.Errorf("siat compras: %w", err)
	}

	return &ResultadoCompras{
		Transaccion:     transaccion,
		CodigoEstado:    codigoEstado,
		CodigoRecepcion: codigoRecepcion,
		Mensajes:        mensajes,
	}, nil
}

// validate valida los prerrequisitos de la solicitud de compras contra los
// lineamientos del SIN (cantidadFacturas menor a 10, periodo 1-12, gestión con
// 4 dígitos y codigoPuntoVenta = 0).
func (s SolicitudCompras) validate() error {
	if s.CodigoAmbiente != AmbienteProduccion && s.CodigoAmbiente != AmbientePruebas {
		return fmt.Errorf("siat compras: codigoAmbiente inválido")
	}
	if strings.TrimSpace(s.CodigoSistema) == "" {
		return fmt.Errorf("siat compras: codigoSistema es obligatorio")
	}
	if strings.TrimSpace(s.Nit) == "" {
		return fmt.Errorf("siat compras: nit es obligatorio")
	}
	if s.CodigoSucursal < 0 || s.CodigoPuntoVenta != 0 {
		return fmt.Errorf("siat compras: codigoPuntoVenta no aplica y debe ser 0")
	}
	if strings.TrimSpace(s.Cuis) == "" || strings.TrimSpace(s.Cufd) == "" {
		return fmt.Errorf("siat compras: cuis y cufd son obligatorios")
	}
	if strings.TrimSpace(s.Archivo) == "" || strings.TrimSpace(s.HashArchivo) == "" {
		return fmt.Errorf("siat compras: archivo y hashArchivo son obligatorios")
	}
	if s.CantidadFacturas <= 0 {
		return fmt.Errorf("siat compras: cantidadFacturas debe ser mayor a 0")
	}
	if s.CantidadFacturas > MaxFacturasCompras {
		return fmt.Errorf("siat compras: cantidadFacturas debe ser menor a 10 (máximo %d)", MaxFacturasCompras)
	}
	if s.Gestion < 2000 || s.Gestion > 2100 {
		return fmt.Errorf("siat compras: gestion inválida (%d)", s.Gestion)
	}
	if s.Periodo < 1 || s.Periodo > 12 {
		return fmt.Errorf("siat compras: periodo inválido (%d), debe estar entre 1 y 12", s.Periodo)
	}
	if s.TipoCompra <= 0 {
		return fmt.Errorf("siat compras: tipoCompra es obligatorio (1 = INTERNO/ACTIVIDADES GRAVADAS)")
	}
	return nil
}

// extraerResultadoCompras lee Transaccion, CodigoEstado, CodigoRecepcion y
// MensajesList de la respuesta de recepcionPaqueteCompras. A diferencia de
// extraerResultadoFacturacion, el contenido lleva el campo RespuestaRecepcion
// (con tag XML RespuestaServicioFacturacion) y no implementa common.Result.
func extraerResultadoCompras(resp any) (transaccion bool, codigoEstado int, codigoRecepcion string, mensajes []Mensaje, err error) {
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
	rs := content.FieldByName("RespuestaRecepcion")
	if !rs.IsValid() {
		return false, 0, "", nil, fmt.Errorf("respuesta del SIAT sin RespuestaRecepcion")
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
