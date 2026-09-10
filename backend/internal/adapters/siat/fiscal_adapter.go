package siat

import (
	"context"
	"encoding/json"
	"log"

	"github.com/brandsrx/supay/internal/ports"
)

// FiscalAdapter adapta el siat.Service concreto al puerto ports.FiscalService,
// manteniendo el dominio desacoplado del SDK go-siat.
type FiscalAdapter struct {
	svc *Service
}

// CodigoSistema expone el codigoSistema del servicio subyacente (para sincronización).
func (a *FiscalAdapter) CodigoSistema() string {
	if a == nil || a.svc == nil {
		return ""
	}
	return a.svc.CodigoSistema()
}

// Svc expone el servicio subyacente (para casos que necesitan config).
func (a *FiscalAdapter) Svc() *Service { return a.svc }

// NewFiscalAdapter crea un adaptador de puerto alrededor de un *Service SIAT.
func NewFiscalAdapter(svc *Service) *FiscalAdapter {
	return &FiscalAdapter{svc: svc}
}

// Compile-time check: FiscalAdapter implementa ports.FiscalService.
var _ ports.FiscalService = (*FiscalAdapter)(nil)
var _ ports.OfflineFiscalService = (*FiscalAdapter)(nil)

func (a *FiscalAdapter) Emit(ctx context.Context, doc ports.FiscalDocument) (ports.FiscalResult, error) {
	res, err := a.svc.EmitirFactura(ctx, toSiatSolicitudFactura(doc))
	if err != nil {
		log.Println("DEBUG F1", err)

		return ports.FiscalResult{}, err
	}
	return fromSiatResultadoEmision(res), nil
}

func (a *FiscalAdapter) PrepareOffline(ctx context.Context, doc ports.FiscalDocument) (ports.FiscalResult, error) {
	res, err := a.svc.PrepararFacturaOffline(ctx, toSiatSolicitudFactura(doc))
	if err != nil {
		return ports.FiscalResult{}, err
	}
	return fromSiatResultadoEmision(res), nil
}

func (a *FiscalAdapter) VerifyStatus(ctx context.Context, query ports.FiscalDocumentQuery) (ports.FiscalDocumentResult, error) {
	res, err := a.svc.VerificarEstado(ctx, toSiatSolicitudDocumento(query))
	if err != nil {
		return ports.FiscalDocumentResult{}, err
	}
	return fromSiatResultadoDocumento(res), nil
}

func (a *FiscalAdapter) Annul(ctx context.Context, query ports.FiscalDocumentQuery, codigoMotivo int) (ports.FiscalDocumentResult, error) {
	res, err := a.svc.AnularFactura(ctx, toSiatSolicitudDocumento(query), codigoMotivo)
	if err != nil {
		return ports.FiscalDocumentResult{}, err
	}
	return fromSiatResultadoDocumento(res), nil
}

func (a *FiscalAdapter) RevertAnnul(ctx context.Context, query ports.FiscalDocumentQuery) (ports.FiscalDocumentResult, error) {
	res, err := a.svc.RevertirAnulacion(ctx, toSiatSolicitudDocumento(query))
	if err != nil {
		return ports.FiscalDocumentResult{}, err
	}
	return fromSiatResultadoDocumento(res), nil
}

func (a *FiscalAdapter) RequestCUIS(ctx context.Context, req ports.CredentialRequest) (ports.CuisResult, error) {
	res, err := a.svc.SolicitarCUIS(ctx, toSiatSolicitudCuis(req))
	if err != nil {
		return ports.CuisResult{}, err
	}
	return fromSiatRespuestaCuis(res), nil
}

func (a *FiscalAdapter) RequestCUFD(ctx context.Context, req ports.CredentialRequest) (ports.CufdResult, error) {
	res, err := a.svc.SolicitarCUFD(ctx, toSiatSolicitudCufd(req))
	if err != nil {
		return ports.CufdResult{}, err
	}
	return fromSiatRespuestaCufd(res), nil
}

func (a *FiscalAdapter) RegisterSignificantEvent(ctx context.Context, ev ports.FiscalEvent) (ports.FiscalEventResult, error) {
	res, err := a.svc.RegistrarEventoSignificativo(ctx, toSiatEvento(ev))
	if err != nil {
		return ports.FiscalEventResult{}, err
	}
	return fromSiatResultadoEvento(res), nil
}

func (a *FiscalAdapter) SendPackage(ctx context.Context, pkg ports.FiscalPackage) (ports.FiscalPackageResult, error) {
	res, err := a.svc.EnviarPaqueteFactura(ctx, toSiatPaquete(pkg))
	if err != nil {
		return ports.FiscalPackageResult{}, err
	}
	return fromSiatResultadoPaquete(res), nil
}

