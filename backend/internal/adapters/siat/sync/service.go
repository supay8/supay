package sync

import (
	"context"

	"github.com/brandsrx/supay/internal/ports"
	goSiat "github.com/ron86i/go-siat/v2"
)

type Service struct{ sdk *goSiat.SiatServices }

func New(sdk *goSiat.SiatServices) *Service { return &Service{sdk: sdk} }

var _ ports.FiscalSynchronizer = (*Service)(nil)

func (a *Service) RequestCUIS(ctx context.Context, req ports.CredentialRequest) (ports.CuisResult, error) {
	res, err := a.SolicitarCUIS(ctx, toSiatSolicitudCuis(req))
	if err != nil {
		return ports.CuisResult{}, err
	}
	return fromSiatRespuestaCuis(res), nil
}

func (a *Service) RequestCUFD(ctx context.Context, req ports.CredentialRequest) (ports.CufdResult, error) {
	res, err := a.SolicitarCUFD(ctx, toSiatSolicitudCufd(req))
	if err != nil {
		return ports.CufdResult{}, err
	}
	return fromSiatRespuestaCufd(res), nil
}

func (a *Service) Synchronize(ctx context.Context, req ports.FiscalSyncRequest, op ports.FiscalSyncOperation) (ports.FiscalSyncResult, error) {
	res, err := a.Sincronizar(ctx, toSiatSolicitudSincronizacion(req), SincronizacionOp(op))
	if err != nil {
		return ports.FiscalSyncResult{}, err
	}
	return fromSiatRespuestaSincronizacion(res), nil
}

func toSiatSolicitudCuis(req ports.CredentialRequest) SolicitudCuis {
	var cuis *string
	if req.Cuis != "" {
		cuis = &req.Cuis
	}
	return SolicitudCuis{
		CodigoAmbiente:   req.CodigoAmbiente,
		CodigoSistema:    req.CodigoSistema,
		Nit:              req.Nit,
		CodigoSucursal:   req.CodigoSucursal,
		CodigoModalidad:  req.CodigoModalidad,
		CodigoPuntoVenta: req.CodigoPuntoVenta,
		Cuis:             cuis,
	}
}

func toSiatSolicitudCufd(req ports.CredentialRequest) SolicitudCufd {
	return SolicitudCufd{
		CodigoAmbiente:   req.CodigoAmbiente,
		CodigoSistema:    req.CodigoSistema,
		Nit:              req.Nit,
		CodigoSucursal:   req.CodigoSucursal,
		CodigoModalidad:  req.CodigoModalidad,
		CodigoPuntoVenta: req.CodigoPuntoVenta,
		Cuis:             req.Cuis,
	}
}

func toSiatSolicitudSincronizacion(req ports.FiscalSyncRequest) SolicitudSincronizacion {
	return SolicitudSincronizacion{
		CodigoAmbiente:   req.CodigoAmbiente,
		CodigoSistema:    req.CodigoSistema,
		Nit:              req.Nit,
		CodigoSucursal:   req.CodigoSucursal,
		CodigoPuntoVenta: req.CodigoPuntoVenta,
		Cuis:             req.Cuis,
	}
}

func fromSiatRespuestaCuis(r *RespuestaCuis) ports.CuisResult {
	if r == nil {
		return ports.CuisResult{}
	}
	return ports.CuisResult{
		Codigo:        r.Codigo,
		FechaVigencia: r.FechaVigencia.Time,
		Transaccion:   r.Transaccion,
		Mensajes:      fromSiatMensajes(r.Mensajes),
	}
}

func fromSiatRespuestaCufd(r *RespuestaCufd) ports.CufdResult {
	if r == nil {
		return ports.CufdResult{}
	}
	return ports.CufdResult{
		Codigo:        r.Codigo,
		CodigoControl: r.CodigoControl,
		CodigoQR:      r.CodigoQR,
		Direccion:     r.Direccion,
		FechaVigencia: r.FechaVigencia.Time,
		Transaccion:   r.Transaccion,
		Mensajes:      fromSiatMensajes(r.Mensajes),
	}
}

func fromSiatRespuestaSincronizacion(r *RespuestaSincronizacion) ports.FiscalSyncResult {
	if r == nil {
		return ports.FiscalSyncResult{}
	}
	return ports.FiscalSyncResult{
		Transaccion:          r.Transaccion,
		FechaHora:            r.FechaHora,
		Codigos:              fromSiatParametricas(r.Codigos),
		Productos:            fromSiatSinProducts(r.Productos),
		Actividades:          fromSiatActividades(r.Actividades),
		Leyendas:             fromSiatLeyendas(r.Leyendas),
		ActividadesDocSector: fromSiatActividadesDocSector(r.ActividadesDocSector),
		Mensajes:             fromSiatMensajes(r.Mensajes),
	}
}

func fromSiatMensajes(msgs []Mensaje) []ports.FiscalMessage {
	out := make([]ports.FiscalMessage, len(msgs))
	for i, m := range msgs {
		out[i] = ports.FiscalMessage{Codigo: m.Codigo, Descripcion: m.Descripcion}
	}
	return out
}

func fromSiatParametricas(c []ParametricaDto) []ports.FiscalParametricItem {
	out := make([]ports.FiscalParametricItem, len(c))
	for i, p := range c {
		out[i] = ports.FiscalParametricItem{CodigoClasificador: p.CodigoClasificador, Descripcion: p.Descripcion}
	}
	return out
}

func fromSiatSinProducts(p []SinProductDto) []ports.FiscalSinProduct {
	out := make([]ports.FiscalSinProduct, len(p))
	for i, x := range p {
		out[i] = ports.FiscalSinProduct{CodigoProductoSin: x.CodigoProductoSin, CodigoActividad: x.CodigoActividad, Descripcion: x.Descripcion}
	}
	return out
}

func fromSiatActividades(a []ActividadDto) []ports.FiscalActivity {
	out := make([]ports.FiscalActivity, len(a))
	for i, x := range a {
		out[i] = ports.FiscalActivity{CodigoCaeb: x.CodigoCaeb, Descripcion: x.Descripcion, TipoActividad: x.TipoActividad}
	}
	return out
}

func fromSiatLeyendas(l []LeyendaDto) []ports.FiscalLegend {
	out := make([]ports.FiscalLegend, len(l))
	for i, x := range l {
		out[i] = ports.FiscalLegend{CodigoActividad: x.CodigoActividad, DescripcionLeyenda: x.DescripcionLeyenda}
	}
	return out
}

func fromSiatActividadesDocSector(a []ActividadDocSectorDto) []ports.FiscalActivityDocSector {
	out := make([]ports.FiscalActivityDocSector, len(a))
	for i, x := range a {
		out[i] = ports.FiscalActivityDocSector{CodigoActividad: x.CodigoActividad, CodigoDocumentoSector: x.CodigoDocumentoSector, TipoDocumentoSector: x.TipoDocumentoSector}
	}
	return out
}
