package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/siat"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

func TestSectoresEndpointDevuelveCatalogo(t *testing.T) {
	h := NewInvoiceHandler(nil)
	r := chi.NewRouter()
	r.Get("/invoices/sectores", h.Sectores)

	req := httptest.NewRequest(http.MethodGet, "/invoices/sectores", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	var sectores []sectorDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &sectores); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if len(sectores) != len(siat.PerfilesSector()) {
		t.Fatalf("sectores=%d, se esperaban %d", len(sectores), len(siat.PerfilesSector()))
	}

	porCodigo := map[int]sectorDTO{}
	for _, s := range sectores {
		porCodigo[s.Codigo] = s
	}
	cv, ok := porCodigo[1]
	if !ok || cv.Ajuste || cv.TipoDocumento != 1 {
		t.Errorf("sector 1 mal serializado: %+v", cv)
	}
	nota, ok := porCodigo[24]
	if !ok || !nota.Ajuste || nota.TipoDocumento != 3 {
		t.Errorf("sector 24 mal serializado: %+v", nota)
	}
	conNombreEstudiante := false
	for _, c := range nota.Campos {
		if c.JSON == "numero_autorizacion_cuf" && c.Requerido {
			conNombreEstudiante = true
		}
	}
	if !conNombreEstudiante {
		t.Errorf("el sector 24 debe declarar numero_autorizacion_cuf requerido")
	}
}

func TestSectoresExponeSoportadoYEtiquetas(t *testing.T) {
	h := NewInvoiceHandler(nil)
	r := chi.NewRouter()
	r.Get("/invoices/sectores", h.Sectores)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/invoices/sectores", nil))

	var sectores []sectorDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &sectores); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	for _, s := range sectores {
		if !s.Soportado && s.Habilitado != nil {
			t.Fatalf("sector %d: habilitado solo aplica con company_id", s.Codigo)
		}
		for _, c := range s.Campos {
			if c.Etiqueta == "" {
				t.Fatalf("sector %d: campo %s sin etiqueta", s.Codigo, c.JSON)
			}
		}
	}
	// el sector 1 debe venir soportado
	porCodigo := map[int]sectorDTO{}
	for _, s := range sectores {
		porCodigo[s.Codigo] = s
	}
	if s := porCodigo[1]; !s.Soportado {
		t.Fatal("sector 1 debe estar soportado")
	}
	// un campo curado debe traer ejemplo
	if s := porCodigo[24]; len(s.Campos) > 0 && s.Campos[0].Ejemplo == "" {
		t.Fatal("los campos curados del sector 24 deben traer ejemplo")
	}
}

// ---- Mocks ----

type mockInvoiceService struct {
	createFunc         func(context.Context, usecase.CreateInvoiceRequest) (*domain.Invoice, error)
	getByIDFunc        func(string) (*domain.Invoice, error)
	listInvoicesFunc   func(domain.InvoiceListFilter) ([]*domain.Invoice, int64, error)
	emitFunc           func(context.Context, string) (*domain.Invoice, error)
	verifyStatusFunc   func(context.Context, string) (*domain.Invoice, error)
	annulFunc          func(context.Context, string, int) (*domain.Invoice, error)
	revertAnnulFunc    func(context.Context, string) (*domain.Invoice, error)
	sectoresFunc       func(string) (map[int]bool, error)
}

