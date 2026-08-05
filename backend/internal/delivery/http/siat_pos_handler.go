package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/brandsrx/supay/internal/siat"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type SiatPosHandler struct {
	siatManager *usecase.SiatManager
}

func NewSiatPosHandler(m *usecase.SiatManager) *SiatPosHandler {
	return &SiatPosHandler{siatManager: m}
}

func (h *SiatPosHandler) GenerateCuis(w http.ResponseWriter, r *http.Request) {
	posID := chi.URLParam(r, "id")
	if posID == "" {
		http.Error(w, "id obligatorio", http.StatusBadRequest)
		return
	}
	var req siat.SolicitudCuis
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "payload inválido", http.StatusBadRequest)
		return
	}

	c, err := h.siatManager.EnsureActiveCuis(context.Background(), posID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c)
}
