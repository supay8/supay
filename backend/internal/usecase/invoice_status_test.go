package usecase

import (
	"errors"
	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
	"strings"
	"testing"
	"time"
)

func TestBuildSolicitudDocumento(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := emittedInvoice()
	inv.Cuf = strPtr("CUF-1")

	req, err := uc.buildSolicitudDocumento(inv)
	if err != nil {
		t.Fatalf("buildSolicitudDocumento: %v", err)
	}
	if req.Cuf != "CUF-1" {
		t.Errorf("Cuf=%q", req.Cuf)
	}
	if req.CodigoAmbiente != 2 {
		t.Errorf("CodigoAmbiente=%d", req.CodigoAmbiente)
	}
	if req.CodigoSucursal != 0 || req.CodigoPuntoVenta != 3 {
		t.Errorf("sucursal/PV=%d/%d", req.CodigoSucursal, req.CodigoPuntoVenta)
	}
	if req.Cuis != "D17EEF19" || req.Cufd != "CUFD-XYZ" {
		t.Errorf("Cuis/Cufd=%q/%q", req.Cuis, req.Cufd)
	}
}

func TestVerifyStatusReconciliaEstado(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(emittedInvoice())
	svc := &fakeEmissionService{docResult: &ports.FiscalDocumentResult{
		Transaccion:  true,
		CodigoEstado: 905,
	}}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)

	got, err := uc.VerifyStatus(invoiceContext(), "inv-1")
	if err != nil {
		t.Fatalf("VerifyStatus: %v", err)
	}
	if got.Status != domain.InvoiceCancelled {
		t.Errorf("status=%s, se esperaba CANCELLED (905 ANULACION CONFIRMADA)", got.Status)
	}
	if repo.updateCalls != 1 {
		t.Errorf("updateCalls=%d, se esperaba 1", repo.updateCalls)
	}
}

func TestVerifyStatusSinCambioNoActualiza(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(emittedInvoice())
	svc := &fakeEmissionService{docResult: &ports.FiscalDocumentResult{
		Transaccion:  true,
		CodigoEstado: 908,
	}}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)

	got, err := uc.VerifyStatus(invoiceContext(), "inv-1")
	if err != nil {
		t.Fatalf("VerifyStatus: %v", err)
	}
	if got.Status != domain.InvoiceAccepted {
		t.Errorf("status=%s, se esperaba ACCEPTED", got.Status)
	}
	if repo.updateCalls != 0 {
		t.Errorf("updateCalls=%d, no debió persistirse", repo.updateCalls)
	}
}

func TestVerifyStatusSinCuf(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(testInvoice())
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, &fakeEmissionService{})

	if _, err := uc.VerifyStatus(invoiceContext(), "inv-1"); err == nil {
		t.Fatal("se esperaba error por falta de CUF")
	}
}

func TestAnnulAccepted(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(emittedInvoice())
	catalog := &fakeCatalogRepo{items: map[string][]*domain.CatalogItem{
		"motivoAnulacion": {
			{Codigo: 1, Descripcion: "Venta con derecho a crédito fiscal", Tipo: "motivoAnulacion"},
		},
	}}
	svc := &fakeEmissionService{docResult: &ports.FiscalDocumentResult{
		Transaccion:     true,
		CodigoEstado:    905,
		CodigoRecepcion: "RCP-ANNUL",
	}}
	uc := newTestUsecase(repo, catalog, svc)

	got, err := uc.Annul(invoiceContext(), "inv-1", 1)
	if err != nil {
		t.Fatalf("Annul: %v", err)
	}
	if got.Status != domain.InvoiceCancelled {
		t.Errorf("status=%s, se esperaba CANCELLED (905 ANULACION CONFIRMADA)", got.Status)
	}
	if got.MotivoAnulacion == nil || *got.MotivoAnulacion != 1 {
		t.Errorf("MotivoAnulacion=%v, se esperaba 1", got.MotivoAnulacion)
	}
	if got.FechaAnulacion == nil {
		t.Error("FechaAnulacion no persistida")
	}
	if got.SiatReceptionCode == nil || *got.SiatReceptionCode != "RCP-ANNUL" {
		t.Errorf("SiatReceptionCode=%v", got.SiatReceptionCode)
	}
}

