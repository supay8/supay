package siat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
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
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(deliveryHttp.WithCompanyID(r.Context(), "comp-1")))
		})
	})
	(&Module{h: h}).RegisterRoutes(r)
	return r
}

func TestSetupValidaBody(t *testing.T) {
	h := newHandler(nil, nil)
	r := newTestRouter(h)

	casos := []struct {
		name string
		body string
	}{
		{"sin body", ""},
		{"body vacío json", "{}"},
		{"solo company_id", `{"company_id":"c1"}`},
		{"company_id no permitido", `{"company_id":"comp-1","point_of_sale_id":"p1"}`},
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
			if companyID != "comp-1" {
				t.Fatalf("tenant=%q, se esperaba comp-1 desde contexto", companyID)
			}
			if posID != "pos-1" {
				t.Fatalf("pos=%q, se esperaba pos-1", posID)
			}
			return &usecase.SetupResultado{Cuis: &ports.CuisResult{Codigo: "CUIS-123"}, Cufd: &domain.Cufd{Cufd: "CUFD-456"}}, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	body := `{"point_of_sale_id":"pos-1"}`
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
			if companyID != "comp-1" {
				t.Fatalf("tenant=%q, se esperaba comp-1 desde contexto", companyID)
			}
			if posID != "pos-1" {
				t.Fatalf("pos=%q, se esperaba pos-1", posID)
			}
			return &usecase.CuisResultado{Success: true, Data: &ports.CuisResult{Codigo: "CUIS-123", FechaVigencia: time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)}}, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/siat/cuis/pos-1", nil)
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
			if companyID != "comp-1" {
				t.Fatalf("tenant=%q, se esperaba comp-1 desde contexto", companyID)
			}
			if posID != "pos-1" {
				t.Fatalf("pos=%q, se esperaba pos-1", posID)
			}
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

	req := httptest.NewRequest(http.MethodPost, "/siat/cufd/pos-1", nil)
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
			if companyID != "comp-1" {
				t.Fatalf("tenant=%q, se esperaba comp-1 desde contexto", companyID)
			}
			if posID != "pos-1" {
				t.Fatalf("pos=%q, se esperaba pos-1", posID)
			}
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

	body := `{"codigo_motivo_evento":1,"descripcion":"Falla energía"}`
	req := httptest.NewRequest(http.MethodPost, "/siat/evento-significativo/pos-1", strings.NewReader(body))
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
			if companyID != "comp-1" {
				t.Fatalf("tenant=%q, se esperaba comp-1 desde contexto", companyID)
			}
			if posID != "pos-1" {
				t.Fatalf("pos=%q, se esperaba pos-1", posID)
			}
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

	body := `{"invoice_ids":["inv-1"]}`
	req := httptest.NewRequest(http.MethodPost, "/siat/paquete/pos-1", strings.NewReader(body))
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
			if companyID != "comp-1" {
				t.Fatalf("tenant=%q, se esperaba comp-1 desde contexto", companyID)
			}
			if posID != "" {
				t.Fatalf("pos=%q, se esperaba vacío", posID)
			}
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

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/siat/paquete/batch-1/validate", strings.NewReader(body))
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
			if companyID != "comp-1" {
				t.Fatalf("tenant=%q, se esperaba comp-1 desde contexto", companyID)
			}
			if posID != "pos-1" {
				t.Fatalf("pos=%q, se esperaba pos-1", posID)
			}
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

	body := `{"invoice_ids":["inv-1"]}`
	req := httptest.NewRequest(http.MethodPost, "/siat/masiva/pos-1", strings.NewReader(body))
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
			if companyID != "comp-1" {
				t.Fatalf("tenant=%q, se esperaba comp-1 desde contexto", companyID)
			}
			if posID != "" {
				t.Fatalf("pos=%q, se esperaba vacío", posID)
			}
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

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/siat/masiva/batch-1/validate", strings.NewReader(body))
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
			if companyID != "comp-1" {
				t.Fatalf("tenant=%q, se esperaba comp-1 desde contexto", companyID)
			}
			if posID != "pos-1" {
				t.Fatalf("pos=%q, se esperaba pos-1", posID)
			}
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

	body := `{"descripcion":"Compras del periodo","gestion":2026,"periodo":9}`
	req := httptest.NewRequest(http.MethodPost, "/siat/compras/pos-1", strings.NewReader(body))
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
			if companyID != "comp-1" {
				t.Fatalf("tenant=%q, se esperaba comp-1 desde contexto", companyID)
			}
			if posID != "pos-1" {
				t.Fatalf("pos=%q, se esperaba pos-1", posID)
			}
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

	body := `{"xml":"<xml/>"}`
	req := httptest.NewRequest(http.MethodPost, "/siat/firma/pos-1", strings.NewReader(body))
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
			if companyID != "comp-1" {
				t.Fatalf("tenant=%q, se esperaba comp-1 desde contexto", companyID)
			}
			if posID != "pos-1" {
				t.Fatalf("pos=%q, se esperaba pos-1", posID)
			}
			return &usecase.SincronizacionResultado{Operations: []usecase.SincronizacionOpResult{{Operation: "tipoMoneda", Codigos: 1}}}, nil
		},
		buildSincronizacionResumenFunc: func(companyID, posID string, res *usecase.SincronizacionResultado) *usecase.SincronizacionResumen {
			if companyID != "comp-1" {
				t.Fatalf("tenant=%q, se esperaba comp-1 desde contexto", companyID)
			}
			if posID != "pos-1" {
				t.Fatalf("pos=%q, se esperaba pos-1", posID)
			}
			return &usecase.SincronizacionResumen{Success: true, Operations: []usecase.SincronizacionOpResult{{Operation: "tipoMoneda", Codigos: 1}}}
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/siat/sincronizar/pos-1", nil)
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
			if companyID != "comp-1" {
				t.Fatalf("tenant=%q, se esperaba comp-1 desde contexto", companyID)
			}
			return []*domain.SinProduct{{ID: "sp-1", CodigoProductoSin: 123, Descripcion: "Producto 1"}}, 1, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/catalogs/products", nil)
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
			if companyID != "comp-1" {
				t.Fatalf("tenant=%q, se esperaba comp-1 desde contexto", companyID)
			}
			return []*domain.SiatActividadDocSector{{CodigoDocumentoSector: 1, CodigoActividad: "ACT-1"}}, 1, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/catalogs/activites-document-sectors", nil)
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
			if companyID != "comp-1" {
				t.Fatalf("tenant=%q, se esperaba comp-1 desde contexto", companyID)
			}
			return &domain.CatalogReadiness{Ready: true}, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/catalogs/readiness?point_of_sale_id=pos-1", nil)
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
			if companyID != "comp-1" {
				t.Fatalf("tenant=%q, se esperaba comp-1 desde contexto", companyID)
			}
			return map[string]any{"tipo": "tipoMoneda", "items": []any{map[string]any{"codigo": 1, "descripcion": "BOLIVIANO"}}}, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/siat/catalogs/tipoMoneda", nil)
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
			if companyID != "comp-1" {
				t.Fatalf("tenant=%q, se esperaba comp-1 desde contexto", companyID)
			}
			return map[string]any{"tipoMoneda": []any{map[string]any{"codigo": 1}}, "actividades": []any{map[string]any{"codigo_caeb": "123"}}}, nil
		},
	}
	h := newHandler(svc, nil)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/siat/catalogs", nil)
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
			if companyID != "comp-1" {
				t.Fatalf("tenant=%q, se esperaba comp-1 desde contexto", companyID)
			}
			if posID != "pos-1" {
				t.Fatalf("pos=%q, se esperaba pos-1", posID)
			}
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

	body := `{"cufFacturaOriginal":"CUF-1","motivo":"Devolución"}`
	req := httptest.NewRequest(http.MethodPost, "/siat/documento-ajuste/pos-1", strings.NewReader(body))
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

