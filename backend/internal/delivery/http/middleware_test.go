package http

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/models"
)

type fakeApiKeyLookup struct {
	key *models.ApiKey
	err error
}

func (f *fakeApiKeyLookup) FindByPrefix(prefix string) (*models.ApiKey, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.key != nil && f.key.KeyPrefix == prefix {
		return f.key, nil
	}
	return nil, errRecordNotFound()
}

func (f *fakeApiKeyLookup) TouchLastUsed(id string) error { return nil }

func TestTenantMiddlewareAcceptsValidApiKey(t *testing.T) {
	oldVerify := verifyAPIKey
	verifyAPIKey = func(plain, hash string) bool { return true }
	defer func() { verifyAPIKey = oldVerify }()

	lookup := &fakeApiKeyLookup{
		key: &models.ApiKey{
			ID:        "key-1",
			CompanyId: "comp-1",
			KeyPrefix: "sup_live_abc",
			KeyHash:   "hash",
			IsActive:  true,
		},
	}

	handler := TenantMiddleware(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := CompanyIDFromContext(r.Context())
		if !ok || id != "comp-1" {
			t.Errorf("esperaba company_id comp-1, got %s ok=%v", id, ok)
		}
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "sup_live_abc_xxxxxxxx")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperaba 200, got %d", rec.Code)
	}
}

func TestTenantMiddlewareRejectsInvalidApiKey(t *testing.T) {
	lookup := &fakeApiKeyLookup{err: errRecordNotFound()}

	handler := TenantMiddleware(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no debería llegar al handler")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "sup_invalid")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperaba 401, got %d", rec.Code)
	}
}

func TestTenantMiddlewareRejectsMissingHeader(t *testing.T) {
	lookup := &fakeApiKeyLookup{}

	handler := TenantMiddleware(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no debería llegar al handler")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperaba 401, got %d", rec.Code)
	}
}

func TestExtractKeyPrefix(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"sup_live_abc_xxxxxxxx", "sup_live_abc"},
		{"sup_test_123", "sup_test"},
		{"single", "single"},
	}
	for _, c := range cases {
		got := extractKeyPrefix(c.in)
		if got != c.want {
			t.Errorf("extractKeyPrefix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// errRecordNotFound simula un error de registro no encontrado sin importar gorm.
func errRecordNotFound() error { return http.ErrNoLocation }

func TestInternalBootstrapMiddleware(t *testing.T) {
	called := false
	handler := InternalBootstrapMiddleware("backend-secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, httptest.NewRequest(http.MethodPost, "/internal/companies", nil))
	if denied.Code != http.StatusUnauthorized || called {
		t.Fatalf("solicitud sin token: status=%d called=%v", denied.Code, called)
	}

	allowed := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/companies", nil)
	req.Header.Set("X-Backend-Token", "backend-secret")
	handler.ServeHTTP(allowed, req)
	if allowed.Code != http.StatusNoContent || !called {
		t.Fatalf("solicitud autenticada: status=%d called=%v", allowed.Code, called)
	}
}

func TestLimitBodyRejectsOversizedPayload(t *testing.T) {
	handler := LimitBody(4)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err == nil {
			t.Error("se esperaba MaxBytesError")
		}
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("12345")))
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d", recorder.Code)
	}
}

func TestRateLimitIPIsolatesClientsAndReturnsActionableError(t *testing.T) {
	handler := RateLimitIP(1, 1, 0)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := func(ipHeader, ip string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		if ipHeader != "" {
			req.Header.Set(ipHeader, ip)
		}
		handler.ServeHTTP(recorder, req)
		return recorder
	}

	if got := request("X-Forwarded-For", "203.0.113.1, 10.0.0.1").Code; got != http.StatusNoContent {
		t.Fatalf("primer cliente status=%d", got)
	}
	limited := request("X-Forwarded-For", "203.0.113.1")
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("segundo request status=%d", limited.Code)
	}
	var env errorEnvelope
	if err := json.Unmarshal(limited.Body.Bytes(), &env); err != nil || env.Error.Code != CodeRateLimited || env.Error.Action == "" {
		t.Fatalf("error de rate limit no accionable: %s err=%v", limited.Body.String(), err)
	}
	if got := request("X-Real-IP", "203.0.113.2").Code; got != http.StatusNoContent {
		t.Fatalf("otro cliente quedó bloqueado: status=%d", got)
	}
}

func TestTokenBucketRefillsAndHelpers(t *testing.T) {
	bucket := newTokenBucket(1000, 2)
	if !bucket.take(1000, 2) || !bucket.take(1000, 2) || bucket.take(1000, 2) {
		t.Fatal("burst inesperado")
	}
	bucket.lastRefill = time.Now().Add(-time.Second)
	if !bucket.take(1000, 2) {
		t.Fatal("el bucket no repuso tokens")
	}
	SetVerifyAPIKey(func(plain, hash string) bool { return plain == hash })
	defer SetVerifyAPIKey(func(string, string) bool { return false })
	if !verifyAPIKey("same", "same") {
		t.Fatal("SetVerifyAPIKey no instaló el verificador")
	}
	cases := []struct {
		raw    string
		fallback int
		want   int
	}{
		{"12", 99, 12},
		{"-1", 8, 8},
		{"bad", 7, 7},
	}
	for _, c := range cases {
		if got := ParseQueryInt(c.raw, c.fallback); got != c.want {
			t.Errorf("ParseQueryInt(%q)=%d want=%d", c.raw, got, c.want)
		}
	}
}
