package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/brandsrx/supay/internal/ports"
	"github.com/brandsrx/supay/internal/usecase"
	"gorm.io/gorm"
)

func TestClassifyError(t *testing.T) {
	cases := []struct {
		name        string
		err         error
		wantStatus  int
		wantCode    string
		wantMessage string
		wantDetails int
	}{
		{
			name:        "bad request tipado",
			err:         domain.NewBadRequestError("el nit es obligatorio"),
			wantStatus:  http.StatusBadRequest,
			wantCode:    CodeValidation,
			wantMessage: "el nit es obligatorio",
		},
		{
			name:        "not found tipado",
			err:         domain.NewNotFoundError("empresa no encontrada"),
			wantStatus:  http.StatusNotFound,
			wantCode:    CodeNotFound,
			wantMessage: "empresa no encontrada",
		},
		{
			name:        "conflict tipado",
			err:         domain.NewConflictError("no hay un cufd vigente"),
			wantStatus:  http.StatusConflict,
			wantCode:    CodeConflict,
			wantMessage: "no hay un cufd vigente",
		},
		{
			name:       "conflicto centinela de dominio (dependencias de POS)",
			err:        domain.ErrPointOfSaleHasDependencies,
			wantStatus: http.StatusConflict,
			wantCode:   CodeConflict,
		},
		{
			name:       "gorm not found",
			err:        gorm.ErrRecordNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   CodeNotFound,
		},
		{
			name:       "siat no disponible",
			err:        usecase.ErrSiatNoDisponible,
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   CodeSiatUnavailable,
		},
		{
			name: "rechazo del siat con detalles",
			err: &usecase.EmissionRejectedError{
				CodigoEstado: 902,
				Mensajes: []ports.FiscalMessage{
					{Codigo: 926, Descripcion: "CUF duplicado"},
					{Codigo: 931, Descripcion: "sector no soportado"},
				},
			},
			wantStatus:  http.StatusUnprocessableEntity,
			wantCode:    CodeSiatRejected,
			wantDetails: 2,
		},
		{
			name:        "error no tipado es interno y no filtra detalle",
			err:         errors.New("sql: connection refused"),
			wantStatus:  http.StatusInternalServerError,
			wantCode:    CodeInternal,
			wantMessage: "error interno del servidor",
		},
		{
			name:        "error tipado envuelto se desenreda",
			err:         fmt.Errorf("emisión: %w", domain.NewBadRequestError("dato inválido")),
			wantStatus:  http.StatusBadRequest,
			wantCode:    CodeValidation,
			wantMessage: "emisión: dato inválido",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := classifyError(tc.err)
			if status != tc.wantStatus {
				t.Fatalf("status=%d, se esperaba %d", status, tc.wantStatus)
			}
			if body.Code != tc.wantCode {
				t.Fatalf("code=%q, se esperaba %q", body.Code, tc.wantCode)
			}
			if tc.wantMessage != "" && body.Message != tc.wantMessage {
				t.Fatalf("message=%q, se esperaba %q", body.Message, tc.wantMessage)
			}
			if len(body.Details) != tc.wantDetails {
				t.Fatalf("details=%d, se esperaban %d", len(body.Details), tc.wantDetails)
			}
		})
	}
}

func TestRespondErrorEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	RespondError(rec, domain.NewBadRequestError("el sku es obligatorio"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
	var env errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("json inválido: %v", err)
	}
	if env.Error.Code != CodeValidation || env.Error.Field != "items[].sku" {
		t.Fatalf("clasificación inesperada: %+v", env.Error)
	}
	if len(env.Error.Suggestions) == 0 || env.Error.Action == "" {
		t.Fatalf("el error no enseña cómo recuperarse: %+v", env.Error)
	}
}

func TestRespondErrorSiatRejectedConDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	RespondError(rec, &usecase.EmissionRejectedError{
		Mensajes: []ports.FiscalMessage{{Codigo: 926, Descripcion: "CUF duplicado"}},
	})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d", rec.Code)
	}
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Details []struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("json inválido: %v", err)
	}
	if env.Error.Code != CodeSiatRejected || len(env.Error.Details) != 1 {
		t.Fatalf("envelope inesperado: %+v", env)
	}
	if env.Error.Details[0].Code != 926 || env.Error.Details[0].Message != "CUF duplicado" {
		t.Fatalf("detail inesperado: %+v", env.Error.Details[0])
	}
}

func TestRespondErrorInternoNoFiltroDetalle(t *testing.T) {
	rec := httptest.NewRecorder()
	RespondError(rec, errors.New("panic en repositorio: SELECT * FROM..."))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "SELECT") {
		t.Fatalf("el cuerpo filtró detalle interno: %s", rec.Body.String())
	}
}

func TestRespondList(t *testing.T) {
	t.Run("slice nil se serializa como lista vacía", func(t *testing.T) {
		rec := httptest.NewRecorder()
		var items []*domain.Customer // nil
		RespondList(rec, items, 0, 0, 0)
		want := `{"items":[],"total":0}`
		if got := strings.TrimSpace(rec.Body.String()); got != want {
			t.Fatalf("body=%s, se esperaba %s", got, want)
		}
	})

	t.Run("listado paginado incluye limit y offset", func(t *testing.T) {
		rec := httptest.NewRecorder()
		RespondList(rec, []int{1, 2}, 10, 2, 4)
		var body struct {
			Items  []int `json:"items"`
			Total  int   `json:"total"`
			Limit  int   `json:"limit"`
			Offset int   `json:"offset"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("json inválido: %v", err)
		}
		if body.Total != 10 || body.Limit != 2 || body.Offset != 4 || len(body.Items) != 2 {
			t.Fatalf("envelope inesperado: %+v", body)
		}
	})
}

func TestTenantMiddlewareEnvelope(t *testing.T) {
	oldVerify := verifyAPIKey
	verifyAPIKey = func(plain, hash string) bool { return plain == "sup_live_abc_xxxxxxxx" }
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
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("sin key", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d", rec.Code)
		}
		var env errorEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatalf("json inválido: %v", err)
		}
		if env.Error.Code != CodeUnauthorized || env.Error.Field != "X-API-Key" || env.Error.Action == "" {
			t.Fatalf("body=%s", strings.TrimSpace(rec.Body.String()))
		}
	})

	t.Run("key válida pasa", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-API-Key", "sup_live_abc_xxxxxxxx")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}