func TestSiatRequiresAuthenticatedTenant(t *testing.T) {
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/siat/cuis/pos-1", ""},
		{http.MethodPost, "/siat/cufd/pos-1", ""},
		{http.MethodPost, "/siat/sincronizar/pos-1", ""},
		{http.MethodPost, "/siat/paquete/pos-1", "{}"},
		{http.MethodPost, "/siat/masiva/pos-1", "{}"},
		{http.MethodPost, "/siat/paquete/batch-1/validate", ""},
		{http.MethodPost, "/siat/masiva/batch-1/validate", ""},
		{http.MethodPost, "/siat/evento-significativo/pos-1", "{}"},
		{http.MethodPost, "/siat/firma/pos-1", "{}"},
		{http.MethodPost, "/siat/compras/pos-1", "{}"},
		{http.MethodPost, "/siat/documento-ajuste/pos-1", "{}"},
		{http.MethodPost, "/siat/setup/pos-1", ""},
		{http.MethodPost, "/setup", `{"company_id":"comp-1","point_of_sale_id":"pos-1"}`},
		{http.MethodPost, "/siat/masiva/comp-1/pos-1", "{}"},
		{http.MethodGet, "/siat/catalogs", ""},
		{http.MethodGet, "/catalogs/comp-1", ""},
		{http.MethodGet, "/catalogs/products?company_id=comp-1", ""},
		{http.MethodGet, "/catalogs/activites-document-sectors?company_id=comp-1", ""},
		{http.MethodGet, "/catalogs/readiness?company_id=comp-1", ""},
	}
	r := chi.NewRouter()
	NewModule(nil, nil).RegisterRoutes(r)
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d body=%s; se esperaba 401", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestSiatRejectsForeignTenantAliases(t *testing.T) {
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/siat/cuis/other-company/pos-1"},
		{http.MethodPost, "/siat/cufd/other-company/pos-1"},
		{http.MethodPost, "/siat/sincronizar/other-company/pos-1"},
		{http.MethodPost, "/siat/paquete/other-company/pos-1"},
		{http.MethodPost, "/siat/masiva/other-company/pos-1"},
		{http.MethodPost, "/siat/evento-significativo/other-company/pos-1"},
		{http.MethodPost, "/siat/firma/other-company/pos-1"},
		{http.MethodPost, "/siat/compras/other-company/pos-1"},
		{http.MethodPost, "/siat/documento-ajuste/other-company/pos-1"},
		{http.MethodGet, "/catalogs/other-company"},
		{http.MethodGet, "/catalogs/other-company/tipoMoneda"},
		{http.MethodGet, "/catalogs/products?company_id=other-company"},
		{http.MethodGet, "/catalogs/activites-document-sectors?company_id=other-company"},
		{http.MethodGet, "/catalogs/readiness?company_id=other-company"},
		{http.MethodGet, "/catalogs/products?company_id=comp-1&company_id=other-company"},
	}
	r := newTestRouter(newHandler(nil, nil))
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status=%d body=%s; se esperaba 403", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestSiatBatchSubmissionContract(t *testing.T) {
	for _, operation := range []string{"masiva", "paquete"} {
		for _, body := range []string{"", "{}", `{"invoice_ids":["inv-1","inv-2"]}`} {
			t.Run(operation+"/"+body, func(t *testing.T) {
				calls := 0
				check := func(companyID, posID string, ids []string) (*usecase.PaqueteResultado, error) {
					calls++
					if companyID != "comp-1" || posID != "pos-1" {
						t.Fatalf("tenant=%q pos=%q", companyID, posID)
					}
					if strings.Contains(body, "invoice_ids") {
						if len(ids) != 2 || ids[0] != "inv-1" || ids[1] != "inv-2" {
							t.Fatalf("invoice_ids=%v", ids)
						}
					} else if len(ids) != 0 {
						t.Fatalf("invoice_ids=%v para selección automática", ids)
					}
					return &usecase.PaqueteResultado{Batches: []usecase.BatchResultado{{BatchID: "batch-1", InvoiceIDs: []string{"inv-1"}}}}, nil
				}
				svc := &mockSiatService{
					enviarMasivaFunc: func(_ context.Context, companyID, posID string, input usecase.MasivaInput) (*usecase.PaqueteResultado, error) {
						return check(companyID, posID, input.FacturaIDs)
					},
					enviarPaqueteFunc: func(_ context.Context, companyID, posID string, input usecase.PaqueteInput) (*usecase.PaqueteResultado, error) {
						return check(companyID, posID, input.FacturaIDs)
					},
				}
				r := newTestRouter(newHandler(svc, nil))
				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/siat/"+operation+"/pos-1", strings.NewReader(body)))
				if rec.Code != http.StatusOK || calls != 1 {
					t.Fatalf("status=%d calls=%d body=%s", rec.Code, calls, rec.Body.String())
				}
				var result struct {
					Batches []struct {
						BatchID    string   `json:"batch_id"`
						InvoiceIDs []string `json:"invoice_ids"`
					} `json:"batches"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if len(result.Batches) != 1 || result.Batches[0].BatchID != "batch-1" || len(result.Batches[0].InvoiceIDs) != 1 {
					t.Fatalf("batches no están disponibles para validación: %s", rec.Body.String())
				}
			})
		}
	}
}

func TestSiatBatchValidationUsesRouteID(t *testing.T) {
	for _, operation := range []string{"masiva", "paquete"} {
		for _, body := range []string{"", "{}"} {
			t.Run(operation+"/"+body, func(t *testing.T) {
				calls := 0
				validate := func(_ context.Context, companyID, posID string, input usecase.PaqueteValidacionInput) (*usecase.PaqueteResultado, error) {
					calls++
					if companyID != "comp-1" || posID != "" || input.BatchID != "batch-123" {
						t.Fatalf("tenant=%q pos=%q batch=%q", companyID, posID, input.BatchID)
					}
					return &usecase.PaqueteResultado{}, nil
				}
				svc := &mockSiatService{validarMasivaFunc: validate, validarPaqueteFunc: validate}
				r := newTestRouter(newHandler(svc, nil))
				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/siat/"+operation+"/batch-123/validate", strings.NewReader(body)))
				if rec.Code != http.StatusOK || calls != 1 {
					t.Fatalf("status=%d calls=%d body=%s", rec.Code, calls, rec.Body.String())
				}
			})
		}
	}
}

func TestSiatRejectsInvalidBatchPayloads(t *testing.T) {
	paths := []string{
		"/siat/masiva/pos-1", "/siat/paquete/pos-1",
		"/siat/masiva/batch-1/validate", "/siat/paquete/batch-1/validate",
	}
	bodies := []string{
		`{"invoice_ids":null}`, `{"invoice_ids":[null]}`, `{"invoice_ids":["inv-1",null]}`,
		`{"invoice_ids":"inv-1"}`, `{"invoice_ids":[1]}`, `{"invoice_ids":{}}`,
		`{"invoice_ids":["inv-1"],"invoice_ids":null}`, `{"invoice_ids":["inv-1"],"invoice_ids":[]}`,
		`{"invoice_ids":[],"invoice_ids":["inv-1"]}`, `{"invoice_ids":["inv-1"],"INVOICE_IDS":[]}`,
		`{"company_id":"other-company"}`, `{"archivo":"<xml/>"}`, `{"codigoDocumentoSector":1}`,
		`{"codigo_recepcion":"receipt-1"}`, `{"batch_id":"other-batch"}`, `{"factura_ids":["inv-1"]}`,
		`{"facturas":[{"xml":"<xml/>"}]}`, `{"unknown":true}`, `null`, `[]`, `""`, `{`, `{} {}`, `{} null`, `{} invalid`,
	}
	r := newTestRouter(newHandler(nil, nil))
	for _, path := range paths {
		for _, body := range bodies {
			t.Run(path+"/"+body, func(t *testing.T) {
				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("status=%d body=%s; se esperaba 400", rec.Code, rec.Body.String())
				}
			})
		}
	}
	for _, path := range paths[2:] {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"invoice_ids":["inv-1"]}`)))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("validación %s permite invoice_ids: status=%d", path, rec.Code)
		}
	}
}

