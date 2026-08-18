package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/siat"
	"gorm.io/gorm"
)

// ---- Fakes ----

type fakeInvoiceRepo struct {
	invoices    map[string]*domain.Invoice
	claimCalls  int
	updateCalls int
	updateErr   error
}

func newFakeInvoiceRepo() *fakeInvoiceRepo {
	return &fakeInvoiceRepo{invoices: map[string]*domain.Invoice{}}
}

func (f *fakeInvoiceRepo) Create(inv *domain.Invoice) error {
	f.invoices[inv.ID] = inv
	return nil
}

func (f *fakeInvoiceRepo) GetByID(id string) (*domain.Invoice, error) {
	inv, ok := f.invoices[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return inv, nil
}

func (f *fakeInvoiceRepo) ListByPointOfSale(string) ([]*domain.Invoice, error) {
	return nil, nil
}

func (f *fakeInvoiceRepo) Update(inv *domain.Invoice) error {
	f.updateCalls++
	if f.updateErr != nil {
		return f.updateErr
	}
	f.invoices[inv.ID] = inv
	return nil
}

func (f *fakeInvoiceRepo) ClaimForEmission(id string) (bool, error) {
	f.claimCalls++
	inv, ok := f.invoices[id]
	if !ok {
		return false, nil
	}
	if inv.Status != domain.InvoicePending {
		return false, nil
	}
	inv.Status = domain.InvoiceSending
	return true, nil
}

func (f *fakeInvoiceRepo) FindActiveCufdForPointOfSale(string, time.Time) (*domain.Cufd, error) {
	return nil, nil
}

type fakeCatalogRepo struct {
	items map[string][]*domain.CatalogItem
}

func (f *fakeCatalogRepo) Replace(string, string, []domain.CatalogItem, time.Time) error {
	return nil
}

func (f *fakeCatalogRepo) List(_ string, tipo string) ([]*domain.CatalogItem, error) {
	return f.items[tipo], nil
}

func (f *fakeCatalogRepo) ListAll(string) (map[string][]*domain.CatalogItem, error) {
	return f.items, nil
}

type fakeEmissionService struct {
	result    *siat.ResultadoEmision
	docResult *siat.ResultadoDocumento
	err       error
	captured  *siat.SolicitudDocumento
}

func (f *fakeEmissionService) EmitirFactura(context.Context, siat.SolicitudFactura) (*siat.ResultadoEmision, error) {
	return f.result, f.err
}

func (f *fakeEmissionService) VerificarEstado(context.Context, siat.SolicitudDocumento) (*siat.ResultadoDocumento, error) {
	return f.docResult, f.err
}

func (f *fakeEmissionService) AnularFactura(_ context.Context, req siat.SolicitudDocumento, _ int) (*siat.ResultadoDocumento, error) {
	f.captured = &req
	return f.docResult, f.err
}

func (f *fakeEmissionService) RevertirAnulacion(_ context.Context, req siat.SolicitudDocumento) (*siat.ResultadoDocumento, error) {
	f.captured = &req
	return f.docResult, f.err
}

type fakeCufdRepo struct {
	vigente *domain.Cufd
	err     error
}

func (f *fakeCufdRepo) Create(_ *domain.Cufd) error { return nil }
func (f *fakeCufdRepo) GetActiveByPos(_ string) (*domain.Cufd, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.vigente, nil
}
func (f *fakeCufdRepo) GetByPosAndWindow(_ string, _, _ time.Time) (*domain.Cufd, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.vigente, nil
}
func (f *fakeCufdRepo) DeactivateExpired() error { return nil }

// ---- Fixture ----

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func testInvoice() *domain.Invoice {
	now := time.Now()
	actividad := "101010"
	sin := "5113100"
	unit := 58
	customerName := "Juan Perez"
	return &domain.Invoice{
		ID:               "inv-1",
		CompanyId:        "comp-1",
		CustomerId:       "cust-1",
		PointOfSaleId:    "pos-1",
		CufdId:           "cufd-1",
		InvoiceNumber:    1,
		EmissionType:     "EN_LINEA",
		CodigoMetodoPago: 1,
		CodigoMoneda:     1,
		TipoCambio:       1,
		IssueDate:        now,
		Total:            200,
		Status:           domain.InvoicePending,
		Company: domain.Company{
			ID:              "comp-1",
			Nit:             "9971522011",
			BusinessName:    "EMPRESA PILOTO SRL",
			CodigoSistema:   "228452C38ED8739408AB6",
			Ambiente:        domain.EnvironmentPiloto,
			Municipio:       "LA PAZ",
			Direccion:       "AV. CAMACHO 123",
			Telefono:        "2444444",
			CodigoActividad: &actividad,
		},
		Customer: domain.Customer{
			ID:             "cust-1",
			DocumentType:   "CI",
			DocumentNumber: "1234567",
			Name:           customerName,
		},
		PointOfSale: domain.PointOfSale{
			ID:               "pos-1",
			CompanyId:        "comp-1",
			CodigoSucursal:   0,
			CodigoPuntoVenta: 3,
			Cuis:             strPtr("D17EEF19"),
			IsActive:         true,
		},
		CufdRecord: domain.Cufd{
			ID:          "cufd-1",
			Cufd:        "CUFD-XYZ",
			ControlCode: "CC-123",
			ValidFrom:   now.Add(-time.Hour),
			ValidTo:     now.Add(time.Hour),
			Active:      true,
		},
		Items: []domain.InvoiceItem{
			{
				ID:                "item-1",
				Code:              "P001",
				Description:       "Producto de prueba",
				CodigoProductoSin: &sin,
				UnitCode:          &unit,
				Quantity:          2,
				UnitPrice:         100,
				Subtotal:          200,
			},
		},
	}
}

func newTestUsecase(repo *fakeInvoiceRepo, catalog *fakeCatalogRepo, svc SiatEmissionService) *InvoiceUsecase {
	return NewInvoiceUsecase(repo, nil, nil, nil, catalog, nil, svc, siat.ModalidadElectronica)
}

// ---- Tests ----

func TestCodigoTipoDocumentoIdentidad(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"CI", 1}, {"cex", 2}, {"PAS", 3}, {"NIT", 4}, {"OD", 5}, {"", -1}, {"XXX", -1},
	}
	for _, c := range cases {
		got, err := codigoTipoDocumentoIdentidad(c.in)
		if c.want == -1 {
			if err == nil {
				t.Errorf("codigoTipoDocumentoIdentidad(%q): se esperaba error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("codigoTipoDocumentoIdentidad(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("codigoTipoDocumentoIdentidad(%q)=%d, se esperaba %d", c.in, got, c.want)
		}
	}
}

func TestResolveLeyenda(t *testing.T) {
	catalog := &fakeCatalogRepo{items: map[string][]*domain.CatalogItem{
		"leyendasFactura": {
			{Codigo: 1, Descripcion: "101010: Leyenda oficial de prueba", Tipo: "leyendasFactura"},
		},
	}}
	uc := newTestUsecase(newFakeInvoiceRepo(), catalog, nil)

	got, err := uc.resolveLeyenda("comp-1", "101010")
	if err != nil {
		t.Fatalf("resolveLeyenda: %v", err)
	}
	if got != "Leyenda oficial de prueba" {
		t.Errorf("resolveLeyenda = %q, se esperaba la leyenda oficial", got)
	}

	// Sin catálogo sincronizado cae al fallback de la Ley 453.
	uc2 := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	got, err = uc2.resolveLeyenda("comp-1", "101010")
	if err != nil {
		t.Fatalf("resolveLeyenda fallback: %v", err)
	}
	if !strings.Contains(got, "453") {
		t.Errorf("fallback no contiene Ley 453: %q", got)
	}
}

func TestBuildSolicitudFactura(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()

	req, err := uc.buildSolicitudFactura(inv)
	if err != nil {
		t.Fatalf("buildSolicitudFactura: %v", err)
	}

	if req.CodigoAmbiente != 2 {
		t.Errorf("CodigoAmbiente=%d, se esperaba 2 (piloto)", req.CodigoAmbiente)
	}
	if req.Nit != "9971522011" {
		t.Errorf("Nit=%q", req.Nit)
	}
	if req.NumeroFactura != 1 {
		t.Errorf("NumeroFactura=%d", req.NumeroFactura)
	}
	if req.Cuis != "D17EEF19" {
		t.Errorf("Cuis=%q", req.Cuis)
	}
	if req.Cufd != "CUFD-XYZ" || req.CodigoControl != "CC-123" {
		t.Errorf("Cufd/CodigoControl=%q/%q", req.Cufd, req.CodigoControl)
	}
	if req.CodigoSucursal != 0 || req.CodigoPuntoVenta != 3 {
		t.Errorf("sucursal/PV=%d/%d", req.CodigoSucursal, req.CodigoPuntoVenta)
	}
	if req.Cliente.CodigoTipoDocumentoIdentidad != 1 {
		t.Errorf("tipoDocumento=%d", req.Cliente.CodigoTipoDocumentoIdentidad)
	}
	if req.Cliente.NumeroDocumento != "1234567" || req.Cliente.CodigoCliente != "cust-1" {
		t.Errorf("cliente documento/codigo=%q/%q", req.Cliente.NumeroDocumento, req.Cliente.CodigoCliente)
	}
	if len(req.Items) != 1 {
		t.Fatalf("items=%d", len(req.Items))
	}
	it := req.Items[0]
	if it.CodigoProductoSin != 5113100 {
		t.Errorf("CodigoProductoSin=%d", it.CodigoProductoSin)
	}
	if it.UnidadMedida != 58 {
		t.Errorf("UnidadMedida=%d", it.UnidadMedida)
	}
	if it.ActividadEconomica != "101010" {
		t.Errorf("ActividadEconomica=%q", it.ActividadEconomica)
	}
	if it.Cantidad != 2 || it.PrecioUnitario != 100 || it.SubTotal != 200 {
		t.Errorf("cantidad/precio/subtotal=%v/%v/%v", it.Cantidad, it.PrecioUnitario, it.SubTotal)
	}
	if req.MontoTotal != 200 {
		t.Errorf("MontoTotal=%v", req.MontoTotal)
	}
	if req.RazonSocialEmisor != "EMPRESA PILOTO SRL" {
		t.Errorf("RazonSocialEmisor=%q", req.RazonSocialEmisor)
	}
}

func TestBuildSolicitudFacturaUsaSiatCode(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	siatsCode := 7
	inv.PointOfSale.SiatCode = &siatsCode

	req, err := uc.buildSolicitudFactura(inv)
	if err != nil {
		t.Fatalf("buildSolicitudFactura: %v", err)
	}
	if req.CodigoPuntoVenta != 7 {
		t.Errorf("CodigoPuntoVenta=%d, se esperaba 7 (SiatCode)", req.CodigoPuntoVenta)
	}
}

func TestBuildSolicitudFacturaFaltaCuis(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.PointOfSale.Cuis = nil

	if _, err := uc.buildSolicitudFactura(inv); err == nil {
		t.Fatal("se esperaba error por CUIS ausente")
	}
}

func TestBuildSolicitudFacturaCufdVencido(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.CufdRecord.ValidTo = time.Now().Add(-time.Hour)

	if _, err := uc.buildSolicitudFactura(inv); err == nil {
		t.Fatal("se esperaba error por CUFD vencido")
	}
}

func TestBuildSolicitudFacturaSinCodigoProductoSin(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.Items[0].CodigoProductoSin = nil

	if _, err := uc.buildSolicitudFactura(inv); err == nil || !strings.Contains(err.Error(), "codigoProductoSin") {
		t.Fatalf("se esperaba error de codigoProductoSin, se obtuvo: %v", err)
	}
}

func TestBuildSolicitudFacturaTipoDocInvalido(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.Customer.DocumentType = "XYZ"

	if _, err := uc.buildSolicitudFactura(inv); err == nil {
		t.Fatal("se esperaba error por tipo de documento inválido")
	}
}

func TestEmitAccepted(t *testing.T) {
	repo := newFakeInvoiceRepo()
	inv := testInvoice()
	_ = repo.Create(inv)
	svc := &fakeEmissionService{result: &siat.ResultadoEmision{
		Cuf:             "CUF-1",
		Transaccion:     true,
		CodigoEstado:    908,
		CodigoRecepcion: "RCP-1",
		Xml:             "<xml/>",
		XmlHash:         "abc123",
	}}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)

	got, err := uc.Emit(context.Background(), "inv-1")
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
		t.Errorf("Xml/XmlHash/SiatReceptionCode no persistidos")
	}
	if repo.claimCalls != 1 || repo.updateCalls != 1 {
		t.Errorf("claimCalls=%d updateCalls=%d, se esperaba 1/1", repo.claimCalls, repo.updateCalls)
	}
}

