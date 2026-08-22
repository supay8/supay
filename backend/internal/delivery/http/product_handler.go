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
		writeJSONError(w, http.StatusBadRequest, "payload JSON inválido")
		return
	}
	product, err := h.uc.Create(req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(product)
}

func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	products, err := h.uc.List(r.URL.Query().Get("companyId"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(products)
}

func (h *ProductHandler) AddMapping(w http.ResponseWriter, r *http.Request) {
	var req usecase.ProductMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "payload JSON inválido")
		return
	}
	companyID := r.URL.Query().Get("companyId")
	if err := h.uc.AddMapping(companyID, chi.URLParam(r, "id"), req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