func TestSetupUsesTenantAndRoutePointOfSale(t *testing.T) {
	calls := 0
	svc := &mockSiatService{setupFunc: func(_ context.Context, companyID, posID string) (*usecase.SetupResultado, error) {
		calls++
		if companyID != "comp-1" || posID != "pos-1" {
			t.Fatalf("tenant=%q pos=%q", companyID, posID)
		}
		return &usecase.SetupResultado{}, nil
	}}
	r := newTestRouter(newHandler(svc, nil))
	for _, body := range []string{"", "{}"} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/siat/setup/pos-1", strings.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
	for _, body := range []string{`{"company_id":"other-company"}`, `{"point_of_sale_id":"other-pos"}`} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/siat/setup/pos-1", strings.NewReader(body)))
		if rec.Code != http.StatusBadRequest || calls != 2 {
			t.Fatalf("status=%d calls=%d body=%s", rec.Code, calls, rec.Body.String())
		}
	}
}

func TestSiatAcceptsMatchingTenantAliases(t *testing.T) {
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/siat/cuis/comp-1/pos-1"},
		{http.MethodPost, "/siat/cufd/comp-1/pos-1"},
		{http.MethodPost, "/siat/sincronizar/comp-1/pos-1"},
		{http.MethodPost, "/siat/paquete/comp-1/pos-1"},
		{http.MethodPost, "/siat/masiva/comp-1/pos-1"},
		{http.MethodPost, "/siat/evento-significativo/comp-1/pos-1"},
		{http.MethodPost, "/siat/firma/comp-1/pos-1"},
		{http.MethodPost, "/siat/compras/comp-1/pos-1"},
		{http.MethodPost, "/siat/documento-ajuste/comp-1/pos-1"},
		{http.MethodGet, "/catalogs/comp-1"},
		{http.MethodGet, "/catalogs/comp-1/tipoMoneda"},
		{http.MethodGet, "/catalogs/products?company_id=comp-1"},
		{http.MethodGet, "/catalogs/activites-document-sectors?company_id=comp-1"},
		{http.MethodGet, "/catalogs/readiness?company_id=comp-1"},
	}
	r := newTestRouter(newHandler(&mockSiatService{}, nil))
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s; se esperaba 200", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestSiatRejectsLegacyValidationRoutes(t *testing.T) {
	r := newTestRouter(newHandler(nil, nil))
	for _, operation := range []string{"masiva", "paquete"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/siat/"+operation+"/comp-1/pos-1/validar", strings.NewReader(`{"codigo_recepcion":"receipt-1"}`))
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("ruta de validación técnica %s aún está expuesta: status=%d", operation, rec.Code)
		}
	}
}
