package invoice

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/adapters/siat"
	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
	"github.com/brandsrx/supay/internal/storage"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type fileRepoForHandler struct{ file *domain.InvoiceFile }

func (r *fileRepoForHandler) BelongsToCompany(_ context.Context, companyID, invoiceID string) (bool, error) {
	return companyID == "company1" && invoiceID == "invoice1", nil
}
func (r *fileRepoForHandler) CreateFile(_ context.Context, file *domain.InvoiceFile) error {
	r.file = file
	return nil
}
func (r *fileRepoForHandler) FindFile(_ context.Context, _, _, _ string) (*domain.InvoiceFile, error) {
	if r.file == nil {
		return nil, domain.ErrNotFound
	}
	return r.file, nil
}
func (r *fileRepoForHandler) DeleteFile(_ context.Context, _, _, _, storageKey string) error {
	if r.file != nil && r.file.StorageKey == storageKey {
		r.file = nil
	}
	return nil
}

func TestDownloadXMLStreamsOnlyOwnCompany(t *testing.T) {
	objects := storage.NewMemoryObjectStorage()
	files := usecase.NewInvoiceFileService(objects, &fileRepoForHandler{}, time.Minute)
	_, err := files.Save(context.Background(), "company1", "invoice1", "CUF", "xml", strings.NewReader("<signed/>"), 9)
	if err != nil {
		t.Fatal(err)
	}
	h := newHandler(&mockInvoiceService{})
	h.files = files
	router := chi.NewRouter()
	router.Get("/invoices/{id}/xml", h.downloadXML)
	for _, tc := range []struct {
		company string
		status  int
		body    string
	}{{"other", http.StatusNotFound, ""}, {"company1", http.StatusOK, "<signed/>"}} {
		req := httptest.NewRequest(http.MethodGet, "/invoices/invoice1/xml", nil)
		req = req.WithContext(deliveryHttp.WithCompanyID(req.Context(), tc.company))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Fatalf("%s: status %d", tc.company, w.Code)
		}
		if tc.body != "" && w.Body.String() != tc.body {
			t.Fatalf("%s: body %q", tc.company, w.Body.String())
		}
	}
}

func TestMissingXMLDoesNotFallbackToDatabase(t *testing.T) {
	xml := "<legacy-database-copy/>"
	h := newHandler(&mockInvoiceService{getByIDFunc: func(string) (*domain.Invoice, error) { return &domain.Invoice{CompanyId: "company1", Xml: &xml}, nil }})
	h.files = usecase.NewInvoiceFileService(storage.NewMemoryObjectStorage(), &fileRepoForHandler{}, time.Minute)
	router := chi.NewRouter()
	router.Get("/invoices/{id}/xml", h.downloadXML)
	for _, tc := range []struct {
		company string
		status  int
	}{{"company1", http.StatusNotFound}, {"other", http.StatusNotFound}} {
		req := httptest.NewRequest(http.MethodGet, "/invoices/invoice1/xml", nil)
		req = req.WithContext(deliveryHttp.WithCompanyID(req.Context(), tc.company))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Fatalf("%s: status %d", tc.company, w.Code)
		}
	}
}

type pdfGeneratorForHandler struct {
	files *usecase.InvoiceFileService
	calls int
}

func (g *pdfGeneratorForHandler) GenerateInvoicePDFWithContext(ctx context.Context, id string) ([]byte, error) {
	g.calls++
	_, err := g.files.Save(ctx, "company1", id, "CUF", "pdf", strings.NewReader("%PDF"), 4)
	return []byte("%PDF"), err
}

