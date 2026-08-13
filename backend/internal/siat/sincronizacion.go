package siat

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ron86i/go-siat/v2/pkg/models"
)

// SincronizacionOp identifica una operación de sincronización de catálogos del SIAT.
type SincronizacionOp string

const (
	OpTipoPuntoVenta             SincronizacionOp = "tipoPuntoVenta"
	OpTipoMoneda                 SincronizacionOp = "tipoMoneda"
	OpTipoMetodoPago             SincronizacionOp = "tipoMetodoPago"
	OpTipoDocumentoIdentidad     SincronizacionOp = "tipoDocumentoIdentidad"
	OpTipoEmision                SincronizacionOp = "tipoEmision"
	OpTiposFactura               SincronizacionOp = "tiposFactura"
	OpTipoHabitacion             SincronizacionOp = "tipoHabitacion"
	OpTipoDocumentoSector        SincronizacionOp = "tipoDocumentoSector"
	OpUnidadMedida               SincronizacionOp = "unidadMedida"
	OpMotivoAnulacion            SincronizacionOp = "motivoAnulacion"
	OpPaisOrigen                 SincronizacionOp = "paisOrigen"
	OpEventosSignificativos      SincronizacionOp = "eventosSignificativos"
	OpMensajesServicios          SincronizacionOp = "mensajesServicios"
	OpActividades                SincronizacionOp = "actividades"
	OpProductosServicios         SincronizacionOp = "productosServicios"
	OpLeyendasFactura            SincronizacionOp = "leyendasFactura"
	OpActividadesDocumentoSector SincronizacionOp = "actividadesDocumentoSector"
	OpFechaHora                  SincronizacionOp = "fechaHora"
	OpVerificarComunicacion      SincronizacionOp = "verificarComunicacion"
)

