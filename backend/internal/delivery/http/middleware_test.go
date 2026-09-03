package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