func (m *mockInvoiceService) Create(ctx context.Context, req usecase.CreateInvoiceRequest) (*domain.Invoice, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, req)
	}
	return sampleInvoice(), nil
}
func (m *mockInvoiceService) GetByID(id string) (*domain.Invoice, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(id)
	}
	return sampleInvoice(), nil
}
func (m *mockInvoiceService) ListInvoices(filter domain.InvoiceListFilter) ([]*domain.Invoice, int64, error) {
	if m.listInvoicesFunc != nil {
		return m.listInvoicesFunc(filter)
	}
	return []*domain.Invoice{sampleInvoice()}, 1, nil
}
func (m *mockInvoiceService) Emit(ctx context.Context, id string) (*domain.Invoice, error) {
	if m.emitFunc != nil {
		return m.emitFunc(ctx, id)
	}
	return sampleInvoice(), nil
}
func (m *mockInvoiceService) VerifyStatus(ctx context.Context, id string) (*domain.Invoice, error) {
	if m.verifyStatusFunc != nil {
		return m.verifyStatusFunc(ctx, id)
	}
	return sampleInvoice(), nil
}
func (m *mockInvoiceService) Annul(ctx context.Context, id string, codigoMotivo int) (*domain.Invoice, error) {
	if m.annulFunc != nil {
		return m.annulFunc(ctx, id, codigoMotivo)
	}
	return sampleInvoice(), nil
}
func (m *mockInvoiceService) RevertAnnul(ctx context.Context, id string) (*domain.Invoice, error) {
	if m.revertAnnulFunc != nil {
		return m.revertAnnulFunc(ctx, id)
	}
	return sampleInvoice(), nil
}
func (m *mockInvoiceService) SectoresHabilitados(companyID string) (map[int]bool, error) {
	if m.sectoresFunc != nil {
		return m.sectoresFunc(companyID)
	}
	return map[int]bool{1: true}, nil
}

func sampleInvoice() *domain.Invoice {
	now := time.Now()
	siatMensajes := "[{\"codigo\":1007,\"descripcion\":\"DIRECCION NO CORRESPONDE A PADRON\"}]"
	cuf := "CUF-123"
	xml := "<xml/>"
	return &domain.Invoice{
		ID:            "inv-1",
		CompanyId:     "comp-1",
		CustomerId:    "cust-1",
		PointOfSaleId: "pos-1",
		CufdId:        "cufd-1",
		InvoiceNumber: 42,
		Status:        domain.InvoiceAccepted,
		Cuf:           &cuf,
		Subtotal:      100,
		Total:         100,
		IssueDate:     now,
		CreatedAt:     now,
		Customer: domain.Customer{
			ID:             "cust-1",
			DocumentType:   "CI",
			DocumentNumber: "1234567",
			Name:           "Juan Perez",
		},
		Company: domain.Company{
			ID:           "comp-1",
			Nit:          "9971522011",
			BusinessName: "EMPRESA PILOTO SRL",
		},
		PointOfSale: domain.PointOfSale{
			ID:               "pos-1",
			CodigoSucursal:   0,
			CodigoPuntoVenta: 3,
		},
		CufdRecord: domain.Cufd{
			ID:  "cufd-1",
			Cufd: "CUFD-XYZ",
		},
		Xml:          &xml,
		XmlHash:      strPtr("hash-1"),
		SiatMensajes: &siatMensajes,
		Items: []domain.InvoiceItem{{
			ID:          "item-1",
			Code:        "P001",
			Description: "Producto",
			Quantity:    1,
			UnitPrice:   100,
			Subtotal:    100,
		}},
	}
}

func strPtr(s string) *string { return &s }

// ---- Tests ----

func TestCreateReturnsSlimDTO(t *testing.T) {
	svc := &mockInvoiceService{}
	h := NewInvoiceHandler(svc)
	r := chi.NewRouter()
	r.Post("/invoices", h.Create)

	body := `{"point_of_sale_id":"pos-1","customer_id":"cust-1","items":[{"code":"P001","description":"Producto","quantity":1,"unit_price":100}]}`
	req := httptest.NewRequest(http.MethodPost, "/invoices", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d, se esperaba 201", rec.Code)
	}
	var dto invoiceDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if dto.ID != "inv-1" {
		t.Errorf("id=%q", dto.ID)
	}
	if dto.Xml != nil {
		t.Error("respuesta slim no debe incluir xml por defecto")
	}
	if dto.Company != nil {
		t.Error("respuesta slim no debe incluir company por defecto")
	}
	if dto.Customer.Name == "" {
		t.Error("customer mínimo debe incluir name")
	}
	if len(dto.SiatMensajes) == 0 || dto.SiatMensajes[0].Codigo != 1007 {
		t.Error("siat_mensajes debe venir parseado como array")
	}
}

