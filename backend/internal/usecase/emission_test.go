package usecase

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/siat"
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
	inv.Status = domain.InvoiceSending
	return true, nil
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
		c.ID = "cust-" + strconv.Itoa(len(f.created)+1)
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

func (f *fakeEmissionService) VerificarNit(_ context.Context, _ string, _ string, _, _, _ int) (bool, error) {
	return true, f.err
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

func newTestUsecase(repo *fakeInvoiceRepo, catalog *fakeCatalogRepo, svc SiatEmissionService) *InvoiceUsecase {
	return NewInvoiceUsecase(repo, nil, nil, nil, catalog, nil, svc, siat.ModalidadElectronica)
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
	valores, err := perfil.PrepararDatosSector(*req)
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
		ID:            "comp-1",
		Nit:           "9971522011",
		BusinessName:  "EMPRESA PILOTO SRL",
		CodigoSistema: "228452C38ED8739408AB6",
		Ambiente:      domain.EnvironmentPiloto,
		Municipio:     "LA PAZ",
		Direccion:     "AV. CAMACHO 123",
	}}
	uc := NewInvoiceUsecase(repo, customerRepo, companyRepo, posRepo, &fakeCatalogRepo{}, nil, nil, siat.ModalidadElectronica)
	nombre := "MARIA TEST"
	periodo := "2026-1"
	req := CreateInvoiceRequest{
		CompanyId:             "comp-1",
		PointOfSaleId:         "pos-1",
		CustomerId:            "cust-1",
		CodigoDocumentoSector: 1,
		NombreEstudiante:      &nombre,
		PeriodoFacturado:      &periodo,
		Items: []CreateInvoiceItemRequest{{
			Code:        "P001",
			Description: "Producto",
			Quantity:    1,
			UnitPrice:   100,
		}},
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
	uc := NewInvoiceUsecase(newFakeInvoiceRepo(), nil, nil, nil, &fakeCatalogRepo{}, cufdRepo, nil, siat.ModalidadElectronica)

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
	uc := NewInvoiceUsecase(newFakeInvoiceRepo(), nil, nil, nil, &fakeCatalogRepo{}, cufdRepo, nil, siat.ModalidadElectronica)

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

func TestListInvoices(t *testing.T) {
	repo := newFakeInvoiceRepo()
	uc := NewInvoiceUsecase(repo, nil, nil, nil, &fakeCatalogRepo{}, nil, nil, siat.ModalidadElectronica)

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
func createTestUsecaseBuilder() (*InvoiceUsecase, *fakeInvoiceRepo, *fakeCustomerRepo, *fakeProductRepository) {
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
		CodigoSistema:   "228452C38ED8739408AB6",
		Ambiente:        domain.EnvironmentPiloto,
		Municipio:       "LA PAZ",
		Direccion:       "AV. CAMACHO 123",
		CodigoActividad: strPtr("101010"),
	}}
	productRepo := &fakeProductRepository{product: &domain.Product{
		ID: "product-1", SKU: "SKU-001", Name: "Producto de prueba mapeado", Active: true,
		Mappings: []domain.ProductMapping{{
			ProductID: "product-1", CodigoProductoSin: 5113100, CodigoActividad: "101010",
			CodigoDocumentoSector: siat.SectorCompraVenta, UnidadMedida: 58, Active: true, SyncedAt: time.Now(),
		}},
	}}
	docSectorRepo := &fakeDocSectorRepo{items: []*domain.SiatActividadDocSector{
		{CodigoActividad: "101010", CodigoDocumentoSector: siat.SectorCompraVenta, TipoDocumentoSector: "FCV"},
	}}
	uc := NewInvoiceUsecase(repo, customerRepo, companyRepo, posRepo, &fakeCatalogRepo{}, nil, nil, siat.ModalidadElectronica, docSectorRepo)
	uc.productRepo = productRepo
	return uc, repo, customerRepo, productRepo
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
		Items: []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
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

func TestCreateReusaCustomerInline(t *testing.T) {
	uc, _, customerRepo, _ := createTestUsecaseBuilder()

	req := CreateInvoiceRequest{
		PointOfSaleId: "pos-1",
		Customer: &CreateInvoiceInlineCustomer{
			DocumentType:   "CI",
			DocumentNumber: "1234567",
			Name:           "OTRO NOMBRE",
		},
		Items: []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
	}

	inv, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if inv.CustomerId != "cust-1" {
		t.Errorf("CustomerId=%q, se esperaba reusar cust-1", inv.CustomerId)
	}
	// El nombre del cliente registrado prevalece: no se sincroniza con el
	// inline (el cliente es la fuente fiscal de verdad).
	if inv.Customer.Name != "Juan Perez" {
		t.Errorf("Customer.Name=%q, se esperaba el nombre del cliente existente", inv.Customer.Name)
	}
	if len(customerRepo.created) != 0 {
		t.Fatalf("no debió crear cliente; creados=%d", len(customerRepo.created))
	}
}

func TestCreateCompanyDerivadoDePOS(t *testing.T) {
	uc, _, _, _ := createTestUsecaseBuilder()

	req := CreateInvoiceRequest{
		PointOfSaleId: "pos-1",
		CustomerId:    "cust-1",
		Items:         []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
	}

	inv, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if inv.CompanyId != "comp-1" {
		t.Errorf("CompanyId=%q, se esperaba comp-1 derivado del POS", inv.CompanyId)
	}
}

func TestCreateDescriptionDefaultDesdeProducto(t *testing.T) {
	uc, _, _, _ := createTestUsecaseBuilder()

	req := CreateInvoiceRequest{
		PointOfSaleId: "pos-1",
		CustomerId:    "cust-1",
		Items:         []CreateInvoiceItemRequest{{SKU: "SKU-001", Quantity: 1, UnitPrice: 100}},
	}

	inv, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(inv.Items) != 1 || inv.Items[0].Description != "Producto de prueba mapeado" {
		t.Fatalf("descripción=%q, se esperaba default del producto", inv.Items[0].Description)
	}
	if inv.Items[0].Code != "SKU-001" {
		t.Errorf("código=%q, se esperaba SKU-001", inv.Items[0].Code)
	}
}

func TestCreateIdempotencia(t *testing.T) {
	uc, repo, _, _ := createTestUsecaseBuilder()

	req := CreateInvoiceRequest{
		PointOfSaleId:    "pos-1",
		CustomerId:       "cust-1",
		IdempotencyKey:   "orden-42",
		CodigoMetodoPago: 1,
		Items:            []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
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
		CustomerId:     "cust-1",
		IdempotencyKey: "orden-race",
		Items:          []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
	}
	ganadora := &domain.Invoice{
		ID:             "inv-ganador",
		CompanyId:      "comp-1",
		CustomerId:     strPtr("cust-1"),
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
		CustomerId:    "cust-1",
		Emit:          true,
		Items:         []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
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
		Items: []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
	}

	inv, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create with receiver: %v", err)
	}

	// Should not create a customer
	if len(customerRepo.created) != 0 {
		t.Fatalf("no debió crear cliente; creados=%d", len(customerRepo.created))
	}

	// CustomerId should be nil
	if inv.CustomerId != nil {
		t.Errorf("CustomerId=%q, se esperaba nil", *inv.CustomerId)
	}

	// Receiver snapshot should be persisted
	if inv.ReceiverName == nil || *inv.ReceiverName != "Juan Perez" {
		val := ""
		if inv.ReceiverName != nil {
			val = *inv.ReceiverName
		}
		t.Errorf("ReceiverName=%q", val)
	}
	if inv.ReceiverDocumentType == nil || *inv.ReceiverDocumentType != "CI" {
		val := ""
		if inv.ReceiverDocumentType != nil {
			val = *inv.ReceiverDocumentType
		}
		t.Errorf("ReceiverDocumentType=%q", val)
	}
	if inv.ReceiverDocument == nil || *inv.ReceiverDocument != "12345678" {
		val := ""
		if inv.ReceiverDocument != nil {
			val = *inv.ReceiverDocument
		}
		t.Errorf("ReceiverDocument=%q", val)
	}
	if inv.ReceiverEmail == nil || *inv.ReceiverEmail != "juan@email.com" {
		val := ""
		if inv.ReceiverEmail != nil {
			val = *inv.ReceiverEmail
		}
		t.Errorf("ReceiverEmail=%q", val)
	}
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
		Items: []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
	}

	inv, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create with receiver: %v", err)
	}

	// Should associate existing customer
	if inv.CustomerId == nil || *inv.CustomerId != "cust-existing" {
		val := ""
		if inv.CustomerId != nil {
			val = *inv.CustomerId
		}
		t.Errorf("CustomerId=%q, se esperaba cust-existing", val)
	}

	// Receiver snapshot should still use the request data (not the customer data)
	if inv.ReceiverName == nil || *inv.ReceiverName != "Juan Perez" {
		val := ""
		if inv.ReceiverName != nil {
			val = *inv.ReceiverName
		}
		t.Errorf("ReceiverName=%q", val)
	}
	if inv.ReceiverDocument == nil || *inv.ReceiverDocument != "12345678" {
		val := ""
		if inv.ReceiverDocument != nil {
			val = *inv.ReceiverDocument
		}
		t.Errorf("ReceiverDocument=%q", val)
	}
	if inv.ReceiverComplement != nil && *inv.ReceiverComplement != "" {
		val := *inv.ReceiverComplement
		t.Errorf("ReceiverComplement=%q, se esperaba vacío", val)
	}
}

func TestCreateValidationExactlyOneCustomerSource(t *testing.T) {
	uc, _, _, _ := createTestUsecaseBuilder()

	tests := []struct {
		name     string
		req      CreateInvoiceRequest
		wantErr  bool
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
			wantErr: true,
		},
		{
			name: "customer_id and receiver",
			req: CreateInvoiceRequest{
				PointOfSaleId: "pos-1",
				CustomerId:    "cust-1",
				Receiver:      &CreateInvoiceReceiver{DocumentType: 1, DocumentNumber: "123", Name: "Test"},
				Items:         []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
			},
			wantErr: true,
		},
		{
			name: "customer and receiver",
			req: CreateInvoiceRequest{
				PointOfSaleId: "pos-1",
				Customer:      &CreateInvoiceInlineCustomer{DocumentType: "CI", DocumentNumber: "123", Name: "Test"},
				Receiver:      &CreateInvoiceReceiver{DocumentType: 1, DocumentNumber: "123", Name: "Test"},
				Items:         []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
			},
			wantErr: true,
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
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.Create(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("error=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}
