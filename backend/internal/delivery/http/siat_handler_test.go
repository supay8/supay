package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestSetupValidaBody(t *testing.T) {
	h := &SiatHandler{}
	r := chi.NewRouter()
	r.Post("/setup", h.Setup)

	casos := []struct {
		name string
		body string
	}{
		{"sin body", ""},
		{"body vacío json", "{}"},
		{"solo company_id", `{"company_id":"c1"}`},
		{"solo point_of_sale_id", `{"point_of_sale_id":"p1"}`},
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