func TestCreateWithInclude(t *testing.T) {
	svc := &mockInvoiceService{}
	h := NewInvoiceHandler(svc)
	r := chi.NewRouter()
	r.Post("/invoices", h.Create)

	body := `{"point_of_sale_id":"pos-1","customer_id":"cust-1","items":[{"code":"P001","description":"Producto","quantity":1,"unit_price":100}]}`
	req := httptest.NewRequest(http.MethodPost, "/invoices?include=xml,company", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d, se esperaba 201", rec.Code)
	}
	var dto invoiceDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if dto.Xml == nil || *dto.Xml != "<xml/>" {
		t.Error("?include=xml no expuso el xml")
	}
	if dto.Company == nil || dto.Company.ID != "comp-1" {
		t.Error("?include=company no expuso la empresa")
	}
}

func TestCreateIdempotencyHeader(t *testing.T) {
	svc := &mockInvoiceService{}
	h := NewInvoiceHandler(svc)
	r := chi.NewRouter()
	r.Post("/invoices", h.Create)

	body := `{"point_of_sale_id":"pos-1","customer_id":"cust-1","items":[{"code":"P001","description":"Producto","quantity":1,"unit_price":100}]}`
	req := httptest.NewRequest(http.MethodPost, "/invoices", strings.NewReader(body))
	req.Header.Set("Idempotency-Key", "orden-123")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200 con Idempotency-Key", rec.Code)
	}
}

func TestCreateEmitTrue(t *testing.T) {
	emitted := sampleInvoice()
	emitted.Status = domain.InvoiceAccepted
	var emitCalled bool
	svc := &mockInvoiceService{
		createFunc: func(_ context.Context, _ usecase.CreateInvoiceRequest) (*domain.Invoice, error) {
			created := sampleInvoice()
			created.Status = domain.InvoicePending
			return created, nil
		},
		emitFunc: func(_ context.Context, id string) (*domain.Invoice, error) {
			emitCalled = true
			if id != "inv-1" {
				t.Errorf("emit id=%q, se esperaba inv-1", id)
			}
			return emitted, nil
		},
	}
	h := NewInvoiceHandler(svc)
	r := chi.NewRouter()
	r.Post("/invoices", h.Create)

	body := `{"point_of_sale_id":"pos-1","customer_id":"cust-1","emit":true,"items":[{"code":"P001","description":"Producto","quantity":1,"unit_price":100}]}`
	req := httptest.NewRequest(http.MethodPost, "/invoices", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d, se esperaba 201", rec.Code)
	}
	if !emitCalled {
		t.Fatal("Emit no fue invocado")
	}
	var dto invoiceDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if dto.Status != domain.InvoiceAccepted {
		t.Errorf("status=%s", dto.Status)
	}
}

func TestEmitRechazoIncluyeInvoiceID(t *testing.T) {
	svc := &mockInvoiceService{
		emitFunc: func(_ context.Context, id string) (*domain.Invoice, error) {
			return nil, &usecase.EmissionRejectedError{
				CodigoEstado: 902,
				Mensajes:     []siat.Mensaje{{Codigo: 123, Descripcion: "rechazo"}},
			}
		},
	}
	h := NewInvoiceHandler(svc)
	r := chi.NewRouter()
	r.Post("/invoices/{id}/emit", h.Emit)

	req := httptest.NewRequest(http.MethodPost, "/invoices/inv-42/emit", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d, se esperaba 422", rec.Code)
	}
	var env errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if env.Error.InvoiceID != "inv-42" {
		t.Errorf("invoice_id=%q, se esperaba inv-42", env.Error.InvoiceID)
	}
}

func TestDownloadXML(t *testing.T) {
	svc := &mockInvoiceService{}
	h := NewInvoiceHandler(svc)
	r := chi.NewRouter()
	r.Get("/invoices/{id}/xml", h.DownloadXML)

	req := httptest.NewRequest(http.MethodGet, "/invoices/inv-1/xml", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/xml" {
		t.Errorf("Content-Type=%q", ct)
	}
	if !strings.Contains(rec.Body.String(), "<xml/>") {
		t.Errorf("body=%q", rec.Body.String())
	}
}

