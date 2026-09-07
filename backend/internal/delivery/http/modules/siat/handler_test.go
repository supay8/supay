package siat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type mockSiatService struct {
	solicitarCUISFunc                func(context.Context, string, string) (*usecase.CuisResultado, error)
	solicitarCUFDFunc                func(context.Context, string, string) (*usecase.CufdResultado, error)
	registrarEventoFunc              func(context.Context, string, string, usecase.EventoSignificativoInput) (*usecase.EventoSignificativoResultado, error)
	enviarPaqueteFunc                func(context.Context, string, string, usecase.PaqueteInput) (*usecase.PaqueteResultado, error)
	validarPaqueteFunc               func(context.Context, string, string, usecase.PaqueteValidacionInput) (*usecase.PaqueteResultado, error)
	enviarMasivaFunc                 func(context.Context, string, string, usecase.MasivaInput) (*usecase.PaqueteResultado, error)
	validarMasivaFunc                func(context.Context, string, string, usecase.PaqueteValidacionInput) (*usecase.PaqueteResultado, error)
	enviarComprasFunc                func(context.Context, string, string, usecase.ComprasInput) (*usecase.ComprasResultado, error)
	firmarFacturaFunc                func(context.Context, string, string, usecase.FirmaInput) (*usecase.FirmaResultado, error)
	setupFunc                        func(context.Context, string, string) (*usecase.SetupResultado, error)
	sincronizarFunc                  func(context.Context, string, string, string) (*usecase.SincronizacionResultado, error)
	buildSincronizacionResumenFunc   func(string, string, *usecase.SincronizacionResultado) *usecase.SincronizacionResumen
	listSinProductsFunc              func(string, string, int, int) ([]*domain.SinProduct, int64, error)
	listActivitesDocumentSectorsFunc func(string, string, int, int) ([]*domain.SiatActividadDocSector, int64, error)
	catalogReadinessFunc             func(string, string) (*domain.CatalogReadiness, error)
	listCatalogFunc                  func(string, string) (any, error)
	emitirDocumentoAjusteFunc        func(context.Context, string, string, usecase.DocumentoAjusteInput) (*usecase.DocumentoAjusteResultado, error)
}

func responseObjectID(response map[string]any, key string) string {
	object, _ := response[key].(map[string]any)
	id, _ := object["id"].(string)
	return id
}

func (m *mockSiatService) SolicitarCUIS(ctx context.Context, companyID, posID string) (*usecase.CuisResultado, error) {
	if m.solicitarCUISFunc != nil {
		return m.solicitarCUISFunc(ctx, companyID, posID)
	}
	return &usecase.CuisResultado{Success: true, Data: &ports.CuisResult{Codigo: "CUIS-123", FechaVigencia: time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)}}, nil
}

func (m *mockSiatService) SolicitarCUFD(ctx context.Context, companyID, posID string) (*usecase.CufdResultado, error) {
	if m.solicitarCUFDFunc != nil {
		return m.solicitarCUFDFunc(ctx, companyID, posID)
	}
	comp := &domain.Company{ID: "comp-1", Nit: "123456789", BusinessName: "Test"}
	pos := &domain.PointOfSale{ID: "pos-1", Description: "POS 1"}
	return &usecase.CufdResultado{
		Company:     comp,
		PointOfSale: pos,
		Response:    &ports.CufdResult{Transaccion: true, Codigo: "CUFD-123", FechaVigencia: time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC), CodigoControl: "CTRL-123"},
	}, nil
}

func (m *mockSiatService) RegistrarEventoSignificativo(ctx context.Context, companyID, posID string, body usecase.EventoSignificativoInput) (*usecase.EventoSignificativoResultado, error) {
	if m.registrarEventoFunc != nil {
		return m.registrarEventoFunc(ctx, companyID, posID, body)
	}
	comp := &domain.Company{ID: "comp-1"}
	pos := &domain.PointOfSale{ID: "pos-1"}
	return &usecase.EventoSignificativoResultado{
		Company:     comp,
		PointOfSale: pos,
		Response:    &ports.FiscalEventResult{Transaccion: true, CodigoRecepcion: "RCPT-123"},
	}, nil
}

