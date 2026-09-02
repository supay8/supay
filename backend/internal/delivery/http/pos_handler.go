package http

import (
	"encoding/json"
	"net/http"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type PosHandler struct {
	usecase *usecase.PointOfSaleUsecase
}

func NewPosHandler(uc *usecase.PointOfSaleUsecase) *PosHandler {
	return &PosHandler{usecase: uc}
}

func (h *PosHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req usecase.RegisterPointOfSaleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondValidation(w, "payload JSON inválido")
		return
	}

	pos, err := h.usecase.Register(req)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, pos)
}

func (h *PosHandler) List(w http.ResponseWriter, r *http.Request) {
	pointsOfSale, err := h.usecase.List(r.URL.Query().Get("company_id"))
	if err != nil {
		respondError(w, err)
		return
	}
	if pointsOfSale == nil {
		pointsOfSale = []*domain.PointOfSale{}
	}
	respondList(w, pointsOfSale, len(pointsOfSale), 0, 0)
}

func (h *PosHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondValidation(w, "el id es obligatorio")
		return
	}

	pos, err := h.usecase.GetByID(id)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pos)
}

func (h *PosHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondValidation(w, "el id es obligatorio")
		return
	}

	var req usecase.UpdatePointOfSaleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondValidation(w, "payload JSON inválido")
		return
	}
	pos, err := h.usecase.Update(req, id)
	if err != nil {
		respondError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pos)
}

func (h *PosHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
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
