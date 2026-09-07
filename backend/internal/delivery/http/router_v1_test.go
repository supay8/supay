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
