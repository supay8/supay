package http

import (
	"encoding/json"
	"net/http"

	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type CustomerHandler struct {
	uc *usecase.CustomerUsecase
}

func NewCustomerHandler(uc *usecase.CustomerUsecase) *CustomerHandler {
	return &CustomerHandler{uc: uc}
}

func (h *CustomerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req usecase.CreateCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondValidation(w, "payload JSON inválido")
		return
	}
	c, err := h.uc.Create(req)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (h *CustomerHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.uc.List(r.URL.Query().Get("company_id"))
	if err != nil {
		respondError(w, err)
		return
	}
	respondList(w, list, len(list), 0, 0)
}

func (h *CustomerHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondValidation(w, "el id es obligatorio")
		return
	}
	c, err := h.uc.GetByID(id)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}
