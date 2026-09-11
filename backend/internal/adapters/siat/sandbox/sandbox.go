// Package sandbox proporciona una implementación determinística de
// ports.FiscalService para desarrollo y CI sin depender del SIAT real.
package sandbox

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/brandsrx/supay/internal/ports"
)

// FiscalService es un sandbox determinístico del servicio fiscal SIAT.
type FiscalService struct {
	counter int64
}

// NewFiscalService crea una instancia del sandbox.
func NewFiscalService() *FiscalService {
	return &FiscalService{}
}

var _ ports.FiscalService = (*FiscalService)(nil)
var _ ports.OfflineFiscalService = (*FiscalService)(nil)

func (s *FiscalService) nextCode(prefix string) string {
	n := atomic.AddInt64(&s.counter, 1)
	return fmt.Sprintf("%s-%s-%012d", prefix, time.Now().UTC().Format("20060102-150405"), n)
}

func (s *FiscalService) Emit(ctx context.Context, doc ports.FiscalDocument) (ports.FiscalResult, error) {
	return ports.FiscalResult{
		Cuf:             s.nextCode("FAKE-CUF"),
		Transaccion:     true,
		CodigoEstado:    908,
		CodigoRecepcion: s.nextCode("FAKE-RCP"),
		Mensajes:        []ports.FiscalMessage{{Codigo: 908, Descripcion: "RECEPCION VALIDADA (sandbox)"}},
		Xml:             "<fake xmlns=\"sandbox\"/>",
		XmlHash:         "FAKE-HASH",
		Archivo:         "FAKE-ARCHIVO",
	}, nil
}

func (s *FiscalService) PrepareOffline(context.Context, ports.FiscalDocument) (ports.FiscalResult, error) {
	return ports.FiscalResult{
		Cuf: s.nextCode("FAKE-CUF-OFFLINE"), Xml: "<fake-offline xmlns=\"sandbox\"/>",
		XmlHash: "FAKE-OFFLINE-HASH", Archivo: "FAKE-OFFLINE-ARCHIVO",
	}, nil
}

func (s *FiscalService) VerifyStatus(ctx context.Context, query ports.FiscalDocumentQuery) (ports.FiscalDocumentResult, error) {
	return ports.FiscalDocumentResult{
		Transaccion:     true,
		CodigoEstado:    908,
		CodigoRecepcion: s.nextCode("FAKE-VER"),
		Mensajes:        []ports.FiscalMessage{{Codigo: 908, Descripcion: "RECEPCION VALIDADA (sandbox)"}},
	}, nil
}

func (s *FiscalService) Annul(ctx context.Context, query ports.FiscalDocumentQuery, codigoMotivo int) (ports.FiscalDocumentResult, error) {
	return ports.FiscalDocumentResult{
		Transaccion:     true,
		CodigoEstado:    905,
		CodigoRecepcion: s.nextCode("FAKE-ANL"),
		Mensajes:        []ports.FiscalMessage{{Codigo: 905, Descripcion: "ANULACION CONFIRMADA (sandbox)"}},
	}, nil
}

func (s *FiscalService) RevertAnnul(ctx context.Context, query ports.FiscalDocumentQuery) (ports.FiscalDocumentResult, error) {
	return ports.FiscalDocumentResult{
		Transaccion:     true,
		CodigoEstado:    907,
		CodigoRecepcion: s.nextCode("FAKE-REV"),
		Mensajes:        []ports.FiscalMessage{{Codigo: 907, Descripcion: "REVERSION DE ANULACION CONFIRMADA (sandbox)"}},
	}, nil
}

func (s *FiscalService) RequestCUIS(ctx context.Context, req ports.CredentialRequest) (ports.CuisResult, error) {
	return ports.CuisResult{
		Codigo:        s.nextCode("FAKE-CUIS"),
		FechaVigencia: time.Now().Add(30 * 24 * time.Hour),
		Transaccion:   true,
		Mensajes:      []ports.FiscalMessage{{Codigo: 980, Descripcion: "CUIS generado (sandbox)"}},
	}, nil
}

func (s *FiscalService) RequestCUFD(ctx context.Context, req ports.CredentialRequest) (ports.CufdResult, error) {
	return ports.CufdResult{
		Codigo:        s.nextCode("FAKE-CUFD"),
		CodigoControl: s.nextCode("FAKE-CTRL"),
		Direccion:     "FAKE DIRECCION (sandbox)",
		FechaVigencia: time.Now().Add(24 * time.Hour),
		Transaccion:   true,
		Mensajes:      []ports.FiscalMessage{{Codigo: 920, Descripcion: "CUFD generado (sandbox)"}},
	}, nil
}

func (s *FiscalService) RegisterSignificantEvent(ctx context.Context, ev ports.FiscalEvent) (ports.FiscalEventResult, error) {
	return ports.FiscalEventResult{
		Transaccion:     true,
		CodigoRecepcion: strings.TrimPrefix(s.nextCode("FAKE-EVT"), "FAKE-EVT-"),
		Mensajes:        []ports.FiscalMessage{{Codigo: 1, Descripcion: "Evento registrado (sandbox)"}},
	}, nil
}

func (s *FiscalService) SendPackage(ctx context.Context, pkg ports.FiscalPackage) (ports.FiscalPackageResult, error) {
	if pkg.Prepared != nil {
		return s.sendPrepared(ctx, pkg.Prepared, false)
	}
	return s.packageResult(len(pkg.Facturas)), nil
}