func (m *mockSiatService) EnviarPaquete(ctx context.Context, companyID, posID string, body usecase.PaqueteInput) (*usecase.PaqueteResultado, error) {
	if m.enviarPaqueteFunc != nil {
		return m.enviarPaqueteFunc(ctx, companyID, posID, body)
	}
	comp := &domain.Company{ID: "comp-1"}
	pos := &domain.PointOfSale{ID: "pos-1"}
	return &usecase.PaqueteResultado{
		Company:     comp,
		PointOfSale: pos,
		Response:    &ports.FiscalPackageResult{Transaccion: true, CodigoRecepcion: "RCPT-123", CantidadFacturas: 1},
	}, nil
}

func (m *mockSiatService) ValidarPaquete(ctx context.Context, companyID, posID string, body usecase.PaqueteValidacionInput) (*usecase.PaqueteResultado, error) {
	if m.validarPaqueteFunc != nil {
		return m.validarPaqueteFunc(ctx, companyID, posID, body)
	}
	comp := &domain.Company{ID: "comp-1"}
	pos := &domain.PointOfSale{ID: "pos-1"}
	return &usecase.PaqueteResultado{
		Company:     comp,
		PointOfSale: pos,
		Response:    &ports.FiscalPackageResult{Transaccion: true, CodigoRecepcion: "RCPT-123", CantidadFacturas: 1},
	}, nil
}

func (m *mockSiatService) EnviarMasiva(ctx context.Context, companyID, posID string, body usecase.MasivaInput) (*usecase.PaqueteResultado, error) {
	if m.enviarMasivaFunc != nil {
		return m.enviarMasivaFunc(ctx, companyID, posID, body)
	}
	comp := &domain.Company{ID: "comp-1"}
	pos := &domain.PointOfSale{ID: "pos-1"}
	return &usecase.PaqueteResultado{
		Company:     comp,
		PointOfSale: pos,
		Response:    &ports.FiscalPackageResult{Transaccion: true, CodigoRecepcion: "RCPT-123", CantidadFacturas: 1},
	}, nil
}

func (m *mockSiatService) ValidarMasiva(ctx context.Context, companyID, posID string, body usecase.PaqueteValidacionInput) (*usecase.PaqueteResultado, error) {
	if m.validarMasivaFunc != nil {
		return m.validarMasivaFunc(ctx, companyID, posID, body)
	}
	comp := &domain.Company{ID: "comp-1"}
	pos := &domain.PointOfSale{ID: "pos-1"}
	return &usecase.PaqueteResultado{
		Company:     comp,
		PointOfSale: pos,
		Response:    &ports.FiscalPackageResult{Transaccion: true, CodigoRecepcion: "RCPT-123", CantidadFacturas: 1},
	}, nil
}

func (m *mockSiatService) EnviarCompras(ctx context.Context, companyID, posID string, body usecase.ComprasInput) (*usecase.ComprasResultado, error) {
	if m.enviarComprasFunc != nil {
		return m.enviarComprasFunc(ctx, companyID, posID, body)
	}
	comp := &domain.Company{ID: "comp-1"}
	pos := &domain.PointOfSale{ID: "pos-1"}
	return &usecase.ComprasResultado{
		Company:     comp,
		PointOfSale: pos,
		Response:    &ports.FiscalPurchaseResult{Transaccion: true, CodigoRecepcion: "RCPT-123"},
	}, nil
}

