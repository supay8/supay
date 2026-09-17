package apikey

import (
	"encoding/json"
	"net/http"

	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type handler struct {
	usecase *usecase.ApiKeyUsecase
}

func newHandler(uc *usecase.ApiKeyUsecase) *handler {
	return &handler{usecase: uc}
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	companyID, ok := deliveryHttp.CompanyIDFromContext(r.Context())
	if !ok {
		deliveryHttp.WriteErrorBody(w, http.StatusUnauthorized, deliveryHttp.CodeUnauthorized, "no autorizado: falta company_id en contexto")
		return
	}
	companyIDCurrent := chi.URLParam(r, "id")

	if companyIDCurrent != companyID {
		deliveryHttp.WriteErrorBody(w, http.StatusUnauthorized, deliveryHttp.CodeUnauthorized, "no autorizado: el company_id en la URL no coincide con el del contexto")
		return
	}

	var req usecase.CreateApiKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		deliveryHttp.RespondValidation(w, "payload JSON inválido")
		return
	}

	if req.Name == "" {
		req.Name = "default"
	}

	resp, err := h.usecase.Create(req, companyID)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}

	deliveryHttp.WriteJSON(w, http.StatusCreated, resp)
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	companyID, ok := deliveryHttp.CompanyIDFromContext(r.Context())
	if !ok {
		deliveryHttp.WriteErrorBody(w, http.StatusUnauthorized, deliveryHttp.CodeUnauthorized, "no autorizado: falta company_id en contexto")
		return
	}

	keys, err := h.usecase.List(companyID)
	if err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}

	deliveryHttp.WriteJSON(w, http.StatusOK, map[string]any{"items": keys})
}

func (h *handler) revoke(w http.ResponseWriter, r *http.Request) {
	companyID, ok := deliveryHttp.CompanyIDFromContext(r.Context())
	if !ok {
		deliveryHttp.WriteErrorBody(w, http.StatusUnauthorized, deliveryHttp.CodeUnauthorized, "no autorizado: falta company_id en contexto")
		return
	}

	keyID := chi.URLParam(r, "keyId")
	if keyID == "" {
		deliveryHttp.RespondValidation(w, "el keyId es obligatorio")
		return
	}

	if err := h.usecase.Revoke(companyID, keyID); err != nil {
		deliveryHttp.RespondError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
