package usecase

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
	"gorm.io/gorm"
)

// ---- Fakes ----

type fakeInvoiceRepo struct {
	invoices           map[string]*domain.Invoice
	claimCalls         int
	updateCalls        int
	updateErr          error
	activeCufd         *domain.Cufd
	conflictingIdemKey string // simula violación del índice único al crear con esta key
}

type fakePDFGenerator struct{ calls int }

func (f *fakePDFGenerator) GenerateAndPersist(context.Context, string) { f.calls++ }

func (f *fakeInvoiceRepo) GetByIDs(ids []string) ([]*domain.Invoice, error) {
	var result []*domain.Invoice
	for _, id := range ids {
		if inv, exists := f.invoices[id]; exists {
			result = append(result, inv)
		}
	}
	return result, nil
}
func (f *fakeInvoiceRepo) ListByPointOfSale(posID string) ([]*domain.Invoice, error) {
	out := make([]*domain.Invoice, 0)
	for _, inv := range f.invoices {
		if inv.PointOfSaleId == posID {
			out = append(out, inv)
		}
	}
	return out, nil
}

func (f *fakeInvoiceRepo) ReleaseStaleSending(d time.Duration) (int64, error) {
	return 0, nil
}

func newFakeInvoiceRepo() *fakeInvoiceRepo {
	return &fakeInvoiceRepo{invoices: map[string]*domain.Invoice{}}
}

