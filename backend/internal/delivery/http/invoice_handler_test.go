package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brandsrx/supay/internal/siat"
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