func (s *FiscalService) ValidatePackage(ctx context.Context, pkg ports.FiscalPackage, codigoRecepcion string) (ports.FiscalPackageResult, error) {
	return s.packageResult(len(pkg.Facturas)), nil
}

func (s *FiscalService) SendBulk(ctx context.Context, bulk ports.FiscalBulk) (ports.FiscalPackageResult, error) {
	if bulk.Prepared != nil {
		return s.sendPrepared(ctx, bulk.Prepared, true)
	}
	return s.packageResult(len(bulk.Facturas)), nil
}

func (s *FiscalService) ValidateBulk(ctx context.Context, bulk ports.FiscalBulk, codigoRecepcion string) (ports.FiscalPackageResult, error) {
	return s.packageResult(len(bulk.Facturas)), nil
}

func (s *FiscalService) packageResult(cantidad int) ports.FiscalPackageResult {
	return ports.FiscalPackageResult{
		Transaccion:      true,
		CodigoEstado:     908,
		CodigoRecepcion:  s.nextCode("FAKE-PKG"),
		Mensajes:         []ports.FiscalMessage{{Codigo: 908, Descripcion: "PAQUETE VALIDADO (sandbox)"}},
		Archivo:          "FAKE-ARCHIVO",
		HashArchivo:      "FAKE-HASH",
		CantidadFacturas: cantidad,
	}
}

func (s *FiscalService) SendPurchases(ctx context.Context, p ports.FiscalPurchase) (ports.FiscalPurchaseResult, error) {
	return ports.FiscalPurchaseResult{
		Transaccion:     true,
		CodigoEstado:    908,
		CodigoRecepcion: s.nextCode("FAKE-CMP"),
		Mensajes:        []ports.FiscalMessage{{Codigo: 908, Descripcion: "COMPRAS REGISTRADAS (sandbox)"}},
	}, nil
}

func (s *FiscalService) SignXML(ctx context.Context, req ports.FiscalSignRequest) (ports.FiscalSignResult, error) {
	return ports.FiscalSignResult{
		XmlFirmado:  "<fake-signed xmlns=\"sandbox\">" + req.Xml + "</fake-signed>",
		Archivo:     "FAKE-ARCHIVO",
		HashArchivo: "FAKE-HASH",
		Firma:       "XAdES-BES (sandbox)",
	}, nil
}

func (s *FiscalService) EmitAdjustment(ctx context.Context, adj ports.FiscalAdjustment) (ports.FiscalAdjustmentResult, error) {
	return ports.FiscalAdjustmentResult{
		Transaccion:     true,
		CodigoEstado:    908,
		CodigoRecepcion: s.nextCode("FAKE-ADJ"),
		Mensajes:        []ports.FiscalMessage{{Codigo: 908, Descripcion: "DOCUMENTO DE AJUSTE VALIDADO (sandbox)"}},
		Cuf:             s.nextCode("FAKE-CUF"),
		Xml:             "<fake-adjustment xmlns=\"sandbox\"/>",
		XmlHash:         "FAKE-HASH",
	}, nil
}

func (s *FiscalService) Synchronize(ctx context.Context, req ports.FiscalSyncRequest, op ports.FiscalSyncOperation) (ports.FiscalSyncResult, error) {
	now := time.Now()
	switch op {
	case ports.OpActividades:
		return ports.FiscalSyncResult{
			Transaccion: true,
			Actividades: []ports.FiscalActivity{
				{CodigoCaeb: "101010", Descripcion: "ACTIVIDAD SANDBOX", TipoActividad: "PRINCIPAL"},
			},
		}, nil
	case ports.OpProductosServicios:
		return ports.FiscalSyncResult{
			Transaccion: true,
			Productos: []ports.FiscalSinProduct{
				{CodigoProductoSin: 5113100, CodigoActividad: 101010, Descripcion: "PRODUCTO SANDBOX"},
			},
		}, nil
	case ports.OpLeyendasFactura:
		return ports.FiscalSyncResult{
			Transaccion: true,
			Leyendas: []ports.FiscalLegend{
				{CodigoActividad: "101010", DescripcionLeyenda: "Ley N 453 (sandbox)"},
			},
		}, nil
	case ports.OpActividadesDocumentoSector:
		return ports.FiscalSyncResult{
			Transaccion: true,
			ActividadesDocSector: []ports.FiscalActivityDocSector{
				{CodigoActividad: "101010", CodigoDocumentoSector: 1, TipoDocumentoSector: "FCV"},
				{CodigoActividad: "8549100", CodigoDocumentoSector: 11, TipoDocumentoSector: "FSEDU"},
			},
		}, nil
	case ports.OpFechaHora:
		return ports.FiscalSyncResult{Transaccion: true, FechaHora: now}, nil
	case ports.OpVerificarComunicacion:
		return ports.FiscalSyncResult{Transaccion: true}, nil
	default:
		// Devuelve un catálogo paramétrico genérico según la operación.
		codigo := 1
		if code, err := strconv.Atoi(string(op)); err == nil {
			codigo = code
		}
		return ports.FiscalSyncResult{
			Transaccion: true,
			Codigos: []ports.FiscalParametricItem{
				{CodigoClasificador: codigo, Descripcion: string(op) + " (sandbox)"},
			},
		}, nil
	}
}
