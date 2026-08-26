package http

import (
	"encoding/json"
	"net/http"

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
		respondValidation(w, "payload JSON inválido")
		return
	}

	company, err := h.usecase.Register(req)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, company)
}

func (h *CompanyHandler) GetByNit(w http.ResponseWriter, r *http.Request) {
	nit := r.URL.Query().Get("nit")
	if nit == "" {
		respondValidation(w, "el parámetro 'nit' es obligatorio")
		return
	}

	company, err := h.usecase.GetByNit(nit)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, company)
}

func (h *CompanyHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondValidation(w, "el id es obligatorio")
		return
	}

	var req usecase.UpdateCompanyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondValidation(w, "payload JSON inválido")
		return
	}
	company, err := h.usecase.Update(req, id)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, company)
}

func (h *CompanyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		id = r.URL.Query().Get("id") // compatibilidad: id como query param
	}
	if id == "" {
		respondValidation(w, "el id es obligatorio")
		return
	}

	if err := h.usecase.Delete(id); err != nil {
		respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
