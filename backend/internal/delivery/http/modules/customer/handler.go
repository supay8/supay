package customer

import (
	"encoding/json"
	"net/http"

	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type handler struct {
	uc *usecase.CustomerUsecase
}

func newHandler(uc *usecase.CustomerUsecase) *handler {
	return &handler{uc: uc}
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	var req usecase.CreateCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		deliveryHttp.RespondValidation(w, "payload JSON inválido")
		return
	}
	c, err := h.uc.Create(req)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusCreated, c)
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	list, err := h.uc.List(r.URL.Query().Get("company_id"))
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.RespondList(w, list, len(list), 0, 0)
}

func (h *handler) getByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		deliveryHttp.RespondValidation(w, "el id es obligatorio")
		return
	}
	c, err := h.uc.GetByID(id)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, c)
}
