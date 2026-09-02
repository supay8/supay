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
	"github.com/brandsrx/supay/internal/siat"
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
			wantCode:    codeValidation,
			wantMessage: "el nit es obligatorio",
		},
		{
			name:        "not found tipado",
			err:         domain.NewNotFoundError("empresa no encontrada"),
			wantStatus:  http.StatusNotFound,
			wantCode:    codeNotFound,
			wantMessage: "empresa no encontrada",
		},
		{
			name:        "conflict tipado",
			err:         domain.NewConflictError("no hay un cufd vigente"),
			wantStatus:  http.StatusConflict,
			wantCode:    codeConflict,
			wantMessage: "no hay un cufd vigente",
		},
		{
			name:       "conflicto centinela de dominio (documento de cliente)",
			err:        domain.ErrCustomerDocumentConflict,
			wantStatus: http.StatusConflict,
			wantCode:   codeConflict,
		},
		{
			name:       "conflicto centinela de dominio (dependencias de POS)",
			err:        domain.ErrPointOfSaleHasDependencies,
			wantStatus: http.StatusConflict,
			wantCode:   codeConflict,
		},
		{
			name:       "gorm not found",
			err:        gorm.ErrRecordNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   codeNotFound,
		},
		{
			name:       "siat no disponible",
			err:        usecase.ErrSiatNoDisponible,
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   codeSiatUnavailable,
		},
		{
			name: "rechazo del siat con detalles",
			err: &usecase.EmissionRejectedError{
				CodigoEstado: 902,
				Mensajes: []siat.Mensaje{
					{Codigo: 926, Descripcion: "CUF duplicado"},
					{Codigo: 931, Descripcion: "sector no soportado"},
				},
			},
			wantStatus:  http.StatusUnprocessableEntity,
			wantCode:    codeSiatRejected,
			wantDetails: 2,
		},
		{
			name:        "error no tipado es interno y no filtra detalle",
			err:         errors.New("sql: connection refused"),
			wantStatus:  http.StatusInternalServerError,
			wantCode:    codeInternal,
			wantMessage: "error interno del servidor",
		},
		{
			name:        "error tipado envuelto se desenreda",
			err:         fmt.Errorf("emisión: %w", domain.NewBadRequestError("dato inválido")),
			wantStatus:  http.StatusBadRequest,
			wantCode:    codeValidation,
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
	respondError(rec, domain.NewBadRequestError("el sku es obligatorio"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
	want := `{"error":{"code":"VALIDATION_ERROR","message":"el sku es obligatorio"}}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body=%s, se esperaba %s", got, want)
	}
}

func TestRespondErrorSiatRejectedConDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	respondError(rec, &usecase.EmissionRejectedError{
		Mensajes: []siat.Mensaje{{Codigo: 926, Descripcion: "CUF duplicado"}},
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
	if env.Error.Code != codeSiatRejected || len(env.Error.Details) != 1 {
		t.Fatalf("envelope inesperado: %+v", env)
	}
	if env.Error.Details[0].Code != 926 || env.Error.Details[0].Message != "CUF duplicado" {
		t.Fatalf("detail inesperado: %+v", env.Error.Details[0])
	}
}

func TestRespondErrorInternoNoFiltroDetalle(t *testing.T) {
	rec := httptest.NewRecorder()
	respondError(rec, errors.New("panic en repositorio: SELECT * FROM..."))

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
		respondList(rec, items, 0, 0, 0)
		want := `{"items":[],"total":0}`
		if got := strings.TrimSpace(rec.Body.String()); got != want {
			t.Fatalf("body=%s, se esperaba %s", got, want)
		}
	})

	t.Run("listado paginado incluye limit y offset", func(t *testing.T) {
		rec := httptest.NewRecorder()
		respondList(rec, []int{1, 2}, 10, 2, 4)
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

func TestRequireAPIKeyEnvelope(t *testing.T) {
	handler := RequireAPIKey("secreto")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("sin key", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d", rec.Code)
		}
		want := `{"error":{"code":"UNAUTHORIZED","message":"no autorizado: falta o es inválido el header X-API-Key"}}`
		if got := strings.TrimSpace(rec.Body.String()); got != want {
			t.Fatalf("body=%s", got)
		}
	})

	t.Run("key válida pasa", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-API-Key", "secreto")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}