// SincronizacionOperations es el listado oficial de operaciones de sincronización
// expuestas por el SDK go-siat. Orden de definición del SDK.
var SincronizacionOperations = []SincronizacionOp{
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

// ParseSincronizacionOp convierte un nombre de operación en su constante tipada.
func ParseSincronizacionOp(raw string) (SincronizacionOp, bool) {
	op := SincronizacionOp(strings.TrimSpace(raw))
	for _, known := range SincronizacionOperations {
		if known == op {
			return op, true
		}
	}
	return "", false
}

// SolicitudSincronizacion son los datos por-solicitud para sincronizar un
// catálogo contra el SIAT. NIT, codigoSistema y codigoAmbiente se inyectan por
// empresa vía config dinámica (igual que CUIS/CUFD).
type SolicitudSincronizacion struct {
	CodigoAmbiente   int    `json:"codigoAmbiente"`
	CodigoSistema    string `json:"codigoSistema"`
	Nit              string `json:"nit"`
	CodigoSucursal   int    `json:"codigoSucursal"`
	CodigoPuntoVenta int    `json:"codigoPuntoVenta"`
	Cuis             string `json:"cuis"`
}

func (s SolicitudSincronizacion) Validate() error {
	if s.CodigoAmbiente != AmbienteProduccion && s.CodigoAmbiente != AmbientePruebas {
		return fmt.Errorf("siat solicitud sincronizacion: codigoAmbiente inválido")
	}
	if strings.TrimSpace(s.CodigoSistema) == "" {
		return fmt.Errorf("siat solicitud sincronizacion: codigoSistema es obligatorio")
	}
	if strings.TrimSpace(s.Nit) == "" {
		return fmt.Errorf("siat solicitud sincronizacion: nit es obligatorio")
	}
	if s.CodigoSucursal < 0 {
		return fmt.Errorf("siat solicitud sincronizacion: codigoSucursal debe ser >= 0")
	}
	if s.CodigoPuntoVenta < 0 {
		return fmt.Errorf("siat solicitud sincronizacion: codigoPuntoVenta debe ser >= 0")
	}
	if strings.TrimSpace(s.Cuis) == "" {
		return fmt.Errorf("siat solicitud sincronizacion: cuis es obligatorio")
	}
	return nil
}

// ParametricaDto es un elemento de un catálogo paramétrico (código + descripción).
type ParametricaDto struct {
	CodigoClasificador int    `json:"codigoClasificador"`
	Descripcion        string `json:"descripcion"`
}

// RespuestaSincronizacion es la respuesta normalizada de una sincronización.
type RespuestaSincronizacion struct {
	Transaccion bool            `json:"transaccion"`
	FechaHora   time.Time       `json:"fechaHora,omitempty"`
	Codigos     []ParametricaDto `json:"codigos,omitempty"`
	Mensajes    []Mensaje       `json:"mensajes,omitempty"`
}

// Sincronizar ejecuta la operación de sincronización indicada contra el SIAT
// usando el SDK go-siat.
//
// Nota: las respuestas de sincronización del SDK no incluyen mensajesList y no
// implementan common.Result, por lo que goSiat.Verify no aplica; solo se detecta
// Transaccion=false (sin el código de error específico del SIAT).
func (s *Service) Sincronizar(ctx context.Context, req SolicitudSincronizacion, op SincronizacionOp) (*RespuestaSincronizacion, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	switch op {
	case OpActividades:
		return s.sincronizarActividades(ctx, req)
	case OpProductosServicios:
		return s.sincronizarProductosServicios(ctx, req)
	case OpLeyendasFactura:
		return s.sincronizarLeyendasFactura(ctx, req)
	case OpActividadesDocumentoSector:
		return s.sincronizarActividadesDocumentoSector(ctx, req)
	case OpFechaHora:
		return s.sincronizarFechaHora(ctx, req)
	case OpVerificarComunicacion:
		return s.sincronizarVerificarComunicacion(ctx)
	case OpTipoPuntoVenta, OpTipoMoneda, OpTipoMetodoPago, OpTipoDocumentoIdentidad,
		OpTipoEmision, OpTiposFactura, OpTipoHabitacion, OpTipoDocumentoSector,
		OpUnidadMedida, OpMotivoAnulacion, OpPaisOrigen, OpEventosSignificativos,
		OpMensajesServicios:
		return s.sincronizarParametrica(ctx, req, op)
	}

	return nil, fmt.Errorf("siat sincronizar: operación desconocida %q", op)
}

// sincronizarParametrica cubre las 13 operaciones paramétricas simples que
// devuelven RespuestaListaParametricas (codigoClasificador + descripcion).
func (s *Service) sincronizarParametrica(ctx context.Context, req SolicitudSincronizacion, op SincronizacionOp) (*RespuestaSincronizacion, error) {
	var (
		resp any
		err  error
	)

	switch op {
	case OpTipoPuntoVenta:
		resp, err = s.sdk.Sincronizacion().SincronizarParametricaTipoPuntoVenta(ctx, models.NewSincronizarParametricaTipoPuntoVentaBuilder().
			WithCodigoSucursal(req.CodigoSucursal).WithCodigoPuntoVenta(req.CodigoPuntoVenta).WithCuis(req.Cuis).Build())
	case OpTipoMoneda:
		resp, err = s.sdk.Sincronizacion().SincronizarParametricaTipoMoneda(ctx, models.NewSincronizarParametricaTipoMonedaBuilder().
			WithCodigoSucursal(req.CodigoSucursal).WithCodigoPuntoVenta(req.CodigoPuntoVenta).WithCuis(req.Cuis).Build())
	case OpTipoMetodoPago:
		resp, err = s.sdk.Sincronizacion().SincronizarParametricaTipoMetodoPago(ctx, models.NewSincronizarParametricaTipoMetodoPagoBuilder().
			WithCodigoSucursal(req.CodigoSucursal).WithCodigoPuntoVenta(req.CodigoPuntoVenta).WithCuis(req.Cuis).Build())
	case OpTipoDocumentoIdentidad:
		resp, err = s.sdk.Sincronizacion().SincronizarParametricaTipoDocumentoIdentidad(ctx, models.NewSincronizarParametricaTipoDocumentoIdentidadBuilder().
			WithCodigoSucursal(req.CodigoSucursal).WithCodigoPuntoVenta(req.CodigoPuntoVenta).WithCuis(req.Cuis).Build())
	case OpTipoEmision:
		resp, err = s.sdk.Sincronizacion().SincronizarParametricaTipoEmision(ctx, models.NewSincronizarParametricaTipoEmisionBuilder().
			WithCodigoSucursal(req.CodigoSucursal).WithCodigoPuntoVenta(req.CodigoPuntoVenta).WithCuis(req.Cuis).Build())
	case OpTiposFactura:
		resp, err = s.sdk.Sincronizacion().SincronizarParametricaTiposFactura(ctx, models.NewSincronizarParametricaTiposFacturaBuilder().
			WithCodigoSucursal(req.CodigoSucursal).WithCodigoPuntoVenta(req.CodigoPuntoVenta).WithCuis(req.Cuis).Build())
	case OpTipoHabitacion:
		resp, err = s.sdk.Sincronizacion().SincronizarParametricaTipoHabitacion(ctx, models.NewSincronizarParametricaTipoHabitacionBuilder().
			WithCodigoSucursal(req.CodigoSucursal).WithCodigoPuntoVenta(req.CodigoPuntoVenta).WithCuis(req.Cuis).Build())
	case OpTipoDocumentoSector:
		resp, err = s.sdk.Sincronizacion().SincronizarParametricaTipoDocumentoSector(ctx, models.NewSincronizarParametricaTipoDocumentoSectorBuilder().
			WithCodigoSucursal(req.CodigoSucursal).WithCodigoPuntoVenta(req.CodigoPuntoVenta).WithCuis(req.Cuis).Build())
	case OpUnidadMedida:
		resp, err = s.sdk.Sincronizacion().SincronizarParametricaUnidadMedida(ctx, models.NewSincronizarParametricaUnidadMedidaBuilder().
			WithCodigoSucursal(req.CodigoSucursal).WithCodigoPuntoVenta(req.CodigoPuntoVenta).WithCuis(req.Cuis).Build())
	case OpMotivoAnulacion:
		resp, err = s.sdk.Sincronizacion().SincronizarParametricaMotivoAnulacion(ctx, models.NewSincronizarParametricaMotivoAnulacionBuilder().
			WithCodigoSucursal(req.CodigoSucursal).WithCodigoPuntoVenta(req.CodigoPuntoVenta).WithCuis(req.Cuis).Build())
	case OpPaisOrigen:
		resp, err = s.sdk.Sincronizacion().SincronizarParametricaPaisOrigen(ctx, models.NewSincronizarParametricaPaisOrigenBuilder().
			WithCodigoSucursal(req.CodigoSucursal).WithCodigoPuntoVenta(req.CodigoPuntoVenta).WithCuis(req.Cuis).Build())
	case OpEventosSignificativos:
		resp, err = s.sdk.Sincronizacion().SincronizarParametricaEventosSignificativos(ctx, models.NewSincronizarParametricaEventosSignificativosBuilder().
			WithCodigoSucursal(req.CodigoSucursal).WithCodigoPuntoVenta(req.CodigoPuntoVenta).WithCuis(req.Cuis).Build())
	case OpMensajesServicios:
		resp, err = s.sdk.Sincronizacion().SincronizarListaMensajesServicios(ctx, models.NewSincronizarListaMensajesServiciosBuilder().
			WithCodigoSucursal(req.CodigoSucursal).WithCodigoPuntoVenta(req.CodigoPuntoVenta).WithCuis(req.Cuis).Build())
	}
	if err != nil {
		return nil, fmt.Errorf("siat sincronizar %s: %w", op, err)
	}

	return toRespuestaParametrica(op, resp)
}

// toRespuestaParametrica normaliza una RespuestaListaParametricas del SDK. Se
// usa reflexión porque el tipo concreto vive en un paquete interno del SDK.
func toRespuestaParametrica(op SincronizacionOp, resp any) (*RespuestaSincronizacion, error) {
	v := reflect.ValueOf(resp)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if !v.IsValid() {
		return nil, fmt.Errorf("siat sincronizar %s: respuesta vacía", op)
	}

	// Navegar EnvelopeResponse.Body.Content.RespuestaListaParametricas.
	if fBody := v.FieldByName("Body"); fBody.IsValid() {
		if fContent := fBody.FieldByName("Content"); fContent.IsValid() {
			if fResp := fContent.FieldByName("RespuestaListaParametricas"); fResp.IsValid() {
				v = fResp
			}
		}
	}

	fTrans := v.FieldByName("Transaccion")
	fLista := v.FieldByName("ListaCodigos")
	if !fTrans.IsValid() {
		return nil, fmt.Errorf("siat sincronizar %s: respuesta inesperada", op)
	}
	transaccion := fTrans.Bool()
	if !transaccion {
		return nil, errSincronizacionRechazada(op)
	}
	return &RespuestaSincronizacion{
		Transaccion: transaccion,
		Codigos:     toParametricas(fLista),
	}, nil
}

// toParametricas convierte una lista de codigos del SDK (codigoClasificador +
// descripcion) en DTOs propios mediante reflexión.
func toParametricas(list reflect.Value) []ParametricaDto {
	if !list.IsValid() || list.Kind() != reflect.Slice {
		return nil
	}
	out := make([]ParametricaDto, 0, list.Len())
	for i := 0; i < list.Len(); i++ {
		elem := list.Index(i)
		if elem.Kind() == reflect.Pointer {
			elem = elem.Elem()
		}
		var p ParametricaDto
		if f := elem.FieldByName("CodigoClasificador"); f.IsValid() {
			p.CodigoClasificador = int(f.Uint())
		}
		if f := elem.FieldByName("Descripcion"); f.IsValid() {
			p.Descripcion = f.String()
		}
		out = append(out, p)
	}
	return out
}

// sincronizarActividades baja el catálogo de actividades económicas (codigoCaeb).
func (s *Service) sincronizarActividades(ctx context.Context, req SolicitudSincronizacion) (*RespuestaSincronizacion, error) {
	request := models.NewSincronizarActividadesBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCuis(req.Cuis).
		Build()
	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.sdk.Sincronizacion().SincronizarActividades(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("siat sincronizar actividades: %w", err)
	}
	r := resp.Body.Content.RespuestaListaActividades
	if !r.Transaccion {
		return nil, errSincronizacionRechazada(OpActividades)
	}
	out := make([]ParametricaDto, 0, len(r.ListaActividades))
	for _, a := range r.ListaActividades {
		out = append(out, ParametricaDto{
			CodigoClasificador: parseCodigoInt(a.CodigoCaeb),
			Descripcion:        a.Descripcion,
		})
	}
	return &RespuestaSincronizacion{Transaccion: r.Transaccion, Codigos: out}, nil
}

// sincronizarProductosServicios baja el catálogo de productos y servicios homologados.
func (s *Service) sincronizarProductosServicios(ctx context.Context, req SolicitudSincronizacion) (*RespuestaSincronizacion, error) {
	request := models.NewSincronizarListaProductosServiciosBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCuis(req.Cuis).
		Build()
	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.sdk.Sincronizacion().SincronizarListaProductosServicios(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("siat sincronizar productosServicios: %w", err)
	}
	r := resp.Body.Content.RespuestaListaProductos
	if !r.Transaccion {
		return nil, errSincronizacionRechazada(OpProductosServicios)
	}
	out := make([]ParametricaDto, 0, len(r.ListaCodigos))
	for _, p := range r.ListaCodigos {
		out = append(out, ParametricaDto{
			CodigoClasificador: int(p.CodigoProducto),
			Descripcion:        p.DescripcionProducto,
		})
	}
	return &RespuestaSincronizacion{Transaccion: r.Transaccion, Codigos: out}, nil
}

// sincronizarLeyendasFactura baja las leyendas asociadas a actividades económicas.
func (s *Service) sincronizarLeyendasFactura(ctx context.Context, req SolicitudSincronizacion) (*RespuestaSincronizacion, error) {
	request := models.NewSincronizarListaLeyendasFacturaBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCuis(req.Cuis).
		Build()
	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.sdk.Sincronizacion().SincronizarListaLeyendasFactura(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("siat sincronizar leyendasFactura: %w", err)
	}
	r := resp.Body.Content.RespuestaListaParametricasLeyendas
	if !r.Transaccion {
		return nil, errSincronizacionRechazada(OpLeyendasFactura)
	}
	var out []ParametricaDto
	if r.ListaLeyendas != nil {
		out = make([]ParametricaDto, 0, len(*r.ListaLeyendas))
		for _, l := range *r.ListaLeyendas {
			out = append(out, ParametricaDto{
				Descripcion: l.CodigoActividad + ": " + l.DescripcionLeyenda,
			})
		}
	}
	return &RespuestaSincronizacion{Transaccion: r.Transaccion, Codigos: out}, nil
}

// sincronizarActividadesDocumentoSector baja la relación actividad/documento sector.
func (s *Service) sincronizarActividadesDocumentoSector(ctx context.Context, req SolicitudSincronizacion) (*RespuestaSincronizacion, error) {
	request := models.NewSincronizarListaActividadesDocumentoSectorBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCuis(req.Cuis).
		Build()
	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.sdk.Sincronizacion().SincronizarListaActividadesDocumentoSector(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("siat sincronizar actividadesDocumentoSector: %w", err)
	}
	r := resp.Body.Content.RespuestaListaActividadesDocumentoSector
	if !r.Transaccion {
		return nil, errSincronizacionRechazada(OpActividadesDocumentoSector)
	}
	out := make([]ParametricaDto, 0, len(r.ListaActividadesDocumentoSector))
	for _, a := range r.ListaActividadesDocumentoSector {
		out = append(out, ParametricaDto{
			CodigoClasificador: int(a.CodigoDocumentoSector),
			Descripcion:        a.CodigoActividad + "|" + a.TipoDocumentoSector,
		})
	}
	return &RespuestaSincronizacion{Transaccion: r.Transaccion, Codigos: out}, nil
}

// sincronizarFechaHora obtiene la fecha y hora oficial del servidor del SIAT.
func (s *Service) sincronizarFechaHora(ctx context.Context, req SolicitudSincronizacion) (*RespuestaSincronizacion, error) {
	request := models.NewSincronizarFechaHoraBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCuis(req.Cuis).
		Build()
	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.sdk.Sincronizacion().SincronizarFechaHora(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("siat sincronizar fechaHora: %w", err)
	}
	r := resp.Body.Content.RespuestaFechaHora
	if !r.Transaccion {
		return nil, errSincronizacionRechazada(OpFechaHora)
	}
	return &RespuestaSincronizacion{Transaccion: r.Transaccion, FechaHora: r.FechaHora.ToTime()}, nil
}

// sincronizarVerificarComunicacion comprueba la conectividad con el SIAT.
func (s *Service) sincronizarVerificarComunicacion(ctx context.Context) (*RespuestaSincronizacion, error) {
	request := models.NewVerificarComunicacionSincronizacionBuilder().Build()
	resp, err := s.sdk.Sincronizacion().VerificarComunicacion(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("siat sincronizar verificarComunicacion: %w", err)
	}
	r := resp.Body.Content.Return
	if !r.Transaccion {
		return nil, errSincronizacionRechazada(OpVerificarComunicacion)
	}
	return &RespuestaSincronizacion{Transaccion: r.Transaccion, Mensajes: toMensajes(r.MensajesList)}, nil
}

func errSincronizacionRechazada(op SincronizacionOp) error {
	return fmt.Errorf("siat sincronizar %s: operación rechazada por el SIAT", op)
}

func parseCodigoInt(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return n
}