func (a *FiscalAdapter) ValidatePackage(ctx context.Context, pkg ports.FiscalPackage, codigoRecepcion string) (ports.FiscalPackageResult, error) {
	res, err := a.svc.ValidarPaqueteFactura(ctx, toSiatPaquete(pkg), codigoRecepcion)
	if err != nil {
		return ports.FiscalPackageResult{}, err
	}
	return fromSiatResultadoPaquete(res), nil
}

func (a *FiscalAdapter) SendBulk(ctx context.Context, bulk ports.FiscalBulk) (ports.FiscalPackageResult, error) {
	res, err := a.svc.EnviarMasivaFacturas(ctx, toSiatMasiva(bulk))
	if err != nil {
		return ports.FiscalPackageResult{}, err
	}
	return fromSiatResultadoPaquete(res), nil
}

func (a *FiscalAdapter) ValidateBulk(ctx context.Context, bulk ports.FiscalBulk, codigoRecepcion string) (ports.FiscalPackageResult, error) {
	res, err := a.svc.ValidarMasivaFacturas(ctx, toSiatMasiva(bulk), codigoRecepcion)
	if err != nil {
		return ports.FiscalPackageResult{}, err
	}
	return fromSiatResultadoPaquete(res), nil
}

func (a *FiscalAdapter) SendPurchases(ctx context.Context, p ports.FiscalPurchase) (ports.FiscalPurchaseResult, error) {
	res, err := a.svc.EnviarCompras(ctx, toSiatCompras(p))
	if err != nil {
		return ports.FiscalPurchaseResult{}, err
	}
	return fromSiatResultadoCompras(res), nil
}

func (a *FiscalAdapter) SignXML(ctx context.Context, req ports.FiscalSignRequest) (ports.FiscalSignResult, error) {
	res, err := a.svc.FirmarFacturaXML(ctx, req.Xml)
	if err != nil {
		return ports.FiscalSignResult{}, err
	}
	return fromSiatResultadoFirma(res), nil
}

func (a *FiscalAdapter) EmitAdjustment(ctx context.Context, adj ports.FiscalAdjustment) (ports.FiscalAdjustmentResult, error) {
	res, err := a.svc.EmitirDocumentoAjuste(ctx, toSiatDocumentoAjuste(adj))
	if err != nil {
		return ports.FiscalAdjustmentResult{}, err
	}
	return fromSiatResultadoDocumentoAjuste(res), nil
}

func (a *FiscalAdapter) Synchronize(ctx context.Context, req ports.FiscalSyncRequest, op ports.FiscalSyncOperation) (ports.FiscalSyncResult, error) {
	res, err := a.svc.Sincronizar(ctx, toSiatSolicitudSincronizacion(req), SincronizacionOp(op))
	if err != nil {
		return ports.FiscalSyncResult{}, err
	}
	return fromSiatRespuestaSincronizacion(res), nil
}

// ---- conversiones ports -> siat ----

func toSiatFiscalCustomer(c ports.FiscalCustomer) ClienteFactura {
	return ClienteFactura{
		NombreRazonSocial:            c.NombreRazonSocial,
		CodigoTipoDocumentoIdentidad: c.CodigoTipoDocumentoIdentidad,
		NumeroDocumento:              c.NumeroDocumento,
		Complemento:                  c.Complemento,
		CodigoCliente:                c.CodigoCliente,
	}
}

func toSiatFiscalItems(items []ports.FiscalItem) []ItemFactura {
	out := make([]ItemFactura, len(items))
	for i, it := range items {
		out[i] = ItemFactura{
			ActividadEconomica: it.ActividadEconomica,
			CodigoProductoSin:  it.CodigoProductoSin,
			CodigoProducto:     it.CodigoProducto,
			Descripcion:        it.Descripcion,
			Cantidad:           it.Cantidad,
			UnidadMedida:       it.UnidadMedida,
			PrecioUnitario:     it.PrecioUnitario,
			MontoDescuento:     it.MontoDescuento,
			SubTotal:           it.SubTotal,
			DatosSector:        it.DatosSector,
		}
	}
	return out
}

