package siat

import (
	"context"

	fiscalsync "github.com/brandsrx/supay/internal/adapters/siat/sync"
)

type SincronizacionOp = fiscalsync.SincronizacionOp
type SolicitudSincronizacion = fiscalsync.SolicitudSincronizacion
type ParametricaDto = fiscalsync.ParametricaDto
type SinProductDto = fiscalsync.SinProductDto
type ActividadDto = fiscalsync.ActividadDto
type LeyendaDto = fiscalsync.LeyendaDto
type ActividadDocSectorDto = fiscalsync.ActividadDocSectorDto
type RespuestaSincronizacion = fiscalsync.RespuestaSincronizacion

const (
	OpTipoPuntoVenta             = fiscalsync.OpTipoPuntoVenta
	OpTipoMoneda                 = fiscalsync.OpTipoMoneda
	OpTipoMetodoPago             = fiscalsync.OpTipoMetodoPago
	OpTipoDocumentoIdentidad     = fiscalsync.OpTipoDocumentoIdentidad
	OpTipoEmision                = fiscalsync.OpTipoEmision
	OpTiposFactura               = fiscalsync.OpTiposFactura
	OpTipoHabitacion             = fiscalsync.OpTipoHabitacion
	OpTipoDocumentoSector        = fiscalsync.OpTipoDocumentoSector
	OpUnidadMedida               = fiscalsync.OpUnidadMedida
	OpMotivoAnulacion            = fiscalsync.OpMotivoAnulacion
	OpPaisOrigen                 = fiscalsync.OpPaisOrigen
	OpEventosSignificativos      = fiscalsync.OpEventosSignificativos
	OpMensajesServicios          = fiscalsync.OpMensajesServicios
	OpActividades                = fiscalsync.OpActividades
	OpProductosServicios         = fiscalsync.OpProductosServicios
	OpLeyendasFactura            = fiscalsync.OpLeyendasFactura
	OpActividadesDocumentoSector = fiscalsync.OpActividadesDocumentoSector
	OpFechaHora                  = fiscalsync.OpFechaHora
	OpVerificarComunicacion      = fiscalsync.OpVerificarComunicacion
)

var SincronizacionOperations = fiscalsync.SincronizacionOperations

func ParseSincronizacionOp(raw string) (SincronizacionOp, bool) {
	return fiscalsync.ParseSincronizacionOp(raw)
}
func (s *Service) Sincronizar(ctx context.Context, req SolicitudSincronizacion, op SincronizacionOp) (*RespuestaSincronizacion, error) {
	return fiscalsync.New(s.sdk).Sincronizar(ctx, req, op)
}