func TestAnnulRechazado(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(emittedInvoice())
	catalog := &fakeCatalogRepo{items: map[string][]*domain.CatalogItem{
		"motivoAnulacion": {{Codigo: 1, Descripcion: "Motivo válido", Tipo: "motivoAnulacion"}},
	}}
	svc := &fakeEmissionService{docResult: &ports.FiscalDocumentResult{
		Transaccion:  false,
		CodigoEstado: 906,
		Mensajes:     []ports.FiscalMessage{{Codigo: 900, Descripcion: "motivo inválido"}},
	}}
	uc := newTestUsecase(repo, catalog, svc)

	_, err := uc.Annul(invoiceContext(), "inv-1", 1)
	var rejected *EmissionRejectedError
	if !errors.As(err, &rejected) {
		t.Fatalf("se esperaba EmissionRejectedError, se obtuvo: %v", err)
	}
	if repo.invoices["inv-1"].Status != domain.InvoiceAccepted {
		t.Errorf("status=%s, no debió cambiar", repo.invoices["inv-1"].Status)
	}
}

func TestAnnulNoAccepted(t *testing.T) {
	repo := newFakeInvoiceRepo()
	inv := testInvoice()
	inv.Cuf = strPtr("CUF-1")
	_ = repo.Create(inv)
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, &fakeEmissionService{})

	if _, err := uc.Annul(invoiceContext(), "inv-1", 1); err == nil {
		t.Fatal("se esperaba error por estado no ACCEPTED")
	}
}

func TestAnnulMotivoInvalido(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(emittedInvoice())
	catalog := &fakeCatalogRepo{items: map[string][]*domain.CatalogItem{
		"motivoAnulacion": {{Codigo: 1, Descripcion: "Solo motivo 1", Tipo: "motivoAnulacion"}},
	}}
	uc := newTestUsecase(repo, catalog, &fakeEmissionService{})

	if _, err := uc.Annul(invoiceContext(), "inv-1", 99); err == nil || !strings.Contains(err.Error(), "motivo") {
		t.Fatalf("se esperaba error de motivo inválido, se obtuvo: %v", err)
	}
}

func TestRevertAnnul(t *testing.T) {
	repo := newFakeInvoiceRepo()
	inv := emittedInvoice()
	motivo := 1
	fecha := time.Now()
	inv.Status = domain.InvoiceCancelled
	inv.MotivoAnulacion = &motivo
	inv.FechaAnulacion = &fecha
	_ = repo.Create(inv)
	svc := &fakeEmissionService{docResult: &ports.FiscalDocumentResult{
		Transaccion:     true,
		CodigoEstado:    907,
		CodigoRecepcion: "RCP-REVERT",
	}}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)

	got, err := uc.RevertAnnul(invoiceContext(), "inv-1")
	if err != nil {
		t.Fatalf("RevertAnnul: %v", err)
	}
	if got.Status != domain.InvoiceAccepted {
		t.Errorf("status=%s, se esperaba ACCEPTED (907 REVERSION CONFIRMADA)", got.Status)
	}
	if got.MotivoAnulacion != nil || got.FechaAnulacion != nil {
		t.Error("MotivoAnulacion/FechaAnulacion debieron limpiarse")
	}
	if got.SiatReceptionCode == nil || *got.SiatReceptionCode != "RCP-REVERT" {
		t.Errorf("SiatReceptionCode=%v", got.SiatReceptionCode)
	}
}

func TestRevertAnnulNoCancelled(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(emittedInvoice())
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, &fakeEmissionService{})

	if _, err := uc.RevertAnnul(invoiceContext(), "inv-1"); err == nil {
		t.Fatal("se esperaba error por estado no CANCELLED")
	}
}

