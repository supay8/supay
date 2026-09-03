package company

import (
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

// Module expone las rutas de gestión de empresas/tenants.
type Module struct {
	h *handler
}

// NewModule construye el módulo company a partir de su usecase.
func NewModule(uc *usecase.CompanyUsecase) *Module {
	return &Module{h: newHandler(uc)}
}

func (m *Module) PathPrefix() string { return "/companies" }

func (m *Module) RegisterRoutes(r chi.Router) {
	r.Post("/", m.h.create)
	r.Get("/", m.h.getByNit)
	r.Patch("/{id}", m.h.update)
	r.Delete("/{id}", m.h.delete)
}
