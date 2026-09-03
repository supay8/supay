package pos

import (
	"encoding/json"
	"net/http"

	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type handler struct {
	usecase *usecase.PointOfSaleUsecase
}

func newHandler(uc *usecase.PointOfSaleUsecase) *handler {
	return &handler{usecase: uc}
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	var req usecase.RegisterPointOfSaleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		deliveryHttp.RespondValidation(w, "payload JSON inválido")
		return
	}

	pos, err := h.usecase.Register(req)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusCreated, pos)
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	pointsOfSale, err := h.usecase.List(r.URL.Query().Get("company_id"))
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	if pointsOfSale == nil {
		pointsOfSale = []*domain.PointOfSale{}
	}
	deliveryHttp.RespondList(w, pointsOfSale, len(pointsOfSale), 0, 0)
}

func (h *handler) getByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		deliveryHttp.RespondValidation(w, "el id es obligatorio")
		return
	}

	pos, err := h.usecase.GetByID(id)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, pos)
}

func (h *handler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		deliveryHttp.RespondValidation(w, "el id es obligatorio")
		return
	}

	var req usecase.UpdatePointOfSaleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		deliveryHttp.RespondValidation(w, "payload JSON inválido")
		return
	}
	pos, err := h.usecase.Update(req, id)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, pos)
}

func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		deliveryHttp.RespondValidation(w, "el id es obligatorio")
		return
	}

	if err := h.usecase.Delete(id); err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
