package usecase

import (
	"context"
	"errors"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
	"strings"
	"testing"
)

func TestEmitAccepted(t *testing.T) {
	repo := newFakeInvoiceRepo()
	inv := testInvoice()
	_ = repo.Create(inv)
	svc := &fakeEmissionService{result: &ports.FiscalResult{
		Cuf:             "CUF-1",
		Transaccion:     true,
		CodigoEstado:    908,
		CodigoRecepcion: "RCP-1",
		Xml:             "<xml/>",
		XmlHash:         "abc123",
	}}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)

	got, err := uc.Emit(invoiceContext(), "inv-1")
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if got.Status != domain.InvoiceAccepted {
		t.Errorf("status=%s, se esperaba ACCEPTED (908 RECEPCION VALIDADA)", got.Status)
	}
	if got.Cuf == nil || *got.Cuf != "CUF-1" {
		t.Errorf("Cuf no persistido: %v", got.Cuf)
	}
	if got.Xml == nil || got.XmlHash == nil || got.SiatReceptionCode == nil {
		t.Errorf("respuesta sin Xml/XmlHash/SiatReceptionCode")
	}
	if _, exists := repo.lastFields["xml"]; exists {
		t.Error("el XML fue enviado a persistencia relacional")
	}
	data, file, readErr := uc.fileService.ReadAll(context.Background(), got.CompanyId, got.ID, "xml")
	if readErr != nil || string(data) != "<xml/>" || got.XmlHash == nil || *got.XmlHash != file.SHA256 {
		t.Fatalf("XML canónico no persistido en object storage: file=%+v err=%v", file, readErr)
	}
	if repo.claimCalls != 1 || repo.updateCalls != 1 {
		t.Errorf("claimCalls=%d updateCalls=%d, se esperaba 1/1", repo.claimCalls, repo.updateCalls)
	}
}

func TestEmitSiempreProcesaSincrono(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(testInvoice())
	svc := &fakeEmissionService{result: &ports.FiscalResult{Cuf: "CUF-1", Xml: "<xml/>", Transaccion: true, CodigoEstado: 908}}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)

	got, err := uc.Emit(invoiceContext(), "inv-1")
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if got.Status != domain.InvoiceAccepted || repo.claimCalls != 1 || svc.emitCalls != 1 {
		t.Fatalf("status=%s claimCalls=%d siatCalls=%d", got.Status, repo.claimCalls, svc.emitCalls)
	}
}

func TestProcessEmissionGeneraPDFDentroDelWorker(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(testInvoice())
	svc := &fakeEmissionService{result: &ports.FiscalResult{Cuf: "CUF-1", Xml: "<xml/>", Transaccion: true, CodigoEstado: 908}}
	pdf := &fakePDFGenerator{}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)
	uc.pdfService = pdf

	if _, err := uc.ProcessEmission(context.Background(), "comp-1", "inv-1"); err != nil {
		t.Fatalf("ProcessEmission: %v", err)
	}
	if pdf.calls != 1 {
		t.Fatalf("PDF calls=%d, se esperaba ejecución síncrona dentro del worker", pdf.calls)
	}
}

func TestEmitObserved(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(testInvoice())
	svc := &fakeEmissionService{result: &ports.FiscalResult{
		Cuf:          "CUF-1",
		Xml:          "<xml/>",
		Transaccion:  true,
		CodigoEstado: 904,
		Mensajes:     []ports.FiscalMessage{{Codigo: 1007, Descripcion: "DIRECCION NO CORRESPONDE A PADRON"}},
	}}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)

	got, err := uc.Emit(invoiceContext(), "inv-1")
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if got.Status != domain.InvoiceObserved {
		t.Errorf("status=%s, se esperaba OBSERVED (904 RECEPCION OBSERVADA)", got.Status)
	}
	if got.SiatMensajes == nil {
		t.Error("SiatMensajes no persistido")
	} else if !strings.Contains(*got.SiatMensajes, "1007") {
		t.Errorf("SiatMensajes=%q, se esperaba el código 1007", *got.SiatMensajes)
	}
}

func TestEmitRejected(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(testInvoice())
	svc := &fakeEmissionService{result: &ports.FiscalResult{
		Cuf:          "CUF-1",
		Xml:          "<xml/>",
		Transaccion:  false,
		CodigoEstado: 902,
		Mensajes:     []ports.FiscalMessage{{Codigo: 123, Descripcion: "descripcion rechazo"}},
	}}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)

	_, err := uc.Emit(invoiceContext(), "inv-1")
	var rejected *EmissionRejectedError
	if !errors.As(err, &rejected) {
		t.Fatalf("se esperaba EmissionRejectedError, se obtuvo: %v", err)
	}
	if !strings.Contains(err.Error(), "123") {
		t.Errorf("el error no incluye el código del mensaje: %v", err)
	}
	stored := repo.invoices["inv-1"]
	if stored.Status != domain.InvoiceRejected {
		t.Errorf("status=%s, se esperaba REJECTED", stored.Status)
	}
}

func TestEmitTransportErrorRevierteAPending(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(testInvoice())
	svc := &fakeEmissionService{err: errors.New("connection reset")}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)

	if _, err := uc.Emit(invoiceContext(), "inv-1"); err == nil {
		t.Fatal("se esperaba error de transporte")
	}
	stored := repo.invoices["inv-1"]
	if stored.Status != domain.InvoicePending {
		t.Errorf("status=%s, se esperaba revert a PENDING", stored.Status)
	}
}

func TestEmitNoDisponibleRevierteAPending(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(testInvoice())
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, nil)

	if _, err := uc.Emit(invoiceContext(), "inv-1"); err == nil {
		t.Fatal("se esperaba error de servicio SIAT no disponible")
	}
	if repo.invoices["inv-1"].Status != domain.InvoicePending {
		t.Errorf("status=%s, se esperaba revert a PENDING", repo.invoices["inv-1"].Status)
	}
}

func TestEmitNoPending(t *testing.T) {
	repo := newFakeInvoiceRepo()
	inv := testInvoice()
	inv.Status = domain.InvoiceAccepted
	_ = repo.Create(inv)
	svc := &fakeEmissionService{result: &ports.FiscalResult{Transaccion: true}}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)

	if _, err := uc.Emit(invoiceContext(), "inv-1"); err == nil {
		t.Fatal("se esperaba error por estado no PENDING")
	}
	if repo.claimCalls != 0 {
		t.Errorf("claimCalls=%d, no debió intentarse el claim", repo.claimCalls)
	}
}

func TestEmitClaimPerdido(t *testing.T) {
	repo := newFakeInvoiceRepo()
	inv := testInvoice()
	inv.Status = domain.InvoiceSending
	_ = repo.Create(inv)
	svc := &fakeEmissionService{result: &ports.FiscalResult{Transaccion: true}}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)

	if _, err := uc.Emit(invoiceContext(), "inv-1"); err == nil {
		t.Fatal("se esperaba error de claim perdido (SENDING)")
	}
}