func TestCreateEmitIdempotencyReplay(t *testing.T) {
	var emitCalls int
	svc := &mockInvoiceService{
		createFunc: func(_ context.Context, _ usecase.CreateInvoiceRequest) (*domain.Invoice, error) {
			inv := sampleInvoice()
			if emitCalls > 0 {
				inv.Status = domain.InvoiceAccepted
			} else {
				inv.Status = domain.InvoicePending
			}
			return inv, nil
		},
		emitFunc: func(_ context.Context, id string) (*domain.Invoice, error) {
			emitCalls++
			emitted := sampleInvoice()
			emitted.Status = domain.InvoiceAccepted
			return emitted, nil
		},
	}
	h := NewInvoiceHandler(svc)
	r := chi.NewRouter()
	r.Post("/invoices", h.Create)

	body := `{"point_of_sale_id":"pos-1","customer_id":"cust-1","emit":true,"items":[{"code":"P001","description":"Producto","quantity":1,"unit_price":100}]}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/invoices", strings.NewReader(body))
		req.Header.Set("Idempotency-Key", "orden-emit-1")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status=%d, se esperaba 200", i+1, rec.Code)
		}
	}
	if emitCalls != 1 {
		t.Errorf("emitCalls=%d, se esperaba 1 (no debe reemitir en replay)", emitCalls)
	}
}

func TestGetByIDExponeCamposSectoriales(t *testing.T) {
	inv := sampleInvoice()
	nombre := "María Fernanda Quispe"
	periodo := "GESTION 1/2026"
	sectorData := json.RawMessage(`{"nombre_estudiante":"María"}`)
	modalidad := 2
	refID := "inv-original-1"
	eventID := "evt-1"
	cufdID := "cufd-9"
	inv.CodigoDocumentoSector = 11
	inv.NombreEstudiante = &nombre
	inv.PeriodoFacturado = &periodo
	inv.SectorData = sectorData
	inv.Modalidad = modalidad
	inv.AjustaFacturaId = &refID
	inv.ContingencyEventId = &eventID
	inv.CufdId = cufdID

	svc := &mockInvoiceService{
		getByIDFunc: func(id string) (*domain.Invoice, error) {
			return inv, nil
		},
	}
	h := NewInvoiceHandler(svc)
	r := chi.NewRouter()
	r.Get("/invoices/{id}", h.GetByID)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/invoices/inv-1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	var dto invoiceDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if dto.NombreEstudiante == nil || *dto.NombreEstudiante != nombre {
		t.Error("nombre_estudiante no expuesto")
	}
	if dto.PeriodoFacturado == nil || *dto.PeriodoFacturado != periodo {
		t.Error("periodo_facturado no expuesto")
	}
	if len(dto.SectorData) == 0 {
		t.Error("sector_data no expuesto")
	}
	if dto.Modalidad != modalidad {
		t.Errorf("modalidad=%d", dto.Modalidad)
	}
	if dto.AjustaFacturaId == nil || *dto.AjustaFacturaId != refID {
		t.Error("ajusta_factura_id no expuesto")
	}
	if dto.ContingencyEventId == nil || *dto.ContingencyEventId != eventID {
		t.Error("contingency_event_id no expuesto")
	}
	if dto.CufdId != cufdID || dto.CompanyId != "comp-1" || dto.PointOfSaleId != "pos-1" || dto.CustomerId != "cust-1" {
		t.Errorf("referencias incompletas: company=%q pos=%q customer=%q cufd=%q",
			dto.CompanyId, dto.PointOfSaleId, dto.CustomerId, dto.CufdId)
	}
}

func TestCreateRejectsOversizedIdempotencyKey(t *testing.T) {
	svc := &mockInvoiceService{}
	h := NewInvoiceHandler(svc)
	r := chi.NewRouter()
	r.Post("/invoices", h.Create)

	body := bytes.NewBufferString(`{"point_of_sale_id":"pos-1","customer_id":"cust-1","items":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/invoices", body)
	req.Header.Set("Idempotency-Key", strings.Repeat("x", 101))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, se esperaba 400", rec.Code)
	}
}
