package product

import (
	"encoding/json"
	"net/http"

	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type handler struct {
	uc *usecase.ProductUsecase
}

func newHandler(uc *usecase.ProductUsecase) *handler {
	return &handler{uc: uc}
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	var req usecase.CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		deliveryHttp.RespondValidation(w, "payload JSON inválido")
		return
	}
	product, err := h.uc.Create(req)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusCreated, product)
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	products, err := h.uc.List(r.URL.Query().Get("company_id"))
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.RespondList(w, products, len(products), 0, 0)
}

func (h *handler) addMapping(w http.ResponseWriter, r *http.Request) {
	var req usecase.ProductMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		deliveryHttp.RespondValidation(w, "payload JSON inválido")
		return
	}
	companyID := r.URL.Query().Get("company_id")
	if companyID == "" {
		deliveryHttp.RespondValidation(w, "company_id es obligatorio")
		return
	}
	if err := h.uc.AddMapping(companyID, chi.URLParam(r, "id"), req); err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
