package siat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brandsrx/supay/internal/config"
	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	"github.com/brandsrx/supay/internal/delivery/http/modules"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/brandsrx/supay/internal/ports"
	"github.com/brandsrx/supay/internal/usecase"
)

func TestSiatBatchExplicitEmptySelectionIsNotAutomatic(t *testing.T) {
	for _, operation := range []string{"masiva", "paquete"} {
		t.Run(operation, func(t *testing.T) {
			calls := 0
			check := func(ids []string) (*usecase.PaqueteResultado, error) {
				calls++
				if ids == nil || len(ids) != 0 {
					t.Fatalf("selección vacía debe conservarse para validación del usecase: %#v", ids)
				}
				return &usecase.PaqueteResultado{}, nil
			}
			svc := &mockSiatService{
				enviarMasivaFunc: func(_ context.Context, _, _ string, input usecase.MasivaInput) (*usecase.PaqueteResultado, error) {
					return check(input.FacturaIDs)
				},
				enviarPaqueteFunc: func(_ context.Context, _, _ string, input usecase.PaqueteInput) (*usecase.PaqueteResultado, error) {
					return check(input.FacturaIDs)
				},
			}
			rec := httptest.NewRecorder()
			newTestRouter(newHandler(svc, nil)).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/siat/"+operation+"/pos-1", strings.NewReader(`{"invoice_ids":[]}`)))
			if rec.Code != http.StatusOK || calls != 1 {
				t.Fatalf("status=%d calls=%d body=%s", rec.Code, calls, rec.Body.String())
			}
		})
	}
}