func (m *mockSiatService) FirmarFactura(ctx context.Context, companyID, posID string, body usecase.FirmaInput) (*usecase.FirmaResultado, error) {
	if m.firmarFacturaFunc != nil {
		return m.firmarFacturaFunc(ctx, companyID, posID, body)
	}
	comp := &domain.Company{ID: "comp-1"}
	pos := &domain.PointOfSale{ID: "pos-1"}
	return &usecase.FirmaResultado{
		Company:     comp,
		PointOfSale: pos,
		Response:    &ports.FiscalSignResult{XmlFirmado: "<xml/>", HashArchivo: "hash"},
	}, nil
}

func (m *mockSiatService) Setup(ctx context.Context, companyID, posID string) (*usecase.SetupResultado, error) {
	if m.setupFunc != nil {
		return m.setupFunc(ctx, companyID, posID)
	}
	return &usecase.SetupResultado{Cuis: &ports.CuisResult{Codigo: "CUIS-123"}, Cufd: &domain.Cufd{Cufd: "CUFD-456"}}, nil
}

func (m *mockSiatService) Sincronizar(ctx context.Context, companyID, posID, opRaw string) (*usecase.SincronizacionResultado, error) {
	if m.sincronizarFunc != nil {
		return m.sincronizarFunc(ctx, companyID, posID, opRaw)
	}
	return &usecase.SincronizacionResultado{Operations: []usecase.SincronizacionOpResult{{Operation: "tipoMoneda", Codigos: 1}}}, nil
}

func (m *mockSiatService) BuildSincronizacionResumen(companyID, posID string, res *usecase.SincronizacionResultado) *usecase.SincronizacionResumen {
	if m.buildSincronizacionResumenFunc != nil {
		return m.buildSincronizacionResumenFunc(companyID, posID, res)
	}
	return &usecase.SincronizacionResumen{Success: true, Operations: []usecase.SincronizacionOpResult{{Operation: "tipoMoneda", Codigos: 1}}}
}

func (m *mockSiatService) ListSinProducts(companyID, query string, limit, offset int) ([]*domain.SinProduct, int64, error) {
	if m.listSinProductsFunc != nil {
		return m.listSinProductsFunc(companyID, query, limit, offset)
	}
	return []*domain.SinProduct{{ID: "sp-1", CodigoProductoSin: 123, Descripcion: "Producto 1"}}, 1, nil
}

func (m *mockSiatService) ListActivitesDocumentSectors(companyID, query string, limit, offset int) ([]*domain.SiatActividadDocSector, int64, error) {
	if m.listActivitesDocumentSectorsFunc != nil {
		return m.listActivitesDocumentSectorsFunc(companyID, query, limit, offset)
	}
	return []*domain.SiatActividadDocSector{{CodigoDocumentoSector: 1, CodigoActividad: "ACT-1"}}, 1, nil
}

func (m *mockSiatService) CatalogReadiness(companyID, pointOfSaleID string) (*domain.CatalogReadiness, error) {
	if m.catalogReadinessFunc != nil {
		return m.catalogReadinessFunc(companyID, pointOfSaleID)
	}
	return &domain.CatalogReadiness{Ready: true}, nil
}

func (m *mockSiatService) ListCatalog(companyID, tipo string) (any, error) {
	if m.listCatalogFunc != nil {
		return m.listCatalogFunc(companyID, tipo)
	}
	return map[string]any{"tipo": tipo, "items": []any{}}, nil
}

func (m *mockSiatService) EmitirDocumentoAjuste(ctx context.Context, companyID, posID string, body usecase.DocumentoAjusteInput) (*usecase.DocumentoAjusteResultado, error) {
	if m.emitirDocumentoAjusteFunc != nil {
		return m.emitirDocumentoAjusteFunc(ctx, companyID, posID, body)
	}
	comp := &domain.Company{ID: "comp-1"}
	pos := &domain.PointOfSale{ID: "pos-1"}
	return &usecase.DocumentoAjusteResultado{
		Company:     comp,
		PointOfSale: pos,
		Response:    &ports.FiscalAdjustmentResult{Transaccion: true, CodigoRecepcion: "RCPT-123", Cuf: "CUF-123"},
	}, nil
}