func TestEmitObserved(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(testInvoice())
	svc := &fakeEmissionService{result: &siat.ResultadoEmision{
		Cuf:          "CUF-1",
		Transaccion:  true,
		CodigoEstado: 904,
		Mensajes:     []siat.Mensaje{{Codigo: 1007, Descripcion: "DIRECCION NO CORRESPONDE A PADRON"}},
	}}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)

	got, err := uc.Emit(context.Background(), "inv-1")
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
	svc := &fakeEmissionService{result: &siat.ResultadoEmision{
		Transaccion:  false,
		CodigoEstado: 902,
		Mensajes:     []siat.Mensaje{{Codigo: 123, Descripcion: "descripcion rechazo"}},
	}}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)

	_, err := uc.Emit(context.Background(), "inv-1")
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

	if _, err := uc.Emit(context.Background(), "inv-1"); err == nil {
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

	if _, err := uc.Emit(context.Background(), "inv-1"); err == nil {
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
	svc := &fakeEmissionService{result: &siat.ResultadoEmision{Transaccion: true}}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)

	if _, err := uc.Emit(context.Background(), "inv-1"); err == nil {
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
	svc := &fakeEmissionService{result: &siat.ResultadoEmision{Transaccion: true}}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)

	if _, err := uc.Emit(context.Background(), "inv-1"); err == nil {
		t.Fatal("se esperaba error de claim perdido (SENDING)")
	}
}

