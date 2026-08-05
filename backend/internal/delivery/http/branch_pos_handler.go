package http

import (
	"encoding/json"
	"net/http"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type BranchPosHandler struct {
	provisionSvc *usecase.PointOfSaleProvisionService
}

func NewBranchPosHandler(svc *usecase.PointOfSaleProvisionService) *BranchPosHandler {
	return &BranchPosHandler{provisionSvc: svc}
}

func (h *BranchPosHandler) Create(w http.ResponseWriter, r *http.Request) {
	branchID := chi.URLParam(r, "branchId")
	if branchID == "" {
		http.Error(w, "branchId obligatorio", http.StatusBadRequest)
		return
	}
	var input usecase.CreatePOSInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "payload invalido", http.StatusBadRequest)
		return
	}
	if input.Name == "" {
		http.Error(w, "name es obligatorio", http.StatusBadRequest)
		return
	}

	pos, err := h.provisionSvc.CreateOperationalPOS(r.Context(), branchID, input)
	if err != nil {
		status := http.StatusBadRequest
		if pos == nil {
			status = http.StatusUnprocessableEntity
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error(), "point_of_sale": pos})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(pos)
}

func (h *BranchPosHandler) ListByBranch(w http.ResponseWriter, r *http.Request) {
	branchID := chi.URLParam(r, "branchId")
	if branchID == "" {
		http.Error(w, "branchId obligatorio", http.StatusBadRequest)
		return
	}

	list, err := h.provisionSvc.ListByBranch(branchID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if list == nil {
		list = []*domain.PointOfSale{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (h *BranchPosHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	posID := chi.URLParam(r, "id")
	if posID == "" {
		http.Error(w, "id obligatorio", http.StatusBadRequest)
		return
	}

	st, err := h.provisionSvc.GetOperationalStatus(posID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st)
}

// SyncTipoPuntoVenta sincroniza el catálogo oficial de tipos de punto de venta
// de una empresa (POST /companies/{companyId}/siat/sync/tipo-punto-venta).
func (h *BranchPosHandler) SyncTipoPuntoVenta(w http.ResponseWriter, r *http.Request) {
	companyID := chi.URLParam(r, "companyId")
	if companyID == "" {
		http.Error(w, "companyId obligatorio", http.StatusBadRequest)
		return
	}
	var input struct {
		CodigoSucursal int `json:"codigo_sucursal"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)

	tipos, err := h.provisionSvc.SyncTipoPuntoVentaCatalog(r.Context(), companyID, input.CodigoSucursal)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(tipos)
}