func newTestRouter(h *handler) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/cuis/{companyId}/{pointOfSaleId}", h.solicitarCUIS)
	r.Post("/cufd/{companyId}/{pointOfSaleId}", h.solicitarCUFD)
	r.Post("/eventos/{companyId}/{pointOfSaleId}", h.registrarEventoSignificativo)
	r.Post("/paquetes/{companyId}/{pointOfSaleId}", h.enviarPaquete)
	r.Post("/paquetes/validar/{companyId}/{pointOfSaleId}", h.validarPaquete)
	r.Post("/masiva/{companyId}/{pointOfSaleId}", h.enviarMasiva)
	r.Post("/masiva/validar/{companyId}/{pointOfSaleId}", h.validarMasiva)
	r.Post("/compras/{companyId}/{pointOfSaleId}", h.enviarCompras)
	r.Post("/firmar/{companyId}/{pointOfSaleId}", h.firmarFactura)
	r.Post("/setup", h.setup)
	r.Post("/sincronizar/{companyId}/{pointOfSaleId}", h.sincronizar)
	r.Get("/productos-sin", h.listSinProducts)
	r.Get("/actividades-documento-sector", h.listActivitesDocumentSectors)
	r.Get("/readiness", h.catalogReadiness)
	r.Get("/catalogs/{companyId}/{tipo}", h.getCatalog)
	r.Get("/catalogs/{companyId}", h.getCatalog)
	r.Post("/documento-ajuste/{companyId}/{pointOfSaleId}", h.emitirDocumentoAjuste)
	return r
}

func TestSetupValidaBody(t *testing.T) {
	h := newHandler(nil, nil)
	r := chi.NewRouter()
	r.Post("/setup", h.setup)

	casos := []struct {
		name string
		body string
	}{
		{"sin body", ""},
		{"body vacío json", "{}"},
		{"solo company_id", `{"company_id":"c1"}`},
		{"solo point_of_sale_id", `{"point_of_sale_id":"p1"}`},
	}
	for _, tc := range casos {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(tc.body))
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d, se esperaba 400", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), `"code":"VALIDATION_ERROR"`) {
				t.Fatalf("body=%s", rec.Body.String())
			}
		})
	}
}

