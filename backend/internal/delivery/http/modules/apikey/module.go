package apikey

import (
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type Module struct {
	h *handler
}

func NewModule(uc *usecase.ApiKeyUsecase) *Module {
	return &Module{h: newHandler(uc)}
}

func (m *Module) PathPrefix() string {
	return "/companies/{id}/api-keys"
}

func (m *Module) RegisterRoutes(r chi.Router) {
	r.Post("/", m.h.create)
	r.Get("/", m.h.list)
	r.Delete("/{keyId}", m.h.revoke)
}
