package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type BranchHandler struct {
	uc *usecase.BranchUsecase
}

func NewBranchHandler(uc *usecase.BranchUsecase) *BranchHandler {
	return &BranchHandler{uc: uc}
}

func (h *BranchHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req usecase.CreateBranchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Payload inválido")
		return
	}
	b, err := h.uc.Create(req)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrBranchSucursalConflict) {
			status = http.StatusConflict
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(b)
}

func (h *BranchHandler) List(w http.ResponseWriter, r *http.Request) {
	companyID := r.URL.Query().Get("companyId")
	list, err := h.uc.List(companyID)
	if err != nil {
		respondError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (h *BranchHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "id obligatorio")
		return
	}
	b, err := h.uc.GetByID(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(b)
}

func (h *BranchHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req usecase.UpdateBranchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Payload inválido")
		return
	}
	b, err := h.uc.Update(req, id)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(b)
}

func (h *BranchHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "id obligatorio")
		return
	}
	if err := h.uc.Delete(id); err != nil {
		respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
