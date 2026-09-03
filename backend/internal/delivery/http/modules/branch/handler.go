package branch

import (
	"encoding/json"
	"net/http"

	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type handler struct {
	uc *usecase.BranchUsecase
}

func newHandler(uc *usecase.BranchUsecase) *handler {
	return &handler{uc: uc}
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	var req usecase.CreateBranchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		deliveryHttp.RespondValidation(w, "payload JSON inválido")
		return
	}
	b, err := h.uc.Create(req)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusCreated, b)
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
	b, err := h.uc.GetByID(id)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, b)
}

func (h *handler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		deliveryHttp.RespondValidation(w, "el id es obligatorio")
		return
	}
	var req usecase.UpdateBranchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		deliveryHttp.RespondValidation(w, "payload JSON inválido")
		return
	}
	b, err := h.uc.Update(req, id)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, b)
}

func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		deliveryHttp.RespondValidation(w, "el id es obligatorio")
		return
	}
	if err := h.uc.Delete(id); err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
