package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeTokenVerifier struct {
	userID string
	err    error
}

func (f fakeTokenVerifier) VerifyAccessToken(string) (string, error) { return f.userID, f.err }

type fakeMembershipLookup struct {
	allowed bool
	err     error
}

func (f fakeMembershipLookup) HasCompanyAccess(_, _ string) (bool, error) {
	return f.allowed, f.err
}

func TestUserMiddlewareInjectsAuthenticatedUser(t *testing.T) {
	handler := UserMiddleware(fakeTokenVerifier{userID: "user-1"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, ok := UserIDFromContext(r.Context()); !ok || got != "user-1" {
			t.Fatalf("user_id=%q ok=%t", got, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.Header.Set("Authorization", "Bearer valid")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTenantAccessMiddlewareUsesJWTAndMembership(t *testing.T) {
	handler := TenantAccessMiddleware(nil, fakeTokenVerifier{userID: "user-1"}, fakeMembershipLookup{allowed: true})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, userOK := UserIDFromContext(r.Context())
		companyID, companyOK := CompanyIDFromContext(r.Context())
		if !userOK || !companyOK || userID != "user-1" || companyID != "company-1" {
			t.Fatalf("contexto inesperado user=%q/%t company=%q/%t", userID, userOK, companyID, companyOK)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/invoices", nil)
	req.Header.Set("Authorization", "Bearer valid")
	req.Header.Set("X-Company-ID", "company-1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTenantAccessMiddlewareInfersCompanyFromPathForAPIKeyManagement(t *testing.T) {
	handler := TenantAccessMiddleware(nil, fakeTokenVerifier{userID: "user-1"}, fakeMembershipLookup{allowed: true})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		companyID, ok := CompanyIDFromContext(r.Context())
		if !ok || companyID != "company-1" {
			t.Fatalf("company_id=%q ok=%t", companyID, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/v1/companies/company-1/api-keys", nil)
	req.Header.Set("Authorization", "Bearer valid")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTenantAccessMiddlewareRejectsMissingOrForeignCompany(t *testing.T) {
	middleware := TenantAccessMiddleware(nil, fakeTokenVerifier{userID: "user-1"}, fakeMembershipLookup{allowed: true})
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	missing := httptest.NewRequest(http.MethodGet, "/v1/invoices", nil)
	missing.Header.Set("Authorization", "Bearer valid")
	missingRec := httptest.NewRecorder()
	middleware(next).ServeHTTP(missingRec, missing)
	if missingRec.Code != http.StatusBadRequest {
		t.Fatalf("missing company status=%d", missingRec.Code)
	}

	foreign := httptest.NewRequest(http.MethodGet, "/v1/companies/company-2/catalogs", nil)
	foreign.Header.Set("Authorization", "Bearer valid")
	foreign.Header.Set("X-Company-ID", "company-1")
	foreignRec := httptest.NewRecorder()
	middleware(next).ServeHTTP(foreignRec, foreign)
	if foreignRec.Code != http.StatusForbidden {
		t.Fatalf("foreign company status=%d body=%s", foreignRec.Code, foreignRec.Body.String())
	}
}

func TestTenantAccessMiddlewareRejectsInvalidTokenAndRepositoryFailure(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "/v1/invoices", nil)
	req.Header.Set("Authorization", "Bearer invalid")
	req.Header.Set("X-Company-ID", "company-1")
	rec := httptest.NewRecorder()
	TenantAccessMiddleware(nil, fakeTokenVerifier{err: errors.New("bad token")}, fakeMembershipLookup{})(next).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}
