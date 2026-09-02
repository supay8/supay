package http

import (
	"encoding/json"
	"net/http"

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
		respondValidation(w, "payload JSON inválido")
		return
	}
	b, err := h.uc.Create(req)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (h *BranchHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.uc.List(r.URL.Query().Get("company_id"))
	if err != nil {
		respondError(w, err)
		return
	}
	respondList(w, list, len(list), 0, 0)
}

func (h *BranchHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondValidation(w, "el id es obligatorio")
		return
	}
	b, err := h.uc.GetByID(id)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (h *BranchHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondValidation(w, "el id es obligatorio")
		return
	}
	var req usecase.UpdateBranchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondValidation(w, "payload JSON inválido")
		return
	}
	b, err := h.uc.Update(req, id)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (h *BranchHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondValidation(w, "el id es obligatorio")
		return
	}
	if err := h.uc.Delete(id); err != nil {
		respondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