func TestSetupExitoso(t *testing.T) {
	svc := &mockSiatService{
		setupFunc: func(ctx context.Context, companyID, posID string) (*usecase.SetupResultado, error) {
			return &usecase.SetupResultado{Cuis: &ports.CuisResult{Codigo: "CUIS-123"}, Cufd: &domain.Cufd{Cufd: "CUFD-456"}}, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	body := `{"company_id":"comp-1","point_of_sale_id":"pos-1"}`
	req := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	var resp usecase.SetupResultado
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if resp.Cuis != nil && resp.Cuis.Codigo != "CUIS-123" {
		t.Errorf("cuis=%q", resp.Cuis.Codigo)
	}
}

func TestSolicitarCUIS(t *testing.T) {
	svc := &mockSiatService{
		solicitarCUISFunc: func(ctx context.Context, companyID, posID string) (*usecase.CuisResultado, error) {
			return &usecase.CuisResultado{Success: true, Data: &ports.CuisResult{Codigo: "CUIS-123", FechaVigencia: time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)}}, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/cuis/comp-1/pos-1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	var resp struct {
		Success       bool   `json:"success"`
		Cuis          string `json:"cuis"`
		FechaVigencia string `json:"fecha_vigencia"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if !resp.Success {
		t.Error("success debe ser true")
	}
	if resp.Cuis != "CUIS-123" {
		t.Errorf("cuis=%q", resp.Cuis)
	}
}

func TestSolicitarCUFD(t *testing.T) {
	svc := &mockSiatService{
		solicitarCUFDFunc: func(ctx context.Context, companyID, posID string) (*usecase.CufdResultado, error) {
			comp := &domain.Company{ID: "comp-1"}
			pos := &domain.PointOfSale{ID: "pos-1"}
			return &usecase.CufdResultado{
				Company:     comp,
				PointOfSale: pos,
				Response:    &ports.CufdResult{Transaccion: true, Codigo: "CUFD-123", FechaVigencia: time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC), CodigoControl: "CTRL-123"},
			}, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/cufd/comp-1/pos-1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if !resp["success"].(bool) {
		t.Error("success debe ser true")
	}
	data := resp["data"].(map[string]any)
	if data["cufd"] != "CUFD-123" {
		t.Errorf("cufd=%q", data["cufd"])
	}
}

func TestRegistrarEventoSignificativo(t *testing.T) {
	svc := &mockSiatService{
		registrarEventoFunc: func(ctx context.Context, companyID, posID string, body usecase.EventoSignificativoInput) (*usecase.EventoSignificativoResultado, error) {
			comp := &domain.Company{ID: "comp-1"}
			pos := &domain.PointOfSale{ID: "pos-1"}
			return &usecase.EventoSignificativoResultado{
				Company:     comp,
				PointOfSale: pos,
				Response:    &ports.FiscalEventResult{Transaccion: true, CodigoRecepcion: "RCPT-123"},
			}, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	body := `{"codigo_evento":1,"descripcion":"Falla energía"}`
	req := httptest.NewRequest(http.MethodPost, "/eventos/comp-1/pos-1", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if !resp["success"].(bool) {
		t.Error("success debe ser true")
	}
	if resp["codigo_recepcion"] != "RCPT-123" {
		t.Errorf("codigo_recepcion=%q", resp["codigo_recepcion"])
	}
}

func TestEnviarPaquete(t *testing.T) {
	svc := &mockSiatService{
		enviarPaqueteFunc: func(ctx context.Context, companyID, posID string, body usecase.PaqueteInput) (*usecase.PaqueteResultado, error) {
			comp := &domain.Company{ID: "comp-1"}
			pos := &domain.PointOfSale{ID: "pos-1"}
			return &usecase.PaqueteResultado{
				Company:     comp,
				PointOfSale: pos,
				Response:    &ports.FiscalPackageResult{Transaccion: true, CodigoRecepcion: "RCPT-123", CantidadFacturas: 1},
			}, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	body := `{"facturas":[{"xml":"<xml/>","hash":"abc"}]}`
	req := httptest.NewRequest(http.MethodPost, "/paquetes/comp-1/pos-1", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if responseObjectID(resp, "company") != "comp-1" {
		t.Errorf("company=%v", resp["company"])
	}
	if responseObjectID(resp, "point_of_sale") != "pos-1" {
		t.Errorf("point_of_sale=%v", resp["point_of_sale"])
	}
}

func TestValidarPaquete(t *testing.T) {
	svc := &mockSiatService{
		validarPaqueteFunc: func(ctx context.Context, companyID, posID string, body usecase.PaqueteValidacionInput) (*usecase.PaqueteResultado, error) {
			comp := &domain.Company{ID: "comp-1"}
			pos := &domain.PointOfSale{ID: "pos-1"}
			return &usecase.PaqueteResultado{
				Company:     comp,
				PointOfSale: pos,
				Response:    &ports.FiscalPackageResult{Transaccion: true, CodigoRecepcion: "RCPT-123", CantidadFacturas: 1},
			}, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	body := `{"codigo_recepcion":"RCPT-123"}`
	req := httptest.NewRequest(http.MethodPost, "/paquetes/validar/comp-1/pos-1", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if responseObjectID(resp, "company") != "comp-1" {
		t.Errorf("company=%v", resp["company"])
	}
}

func TestEnviarMasiva(t *testing.T) {
	svc := &mockSiatService{
		enviarMasivaFunc: func(ctx context.Context, companyID, posID string, body usecase.MasivaInput) (*usecase.PaqueteResultado, error) {
			comp := &domain.Company{ID: "comp-1"}
			pos := &domain.PointOfSale{ID: "pos-1"}
			return &usecase.PaqueteResultado{
				Company:     comp,
				PointOfSale: pos,
				Response:    &ports.FiscalPackageResult{Transaccion: true, CodigoRecepcion: "RCPT-123", CantidadFacturas: 1},
			}, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	body := `{"facturas":[{"xml":"<xml/>","hash":"abc"}]}`
	req := httptest.NewRequest(http.MethodPost, "/masiva/comp-1/pos-1", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if responseObjectID(resp, "company") != "comp-1" {
		t.Errorf("company=%v", resp["company"])
	}
}

func TestValidarMasiva(t *testing.T) {
	svc := &mockSiatService{
		validarMasivaFunc: func(ctx context.Context, companyID, posID string, body usecase.PaqueteValidacionInput) (*usecase.PaqueteResultado, error) {
			comp := &domain.Company{ID: "comp-1"}
			pos := &domain.PointOfSale{ID: "pos-1"}
			return &usecase.PaqueteResultado{
				Company:     comp,
				PointOfSale: pos,
				Response:    &ports.FiscalPackageResult{Transaccion: true, CodigoRecepcion: "RCPT-123", CantidadFacturas: 1},
			}, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	body := `{"codigo_recepcion":"RCPT-123"}`
	req := httptest.NewRequest(http.MethodPost, "/masiva/validar/comp-1/pos-1", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if responseObjectID(resp, "company") != "comp-1" {
		t.Errorf("company=%v", resp["company"])
	}
}

func TestEnviarCompras(t *testing.T) {
	svc := &mockSiatService{
		enviarComprasFunc: func(ctx context.Context, companyID, posID string, body usecase.ComprasInput) (*usecase.ComprasResultado, error) {
			comp := &domain.Company{ID: "comp-1"}
			pos := &domain.PointOfSale{ID: "pos-1"}
			return &usecase.ComprasResultado{
				Company:     comp,
				PointOfSale: pos,
				Response:    &ports.FiscalPurchaseResult{Transaccion: true, CodigoRecepcion: "RCPT-123"},
			}, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	body := `{"compras":[{"nit_proveedor":"123","numero_factura":"001"}]}`
	req := httptest.NewRequest(http.MethodPost, "/compras/comp-1/pos-1", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if responseObjectID(resp, "company") != "comp-1" {
		t.Errorf("company=%v", resp["company"])
	}
}

func TestFirmarFactura(t *testing.T) {
	svc := &mockSiatService{
		firmarFacturaFunc: func(ctx context.Context, companyID, posID string, body usecase.FirmaInput) (*usecase.FirmaResultado, error) {
			comp := &domain.Company{ID: "comp-1"}
			pos := &domain.PointOfSale{ID: "pos-1"}
			return &usecase.FirmaResultado{
				Company:     comp,
				PointOfSale: pos,
				Response:    &ports.FiscalSignResult{XmlFirmado: "<xml/>", HashArchivo: "hash"},
			}, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	body := `{"xml":"<xml/>","hash":"abc"}`
	req := httptest.NewRequest(http.MethodPost, "/firmar/comp-1/pos-1", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if responseObjectID(resp, "company") != "comp-1" {
		t.Errorf("company=%v", resp["company"])
	}
}

func TestSincronizar(t *testing.T) {
	svc := &mockSiatService{
		sincronizarFunc: func(ctx context.Context, companyID, posID, opRaw string) (*usecase.SincronizacionResultado, error) {
			return &usecase.SincronizacionResultado{Operations: []usecase.SincronizacionOpResult{{Operation: "tipoMoneda", Codigos: 1}}}, nil
		},
		buildSincronizacionResumenFunc: func(companyID, posID string, res *usecase.SincronizacionResultado) *usecase.SincronizacionResumen {
			return &usecase.SincronizacionResumen{Success: true, Operations: []usecase.SincronizacionOpResult{{Operation: "tipoMoneda", Codigos: 1}}}
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/sincronizar/comp-1/pos-1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	var resp usecase.SincronizacionResumen
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if !resp.Success {
		t.Error("success debe ser true")
	}
	if len(resp.Operations) != 1 {
		t.Errorf("operations=%d", len(resp.Operations))
	}
}

func TestListSinProducts(t *testing.T) {
	svc := &mockSiatService{
		listSinProductsFunc: func(companyID, query string, limit, offset int) ([]*domain.SinProduct, int64, error) {
			return []*domain.SinProduct{{ID: "sp-1", CodigoProductoSin: 123, Descripcion: "Producto 1"}}, 1, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/productos-sin?company_id=comp-1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	items := resp["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("esperaba 1 item, got %d", len(items))
	}
}

func TestListActivitesDocumentSectors(t *testing.T) {
	svc := &mockSiatService{
		listActivitesDocumentSectorsFunc: func(companyID, query string, limit, offset int) ([]*domain.SiatActividadDocSector, int64, error) {
			return []*domain.SiatActividadDocSector{{CodigoDocumentoSector: 1, CodigoActividad: "ACT-1"}}, 1, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/actividades-documento-sector?company_id=comp-1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	items := resp["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("esperaba 1 item, got %d", len(items))
	}
}

func TestCatalogReadiness(t *testing.T) {
	svc := &mockSiatService{
		catalogReadinessFunc: func(companyID, pointOfSaleID string) (*domain.CatalogReadiness, error) {
			return &domain.CatalogReadiness{Ready: true}, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/readiness?company_id=comp-1&point_of_sale_id=pos-1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	var resp domain.CatalogReadiness
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if !resp.Ready {
		t.Error("ready debe ser true")
	}
}

func TestGetCatalog(t *testing.T) {
	svc := &mockSiatService{
		listCatalogFunc: func(companyID, tipo string) (any, error) {
			return map[string]any{"tipo": "tipoMoneda", "items": []any{map[string]any{"codigo": 1, "descripcion": "BOLIVIANO"}}}, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/catalogs/comp-1/tipoMoneda", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if resp["tipo"] != "tipoMoneda" {
		t.Errorf("tipo=%q", resp["tipo"])
	}
}

func TestGetCatalogAll(t *testing.T) {
	svc := &mockSiatService{
		listCatalogFunc: func(companyID, tipo string) (any, error) {
			return map[string]any{"tipoMoneda": []any{map[string]any{"codigo": 1}}, "actividades": []any{map[string]any{"codigo_caeb": "123"}}}, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/catalogs/comp-1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if len(resp) == 0 {
		t.Error("respuesta vacía")
	}
}

func TestEmitirDocumentoAjuste(t *testing.T) {
	svc := &mockSiatService{
		emitirDocumentoAjusteFunc: func(ctx context.Context, companyID, posID string, body usecase.DocumentoAjusteInput) (*usecase.DocumentoAjusteResultado, error) {
			comp := &domain.Company{ID: "comp-1"}
			pos := &domain.PointOfSale{ID: "pos-1"}
			return &usecase.DocumentoAjusteResultado{
				Company:     comp,
				PointOfSale: pos,
				Response:    &ports.FiscalAdjustmentResult{Transaccion: true, CodigoRecepcion: "RCPT-123", Cuf: "CUF-123"},
			}, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	body := `{"codigo_motivo":1,"factura_original_id":"inv-1"}`
	req := httptest.NewRequest(http.MethodPost, "/documento-ajuste/comp-1/pos-1", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, se esperaba 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if responseObjectID(resp, "company") != "comp-1" {
		t.Errorf("company=%v", resp["company"])
	}
}
