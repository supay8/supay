package siat

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/ron86i/go-siat/v2/pkg/models"
)

// SolicitudEventoSignificativo son los datos para registrar un evento
// significativo ante el SIAT (registroEventoSignificativo), p.ej. una
// contingencia por corte del servicio de internet (codigoMotivoEvento=1).
type SolicitudEventoSignificativo struct {
	CodigoAmbiente   int    `json:"codigoAmbiente"`
	CodigoSistema    string `json:"codigoSistema"`
	Nit              string `json:"nit"`
	CodigoSucursal   int    `json:"codigoSucursal"`
	CodigoPuntoVenta int    `json:"codigoPuntoVenta"`
	Cuis             string `json:"cuis"`
	Cufd             string `json:"cufd"`
	// CufdEvento es el CUFD con el que se generó o debió generarse el evento
	// (p.ej. el CUFD vencido durante la contingencia).
	CufdEvento string `json:"cufdEvento"`
	// CodigoMotivoEvento es el motivo del catálogo sincronizado
	// eventosSignificativos (1 = corte del servicio de internet).
	CodigoMotivoEvento int `json:"codigoMotivoEvento"`
	// Descripcion es la descripción del evento (obligatoria según el esquema SIAT).
	Descripcion string `json:"descripcion"`
	// FechaHoraInicioEvento y FechaHoraFinEvento definen la ventana del evento
	// en formato UTC extendido (YYYY-MM-DDTHH:mm:ss.SSS).
	FechaHoraInicioEvento time.Time `json:"fechaHoraInicioEvento"`
	FechaHoraFinEvento    time.Time `json:"fechaHoraFinEvento"`
}

// ResultadoEventoSignificativo es la respuesta procesada de
// registroEventoSignificativo (RespuestaListaEventos del SIAT).
type ResultadoEventoSignificativo struct {
	Transaccion     bool      `json:"transaccion"`
	CodigoRecepcion string    `json:"codigoRecepcion,omitempty"`
	Mensajes        []Mensaje `json:"mensajes,omitempty"`
}

// MotivoCorteInternet es el código de motivo del catálogo eventosSignificativos
// para contingencia por corte del servicio de internet.
const MotivoCorteInternet = 1

// RegistrarEventoSignificativo registra un evento significativo (p.ej. una
// contingencia por corte de internet) ante el SIAT usando el SDK go-siat. El
// NIT, codigoSistema y codigoAmbiente se inyectan dinámicamente desde la
// configuración por empresa en el contexto de la petición.
func (s *Service) RegistrarEventoSignificativo(ctx context.Context, req SolicitudEventoSignificativo) (*ResultadoEventoSignificativo, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}
	if s.sdk == nil {
		return nil, fmt.Errorf("siat evento significativo: servicio SIAT no inicializado")
	}

	// El SIAT exige fechas del evento en UTC; el dominio del SDK serializa
	// time.Time con su zona (Z/-04:00), así que normalizamos a UTC aquí.
	request := models.NewRegistroEventoSignificativoBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCuis(req.Cuis).
		WithCufd(req.Cufd).
		WithCufdEvento(req.CufdEvento).
		WithCodigoMotivoEvento(req.CodigoMotivoEvento).
		WithDescripcion(req.Descripcion).
		WithFechaInicio(req.FechaHoraInicioEvento.UTC()).
		WithFechaFin(req.FechaHoraFinEvento.UTC()).
		Build()

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.sdk.Operaciones().RegistroEventosSignificativos(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("siat evento significativo: %w", err)
	}

	transaccion, codigoRecepcion, mensajes, err := extraerResultadoEvento(resp)
	if err != nil {
		return nil, fmt.Errorf("siat evento significativo: %w", err)
	}
	if !transaccion {
		return nil, &eventoRejectedError{CodigoRecepcion: codigoRecepcion, Mensajes: mensajes}
	}
	return &ResultadoEventoSignificativo{
		Transaccion:     transaccion,
		CodigoRecepcion: codigoRecepcion,
		Mensajes:        mensajes,
	}, nil
}

// eventoRejectedError indica que el SIAT respondió y rechazó el registro del
// evento significativo (Transaccion=false).
type eventoRejectedError struct {
	CodigoRecepcion string
	Mensajes        []Mensaje
}

