package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type InvoiceHandler struct {
	uc *usecase.InvoiceUsecase
}

func NewInvoiceHandler(uc *usecase.InvoiceUsecase) *InvoiceHandler {
	return &InvoiceHandler{uc: uc}
}

func (h *InvoiceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req usecase.CreateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "payload JSON inválido")
		return
	}
	inv, err := h.uc.Create(req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(inv)
}

func (h *InvoiceHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "id obligatorio")
		return
	}
	inv, err := h.uc.GetByID(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inv)
}

func (h *InvoiceHandler) ListByPointOfSale(w http.ResponseWriter, r *http.Request) {
	pointOfSaleID := r.URL.Query().Get("pointOfSaleId")
	if pointOfSaleID == "" {
		writeJSONError(w, http.StatusBadRequest, "pointOfSaleId es obligatorio")
		return
	}
	list, err := h.uc.ListByPointOfSale(pointOfSaleID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (h *InvoiceHandler) Emit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "id obligatorio")
		return
	}
	inv, err := h.uc.Emit(r.Context(), id)
	if err != nil {
		var rejected *usecase.EmissionRejectedError
		if errors.As(err, &rejected) {
			writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeJSONError(w, http.StatusConflict, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inv)
}

func (h *InvoiceHandler) SiatStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "id obligatorio")
		return
	}
	inv, err := h.uc.VerifyStatus(r.Context(), id)
	if err != nil {
		writeJSONError(w, http.StatusConflict, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inv)
}

func (h *InvoiceHandler) Annul(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "id obligatorio")
		return
	}
	var req struct {
		CodigoMotivo int `json:"codigo_motivo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "payload JSON inválido")
		return
	}
	inv, err := h.uc.Annul(r.Context(), id, req.CodigoMotivo)
	if err != nil {
		var rejected *usecase.EmissionRejectedError
		if errors.As(err, &rejected) {
			writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeJSONError(w, http.StatusConflict, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inv)
}

func (h *InvoiceHandler) RevertAnnul(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "id obligatorio")
		return
	}
	inv, err := h.uc.RevertAnnul(r.Context(), id)
	if err != nil {
		var rejected *usecase.EmissionRejectedError
		if errors.As(err, &rejected) {
			writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeJSONError(w, http.StatusConflict, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(inv)
}