func (f *fakeInvoiceRepo) Create(inv *domain.Invoice) error {
	// Simula la ventana de race: otro request insertó primero esta clave.
	if f.conflictingIdemKey != "" && inv.IdempotencyKey != nil && *inv.IdempotencyKey == f.conflictingIdemKey {
		return errors.New("ERROR: duplicate key value violates unique constraint \"idx_invoice_idem_key\" (SQLSTATE 23505)")
	}
	if inv.ID == "" {
		inv.ID = "inv-" + strconv.Itoa(len(f.invoices)+1)
	}
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

func (f *fakeInvoiceRepo) ListFiltered(filter domain.InvoiceListFilter) ([]*domain.Invoice, int64, error) {
	out := make([]*domain.Invoice, 0)
	for _, inv := range f.invoices {
		if inv.PointOfSaleId != filter.PointOfSaleID {
			continue
		}
		if filter.Status != nil && inv.Status != *filter.Status {
			continue
		}
		out = append(out, inv)
	}
	total := int64(len(out))
	if filter.Offset < len(out) {
		out = out[filter.Offset:]
	} else {
		out = nil
	}
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, total, nil
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
	claimed := *inv
	claimed.Status = domain.InvoiceSending
	f.invoices[id] = &claimed
	return true, nil
}

func (f *fakeInvoiceRepo) TransitionStatus(id string, from, to domain.InvoiceStatus, reason domain.InvoiceTransitionReason, fields map[string]any, event *domain.InvoiceEvent) (bool, error) {
	if err := (domain.InvoiceStateMachine{}).Transition(from, to, reason); err != nil {
		return false, err
	}
	claimed, err := f.ClaimStatus(id, from, to, fields)
	if claimed {
		f.updateCalls++
	}
	return claimed, err
}

func (f *fakeInvoiceRepo) ClaimStatus(id string, from, to domain.InvoiceStatus, fields map[string]any) (bool, error) {
	inv, ok := f.invoices[id]
	if !ok || inv.Status != from {
		return false, nil
	}
	inv.Status = to
	for k, v := range fields {
		switch k {
		case "motivo_anulacion":
			if m, ok := v.(int); ok {
				inv.MotivoAnulacion = &m
			} else if m, ok := v.(*int); ok {
				inv.MotivoAnulacion = m
			}
		case "fecha_anulacion":
			if t, ok := v.(time.Time); ok {
				inv.FechaAnulacion = &t
			} else if t, ok := v.(*time.Time); ok {
				inv.FechaAnulacion = t
			}
		case "siat_reception_code":
			if c, ok := v.(string); ok && c != "" {
				inv.SiatReceptionCode = &c
			}
		case "cuf":
			inv.Cuf, _ = v.(*string)
		case "xml":
			inv.Xml, _ = v.(*string)
		case "xml_hash":
			inv.XmlHash, _ = v.(*string)
		case "archivo":
			inv.Archivo, _ = v.(string)
		case "hash_archivo":
			inv.HashArchivo, _ = v.(string)
		case "contingency_event_id":
			if id, ok := v.(string); ok {
				inv.ContingencyEventId = &id
			}
		case "emission_type":
			inv.EmissionType, _ = v.(string)
		}
	}
	return true, nil
}

func (f *fakeInvoiceRepo) FindActiveCufdForPointOfSale(string, time.Time) (*domain.Cufd, error) {
	return f.activeCufd, nil
}

func (f *fakeInvoiceRepo) GetByIdempotencyKey(pointOfSaleID, key string) (*domain.Invoice, error) {
	for _, inv := range f.invoices {
		if inv.PointOfSaleId == pointOfSaleID && inv.IdempotencyKey != nil && *inv.IdempotencyKey == key {
			return inv, nil
		}
	}
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

type fakeCompanyRepo struct {
	company domain.Company
}

func (f *fakeCompanyRepo) Create(*domain.Company) error             { return nil }
func (f *fakeCompanyRepo) GetByNit(string) (*domain.Company, error) { return &f.company, nil }
func (f *fakeCompanyRepo) GetByID(string) (*domain.Company, error)  { return &f.company, nil }
func (f *fakeCompanyRepo) Update(*domain.Company) error             { return nil }
func (f *fakeCompanyRepo) Delete(string) error                      { return nil }

type fakeCustomerRepo struct {
	byID       map[string]*domain.Customer
	byDocument map[string]*domain.Customer
	created    []*domain.Customer
}

func newFakeCustomerRepo(customers ...domain.Customer) *fakeCustomerRepo {
	f := &fakeCustomerRepo{
		byID:       map[string]*domain.Customer{},
		byDocument: map[string]*domain.Customer{},
	}
	for i := range customers {
		c := &customers[i]
		f.byID[c.ID] = c
		f.byDocument[documentKey(c.DocumentType, c.DocumentNumber)] = c
	}
	return f
}

func documentKey(documentType, documentNumber string) string {
	return documentType + "|" + documentNumber
}

func (f *fakeCustomerRepo) Create(c *domain.Customer) error {
	if c.ID == "" {
		c.ID = "cust-" + strconv.Itoa(len(f.byID)+1)
	}
	f.created = append(f.created, c)
	f.byID[c.ID] = c
	f.byDocument[documentKey(c.DocumentType, c.DocumentNumber)] = c
	return nil
}
func (f *fakeCustomerRepo) GetByID(id string) (*domain.Customer, error) {
	c, ok := f.byID[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return c, nil
}
func (f *fakeCustomerRepo) GetByCompanyAndDocument(_, documentType, documentNumber string) (*domain.Customer, error) {
	c, ok := f.byDocument[documentKey(documentType, documentNumber)]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return c, nil
}
func (f *fakeCustomerRepo) GetByCompanyAndFiscalIdentity(companyID, documentType, documentNumber string, complement *string, name string, email string) (*domain.Customer, error) {
	c, err := f.GetByCompanyAndDocument(companyID, documentType, documentNumber)
	if err != nil || c.Name != name || cadenaOpcional(c.Complement) != cadenaOpcional(complement) {
		return nil, gorm.ErrRecordNotFound
	}
	return c, nil
}
func (f *fakeCustomerRepo) List(string) ([]*domain.Customer, error) { return nil, nil }

type fakePointOfSaleRepo struct {
	pos domain.PointOfSale
}

func (f *fakePointOfSaleRepo) Create(*domain.PointOfSale) error { return nil }
func (f *fakePointOfSaleRepo) GetByID(string) (*domain.PointOfSale, error) {
	return &f.pos, nil
}
func (f *fakePointOfSaleRepo) List(string) ([]*domain.PointOfSale, error) { return nil, nil }
func (f *fakePointOfSaleRepo) ListByBranch(string) ([]*domain.PointOfSale, error) {
	return nil, nil
}
func (f *fakePointOfSaleRepo) Update(*domain.PointOfSale) error { return nil }
func (f *fakePointOfSaleRepo) Delete(string) error              { return nil }

type fakeEmissionService struct {
	result        *ports.FiscalResult
	offlineResult *ports.FiscalResult
	docResult     *ports.FiscalDocumentResult
	eventResult   ports.FiscalEventResult
	eventCaptured *ports.FiscalEvent
	err           error
	offlineErr    error
	emitCalls     int
	offlineCalls  int
	captured      *ports.FiscalDocumentQuery
}

func (f *fakeEmissionService) Emit(context.Context, ports.FiscalDocument) (ports.FiscalResult, error) {
	f.emitCalls++
	if f.result == nil {
		return ports.FiscalResult{}, f.err
	}
	return *f.result, f.err
}

func (f *fakeEmissionService) PrepareOffline(context.Context, ports.FiscalDocument) (ports.FiscalResult, error) {
	f.offlineCalls++
	if f.offlineResult == nil {
		if f.offlineErr != nil {
			return ports.FiscalResult{}, f.offlineErr
		}
		return ports.FiscalResult{}, errors.New("offline no configurado")
	}
	return *f.offlineResult, f.offlineErr
}

type fakeContingencyRepo struct {
	latest  *domain.ContingencyEvent
	created []*domain.ContingencyEvent
	updates int
	err     error
}

func (f *fakeContingencyRepo) Create(event *domain.ContingencyEvent) error {
	if f.err != nil {
		return f.err
	}
	if event.ID == "" {
		event.ID = "event-1"
	}
	f.created = append(f.created, event)
	f.latest = event
	return nil
}

func (f *fakeContingencyRepo) Update(event *domain.ContingencyEvent) error {
	if f.err != nil {
		return f.err
	}
	f.latest = event
	f.updates++
	return nil
}

func (f *fakeContingencyRepo) GetLatestByPointOfSale(string) (*domain.ContingencyEvent, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.latest == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return f.latest, nil
}

func (f *fakeContingencyRepo) GetBySiatCode(string) (*domain.ContingencyEvent, error) {
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeEmissionService) VerifyStatus(context.Context, ports.FiscalDocumentQuery) (ports.FiscalDocumentResult, error) {
	if f.docResult == nil {
		return ports.FiscalDocumentResult{}, f.err
	}
	return *f.docResult, f.err
}

func (f *fakeEmissionService) Annul(_ context.Context, req ports.FiscalDocumentQuery, _ int) (ports.FiscalDocumentResult, error) {
	f.captured = &req
	if f.docResult == nil {
		return ports.FiscalDocumentResult{}, f.err
	}
	return *f.docResult, f.err
}

func (f *fakeEmissionService) RevertAnnul(_ context.Context, req ports.FiscalDocumentQuery) (ports.FiscalDocumentResult, error) {
	f.captured = &req
	if f.docResult == nil {
		return ports.FiscalDocumentResult{}, f.err
	}
	return *f.docResult, f.err
}

func (f *fakeEmissionService) RequestCUIS(context.Context, ports.CredentialRequest) (ports.CuisResult, error) {
	return ports.CuisResult{}, nil
}

func (f *fakeEmissionService) RequestCUFD(context.Context, ports.CredentialRequest) (ports.CufdResult, error) {
	return ports.CufdResult{}, nil
}

func (f *fakeEmissionService) RegisterSignificantEvent(_ context.Context, event ports.FiscalEvent) (ports.FiscalEventResult, error) {
	f.eventCaptured = &event
	return f.eventResult, nil
}

func (f *fakeEmissionService) SendPackage(context.Context, ports.FiscalPackage) (ports.FiscalPackageResult, error) {
	return ports.FiscalPackageResult{}, nil
}

func (f *fakeEmissionService) ValidatePackage(context.Context, ports.FiscalPackage, string) (ports.FiscalPackageResult, error) {
	return ports.FiscalPackageResult{}, nil
}

func (f *fakeEmissionService) SendBulk(context.Context, ports.FiscalBulk) (ports.FiscalPackageResult, error) {
	return ports.FiscalPackageResult{}, nil
}

func (f *fakeEmissionService) ValidateBulk(context.Context, ports.FiscalBulk, string) (ports.FiscalPackageResult, error) {
	return ports.FiscalPackageResult{}, nil
}

func (f *fakeEmissionService) SendPurchases(context.Context, ports.FiscalPurchase) (ports.FiscalPurchaseResult, error) {
	return ports.FiscalPurchaseResult{}, nil
}

func (f *fakeEmissionService) SignXML(context.Context, ports.FiscalSignRequest) (ports.FiscalSignResult, error) {
	return ports.FiscalSignResult{}, nil
}

func (f *fakeEmissionService) EmitAdjustment(context.Context, ports.FiscalAdjustment) (ports.FiscalAdjustmentResult, error) {
	return ports.FiscalAdjustmentResult{}, nil
}

func (f *fakeEmissionService) Synchronize(context.Context, ports.FiscalSyncRequest, ports.FiscalSyncOperation) (ports.FiscalSyncResult, error) {
	return ports.FiscalSyncResult{}, nil
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
			CodigoCliente:  "CI1234567",
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

func newTestUsecase(repo *fakeInvoiceRepo, catalog *fakeCatalogRepo, svc ports.FiscalService) *InvoiceUsecase {
	return NewInvoiceUsecase(repo, nil, nil, nil, catalog, nil, svc, siat.ModalidadElectronica,
		nil, nil, nil, nil, nil, false, nil)
}

type fakeDocSectorRepo struct {
	items []*domain.SiatActividadDocSector
}

func (f *fakeDocSectorRepo) Replace(string, []domain.SiatActividadDocSector, time.Time) error {
	return nil
}

func (f *fakeDocSectorRepo) List(string) ([]*domain.SiatActividadDocSector, error) {
	return f.items, nil
}

func (f *fakeDocSectorRepo) ListByActividad(_ string, codigoActividad string) ([]*domain.SiatActividadDocSector, error) {
	out := make([]*domain.SiatActividadDocSector, 0)
	for _, item := range f.items {
		if item.CodigoActividad == codigoActividad {
			out = append(out, item)
		}
	}
	return out, nil
}

type fakeLeyendaRepo struct {
	items []*domain.SiatLeyenda
}

func (f *fakeLeyendaRepo) Replace(string, []domain.SiatLeyenda, time.Time) error {
	return nil
}

func (f *fakeLeyendaRepo) List(string) ([]*domain.SiatLeyenda, error) {
	return f.items, nil
}

func (f *fakeLeyendaRepo) ListByActividad(_ string, codigoActividad string) ([]*domain.SiatLeyenda, error) {
	out := make([]*domain.SiatLeyenda, 0)
	for _, item := range f.items {
		if item.CodigoActividad == codigoActividad {
			out = append(out, item)
		}
	}
	return out, nil
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
	leyendas := &fakeLeyendaRepo{items: []*domain.SiatLeyenda{
		{CodigoActividad: "101010", DescripcionLeyenda: "Leyenda oficial de prueba"},
	}}
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	uc.leyendaRepo = leyendas

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

	req, err := uc.buildSolicitudFactura(context.Background(), inv)
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
	custID := ""
	if req.Cliente.CodigoCliente != nil {
		custID = *req.Cliente.CodigoCliente
	}
	if req.Cliente.NumeroDocumento != "1234567" || custID != "CI1234567" {
		t.Errorf("cliente documento/codigo=%q/%q", req.Cliente.NumeroDocumento, custID)
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

	req, err := uc.buildSolicitudFactura(context.Background(), inv)
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

	if _, err := uc.buildSolicitudFactura(context.Background(), inv); err == nil {
		t.Fatal("se esperaba error por CUIS ausente")
	}
}

func TestBuildSolicitudFacturaCufdVencido(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.CufdRecord.ValidTo = time.Now().Add(-time.Hour)

	if _, err := uc.buildSolicitudFactura(context.Background(), inv); err == nil {
		t.Fatal("se esperaba error por CUFD vencido")
	}
}

func TestBuildSolicitudFacturaSinCodigoProductoSin(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.Items[0].CodigoProductoSin = nil

	if _, err := uc.buildSolicitudFactura(context.Background(), inv); err == nil || !strings.Contains(err.Error(), "codigo_producto_sin") {
		t.Fatalf("se esperaba error de codigo_producto_sin, se obtuvo: %v", err)
	}
}

func TestBuildSolicitudFacturaTipoDocInvalido(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.Customer.DocumentType = "XYZ"

	if _, err := uc.buildSolicitudFactura(context.Background(), inv); err == nil {
		t.Fatal("se esperaba error por tipo de documento inválido")
	}
}

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

func TestEmitSiempreProcesaSincrono(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(testInvoice())
	svc := &fakeEmissionService{result: &ports.FiscalResult{Transaccion: true, CodigoEstado: 908}}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)

	got, err := uc.Emit(context.Background(), "inv-1")
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if got.Status != domain.InvoiceAccepted || repo.claimCalls != 1 || svc.emitCalls != 1 {
		t.Fatalf("status=%s claimCalls=%d siatCalls=%d", got.Status, repo.claimCalls, svc.emitCalls)
	}
}

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

	got, err := uc.Emit(context.Background(), "inv-1")
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

func TestProcessEmissionGeneraPDFDentroDelWorker(t *testing.T) {
	repo := newFakeInvoiceRepo()
	_ = repo.Create(testInvoice())
	svc := &fakeEmissionService{result: &ports.FiscalResult{Transaccion: true, CodigoEstado: 908}}
	pdf := &fakePDFGenerator{}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, svc)
	uc.pdfService = pdf

	if _, err := uc.ProcessEmission(context.Background(), "inv-1"); err != nil {
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
		Transaccion:  true,
		CodigoEstado: 904,
		Mensajes:     []ports.FiscalMessage{{Codigo: 1007, Descripcion: "DIRECCION NO CORRESPONDE A PADRON"}},
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
	svc := &fakeEmissionService{result: &ports.FiscalResult{
		Transaccion:  false,
		CodigoEstado: 902,
		Mensajes:     []ports.FiscalMessage{{Codigo: 123, Descripcion: "descripcion rechazo"}},
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
	svc := &fakeEmissionService{result: &ports.FiscalResult{Transaccion: true}}
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
	svc := &fakeEmissionService{result: &ports.FiscalResult{Transaccion: true}}
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
	svc := &fakeEmissionService{docResult: &ports.FiscalDocumentResult{
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
	svc := &fakeEmissionService{docResult: &ports.FiscalDocumentResult{
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
	svc := &fakeEmissionService{docResult: &ports.FiscalDocumentResult{
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
	svc := &fakeEmissionService{docResult: &ports.FiscalDocumentResult{
		Transaccion:  false,
		CodigoEstado: 906,
		Mensajes:     []ports.FiscalMessage{{Codigo: 900, Descripcion: "motivo inválido"}},
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
	svc := &fakeEmissionService{docResult: &ports.FiscalDocumentResult{
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
	svc := &fakeEmissionService{docResult: &ports.FiscalDocumentResult{Transaccion: true, CodigoEstado: 905}}
	uc := NewInvoiceUsecase(repo, nil, nil, nil, catalog, &fakeCufdRepo{vigente: &domain.Cufd{Cufd: "CUFD-VIGENTE", ValidFrom: time.Now().Add(-time.Hour), ValidTo: time.Now().Add(time.Hour)}}, svc, siat.ModalidadElectronica,
		nil, nil, nil, nil, nil, false, nil)

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
	svc := &fakeEmissionService{docResult: &ports.FiscalDocumentResult{Transaccion: true, CodigoEstado: 905}}
	uc := NewInvoiceUsecase(repo, nil, nil, nil, catalog, &fakeCufdRepo{}, svc, siat.ModalidadElectronica,
		nil, nil, nil, nil, nil, false, nil)

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
	svc := &fakeEmissionService{docResult: &ports.FiscalDocumentResult{Transaccion: true, CodigoEstado: 907}}
	uc := NewInvoiceUsecase(repo, nil, nil, nil, &fakeCatalogRepo{}, &fakeCufdRepo{vigente: &domain.Cufd{Cufd: "CUFD-VIGENTE", ValidFrom: time.Now().Add(-time.Hour), ValidTo: time.Now().Add(time.Hour)}}, svc, siat.ModalidadElectronica,
		nil, nil, nil, nil, nil, false, nil)

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
	sectores := &fakeDocSectorRepo{items: []*domain.SiatActividadDocSector{
		{CodigoActividad: "8549910", CodigoDocumentoSector: 1, TipoDocumentoSector: "FCV"},
		{CodigoActividad: "8550100", CodigoDocumentoSector: 1, TipoDocumentoSector: "FCV"},
		{CodigoActividad: "8549100", CodigoDocumentoSector: 11, TipoDocumentoSector: "FSEDU"},
		{CodigoActividad: "8549100", CodigoDocumentoSector: 24, TipoDocumentoSector: "NCD"},
		{CodigoActividad: "8549100", CodigoDocumentoSector: 47, TipoDocumentoSector: "NCDDE"},
	}}
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	uc.docSectorRepo = sectores

	if got, err := uc.resolveDocumentoSector("comp-1", "8549100"); err != nil || got != 11 {
		t.Errorf("resolveDocumentoSector(8549100)=%d/%v, se esperaba 11 (FSEDU)", got, err)
	}
	if got, err := uc.resolveDocumentoSector("comp-1", "8549910"); err != nil || got != 1 {
		t.Errorf("resolveDocumentoSector(8549910)=%d/%v, se esperaba 1 (FCV)", got, err)
	}
	// Actividad sin asociación: no se permite un fallback estático.
	if _, err := uc.resolveDocumentoSector("comp-1", "9999999"); err == nil {
		t.Fatal("se esperaba error para actividad no sincronizada")
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

	req, err := uc.buildSolicitudFactura(context.Background(), inv)
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

func TestBuildSolicitudFacturaNoArrastraCamposEducativosFueraDeSector11(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.CodigoDocumentoSector = 1
	nombre := "No debe viajar"
	periodo := "2026-1"
	inv.NombreEstudiante = &nombre
	inv.PeriodoFacturado = &periodo

	req, err := uc.buildSolicitudFactura(context.Background(), inv)
	if err != nil {
		t.Fatalf("buildSolicitudFactura: %v", err)
	}
	// Nuevo contrato: el usecase siempre pasa los campos legados y la capa siat
	// los ignora fuera de los sectores educativos (11/46): los datos específicos
	// resultantes deben quedar vacíos para compraventa.
	perfil, err := siat.PerfilSector(1)
	if err != nil {
		t.Fatalf("PerfilSector(1): %v", err)
	}
	valores, err := perfil.PrepararDatosSector(toSiatSolicitudFactura(*req))
	if err != nil {
		t.Fatalf("PrepararDatosSector: %v", err)
	}
	if len(valores) != 0 {
		t.Errorf("datos_sector normalizados=%v, se esperaba vacío fuera del sector 11", valores)
	}
}

func TestCreatePurgeaCamposEducativosFueraDeSector11(t *testing.T) {
	repo := newFakeInvoiceRepo()
	repo.activeCufd = &domain.Cufd{
		ID:          "cufd-1",
		Cufd:        "CUFD-XYZ",
		ControlCode: "CC-123",
		ValidFrom:   time.Now().Add(-time.Hour),
		ValidTo:     time.Now().Add(time.Hour),
		Active:      true,
	}
	posRepo := &fakePointOfSaleRepo{pos: domain.PointOfSale{
		ID:               "pos-1",
		CompanyId:        "comp-1",
		CodigoSucursal:   0,
		CodigoPuntoVenta: 3,
		Cuis:             strPtr("D17EEF19"),
		IsActive:         true,
	}}
	customerRepo := newFakeCustomerRepo(domain.Customer{
		ID:             "cust-1",
		CompanyId:      "comp-1",
		DocumentType:   "CI",
		DocumentNumber: "1234567",
		Name:           "Juan Perez",
	})
	companyRepo := &fakeCompanyRepo{company: domain.Company{
		ID:           "comp-1",
		Nit:          "9971522011",
		BusinessName: "EMPRESA PILOTO SRL",
		Ambiente:     domain.EnvironmentPiloto,
		Municipio:    "LA PAZ",
		Direccion:    "AV. CAMACHO 123",
	}}
	uc := NewInvoiceUsecase(repo, customerRepo, companyRepo, posRepo, &fakeCatalogRepo{}, nil, nil, siat.ModalidadElectronica,
		nil, nil, nil, nil, nil, false, nil)
	nombre := "MARIA TEST"
	periodo := "2026-1"
	req := CreateInvoiceRequest{
		CompanyId:             "comp-1",
		PointOfSaleId:         "pos-1",
		Customer:              &CreateInvoiceInlineCustomer{DocumentType: "CI", DocumentNumber: "1234567", Name: "Juan Perez"},
		CodigoDocumentoSector: 1,
		NombreEstudiante:      &nombre,
		PeriodoFacturado:      &periodo,
		Items:                 []CreateInvoiceItemRequest{fiscalTestItem("P001", "Producto", 1, 100)},
	}

	inv, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Nuevo contrato: Create persiste los campos tal cual llegan (auditoría);
	// el filtrado ocurre al construir el XML, no al guardar.
	if inv.NombreEstudiante == nil || *inv.NombreEstudiante != nombre {
		t.Fatalf("NombreEstudiante=%v, se esperaba %q persistido", inv.NombreEstudiante, nombre)
	}
	if inv.PeriodoFacturado == nil || *inv.PeriodoFacturado != periodo {
		t.Fatalf("PeriodoFacturado=%v, se esperaba %q persistido", inv.PeriodoFacturado, periodo)
	}
	if len(inv.SectorData) != 0 && string(inv.SectorData) != "{}" {
		t.Errorf("SectorData=%s, se esperaba vacío para compraventa sin datos específicos", inv.SectorData)
	}
}

func TestBuildSolicitudFacturaDireccionPadron(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.CufdRecord.Direccion = "URBANIZACION: LA FLORIDA, AVENIDA: AROMA, NRO.: 364"

	req, err := uc.buildSolicitudFactura(context.Background(), inv)
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
	uc := NewInvoiceUsecase(newFakeInvoiceRepo(), nil, nil, nil, &fakeCatalogRepo{}, cufdRepo, nil, siat.ModalidadElectronica,
		nil, nil, nil, nil, nil, false, nil)

	req, err := uc.buildSolicitudFactura(context.Background(), inv)
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
	uc := NewInvoiceUsecase(newFakeInvoiceRepo(), nil, nil, nil, &fakeCatalogRepo{}, cufdRepo, nil, siat.ModalidadElectronica,
		nil, nil, nil, nil, nil, false, nil)

	req, err := uc.buildSolicitudFactura(context.Background(), inv)
	if err != nil {
		t.Fatalf("buildSolicitudFactura con CUFD de factura vencido pero CUFD nuevo vigente: %v", err)
	}
	if req.Cufd != "CUFD-NUEVO" {
		t.Errorf("Cufd=%q, se esperaba CUFD-NUEVO", req.Cufd)
	}
}

// --- lazy credentials ---

type fakeCredentialProvider struct {
	cuisErr   error
	cufd      *domain.Cufd
	cufdErr   error
	cuisCalls int
	cufdCalls int
}

func (f *fakeCredentialProvider) EnsureCuis(_ context.Context, _ *domain.Company, pos *domain.PointOfSale) error {
	f.cuisCalls++
	if f.cuisErr != nil {
		return f.cuisErr
	}
	cuis := "CUIS-LAZY"
	pos.Cuis = &cuis
	return nil
}

func (f *fakeCredentialProvider) EnsureCufd(_ context.Context, _ *domain.Company, _ *domain.PointOfSale) (*domain.Cufd, error) {
	f.cufdCalls++
	return f.cufd, f.cufdErr
}

func TestBuildSolicitudFacturaLazyCredenciales(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	provider := &fakeCredentialProvider{
		cufd: &domain.Cufd{ID: "cufd-lazy", Cufd: "CUFD-LAZY", ControlCode: "CTRL-LAZY", Active: true,
			ValidFrom: time.Now().Add(-time.Minute), ValidTo: time.Now().Add(time.Hour),
			Direccion: "AV. PADRON 123"},
	}
	uc.credentials = provider

	inv := testInvoice()
	inv.PointOfSale.Cuis = nil // sin cuis ni cufd vigente

	req, err := uc.buildSolicitudFactura(context.Background(), inv)
	if err != nil {
		t.Fatalf("buildSolicitudFactura con credenciales lazy: %v", err)
	}
	if provider.cuisCalls != 1 || provider.cufdCalls != 1 {
		t.Fatalf("llamadas cuis=%d cufd=%d, se esperaban 1 y 1", provider.cuisCalls, provider.cufdCalls)
	}
	if req.Cuis != "CUIS-LAZY" || req.Cufd != "CUFD-LAZY" || req.CodigoControl != "CTRL-LAZY" {
		t.Fatalf("solicitud con credenciales lazy incorrectas: cuis=%s cufd=%s ctrl=%s", req.Cuis, req.Cufd, req.CodigoControl)
	}
}

func TestBuildSolicitudFacturaSinProviderMantieneError(t *testing.T) {
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	inv := testInvoice()
	inv.PointOfSale.Cuis = nil

	if _, err := uc.buildSolicitudFactura(context.Background(), inv); err == nil {
		t.Fatal("sin provider de credenciales el cuis ausente debe seguir siendo error")
	}
}

func TestEmitEsperaCufdAntesDeClaim(t *testing.T) {
	repo := newFakeInvoiceRepo()
	inv := testInvoice()
	repo.invoices[inv.ID] = inv
	provider := &fakeCredentialProvider{cufdErr: errors.New("renovación CUFD temporalmente no disponible")}
	uc := newTestUsecase(repo, &fakeCatalogRepo{}, &fakeEmissionService{})
	uc.credentials = provider

	if _, err := uc.Emit(context.Background(), inv.ID); err == nil {
		t.Fatal("se esperaba que la emisión esperara la renovación del CUFD")
	}
	if repo.claimCalls != 0 {
		t.Fatalf("ClaimForEmission fue llamado %d veces antes de tener CUFD", repo.claimCalls)
	}
	if inv.Status != domain.InvoicePending {
		t.Fatalf("status=%s, la factura debe permanecer PENDING mientras espera CUFD", inv.Status)
	}
}

func TestListInvoices(t *testing.T) {
	repo := newFakeInvoiceRepo()
	uc := NewInvoiceUsecase(repo, nil, nil, nil, &fakeCatalogRepo{}, nil, nil, siat.ModalidadElectronica,
		nil, nil, nil, nil, nil, false, nil)

	aceptada := testInvoice()
	aceptada.ID = "inv-ok"
	aceptada.Status = domain.InvoiceAccepted
	_ = repo.Create(aceptada)
	pending := testInvoice()
	pending.ID = "inv-pending"
	pending.Status = domain.InvoicePending
	_ = repo.Create(pending)

	t.Run("sin filtro de estado devuelve todo", func(t *testing.T) {
		list, total, err := uc.ListInvoices(domain.InvoiceListFilter{PointOfSaleID: "pos-1"})
		if err != nil || total != 2 || len(list) != 2 {
			t.Fatalf("list=%d total=%d err=%v", len(list), total, err)
		}
	})

	t.Run("filtra por estado", func(t *testing.T) {
		status := domain.InvoiceAccepted
		list, total, err := uc.ListInvoices(domain.InvoiceListFilter{PointOfSaleID: "pos-1", Status: &status})
		if err != nil || total != 1 || len(list) != 1 || list[0].ID != "inv-ok" {
			t.Fatalf("list=%d total=%d err=%v", len(list), total, err)
		}
	})

	t.Run("point_of_sale_id obligatorio", func(t *testing.T) {
		if _, _, err := uc.ListInvoices(domain.InvoiceListFilter{}); err == nil {
			t.Fatal("se esperaba error de validación")
		}
	})

	t.Run("estado inválido rechazado", func(t *testing.T) {
		status := domain.InvoiceStatus("NO_EXISTE")
		if _, _, err := uc.ListInvoices(domain.InvoiceListFilter{PointOfSaleID: "pos-1", Status: &status}); err == nil {
			t.Fatal("se esperaba error por estado inválido")
		}
	})

	t.Run("paginación respeta limit/offset", func(t *testing.T) {
		list, total, err := uc.ListInvoices(domain.InvoiceListFilter{PointOfSaleID: "pos-1", Limit: 1, Offset: 1})
		if err != nil || total != 2 || len(list) != 1 {
			t.Fatalf("list=%d total=%d err=%v", len(list), total, err)
		}
	})
}

// createTestUsecaseBuilder arma un InvoiceUsecase listo para tests de Create.
func createTestUsecaseBuilder() (*InvoiceUsecase, *fakeInvoiceRepo, *fakeCustomerRepo, struct{}) {
	repo := newFakeInvoiceRepo()
	repo.activeCufd = &domain.Cufd{
		ID:          "cufd-1",
		Cufd:        "CUFD-XYZ",
		ControlCode: "CC-123",
		ValidFrom:   time.Now().Add(-time.Hour),
		ValidTo:     time.Now().Add(time.Hour),
		Active:      true,
	}
	posRepo := &fakePointOfSaleRepo{pos: domain.PointOfSale{
		ID:               "pos-1",
		CompanyId:        "comp-1",
		CodigoSucursal:   0,
		CodigoPuntoVenta: 3,
		Cuis:             strPtr("D17EEF19"),
		IsActive:         true,
	}}
	customerRepo := newFakeCustomerRepo(domain.Customer{
		ID:             "cust-1",
		CompanyId:      "comp-1",
		DocumentType:   "CI",
		DocumentNumber: "1234567",
		Name:           "Juan Perez",
	})
	companyRepo := &fakeCompanyRepo{company: domain.Company{
		ID:              "comp-1",
		Nit:             "9971522011",
		BusinessName:    "EMPRESA PILOTO SRL",
		Ambiente:        domain.EnvironmentPiloto,
		Municipio:       "LA PAZ",
		Direccion:       "AV. CAMACHO 123",
		CodigoActividad: strPtr("101010"),
	}}
	docSectorRepo := &fakeDocSectorRepo{items: []*domain.SiatActividadDocSector{
		{CodigoActividad: "101010", CodigoDocumentoSector: siat.SectorCompraVenta, TipoDocumentoSector: "FCV"},
	}}
	uc := NewInvoiceUsecase(repo, customerRepo, companyRepo, posRepo, &fakeCatalogRepo{}, nil, nil, siat.ModalidadElectronica,
		nil, nil, docSectorRepo, nil, nil, false, nil)
	return uc, repo, customerRepo, struct{}{}
}

func fiscalTestItem(code, description string, quantity, unitPrice float64) CreateInvoiceItemRequest {
	activity, sinCode, unit := "101010", "5113100", 58
	return CreateInvoiceItemRequest{
		Code: code, Description: description, CodigoActividad: &activity,
		CodigoProductoSin: &sinCode, UnitCode: &unit,
		Quantity: quantity, UnitPrice: unitPrice,
	}
}

func historicalCustomerRequest() *CreateInvoiceInlineCustomer {
	return &CreateInvoiceInlineCustomer{DocumentType: "CI", DocumentNumber: "1234567", Name: "Juan Perez"}
}

func TestCreateCustomerInline(t *testing.T) {
	uc, _, customerRepo, _ := createTestUsecaseBuilder()

	req := CreateInvoiceRequest{
		PointOfSaleId: "pos-1",
		Customer: &CreateInvoiceInlineCustomer{
			DocumentType:   "nit",
			DocumentNumber: "123456789",
			Name:           "CLIENTE NUEVO",
		},
		Items: []CreateInvoiceItemRequest{fiscalTestItem("P001", "Producto", 1, 100)},
	}

	inv, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if inv.CustomerId == "" {
		t.Fatal("CustomerId no asignado")
	}
	if len(customerRepo.created) != 1 {
		t.Fatalf("clientes creados=%d, se esperaba 1", len(customerRepo.created))
	}
	if customerRepo.created[0].Name != "CLIENTE NUEVO" {
		t.Errorf("nombre=%q", customerRepo.created[0].Name)
	}
	if customerRepo.created[0].CodigoCliente != "NIT123456789" {
		t.Errorf("codigo_cliente=%q, se esperaba NIT123456789 generado al crear", customerRepo.created[0].CodigoCliente)
	}
}

func TestCreateMismoDocumentoNombreDistintoCreaNuevaVersion(t *testing.T) {
	uc, _, customerRepo, _ := createTestUsecaseBuilder()

	req := CreateInvoiceRequest{
		PointOfSaleId: "pos-1",
		Customer: &CreateInvoiceInlineCustomer{
			DocumentType:   "CI",
			DocumentNumber: "1234567",
			Name:           "OTRO NOMBRE",
		},
		Items: []CreateInvoiceItemRequest{fiscalTestItem("P001", "Producto", 1, 100)},
	}

	inv, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if inv.CustomerId == "cust-1" {
		t.Errorf("CustomerId=%q, no debía reusar la versión con otro nombre", inv.CustomerId)
	}
	if inv.Customer.Name != "OTRO NOMBRE" {
		t.Errorf("Customer.Name=%q, se esperaba conservar el snapshot nuevo", inv.Customer.Name)
	}
	if len(customerRepo.created) != 1 {
		t.Fatalf("debió crear una nueva versión histórica; creados=%d", len(customerRepo.created))
	}
}

func TestCreateCompanyDerivadoDePOS(t *testing.T) {
	uc, _, _, _ := createTestUsecaseBuilder()

	req := CreateInvoiceRequest{
		PointOfSaleId: "pos-1",
		Customer:      historicalCustomerRequest(),
		Items:         []CreateInvoiceItemRequest{fiscalTestItem("P001", "Producto", 1, 100)},
	}

	inv, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if inv.CompanyId != "comp-1" {
		t.Errorf("CompanyId=%q, se esperaba comp-1 derivado del POS", inv.CompanyId)
	}
}

func TestCreateRequiereDescripcionSnapshot(t *testing.T) {
	uc, _, _, _ := createTestUsecaseBuilder()

	req := CreateInvoiceRequest{
		PointOfSaleId: "pos-1",
		Customer:      historicalCustomerRequest(),
		Items:         []CreateInvoiceItemRequest{fiscalTestItem("SKU-001", "", 1, 100)},
	}

	inv, err := uc.Create(context.Background(), req)
	if err == nil || inv != nil {
		t.Fatalf("se esperaba rechazar el ítem sin descripción propia: inv=%+v err=%v", inv, err)
	}
}

func TestCreateIdempotencia(t *testing.T) {
	uc, repo, _, _ := createTestUsecaseBuilder()

	req := CreateInvoiceRequest{
		PointOfSaleId:    "pos-1",
		Customer:         historicalCustomerRequest(),
		IdempotencyKey:   "orden-42",
		CodigoMetodoPago: 1,
		Items:            []CreateInvoiceItemRequest{fiscalTestItem("P001", "Producto", 1, 100)},
	}

	inv1, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create #1: %v", err)
	}

	// Segunda llamada con la misma clave debe devolver la misma factura sin crear otra.
	inv2, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create #2: %v", err)
	}
	if inv1.ID != inv2.ID {
		t.Errorf("ids distintos: %s vs %s", inv1.ID, inv2.ID)
	}
	if len(repo.invoices) != 1 {
		t.Errorf("facturas persistidas=%d, se esperaba 1", len(repo.invoices))
	}

	// Clave distinta crea otra factura.
	req.IdempotencyKey = "orden-43"
	inv3, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create #3: %v", err)
	}
	if inv3.ID == inv1.ID {
		t.Error("claves distintas no deben devolver la misma factura")
	}
}

func TestCreateRaceIdempotencia(t *testing.T) {
	uc, repo, _, _ := createTestUsecaseBuilder()
	// Simula que otro request ganó la carrera e insertó la clave primero.
	repo.conflictingIdemKey = "orden-race"

	req := CreateInvoiceRequest{
		PointOfSaleId:  "pos-1",
		Customer:       historicalCustomerRequest(),
		IdempotencyKey: "orden-race",
		Items:          []CreateInvoiceItemRequest{fiscalTestItem("P001", "Producto", 1, 100)},
	}
	ganadora := &domain.Invoice{
		ID:             "inv-ganador",
		CompanyId:      "comp-1",
		CustomerId:     "cust-1",
		PointOfSaleId:  "pos-1",
		Status:         domain.InvoicePending,
		IdempotencyKey: strPtr("orden-race"),
	}
	repo.invoices["inv-ganador"] = ganadora

	got, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create con conflicto de idempotencia: %v", err)
	}
	if got.ID != "inv-ganador" {
		t.Errorf("id=%q, se esperaba el replay de inv-ganador", got.ID)
	}

	// Con conflicto de clave pero sin factura existente que re-leer
	// (p.ej. la fila ganadora aún no visible), el error debe propagarse.
	req.IdempotencyKey = "orden-huerfana"
	repo.conflictingIdemKey = "orden-huerfana"
	if _, err := uc.Create(context.Background(), req); err == nil {
		t.Fatal("conflicto sin factura existente debe propagar el error")
	}
}

func TestCreateIgnoresEmitFlag(t *testing.T) {
	uc, _, _, _ := createTestUsecaseBuilder()

	req := CreateInvoiceRequest{
		PointOfSaleId: "pos-1",
		Customer:      historicalCustomerRequest(),
		Emit:          true,
		Items:         []CreateInvoiceItemRequest{fiscalTestItem("P001", "Producto", 1, 100)},
	}

	// El flag Emit es responsabilidad del handler; el usecase Create solo crea.
	inv, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if inv.Status != domain.InvoicePending {
		t.Errorf("status=%s, se esperaba PENDING (Emit no afecta Create)", inv.Status)
	}
}

func TestCreateWithReceiver(t *testing.T) {
	uc, _, customerRepo, _ := createTestUsecaseBuilder()

	req := CreateInvoiceRequest{
		PointOfSaleId: "pos-1",
		Receiver: &CreateInvoiceReceiver{
			DocumentType:   1, // CI
			DocumentNumber: "12345678",
			Name:           "Juan Perez",
			Email:          strPtr("juan@email.com"),
		},
		Items: []CreateInvoiceItemRequest{fiscalTestItem("P001", "Producto", 1, 100)},
	}

	inv, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create with receiver: %v", err)
	}

	// Legacy Receiver ahora mapea a Customer (create-only)
	if len(customerRepo.created) != 1 {
		t.Fatalf("debió crear 1 cliente via Receiver legacy; creados=%d", len(customerRepo.created))
	}
	if inv.CustomerId == "" {
		t.Errorf("CustomerId se esperaba con valor")
	}
	_ = inv
}

func TestCreateWithReceiverAssociatesExistingCustomer(t *testing.T) {
	uc, _, customerRepo, _ := createTestUsecaseBuilder()

	// Pre-create a customer with the same document
	existingCustomer := &domain.Customer{
		ID:             "cust-existing",
		CompanyId:      "comp-1",
		DocumentType:   "CI",
		DocumentNumber: "12345678",
		Name:           "Juan Perez Existente",
		Complement:     strPtr("COMP"),
	}
	if err := customerRepo.Create(existingCustomer); err != nil {
		t.Fatalf("setup customer: %v", err)
	}

	req := CreateInvoiceRequest{
		PointOfSaleId: "pos-1",
		Receiver: &CreateInvoiceReceiver{
			DocumentType:   1, // CI
			DocumentNumber: "12345678",
			Name:           "Juan Perez", // Different name, but same document
			Email:          strPtr("juan@email.com"),
		},
		Items: []CreateInvoiceItemRequest{fiscalTestItem("P001", "Producto", 1, 100)},
	}

	inv, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create with receiver: %v", err)
	}

	// El nombre diferente crea otra versión y preserva exactamente el request.
	if inv.CustomerId == "cust-existing" || inv.Customer.Name != "Juan Perez" {
		t.Errorf("snapshot/versionado inesperado: customer_id=%q customer=%+v", inv.CustomerId, inv.Customer)
	}
}

func TestCreateValidationExactlyOneCustomerSource(t *testing.T) {
	uc, _, _, _ := createTestUsecaseBuilder()

	tests := []struct {
		name    string
		req     CreateInvoiceRequest
		wantErr bool
	}{
		{
			name: "none provided",
			req: CreateInvoiceRequest{
				PointOfSaleId: "pos-1",
				Items:         []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
			},
			wantErr: true,
		},
		{
			name: "customer_id and customer",
			req: CreateInvoiceRequest{
				PointOfSaleId: "pos-1",
				CustomerId:    "cust-1",
				Customer:      &CreateInvoiceInlineCustomer{DocumentType: "CI", DocumentNumber: "123", Name: "Test"},
				Items:         []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
			},
			wantErr: false,
		},
		{
			name: "customer_id and receiver",
			req: CreateInvoiceRequest{
				PointOfSaleId: "pos-1",
				CustomerId:    "cust-1",
				Receiver:      &CreateInvoiceReceiver{DocumentType: 1, DocumentNumber: "123", Name: "Test"},
				Items:         []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
			},
			wantErr: false,
		},
		{
			name: "customer and receiver",
			req: CreateInvoiceRequest{
				PointOfSaleId: "pos-1",
				Customer:      &CreateInvoiceInlineCustomer{DocumentType: "CI", DocumentNumber: "123", Name: "Test"},
				Receiver:      &CreateInvoiceReceiver{DocumentType: 1, DocumentNumber: "123", Name: "Test"},
				Items:         []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
			},
			wantErr: false,
		},
		{
			name: "three sources",
			req: CreateInvoiceRequest{
				PointOfSaleId: "pos-1",
				CustomerId:    "cust-1",
				Customer:      &CreateInvoiceInlineCustomer{DocumentType: "CI", DocumentNumber: "123", Name: "Test"},
				Receiver:      &CreateInvoiceReceiver{DocumentType: 1, DocumentNumber: "123", Name: "Test"},
				Items:         []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := range tt.req.Items {
				item := fiscalTestItem(tt.req.Items[i].Code, tt.req.Items[i].Description, tt.req.Items[i].Quantity, tt.req.Items[i].UnitPrice)
				tt.req.Items[i] = item
			}
			_, err := uc.Create(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("error=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

// toSiatSolicitudFactura convierte un FiscalDocument a SolicitudFactura para
// poder reutilizar helpers de perfil en tests de buildSolicitudFactura.
func toSiatSolicitudFactura(f ports.FiscalDocument) siat.SolicitudFactura {
	return siat.SolicitudFactura{
		CodigoAmbiente:        f.CodigoAmbiente,
		CodigoSistema:         f.CodigoSistema,
		Nit:                   f.Nit,
		Modalidad:             f.Modalidad,
		NumeroFactura:         f.NumeroFactura,
		NumeroFacturaOriginal: f.NumeroFacturaOriginal,
		CodigoSucursal:        f.CodigoSucursal,
		CodigoPuntoVenta:      f.CodigoPuntoVenta,
		Cuis:                  f.Cuis,
		Cufd:                  f.Cufd,
		CodigoControl:         f.CodigoControl,
		FechaEmision:          f.FechaEmision,
		Usuario:               f.Usuario,
		Leyenda:               f.Leyenda,
		RazonSocialEmisor:     f.RazonSocialEmisor,
		Municipio:             f.Municipio,
		Direccion:             f.Direccion,
		Telefono:              f.Telefono,
		CodigoMetodoPago:      f.CodigoMetodoPago,
		CodigoMoneda:          f.CodigoMoneda,
		TipoCambio:            f.TipoCambio,
		MontoTotal:            f.MontoTotal,
		CodigoDocumentoSector: f.CodigoDocumentoSector,
		Layout:                f.Layout,
		CodigoTipoFactura:     f.CodigoTipoFactura,
		Cafc:                  f.Cafc,
		DatosSector:           f.DatosSector,
		NombreEstudiante:      f.NombreEstudiante,
		PeriodoFacturado:      f.PeriodoFacturado,
		Archivo:               f.Archivo,
		HashArchivo:           f.HashArchivo,
		Cuf:                   f.Cuf,
	}
}