func (e *eventoRejectedError) Error() string {
	var parts []string
	for _, m := range e.Mensajes {
		parts = append(parts, fmt.Sprintf("[%d] %s", m.Codigo, m.Descripcion))
	}
	if len(parts) == 0 {
		return "el evento significativo fue rechazado por el SIAT"
	}
	return "el evento significativo fue rechazado por el SIAT: " + strings.Join(parts, "; ")
}

// extraerResultadoEvento lee Transaccion, CodigoRecepcionEventoSignificativo y
// MensajesList de la RespuestaListaEventos de la respuesta de registro de
// eventos del SDK. Se usa reflexión porque los tipos de respuesta viven en
// paquetes internos del SDK (no importables por nombre).
func extraerResultadoEvento(resp any) (transaccion bool, codigoRecepcion string, mensajes []Mensaje, err error) {
	v := reflect.ValueOf(resp)
	if !v.IsValid() || v.Kind() != reflect.Pointer {
		return false, "", nil, fmt.Errorf("respuesta del SIAT inválida")
	}
	body := v.Elem().FieldByName("Body")
	if !body.IsValid() {
		return false, "", nil, fmt.Errorf("respuesta del SIAT sin Body")
	}
	content := body.FieldByName("Content")
	if !content.IsValid() {
		return false, "", nil, fmt.Errorf("respuesta del SIAT sin Content")
	}
	rs := content.FieldByName("Respuesta")
	if !rs.IsValid() {
		return false, "", nil, fmt.Errorf("respuesta del SIAT sin Respuesta")
	}
	if f := rs.FieldByName("Transaccion"); f.IsValid() {
		transaccion = f.Bool()
	}
	if f := rs.FieldByName("CodigoRecepcionEventoSignificativo"); f.IsValid() {
		codigoRecepcion = fmt.Sprintf("%d", f.Int())
	}
	if f := rs.FieldByName("MensajesList"); f.IsValid() {
		mensajes = toMensajes(f.Interface())
	}
	return transaccion, codigoRecepcion, mensajes, nil
}

func (s SolicitudEventoSignificativo) validate() error {
	if s.CodigoAmbiente != AmbienteProduccion && s.CodigoAmbiente != AmbientePruebas {
		return fmt.Errorf("siat evento significativo: codigoAmbiente inválido")
	}
	if strings.TrimSpace(s.CodigoSistema) == "" {
		return fmt.Errorf("siat evento significativo: codigoSistema es obligatorio")
	}
	if strings.TrimSpace(s.Nit) == "" {
		return fmt.Errorf("siat evento significativo: nit es obligatorio")
	}
	if s.CodigoSucursal < 0 || s.CodigoPuntoVenta < 0 {
		return fmt.Errorf("siat evento significativo: codigoSucursal y codigoPuntoVenta deben ser >= 0")
	}
	if strings.TrimSpace(s.Cuis) == "" {
		return fmt.Errorf("siat evento significativo: cuis es obligatorio")
	}
	if strings.TrimSpace(s.Cufd) == "" {
		return fmt.Errorf("siat evento significativo: cufd es obligatorio")
	}
	if strings.TrimSpace(s.CufdEvento) == "" {
		return fmt.Errorf("siat evento significativo: cufdEvento es obligatorio")
	}
	if s.CodigoMotivoEvento <= 0 {
		return fmt.Errorf("siat evento significativo: codigoMotivoEvento debe ser mayor a cero")
	}
	if strings.TrimSpace(s.Descripcion) == "" {
		return fmt.Errorf("siat evento significativo: descripcion es obligatoria")
	}
	if s.FechaHoraInicioEvento.IsZero() || s.FechaHoraFinEvento.IsZero() {
		return fmt.Errorf("siat evento significativo: fechaHoraInicioEvento y fechaHoraFinEvento son obligatorias")
	}
	if !s.FechaHoraFinEvento.After(s.FechaHoraInicioEvento) {
		return fmt.Errorf("siat evento significativo: fechaHoraFinEvento debe ser posterior a fechaHoraInicioEvento")
	}
	return nil
}