func toSiatSolicitudFactura(doc ports.FiscalDocument) SolicitudFactura {
	return SolicitudFactura{
		XML:                   doc.XML,
		CodigoAmbiente:        doc.CodigoAmbiente,
		CodigoSistema:         doc.CodigoSistema,
		Nit:                   doc.Nit,
		Modalidad:             doc.Modalidad,
		NumeroFactura:         doc.NumeroFactura,
		NumeroFacturaOriginal: doc.NumeroFacturaOriginal,
		CodigoSucursal:        doc.CodigoSucursal,
		CodigoPuntoVenta:      doc.CodigoPuntoVenta,
		Cuis:                  doc.Cuis,
		Cufd:                  doc.Cufd,
		CodigoControl:         doc.CodigoControl,
		FechaEmision:          doc.FechaEmision,
		Usuario:               doc.Usuario,
		Leyenda:               doc.Leyenda,
		RazonSocialEmisor:     doc.RazonSocialEmisor,
		Municipio:             doc.Municipio,
		Direccion:             doc.Direccion,
		Telefono:              doc.Telefono,
		CodigoMetodoPago:      doc.CodigoMetodoPago,
		CodigoMoneda:          doc.CodigoMoneda,
		TipoCambio:            doc.TipoCambio,
		MontoTotal:            doc.MontoTotal,
		CodigoDocumentoSector: doc.CodigoDocumentoSector,
		Layout:                doc.Layout,
		CodigoTipoFactura:     doc.CodigoTipoFactura,
		Cafc:                  doc.Cafc,
		DatosSector:           doc.DatosSector,
		NombreEstudiante:      doc.NombreEstudiante,
		PeriodoFacturado:      doc.PeriodoFacturado,
		Archivo:               doc.Archivo,
		HashArchivo:           doc.HashArchivo,
		Cuf:                   doc.Cuf,
		Cliente:               toSiatFiscalCustomer(doc.Cliente),
		Items:                 toSiatFiscalItems(doc.Items),
		OriginalItems:         toSiatFiscalItems(doc.OriginalItems),
	}
}

