package usecase

import (
	"context"
	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
	"testing"
	"time"
)

func TestEmitTimeoutActivaContingenciaOffline(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(testInvoice())
	svc := &fakeEmissionService{
		err: context.DeadlineExceeded,
		offlineResult: &ports.FiscalResult{
			Cuf: "CUF-OFFLINE", Xml: "<offline/>", XmlHash: "hash-offline", Archivo: "gzip-offline",
		},
	}
	contingencies := &fakeContingencyRepo{}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)
	uc.SetContingencyRepository(contingencies)

	got, err := uc.Emit(invoiceContext(), "inv-1")
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if got.Status != domain.InvoiceOffline || got.EmissionType != "OFFLINE" {
		t.Fatalf("status=%s emissionType=%s", got.Status, got.EmissionType)
	}
	if got.ContingencyEventId == nil || *got.ContingencyEventId != "event-1" {
		t.Fatalf("contingencyEventId=%v", got.ContingencyEventId)
	}
	if got.Cuf == nil || *got.Cuf != "CUF-OFFLINE" || got.Xml == nil || got.Archivo == "" {
		t.Fatalf("artefactos offline incompletos: %+v", got)
	}
	if svc.emitCalls != 1 || svc.offlineCalls != 1 || len(contingencies.created) != 1 {
		t.Fatalf("emit=%d offline=%d eventos=%d", svc.emitCalls, svc.offlineCalls, len(contingencies.created))
	}
}

func TestRegistrarEventoCompletaLaContingenciaAbiertaPorEmit(t *testing.T) {
	inv := testInvoice()
	pending := &domain.ContingencyEvent{
		ID: "event-1", PointOfSaleID: inv.PointOfSaleId,
		Reason: "FALLA_CONEXION_INTERNET", StartDate: inv.IssueDate.Add(-time.Minute),
	}
	contingencies := &fakeContingencyRepo{latest: pending}
	svc := &fakeEmissionService{eventResult: ports.FiscalEventResult{
		Transaccion: true, CodigoRecepcion: "987654",
	}}
	uc := &SiatUsecase{
		companyRepo:     &fakeCompanyRepo{company: inv.Company},
		pointOfSaleRepo: &fakePointOfSaleRepo{pos: inv.PointOfSale},
		cufdRepo:        &fakeCufdRepo{vigente: &inv.CufdRecord},
		contingencyRepo: contingencies,
		siatService:     svc,
		modalidad:       siat.ModalidadElectronica,
	}

	if _, err := uc.RegistrarEventoSignificativo(context.Background(), inv.CompanyId, inv.PointOfSaleId, EventoSignificativoInput{}); err != nil {
		t.Fatalf("RegistrarEventoSignificativo: %v", err)
	}
	if contingencies.updates != 1 || len(contingencies.created) != 0 {
		t.Fatalf("updates=%d creates=%d", contingencies.updates, len(contingencies.created))
	}
	if !pending.IsSynced || pending.SiatEventCode == nil || *pending.SiatEventCode != "987654" || pending.EndDate == nil {
		t.Fatalf("evento no completado: %+v", pending)
	}
	if svc.eventCaptured == nil || !svc.eventCaptured.FechaHoraInicioEvento.Equal(pending.StartDate) {
		t.Fatalf("inicio enviado al SIAT no conserva el evento local: %+v", svc.eventCaptured)
	}
}
