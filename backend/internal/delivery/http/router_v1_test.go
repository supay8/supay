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

type authRoutesTest struct{}

func (authRoutesTest) RegisterPublicRoutes(r chi.Router) {
	r.Post("/login", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

func (authRoutesTest) RegisterProtectedRoutes(r chi.Router) {
	r.Get("/me", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
}

func TestRouterExponeAuthPublicoYProtegido(t *testing.T) {
	router := NewRouter(config.Config{}, nil, nil, nil, AuthOptions{
		Routes: authRoutesTest{}, Tokens: fakeTokenVerifier{userID: "user-1"},
	})

	login := httptest.NewRecorder()
	router.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil))
	if login.Code != http.StatusOK {
		t.Fatalf("login status=%d", login.Code)
	}

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("me sin token status=%d", unauthorized.Code)
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	meRequest.Header.Set("Authorization", "Bearer valid")
	me := httptest.NewRecorder()
	router.ServeHTTP(me, meRequest)
	if me.Code != http.StatusNoContent {
		t.Fatalf("me status=%d body=%s", me.Code, me.Body.String())
	}
}

type apiKeyManagementTestModule struct{}

func (apiKeyManagementTestModule) PathPrefix() string { return "/companies/{id}/api-keys" }
func (apiKeyManagementTestModule) RegisterRoutes(r chi.Router) {
	r.Post("/", func(w http.ResponseWriter, request *http.Request) {
		companyID, ok := CompanyIDFromContext(request.Context())
		if !ok || companyID != chi.URLParam(request, "id") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})
}

func TestJWTCanCreateFirstAPIKeyWithoutAPIKeyOrCompanyHeader(t *testing.T) {
	router := NewRouter(config.Config{}, []modules.Module{apiKeyManagementTestModule{}}, nil, nil, AuthOptions{
		Tokens:      fakeTokenVerifier{userID: "user-1"},
		Memberships: fakeMembershipLookup{allowed: true},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/companies/company-1/api-keys", nil)
	req.Header.Set("Authorization", "Bearer valid")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
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
