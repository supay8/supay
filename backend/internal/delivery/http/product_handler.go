package http

import (
	"encoding/json"
	"net/http"

	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type ProductHandler struct {
	uc *usecase.ProductUsecase
}

func NewProductHandler(uc *usecase.ProductUsecase) *ProductHandler {
	return &ProductHandler{uc: uc}
}

func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req usecase.CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondValidation(w, "payload JSON inválido")
		return
	}
	product, err := h.uc.Create(req)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, product)
}

func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	products, err := h.uc.List(r.URL.Query().Get("company_id"))
	if err != nil {
		respondError(w, err)
		return
	}
	respondList(w, products, len(products), 0, 0)
}

func (h *ProductHandler) AddMapping(w http.ResponseWriter, r *http.Request) {
	var req usecase.ProductMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondValidation(w, "payload JSON inválido")
		return
	}
	companyID := r.URL.Query().Get("company_id")
	if companyID == "" {
		respondValidation(w, "company_id es obligatorio")
		return
	}
	if err := h.uc.AddMapping(companyID, chi.URLParam(r, "id"), req); err != nil {
		respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
