package company

import (
	"encoding/json"
	"net/http"

	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type handler struct {
	usecase *usecase.CompanyUsecase
}

func newHandler(uc *usecase.CompanyUsecase) *handler {
	return &handler{usecase: uc}
}

func (h *handler) getByNit(w http.ResponseWriter, r *http.Request) {
	nit := r.URL.Query().Get("nit")
	if nit == "" {
		deliveryHttp.RespondValidation(w, "el parámetro 'nit' es obligatorio")
		return
	}

	var company any
	var err error
	if companyID, ok := deliveryHttp.CompanyIDFromContext(r.Context()); ok {
		current, getErr := h.usecase.GetByID(companyID)
		if getErr == nil && current.Nit != nit {
			deliveryHttp.WriteErrorBody(w, http.StatusForbidden, deliveryHttp.CodeForbidden, "el NIT no pertenece a la empresa autenticada")
			return
		}
		company, err = current, getErr
	} else {
		company, err = h.usecase.GetByNit(nit)
	}
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, company)
}

func (h *handler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		deliveryHttp.RespondValidation(w, "el id es obligatorio")
		return
	}
	if companyID, ok := deliveryHttp.CompanyIDFromContext(r.Context()); ok && companyID != id {
		deliveryHttp.WriteErrorBody(w, http.StatusForbidden, deliveryHttp.CodeForbidden, "no puede modificar otra empresa")
		return
	}

	var req usecase.UpdateCompanyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		deliveryHttp.RespondValidation(w, "payload JSON inválido")
		return
	}
	company, err := h.usecase.Update(req, id)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	deliveryHttp.WriteJSON(w, http.StatusOK, company)
}

func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		id = r.URL.Query().Get("id")
	}
	if id == "" {
		deliveryHttp.RespondValidation(w, "el id es obligatorio")
		return
	}
	if companyID, ok := deliveryHttp.CompanyIDFromContext(r.Context()); ok && companyID != id {
		deliveryHttp.WriteErrorBody(w, http.StatusForbidden, deliveryHttp.CodeForbidden, "no puede eliminar otra empresa")
		return
	}

	if err := h.usecase.Delete(id); err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