func emittedInvoice() *domain.Invoice {
	inv := testInvoice()
	cuf := "CUF-EMITIDO"
	inv.Cuf = &cuf
	inv.Status = domain.InvoiceAccepted
	return inv
}

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
	svc := &fakeEmissionService{docResult: &siat.ResultadoDocumento{
		Transaccion:  true,
		CodigoEstado: 905,
	}}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)

	got, err := uc.VerifyStatus(context.Background(), "inv-1")
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
	svc := &fakeEmissionService{docResult: &siat.ResultadoDocumento{
		Transaccion:  true,
		CodigoEstado: 908,
	}}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)

	got, err := uc.VerifyStatus(context.Background(), "inv-1")
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

	if _, err := uc.VerifyStatus(context.Background(), "inv-1"); err == nil {
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
	svc := &fakeEmissionService{docResult: &siat.ResultadoDocumento{
		Transaccion:     true,
		CodigoEstado:    905,
		CodigoRecepcion: "RCP-ANNUL",
	}}
	uc := newTestUsecase(repo, catalog, svc)

	got, err := uc.Annul(context.Background(), "inv-1", 1)
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
	svc := &fakeEmissionService{docResult: &siat.ResultadoDocumento{
		Transaccion:  false,
		CodigoEstado: 906,
		Mensajes:     []siat.Mensaje{{Codigo: 900, Descripcion: "motivo inválido"}},
	}}
	uc := newTestUsecase(repo, catalog, svc)

	_, err := uc.Annul(context.Background(), "inv-1", 1)
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

	if _, err := uc.Annul(context.Background(), "inv-1", 1); err == nil {
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

	if _, err := uc.Annul(context.Background(), "inv-1", 99); err == nil || !strings.Contains(err.Error(), "motivo") {
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
	svc := &fakeEmissionService{docResult: &siat.ResultadoDocumento{
		Transaccion:     true,
		CodigoEstado:    907,
		CodigoRecepcion: "RCP-REVERT",
	}}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)

	got, err := uc.RevertAnnul(context.Background(), "inv-1")
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

	if _, err := uc.RevertAnnul(context.Background(), "inv-1"); err == nil {
		t.Fatal("se esperaba error por estado no CANCELLED")
	}
}

func TestAnnulUsaCufdVigente(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(emittedInvoice())
	catalog := &fakeCatalogRepo{items: map[string][]*domain.CatalogItem{
		"motivoAnulacion": {{Codigo: 1, Descripcion: "Motivo válido", Tipo: "motivoAnulacion"}},
	}}
	svc := &fakeEmissionService{docResult: &siat.ResultadoDocumento{Transaccion: true, CodigoEstado: 905}}
	uc := NewInvoiceUsecase(repo, nil, nil, nil, catalog, &fakeCufdRepo{vigente: &domain.Cufd{Cufd: "CUFD-VIGENTE", ValidFrom: time.Now().Add(-time.Hour), ValidTo: time.Now().Add(time.Hour)}}, svc, siat.ModalidadElectronica)

	if _, err := uc.Annul(context.Background(), "inv-1", 1); err != nil {
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
	svc := &fakeEmissionService{docResult: &siat.ResultadoDocumento{Transaccion: true, CodigoEstado: 905}}
	uc := NewInvoiceUsecase(repo, nil, nil, nil, catalog, &fakeCufdRepo{}, svc, siat.ModalidadElectronica)

	if _, err := uc.Annul(context.Background(), "inv-1", 1); err != nil {
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
	svc := &fakeEmissionService{docResult: &siat.ResultadoDocumento{Transaccion: true, CodigoEstado: 907}}
	uc := NewInvoiceUsecase(repo, nil, nil, nil, &fakeCatalogRepo{}, &fakeCufdRepo{vigente: &domain.Cufd{Cufd: "CUFD-VIGENTE", ValidFrom: time.Now().Add(-time.Hour), ValidTo: time.Now().Add(time.Hour)}}, svc, siat.ModalidadElectronica)

	if _, err := uc.RevertAnnul(context.Background(), "inv-1"); err != nil {
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

func TestResolveDocumentoSector(t *testing.T) {
	catalog := &fakeCatalogRepo{items: map[string][]*domain.CatalogItem{
		"actividadesDocumentoSector": {
			{Codigo: 1, Descripcion: "8549910|FCV", Tipo: "actividadesDocumentoSector"},
			{Codigo: 1, Descripcion: "8550100|FCV", Tipo: "actividadesDocumentoSector"},
			{Codigo: 11, Descripcion: "8549100|FSEDU", Tipo: "actividadesDocumentoSector"},
			{Codigo: 24, Descripcion: "8549100|NCD", Tipo: "actividadesDocumentoSector"},
			{Codigo: 47, Descripcion: "8549100|NCDDE", Tipo: "actividadesDocumentoSector"},
		},
	}}
	uc := newTestUsecase(newFakeInvoiceRepo(), catalog, nil)

	if got := uc.resolveDocumentoSector("comp-1", "8549100"); got != 11 {
		t.Errorf("resolveDocumentoSector(8549100)=%d, se esperaba 11 (FSEDU)", got)
	}
	if got := uc.resolveDocumentoSector("comp-1", "8549910"); got != 1 {
		t.Errorf("resolveDocumentoSector(8549910)=%d, se esperaba 1 (FCV)", got)
	}
	// Actividad sin asociación: cae a compraventa.
	if got := uc.resolveDocumentoSector("comp-1", "9999999"); got != 1 {
		t.Errorf("resolveDocumentoSector(9999999)=%d, se esperaba 1", got)
	}
}

func TestBuildSolicitudFacturaFSEDU(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.CodigoDocumentoSector = 11
	inv.CodigoTipoFactura = 1
	nombre := "María Fernanda Quispe"
	periodo := "GESTION 1/2026"
	inv.NombreEstudiante = &nombre
	inv.PeriodoFacturado = &periodo

	req, err := uc.buildSolicitudFactura(inv)
	if err != nil {
		t.Fatalf("buildSolicitudFactura FSEDU: %v", err)
	}
	if req.CodigoDocumentoSector != 11 {
		t.Errorf("CodigoDocumentoSector=%d, se esperaba 11", req.CodigoDocumentoSector)
	}
	if req.NombreEstudiante != nombre {
		t.Errorf("NombreEstudiante=%q", req.NombreEstudiante)
	}
	if req.PeriodoFacturado != periodo {
		t.Errorf("PeriodoFacturado=%q", req.PeriodoFacturado)
	}
}

func TestBuildSolicitudFacturaDireccionPadron(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.CufdRecord.Direccion = "URBANIZACION: LA FLORIDA, AVENIDA: AROMA, NRO.: 364"

	req, err := uc.buildSolicitudFactura(inv)
	if err != nil {
		t.Fatalf("buildSolicitudFactura: %v", err)
	}
	if req.Direccion != inv.CufdRecord.Direccion {
		t.Errorf("Direccion=%q, se esperaba la del padrón (CUFD)", req.Direccion)
	}
}

func TestBuildSolicitudFacturaUsaCufdMasReciente(t *testing.T) {
	inv := testInvoice()
	inv.CufdRecord.Cufd = "CUFD-VIEJO"
	inv.CufdRecord.ControlCode = "CC-VIEJO"

	cufdRepo := &fakeCufdRepo{vigente: &domain.Cufd{
		ID:          "cufd-nuevo",
		Cufd:        "CUFD-NUEVO",
		ControlCode: "CC-NUEVO",
		ValidFrom:   time.Now().Add(-time.Hour),
		ValidTo:     time.Now().Add(time.Hour),
		Active:      true,
	}}
	uc := NewInvoiceUsecase(newFakeInvoiceRepo(), nil, nil, nil, &fakeCatalogRepo{}, cufdRepo, nil, siat.ModalidadElectronica)

	req, err := uc.buildSolicitudFactura(inv)
	if err != nil {
		t.Fatalf("buildSolicitudFactura: %v", err)
	}
	if req.Cufd != "CUFD-NUEVO" || req.CodigoControl != "CC-NUEVO" {
		t.Errorf("debío usar el CUFD más reciente, se obtuvo Cufd=%q CodigoControl=%q", req.Cufd, req.CodigoControl)
	}
}

func TestBuildSolicitudFacturaCufdVencidoConVigente(t *testing.T) {
	inv := testInvoice()
	inv.CufdRecord.ValidTo = time.Now().Add(-time.Hour)

	cufdRepo := &fakeCufdRepo{vigente: &domain.Cufd{
		ID:          "cufd-nuevo",
		Cufd:        "CUFD-NUEVO",
		ControlCode: "CC-NUEVO",
		ValidFrom:   time.Now().Add(-time.Hour),
		ValidTo:     time.Now().Add(time.Hour),
		Active:      true,
	}}
	uc := NewInvoiceUsecase(newFakeInvoiceRepo(), nil, nil, nil, &fakeCatalogRepo{}, cufdRepo, nil, siat.ModalidadElectronica)

	req, err := uc.buildSolicitudFactura(inv)
	if err != nil {
		t.Fatalf("buildSolicitudFactura con CUFD de factura vencido pero CUFD nuevo vigente: %v", err)
	}
	if req.Cufd != "CUFD-NUEVO" {
		t.Errorf("Cufd=%q, se esperaba CUFD-NUEVO", req.Cufd)
	}
}
