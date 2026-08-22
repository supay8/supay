package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type CompanyHandler struct {
	usecase *usecase.CompanyUsecase
}

func NewCompanyHandler(uc *usecase.CompanyUsecase) *CompanyHandler {
	return &CompanyHandler{usecase: uc}
}

func (h *CompanyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req usecase.RegisterCompanyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Payload JSON inválido")
		return
	}

	company, err := h.usecase.Register(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(company)
}

func (h *CompanyHandler) GetByNit(w http.ResponseWriter, r *http.Request) {
	nit := r.URL.Query().Get("nit")
	if nit == "" {
		writeJSONError(w, http.StatusBadRequest, "El parámetro 'nit' es obligatorio")
		return
	}

	company, err := h.usecase.GetByNit(nit)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Empresa no encontrada"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(company)
}

func (h *CompanyHandler) Update(w http.ResponseWriter, r *http.Request) {
	// Ejemplo usando Chi para obtener el ID de la URL: /companies/{id}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "El ID es obligatorio")
		return
	}

	var req usecase.UpdateCompanyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Payload JSON inválido")
		return
	}
	company, err := h.usecase.Update(req, id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(company)
}

func (h *CompanyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	// Obteniendo el ID de la URL o por query parameter
	id := chi.URLParam(r, "id")
	if id == "" {
		id = r.URL.Query().Get("id") // Fallback por si lo mandan como query
	}

	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "El parámetro 'id' es obligatorio")
		return
	}

	err := h.usecase.Delete(id)
	if err != nil {
		if errors.Is(err, domain.ErrCompanyHasDependencies) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Empresa no encontrada o no se pudo eliminar"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Empresa eliminada exitosamente"})
}