func TestPDFGenerationFallbackChecksCompany(t *testing.T) {
	files := usecase.NewInvoiceFileService(storage.NewMemoryObjectStorage(), &fileRepoForHandler{}, time.Minute)
	cuf := "CUF"
	h := newHandler(&mockInvoiceService{getByIDFunc: func(string) (*domain.Invoice, error) { return &domain.Invoice{CompanyId: "company1", Cuf: &cuf}, nil }})
	h.files = files
	generator := &pdfGeneratorForHandler{files: files}
	h.pdfGenerator = generator
	router := chi.NewRouter()
	router.Get("/invoices/{id}/pdf", h.downloadPDF)
	for _, tc := range []struct {
		company string
		status  int
	}{{"other", http.StatusNotFound}, {"company1", http.StatusOK}} {
		req := httptest.NewRequest(http.MethodGet, "/invoices/invoice1/pdf", nil)
		req = req.WithContext(deliveryHttp.WithCompanyID(req.Context(), tc.company))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Fatalf("%s: status %d", tc.company, w.Code)
		}
		if tc.status == http.StatusOK && w.Body.String() != "%PDF" {
			t.Fatalf("pdf body: %q", w.Body.String())
		}
	}
	if generator.calls != 1 {
		t.Fatalf("generation called %d times", generator.calls)
	}
}

func TestSectoresEndpointDevuelveCatalogo(t *testing.T) {
	h := newHandler(nil)
	r := chi.NewRouter()
	r.Get("/invoices/sectores", h.sectores)

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
	h := newHandler(nil)
	r := chi.NewRouter()
	r.Get("/invoices/sectores", h.sectores)

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
	createFunc        func(context.Context, usecase.CreateInvoiceRequest) (*domain.Invoice, error)
	createSimpleFunc  func(context.Context, usecase.MinimalInvoiceRequest, string) (*domain.Invoice, error)
	previewSimpleFunc func(context.Context, usecase.MinimalInvoiceRequest) (*usecase.InvoicePreview, error)
	emitSimpleFunc    func(context.Context, usecase.MinimalInvoiceRequest, string) (*domain.Invoice, error)
	getByIDFunc       func(string) (*domain.Invoice, error)
	listInvoicesFunc  func(domain.InvoiceListFilter) ([]*domain.Invoice, int64, error)
	emitFunc          func(context.Context, string) (*domain.Invoice, error)
	verifyStatusFunc  func(context.Context, string) (*domain.Invoice, error)
	annulFunc         func(context.Context, string, int) (*domain.Invoice, error)
	revertAnnulFunc   func(context.Context, string) (*domain.Invoice, error)
	sectoresFunc      func(string) (map[int]bool, error)
}

func (m *mockInvoiceService) Create(ctx context.Context, req usecase.CreateInvoiceRequest) (*domain.Invoice, error) {
	if m.createFunc != nil {
		return m.createFunc(ctx, req)
	}
	return sampleInvoice(), nil
}
func (m *mockInvoiceService) CreateSimplified(ctx context.Context, req usecase.MinimalInvoiceRequest, key string) (*domain.Invoice, error) {
	if m.createSimpleFunc != nil {
		return m.createSimpleFunc(ctx, req, key)
	}
	return sampleInvoice(), nil
}
func (m *mockInvoiceService) PreviewSimplified(ctx context.Context, req usecase.MinimalInvoiceRequest) (*usecase.InvoicePreview, error) {
	if m.previewSimpleFunc != nil {
		return m.previewSimpleFunc(ctx, req)
	}
	return &usecase.InvoicePreview{PointOfSaleID: req.PointOfSaleID, Total: 100}, nil
}
func (m *mockInvoiceService) EmitSimplified(ctx context.Context, req usecase.MinimalInvoiceRequest, key string) (*domain.Invoice, error) {
	if m.emitSimpleFunc != nil {
		return m.emitSimpleFunc(ctx, req, key)
	}
	inv := sampleInvoice()
	inv.Status = domain.InvoicePending
	return inv, nil
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
			ID:   "cufd-1",
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
	h := newHandler(svc)
	r := chi.NewRouter()
	r.Post("/invoices", h.create)

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
	h := newHandler(svc)
	r := chi.NewRouter()
	r.Post("/invoices", h.create)

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
	h := newHandler(svc)
	r := chi.NewRouter()
	r.Post("/invoices", h.create)

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
	h := newHandler(svc)
	r := chi.NewRouter()
	r.Post("/invoices", h.create)

	body := `{"point_of_sale_id":"pos-1","customer_id":"cust-1","emit":true,"items":[{"code":"P001","description":"Producto","quantity":1,"unit_price":100}]}`
	req := httptest.NewRequest(http.MethodPost, "/invoices", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
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
	if location := rec.Header().Get("Location"); location != "/invoices/inv-1" {
		t.Errorf("Location=%q", location)
	}
}

func TestEmitRechazoIncluyeInvoiceID(t *testing.T) {
	svc := &mockInvoiceService{
		emitFunc: func(_ context.Context, id string) (*domain.Invoice, error) {
			return nil, &usecase.EmissionRejectedError{
				CodigoEstado: 902,
				Mensajes:     []ports.FiscalMessage{{Codigo: 123, Descripcion: "rechazo"}},
			}
		},
	}
	h := newHandler(svc)
	r := chi.NewRouter()
	r.Post("/invoices/{id}/emit", h.emit)

	req := httptest.NewRequest(http.MethodPost, "/invoices/inv-42/emit", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d, se esperaba 422", rec.Code)
	}
	type errorEnvelope struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			InvoiceID string `json:"invoice_id"`
		} `json:"error"`
	}
	var env errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if env.Error.InvoiceID != "inv-42" {
		t.Errorf("invoice_id=%q, se esperaba inv-42", env.Error.InvoiceID)
	}
}