func TestAnnulUsaCufdVigente(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(emittedInvoice())
	catalog := &fakeCatalogRepo{items: map[string][]*domain.CatalogItem{
		"motivoAnulacion": {{Codigo: 1, Descripcion: "Motivo válido", Tipo: "motivoAnulacion"}},
	}}
	svc := &fakeEmissionService{docResult: &ports.FiscalDocumentResult{Transaccion: true, CodigoEstado: 905}}
	uc := NewInvoiceUsecase(repo, nil, nil, nil, catalog, &fakeCufdRepo{vigente: &domain.Cufd{Cufd: "CUFD-VIGENTE", ValidFrom: time.Now().Add(-time.Hour), ValidTo: time.Now().Add(time.Hour)}}, svc, siat.ModalidadElectronica,
		nil, nil, nil, nil, nil, false, nil)

	if _, err := uc.Annul(invoiceContext(), "inv-1", 1); err != nil {
		t.Fatalf("Annul: %v", err)
	}
	if svc.captured == nil || svc.captured.Cufd != "CUFD-VIGENTE" {
		t.Fatalf("anulacion debió enviar el CUFD vigente, se obtuvo: %+v", svc.captured)
	}
}

func TestAnnulCaeAlCufdDeEmisionSinVigente(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(emittedInvoice())
	catalog := &fakeCatalogRepo{items: map[string][]*domain.CatalogItem{
		"motivoAnulacion": {{Codigo: 1, Descripcion: "Motivo válido", Tipo: "motivoAnulacion"}},
	}}
	svc := &fakeEmissionService{docResult: &ports.FiscalDocumentResult{Transaccion: true, CodigoEstado: 905}}
	uc := NewInvoiceUsecase(repo, nil, nil, nil, catalog, &fakeCufdRepo{}, svc, siat.ModalidadElectronica,
		nil, nil, nil, nil, nil, false, nil)

	if _, err := uc.Annul(invoiceContext(), "inv-1", 1); err != nil {
		t.Fatalf("Annul: %v", err)
	}
	if svc.captured == nil || svc.captured.Cufd != "CUFD-XYZ" {
		t.Fatalf("sin CUFD vigente debió usar el CUFD de emisión (CUFD-XYZ), se obtuvo: %+v", svc.captured)
	}
}

func TestRevertAnnulUsaCufdVigente(t *testing.T) {
	repo := newFakeInvoiceRepo()
	inv := emittedInvoice()
	motivo := 1
	inv.Status = domain.InvoiceCancelled
	inv.MotivoAnulacion = &motivo
	_ = repo.Create(inv)
	svc := &fakeEmissionService{docResult: &ports.FiscalDocumentResult{Transaccion: true, CodigoEstado: 907}}
	uc := NewInvoiceUsecase(repo, nil, nil, nil, &fakeCatalogRepo{}, &fakeCufdRepo{vigente: &domain.Cufd{Cufd: "CUFD-VIGENTE", ValidFrom: time.Now().Add(-time.Hour), ValidTo: time.Now().Add(time.Hour)}}, svc, siat.ModalidadElectronica,
		nil, nil, nil, nil, nil, false, nil)

	if _, err := uc.RevertAnnul(invoiceContext(), "inv-1"); err != nil {
		t.Fatalf("RevertAnnul: %v", err)
	}
	if svc.captured == nil || svc.captured.Cufd != "CUFD-VIGENTE" {
		t.Fatalf("reversion debió enviar el CUFD vigente, se obtuvo: %+v", svc.captured)
	}
}

func TestSiatEstadoToDomain(t *testing.T) {
	cases := []struct {
		in      int
		want    domain.InvoiceStatus
		matches bool
	}{
		{908, domain.InvoiceAccepted, true},
		{907, domain.InvoiceAccepted, true},
		{904, domain.InvoiceObserved, true},
		{902, domain.InvoiceRejected, true},
		{906, domain.InvoiceRejected, true},
		{909, domain.InvoiceRejected, true},
		{905, domain.InvoiceCancelled, true},
		{0, "", false},
		{999, "", false},
	}
	for _, c := range cases {
		got, ok := siatEstadoToDomain(c.in)
		if ok != c.matches || (ok && got != c.want) {
			t.Errorf("siatEstadoToDomain(%d)=%s/%v, se esperaba %s/%v", c.in, got, ok, c.want, c.matches)
		}
	}
}
