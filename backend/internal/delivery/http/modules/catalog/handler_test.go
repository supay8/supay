package catalog

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/config"
	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	"github.com/brandsrx/supay/internal/delivery/http/modules"
	"github.com/brandsrx/supay/internal/delivery/http/modules/siat"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/usecase"
)

type httpCatalogRepo struct {
	items map[string][]*domain.CatalogItem
}

func (r *httpCatalogRepo) Replace(string, string, []domain.CatalogItem, time.Time) error { return nil }
func (r *httpCatalogRepo) List(_ string, tipo string) ([]*domain.CatalogItem, error) {
	return r.items[tipo], nil
}
func (r *httpCatalogRepo) ListAll(string) (map[string][]*domain.CatalogItem, error) {
	return r.items, nil
}

type httpActividadRepo struct {
	items []domain.SiatActividad
}

func (r *httpActividadRepo) Replace(string, []domain.SiatActividad, time.Time) error { return nil }
func (r *httpActividadRepo) List(string) ([]*domain.SiatActividad, error) {
	out := make([]*domain.SiatActividad, 0, len(r.items))
	for i := range r.items {
		out = append(out, &r.items[i])
	}
	return out, nil
}

func TestCatalogRoutesDomainSlugs(t *testing.T) {
	cfg := config.Load()
	uc := usecase.NewSiatUsecase(nil, nil, nil, nil, &httpCatalogRepo{items: map[string][]*domain.CatalogItem{
		"tipoMoneda": {{Codigo: 1, Descripcion: "BOLIVIANO", Tipo: "tipoMoneda"}},
	}}, nil, nil, nil, 0, nil, nil, &httpActividadRepo{items: []domain.SiatActividad{
		{CodigoCaeb: "620100", Descripcion: "Software", TipoActividad: "P"},
	}}, nil, nil, nil, nil)
	router := deliveryHttp.NewRouter(cfg, []modules.Module{
		NewModule(uc),
		siat.NewModule(uc, nil),
	}, nil, nil)

	t.Run("parametrico", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/companies/comp-1/catalogs/tipos-moneda", nil)
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var body usecase.CatalogItemsResult
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Catalog != "tipos-moneda" || body.Total != 1 {
			t.Fatalf("body=%+v", body)
		}
	})

	t.Run("actividades", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/companies/comp-1/catalogs/actividades-economicas", nil)
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var body usecase.ActividadesEconomicasResult
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Total != 1 {
			t.Fatalf("body=%+v", body)
		}
	})

	t.Run("perfiles", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/catalogs/perfiles-documento-sector", nil)
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("slug desconocido", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/companies/comp-1/catalogs/no-existe", nil)
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}