func toSiatSolicitudDocumento(q ports.FiscalDocumentQuery) SolicitudDocumento {
	return SolicitudDocumento{
		CodigoAmbiente:        q.CodigoAmbiente,
		CodigoSistema:         q.CodigoSistema,
		Nit:                   q.Nit,
		Modalidad:             q.Modalidad,
		Cuf:                   q.Cuf,
		CodigoSucursal:        q.CodigoSucursal,
		CodigoPuntoVenta:      q.CodigoPuntoVenta,
		Cuis:                  q.Cuis,
		Cufd:                  q.Cufd,
		CodigoDocumentoSector: q.CodigoDocumentoSector,
		CodigoTipoFactura:     q.CodigoTipoFactura,
		Layout:                q.Layout,
	}
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

func toSiatEvento(ev ports.FiscalEvent) SolicitudEventoSignificativo {
	return SolicitudEventoSignificativo{
		CodigoAmbiente:        ev.CodigoAmbiente,
		CodigoSistema:         ev.CodigoSistema,
		Nit:                   ev.Nit,
		CodigoSucursal:        ev.CodigoSucursal,
		CodigoPuntoVenta:      ev.CodigoPuntoVenta,
		Cuis:                  ev.Cuis,
		Cufd:                  ev.Cufd,
		CufdEvento:            ev.CufdEvento,
		CodigoMotivoEvento:    ev.CodigoMotivoEvento,
		Descripcion:           ev.Descripcion,
		FechaHoraInicioEvento: ev.FechaHoraInicioEvento,
		FechaHoraFinEvento:    ev.FechaHoraFinEvento,
	}
}

func toSiatPaquete(pkg ports.FiscalPackage) SolicitudPaqueteFactura {
	return SolicitudPaqueteFactura{
		CodigoAmbiente:        pkg.CodigoAmbiente,
		CodigoSistema:         pkg.CodigoSistema,
		Nit:                   pkg.Nit,
		Modalidad:             pkg.Modalidad,
		CodigoSucursal:        pkg.CodigoSucursal,
		CodigoPuntoVenta:      pkg.CodigoPuntoVenta,
		Cuis:                  pkg.Cuis,
		Cufd:                  pkg.Cufd,
		CodigoControl:         pkg.CodigoControl,
		CodigoDocumentoSector: pkg.CodigoDocumentoSector,
		Layout:                pkg.Layout,
		CodigoTipoFactura:     pkg.CodigoTipoFactura,
		CodigoEmision:         pkg.CodigoEmision,
		CodigoEvento:          pkg.CodigoEvento,
		Archivo:               pkg.Archivo,
		HashArchivo:           pkg.HashArchivo,
		Descripcion:           pkg.Descripcion,
		Facturas:              toSiatSolicitudFacturas(pkg.Facturas),
	}
}

func toSiatMasiva(bulk ports.FiscalBulk) SolicitudMasivaFactura {
	return SolicitudMasivaFactura{
		CodigoAmbiente:        bulk.CodigoAmbiente,
		CodigoSistema:         bulk.CodigoSistema,
		Nit:                   bulk.Nit,
		Modalidad:             bulk.Modalidad,
		CodigoSucursal:        bulk.CodigoSucursal,
		CodigoPuntoVenta:      bulk.CodigoPuntoVenta,
		Cuis:                  bulk.Cuis,
		Cufd:                  bulk.Cufd,
		CodigoControl:         bulk.CodigoControl,
		CodigoDocumentoSector: bulk.CodigoDocumentoSector,
		Layout:                bulk.Layout,
		CodigoTipoFactura:     bulk.CodigoTipoFactura,
		CodigoEmision:         bulk.CodigoEmision,
		Archivo:               bulk.Archivo,
		HashArchivo:           bulk.HashArchivo,
		Facturas:              toSiatSolicitudFacturas(bulk.Facturas),
	}
}

func toSiatSolicitudFacturas(docs []ports.FiscalDocument) []SolicitudFactura {
	out := make([]SolicitudFactura, len(docs))
	for i, d := range docs {
		out[i] = toSiatSolicitudFactura(d)
	}
	return out
}

func toSiatCompras(p ports.FiscalPurchase) SolicitudCompras {
	return SolicitudCompras{
		Descripcion:      p.Descripcion,
		TipoCompra:       p.TipoCompra,
		CodigoAmbiente:   p.CodigoAmbiente,
		CodigoSistema:    p.CodigoSistema,
		Nit:              p.Nit,
		CodigoSucursal:   p.CodigoSucursal,
		CodigoPuntoVenta: p.CodigoPuntoVenta,
		Cuis:             p.Cuis,
		Cufd:             p.Cufd,
		Archivo:          p.Archivo,
		HashArchivo:      p.HashArchivo,
		CantidadFacturas: p.CantidadFacturas,
		Gestion:          p.Gestion,
		Periodo:          p.Periodo,
		FechaEnvio:       p.FechaEnvio,
	}
}

func toSiatDocumentoAjuste(adj ports.FiscalAdjustment) SolicitudDocumentoAjuste {
	return SolicitudDocumentoAjuste{
		CodigoAmbiente:        adj.CodigoAmbiente,
		CodigoSistema:         adj.CodigoSistema,
		Nit:                   adj.Nit,
		Modalidad:             adj.Modalidad,
		NumeroFactura:         adj.NumeroFactura,
		CodigoSucursal:        adj.CodigoSucursal,
		CodigoPuntoVenta:      adj.CodigoPuntoVenta,
		Cuis:                  adj.Cuis,
		Cufd:                  adj.Cufd,
		CodigoControl:         adj.CodigoControl,
		FechaEmision:          adj.FechaEmision,
		Usuario:               adj.Usuario,
		TipoNota:              TipoNota(adj.TipoNota),
		CufFacturaOriginal:    adj.CufFacturaOriginal,
		CodigoDocumentoSector: adj.CodigoDocumentoSector,
		Layout:                adj.Layout,
		CodigoTipoFactura:     adj.CodigoTipoFactura,
		RazonSocialEmisor:     adj.RazonSocialEmisor,
		Municipio:             adj.Municipio,
		Direccion:             adj.Direccion,
		Telefono:              adj.Telefono,
		Cliente:               toSiatFiscalCustomer(adj.Cliente),
		CodigoMetodoPago:      adj.CodigoMetodoPago,
		CodigoMoneda:          adj.CodigoMoneda,
		TipoCambio:            adj.TipoCambio,
		MontoTotal:            adj.MontoTotal,
		Leyenda:               adj.Leyenda,
		Motivo:                adj.Motivo,
		Items:                 toSiatFiscalItems(adj.Items),
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

// ---- conversiones siat -> ports ----

func fromSiatMensajes(msgs []Mensaje) []ports.FiscalMessage {
	out := make([]ports.FiscalMessage, len(msgs))
	for i, m := range msgs {
		out[i] = ports.FiscalMessage{Codigo: m.Codigo, Descripcion: m.Descripcion}
	}
	return out
}

func fromSiatFiscalCustomer(c ClienteFactura) ports.FiscalCustomer {
	return ports.FiscalCustomer{
		NombreRazonSocial:            c.NombreRazonSocial,
		CodigoTipoDocumentoIdentidad: c.CodigoTipoDocumentoIdentidad,
		NumeroDocumento:              c.NumeroDocumento,
		Complemento:                  c.Complemento,
		CodigoCliente:                c.CodigoCliente,
	}
}

func fromSiatFiscalItems(items []ItemFactura) []ports.FiscalItem {
	out := make([]ports.FiscalItem, len(items))
	for i, it := range items {
		out[i] = ports.FiscalItem{
			ActividadEconomica: it.ActividadEconomica,
			CodigoProductoSin:  it.CodigoProductoSin,
			CodigoProducto:     it.CodigoProducto,
			Descripcion:        it.Descripcion,
			Cantidad:           it.Cantidad,
			UnidadMedida:       it.UnidadMedida,
			PrecioUnitario:     it.PrecioUnitario,
			MontoDescuento:     it.MontoDescuento,
			SubTotal:           it.SubTotal,
			DatosSector:        it.DatosSector,
		}
	}
	return out
}

func fromSiatResultadoEmision(r *ResultadoEmision) ports.FiscalResult {
	if r == nil {
		return ports.FiscalResult{}
	}
	return ports.FiscalResult{
		Cuf:             r.Cuf,
		Transaccion:     r.Transaccion,
		CodigoEstado:    r.CodigoEstado,
		CodigoRecepcion: r.CodigoRecepcion,
		Mensajes:        fromSiatMensajes(r.Mensajes),
		Xml:             r.Xml,
		XmlHash:         r.XmlHash,
		Archivo:         r.Archivo,
	}
}

func fromSiatResultadoDocumento(r *ResultadoDocumento) ports.FiscalDocumentResult {
	if r == nil {
		return ports.FiscalDocumentResult{}
	}
	return ports.FiscalDocumentResult{
		Transaccion:     r.Transaccion,
		CodigoEstado:    r.CodigoEstado,
		CodigoRecepcion: r.CodigoRecepcion,
		Mensajes:        fromSiatMensajes(r.Mensajes),
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

func fromSiatResultadoEvento(r *ResultadoEventoSignificativo) ports.FiscalEventResult {
	if r == nil {
		return ports.FiscalEventResult{}
	}
	return ports.FiscalEventResult{
		Transaccion:     r.Transaccion,
		CodigoRecepcion: r.CodigoRecepcion,
		Mensajes:        fromSiatMensajes(r.Mensajes),
	}
}

func fromSiatResultadoPaquete(r *ResultadoPaquete) ports.FiscalPackageResult {
	if r == nil {
		return ports.FiscalPackageResult{}
	}
	return ports.FiscalPackageResult{
		Transaccion:      r.Transaccion,
		CodigoEstado:     r.CodigoEstado,
		CodigoRecepcion:  r.CodigoRecepcion,
		Mensajes:         fromSiatMensajes(r.Mensajes),
		Archivo:          r.Archivo,
		HashArchivo:      r.HashArchivo,
		CantidadFacturas: r.CantidadFacturas,
		Cufs:             r.Cufs,
	}
}

func fromSiatResultadoCompras(r *ResultadoCompras) ports.FiscalPurchaseResult {
	if r == nil {
		return ports.FiscalPurchaseResult{}
	}
	return ports.FiscalPurchaseResult{
		Transaccion:     r.Transaccion,
		CodigoEstado:    r.CodigoEstado,
		CodigoRecepcion: r.CodigoRecepcion,
		Mensajes:        fromSiatMensajes(r.Mensajes),
	}
}

func fromSiatResultadoFirma(r *ResultadoFirma) ports.FiscalSignResult {
	if r == nil {
		return ports.FiscalSignResult{}
	}
	return ports.FiscalSignResult{
		XmlFirmado:  r.XmlFirmado,
		Archivo:     r.Archivo,
		HashArchivo: r.HashArchivo,
		Firma:       r.Firma,
	}
}

func fromSiatResultadoDocumentoAjuste(r *ResultadoDocumentoAjuste) ports.FiscalAdjustmentResult {
	if r == nil {
		return ports.FiscalAdjustmentResult{}
	}
	return ports.FiscalAdjustmentResult{
		Transaccion:     r.Transaccion,
		CodigoEstado:    r.CodigoEstado,
		CodigoRecepcion: r.CodigoRecepcion,
		Mensajes:        fromSiatMensajes(r.Mensajes),
		Cuf:             r.Cuf,
		Xml:             r.Xml,
		XmlHash:         r.XmlHash,
	}
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

// MarshalFiscalMessages serializa mensajes fiscales a JSON.
func MarshalFiscalMessages(msgs []ports.FiscalMessage) (string, error) {
	if len(msgs) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(msgs)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
