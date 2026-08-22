package http

import (
	"encoding/json"
	"errors"
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
		writeJSONError(w, http.StatusBadRequest, "Payload JSON inválido")
		return
	}

	pos, err := h.usecase.Register(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(pos)
}

func (h *PosHandler) List(w http.ResponseWriter, r *http.Request) {
	companyID := r.URL.Query().Get("companyId")

	pointsOfSale, err := h.usecase.List(companyID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Error al obtener los puntos de venta"})
		return
	}

	if pointsOfSale == nil {
		pointsOfSale = []*domain.PointOfSale{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(pointsOfSale)
}

func (h *PosHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "El ID es obligatorio")
		return
	}

	pos, err := h.usecase.GetByID(id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Punto de venta no encontrado"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(pos)
}

func (h *PosHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "El ID es obligatorio")
		return
	}

	var req usecase.UpdatePointOfSaleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Payload JSON inválido")
		return
	}
	pos, err := h.usecase.Update(req, id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(pos)
}

func (h *PosHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "El parámetro 'id' es obligatorio")
		return
	}

	err := h.usecase.Delete(id)
	if err != nil {
		if errors.Is(err, domain.ErrPointOfSaleHasDependencies) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Punto de venta no encontrado o no se pudo eliminar"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Punto de venta eliminado exitosamente"})
}