func TestEmitDevuelveResultadoSincrono(t *testing.T) {
	emitted := sampleInvoice()
	emitted.Status = domain.InvoiceAccepted
	h := newHandler(&mockInvoiceService{emitFunc: func(context.Context, string) (*domain.Invoice, error) {
		return emitted, nil
	}})
	r := chi.NewRouter()
	r.Post("/invoices/{id}/emit", h.emit)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/invoices/inv-1/emit", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "/invoices/inv-1" {
		t.Fatalf("Location=%q", location)
	}
}

func TestDownloadXML(t *testing.T) {
	svc := &mockInvoiceService{}
	h := newHandler(svc)
	objects := storage.NewMemoryObjectStorage()
	h.files = usecase.NewInvoiceFileService(objects, &fileRepoForHandler{}, time.Minute)
	if _, err := h.files.Save(context.Background(), "company1", "invoice1", "CUF", "xml", strings.NewReader("<xml/>"), 6); err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	r.Get("/invoices/{id}/xml", h.downloadXML)

	req := httptest.NewRequest(http.MethodGet, "/invoices/invoice1/xml", nil)
	req = req.WithContext(deliveryHttp.WithCompanyID(req.Context(), "company1"))
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
	h := newHandler(svc)
	r := chi.NewRouter()
	r.Post("/invoices", h.create)

	body := `{"point_of_sale_id":"pos-1","customer_id":"cust-1","emit":true,"items":[{"code":"P001","description":"Producto","quantity":1,"unit_price":100}]}`
	expectedStatuses := []int{http.StatusOK, http.StatusOK}
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/invoices", strings.NewReader(body))
		req.Header.Set("Idempotency-Key", "orden-emit-1")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != expectedStatuses[i] {
			t.Fatalf("request %d: status=%d, se esperaba %d", i+1, rec.Code, expectedStatuses[i])
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
	h := newHandler(svc)
	r := chi.NewRouter()
	r.Get("/invoices/{id}", h.getByID)

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
	custVal := dto.CustomerId
	if dto.CufdId != cufdID || dto.CompanyId != "comp-1" || dto.PointOfSaleId != "pos-1" || custVal != "cust-1" {
		t.Errorf("referencias incompletas: company=%q pos=%q customer=%q cufd=%q",
			dto.CompanyId, dto.PointOfSaleId, custVal, dto.CufdId)
	}
}

func TestCreateRejectsOversizedIdempotencyKey(t *testing.T) {
	svc := &mockInvoiceService{}
	h := newHandler(svc)
	r := chi.NewRouter()
	r.Post("/invoices", h.create)

	body := bytes.NewBufferString(`{"point_of_sale_id":"pos-1","customer_id":"cust-1","items":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/invoices", body)
	req.Header.Set("Idempotency-Key", strings.Repeat("x", 101))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, se esperaba 400", rec.Code)
	}
}

func TestV1PreviewUsaContratoMinimo(t *testing.T) {
	var captured usecase.MinimalInvoiceRequest
	svc := &mockInvoiceService{previewSimpleFunc: func(_ context.Context, req usecase.MinimalInvoiceRequest) (*usecase.InvoicePreview, error) {
		captured = req
		return &usecase.InvoicePreview{PointOfSaleID: req.PointOfSaleID, CodigoDocumentoSector: 1, Total: 25}, nil
	}}
	h := newHandler(svc)
	r := chi.NewRouter()
	r.Post("/v1/invoices/preview", h.previewV1)

	body := `{"point_of_sale_id":"pos-1","customer":{"document_number":"123"},"items":[{"sku":"SKU-1","quantity":2,"price":12.5}],"invoice_type":"venta","sector":"auto"}`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/invoices/preview", strings.NewReader(body)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if captured.PointOfSaleID != "pos-1" || len(captured.Items) != 1 || captured.Items[0].Price != 12.5 {
		t.Fatalf("payload mínimo no propagado: %+v", captured)
	}
}

func TestRegisterV1RoutesExponePreviewSinSlashFinal(t *testing.T) {
	svc := &mockInvoiceService{}
	module := &Module{h: newHandler(svc)}
	r := chi.NewRouter()
	r.Route("/v1/invoices", module.RegisterV1Routes)

	body := `{"point_of_sale_id":"pos-1","customer":{"id":"cust-1"},"items":[{"sku":"SKU-1","quantity":1,"price":100}]}`
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/invoices/preview", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestV1DecodesAdjustmentOptionsAndRejectsPaymentTypos(t *testing.T) {
	body := `{"point_of_sale_id":"pos-1","customer":{"id":"cust-1"},"items":[{"sku":"SKU-1","quantity":1,"price":100,"data":{"marca_ice":1}}],"sector":"24","layout":"nota_credito_debito","reference_invoice_id":"original-1","payment":{"method_code":1,"currency_code":2,"exchange_rate":6.96}}`
	req, err := decodeMinimalInvoice(httptest.NewRequest(http.MethodPost, "/v1/invoices", strings.NewReader(body)))
	if err != nil {
		t.Fatal(err)
	}
	if req.ReferenceInvoiceID == nil || *req.ReferenceInvoiceID != "original-1" || req.Layout != "nota_credito_debito" || req.Payment == nil || req.Payment.ExchangeRate != 6.96 {
		t.Fatalf("lost options: %+v", req)
	}
	typo := strings.Replace(body, "exchange_rate", "exhange_rate", 1)
	if _, err := decodeMinimalInvoice(httptest.NewRequest(http.MethodPost, "/v1/invoices", strings.NewReader(typo))); err == nil {
		t.Fatal("unknown payment field accepted")
	}
}

func TestV1EmitDevuelveResultadoSincrono(t *testing.T) {
	var capturedKey string
	svc := &mockInvoiceService{emitSimpleFunc: func(_ context.Context, _ usecase.MinimalInvoiceRequest, key string) (*domain.Invoice, error) {
		capturedKey = key
		inv := sampleInvoice()
		inv.Status = domain.InvoiceAccepted
		return inv, nil
	}}
	h := newHandler(svc)
	r := chi.NewRouter()
	r.Post("/v1/invoices/emit", h.emitV1)

	body := `{"point_of_sale_id":"pos-1","customer":{"id":"cust-1"},"items":[{"sku":"SKU-1","quantity":1,"price":100}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/invoices/emit", strings.NewReader(body))
	req.Header.Set("Idempotency-Key", "sale-42")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if capturedKey != "sale-42" {
		t.Fatalf("idempotency key=%q", capturedKey)
	}
	if got := rec.Header().Get("Location"); got != "/v1/invoices/inv-1" {
		t.Fatalf("Location=%q", got)
	}
}

func TestV1RechazaCamposFueraDelContrato(t *testing.T) {
	h := newHandler(&mockInvoiceService{})
	r := chi.NewRouter()
	r.Post("/v1/invoices", h.createV1)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/invoices", strings.NewReader(`{"point_of_sale_id":"pos-1","customer":{},"items":[],"codigo_moneda":1}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, se esperaba 400", rec.Code)
	}
}
