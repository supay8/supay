package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brandsrx/supay/internal/config"
	"github.com/brandsrx/supay/internal/delivery/http/modules"
	"github.com/go-chi/chi/v5"
)

type versionedTestModule struct{}

func (versionedTestModule) PathPrefix() string { return "/probe" }
func (versionedTestModule) RegisterRoutes(r chi.Router) {
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
}
func (versionedTestModule) RegisterV1Routes(r chi.Router) {
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) })
}

func TestRouterExponeModulosBajoV1(t *testing.T) {
	registered := []modules.Module{versionedTestModule{}}
	router := NewRouter(config.Config{}, registered, nil, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) })

	legacy := httptest.NewRecorder()
	router.ServeHTTP(legacy, httptest.NewRequest(http.MethodGet, "/probe", nil))
	if legacy.Code != http.StatusNoContent {
		t.Fatalf("legacy status=%d", legacy.Code)
	}

	v1 := httptest.NewRecorder()
	router.ServeHTTP(v1, httptest.NewRequest(http.MethodGet, "/v1/probe", nil))
	if v1.Code != http.StatusAccepted {
		t.Fatalf("v1 status=%d", v1.Code)
	}
	if got := v1.Header().Get("X-API-Version"); got != "v1" {
		t.Fatalf("X-API-Version=%q", got)
	}
}

func TestRouterExponeHealthYBootstrapVersionados(t *testing.T) {
	router := NewRouter(config.Config{}, nil, nil, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) })

	health := httptest.NewRecorder()
	router.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/v1/health", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status=%d", health.Code)
	}
}

type companyRouteTestModule struct{}

func (companyRouteTestModule) PathPrefix() string { return "/companies" }
func (companyRouteTestModule) RegisterRoutes(r chi.Router) {
	r.Get("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Patch("/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Delete("/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
}

func TestRouterSoloPermiteCrearCompanyDesdeBootstrapInterno(t *testing.T) {
	router := NewRouter(config.Config{}, []modules.Module{companyRouteTestModule{}}, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	publicCreate := httptest.NewRecorder()
	router.ServeHTTP(publicCreate, httptest.NewRequest(http.MethodPost, "/companies", nil))
	if publicCreate.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /companies status=%d, se esperaba 405", publicCreate.Code)
	}

	internalCreate := httptest.NewRecorder()
	router.ServeHTTP(internalCreate, httptest.NewRequest(http.MethodPost, "/internal/companies", nil))
	if internalCreate.Code != http.StatusCreated {
		t.Fatalf("POST /internal/companies status=%d", internalCreate.Code)
	}
}