func TestSiatBatchResponseLimitsCompanyAndPointOfSaleData(t *testing.T) {
	cuis := "private-cuis"
	result := &usecase.PaqueteResultado{
		Company: &domain.Company{
			ID: "comp-1", Nit: "123456789", BusinessName: "Emisor",
			CodigoSistema: "system-code", UsuarioSiat: "private-user",
			CertificateWebhookURL: "https://example.test/certificate?token=private-token",
		},
		PointOfSale: &domain.PointOfSale{
			ID: "pos-1", CodigoSucursal: 2, CodigoPuntoVenta: 3, Description: "Sucursal",
			Cuis: &cuis, SiatResponse: json.RawMessage(`{"internal":"private-response"}`),
		},
		Response: &ports.FiscalPackageResult{Transaccion: false, CantidadFacturas: 2},
		Batches: []usecase.BatchResultado{
			{BatchID: "batch-1", InvoiceIDs: []string{"inv-1"}, Status: domain.PackageStatusPending,
				Response: &ports.FiscalPackageResult{Transaccion: true, CodigoRecepcion: "receipt-1"}},
			{BatchID: "batch-2", InvoiceIDs: []string{"inv-2"}, Status: domain.PackageStatusUnknown, Error: "resultado SIAT incierto"},
		},
	}
	validate := func(context.Context, string, string, usecase.PaqueteValidacionInput) (*usecase.PaqueteResultado, error) {
		return result, nil
	}
	svc := &mockSiatService{
		enviarMasivaFunc: func(context.Context, string, string, usecase.MasivaInput) (*usecase.PaqueteResultado, error) {
			return result, nil
		},
		enviarPaqueteFunc: func(context.Context, string, string, usecase.PaqueteInput) (*usecase.PaqueteResultado, error) {
			return result, nil
		},
		validarMasivaFunc: validate, validarPaqueteFunc: validate,
	}
	router := newTestRouter(newHandler(svc, nil))
	for _, path := range []string{"/siat/masiva/pos-1", "/siat/paquete/pos-1", "/siat/masiva/batch-1/validate", "/siat/paquete/batch-1/validate"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			var response struct {
				Company     map[string]any            `json:"company"`
				PointOfSale map[string]any            `json:"point_of_sale"`
				Response    ports.FiscalPackageResult `json:"response"`
				Batches     []usecase.BatchResultado  `json:"batches"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if len(response.Company) != 3 || response.Company["id"] != "comp-1" || response.Company["nit"] != "123456789" || response.Company["business_name"] != "Emisor" {
				t.Fatalf("datos de empresa inesperados: %v", response.Company)
			}
			if len(response.PointOfSale) != 4 || response.PointOfSale["id"] != "pos-1" || response.PointOfSale["codigo_sucursal"] != float64(2) || response.PointOfSale["codigo_punto_venta"] != float64(3) || response.PointOfSale["description"] != "Sucursal" {
				t.Fatalf("datos de punto de venta inesperados: %v", response.PointOfSale)
			}
			if strings.Contains(rec.Body.String(), "private-") {
				t.Fatalf("la respuesta expone configuración privada: %s", rec.Body.String())
			}
			if response.Response.Transaccion || len(response.Batches) != 2 || response.Batches[0].Response.CodigoRecepcion != "receipt-1" || response.Batches[1].Status != domain.PackageStatusUnknown || response.Batches[1].Error == "" {
				t.Fatalf("se perdió el resultado parcial que permite conciliar los lotes: %s", rec.Body.String())
			}
		})
	}
}

type batchRouteKeyLookup struct{}

func (batchRouteKeyLookup) FindByPrefix(prefix string) (*models.ApiKey, error) {
	if prefix != "sup_batch" {
		return nil, http.ErrNoLocation
	}
	return &models.ApiKey{ID: "key-1", CompanyId: "comp-1", KeyHash: "test-hash", IsActive: true}, nil
}

func (batchRouteKeyLookup) TouchLastUsed(string) error { return nil }

func TestSiatBatchesMountedWithTenantAuthentication(t *testing.T) {
	deliveryHttp.SetVerifyAPIKey(func(plain, hash string) bool { return plain == "sup_batch_valid" && hash == "test-hash" })
	t.Cleanup(func() { deliveryHttp.SetVerifyAPIKey(func(string, string) bool { return false }) })
	calls := 0
	check := func(companyID string) (*usecase.PaqueteResultado, error) {
		calls++
		if companyID != "comp-1" {
			t.Fatalf("tenant debe provenir de API key: %q", companyID)
		}
		return &usecase.PaqueteResultado{}, nil
	}
	validate := func(_ context.Context, companyID, posID string, input usecase.PaqueteValidacionInput) (*usecase.PaqueteResultado, error) {
		if posID != "" || input.BatchID != "batch-1" {
			t.Fatalf("validación debe usar batchId: pos=%q batch=%q", posID, input.BatchID)
		}
		return check(companyID)
	}
	svc := &mockSiatService{
		enviarMasivaFunc: func(_ context.Context, companyID, posID string, _ usecase.MasivaInput) (*usecase.PaqueteResultado, error) {
			if posID != "pos-1" {
				t.Fatalf("punto de venta=%q", posID)
			}
			return check(companyID)
		},
		enviarPaqueteFunc: func(_ context.Context, companyID, posID string, _ usecase.PaqueteInput) (*usecase.PaqueteResultado, error) {
			if posID != "pos-1" {
				t.Fatalf("punto de venta=%q", posID)
			}
			return check(companyID)
		},
		validarMasivaFunc: validate, validarPaqueteFunc: validate,
	}
	router := deliveryHttp.NewRouter(config.Config{}, []modules.Module{NewModule(svc, nil)}, batchRouteKeyLookup{}, func(http.ResponseWriter, *http.Request) {})
	for _, prefix := range []string{"", "/v1"} {
		for _, operation := range []string{"masiva", "paquete"} {
			for _, suffix := range []string{"/pos-1", "/comp-1/pos-1", "/batch-1/validate"} {
				path := prefix + "/siat/" + operation + suffix
				for _, tc := range []struct {
					name, key, query string
					status           int
				}{
					{"missing key", "", "", http.StatusUnauthorized},
					{"invalid key", "sup_batch_invalid", "", http.StatusUnauthorized},
					{"foreign query", "sup_batch_valid", "?company_id=other", http.StatusForbidden},
					{"duplicate foreign query", "sup_batch_valid", "?company_id=comp-1&company_id=other", http.StatusForbidden},
					{"valid key", "sup_batch_valid", "", http.StatusOK},
				} {
					t.Run(path+"/"+tc.name, func(t *testing.T) {
						before := calls
						req := httptest.NewRequest(http.MethodPost, path+tc.query, nil)
						req.Header.Set("X-API-Key", tc.key)
						req.Header.Set("X-Company-Id", "spoofed-tenant")
						rec := httptest.NewRecorder()
						router.ServeHTTP(rec, req)
						if rec.Code != tc.status {
							t.Fatalf("status=%d esperado=%d body=%s", rec.Code, tc.status, rec.Body.String())
						}
						wantCalls := before
						if tc.status == http.StatusOK {
							wantCalls++
						}
						if calls != wantCalls {
							t.Fatalf("llamadas usecase=%d esperadas=%d", calls, wantCalls)
						}
					})
				}
			}
		}
	}
}
