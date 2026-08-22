package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/brandsrx/supay/internal/domain"
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
		writeJSONError(w, http.StatusBadRequest, "Payload inválido")
		return
	}
	c, err := h.uc.Create(req)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrCustomerDocumentConflict) {
			status = http.StatusConflict
		}
		writeJSONError(w, status, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(c)
}

func (h *CustomerHandler) List(w http.ResponseWriter, r *http.Request) {
	companyID := r.URL.Query().Get("companyId")
	list, err := h.uc.List(companyID)
	if err != nil {
		respondError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (h *CustomerHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "id obligatorio")
		return
	}
	c, err := h.uc.GetByID(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
