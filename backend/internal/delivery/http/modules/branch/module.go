package branch

import (
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

// Module expone las rutas de gestión de sucursales.
type Module struct {
	h *handler
}

// NewModule construye el módulo branch a partir de su usecase.
func NewModule(uc *usecase.BranchUsecase) *Module {
	return &Module{h: newHandler(uc)}
}

func (m *Module) PathPrefix() string { return "/branches" }

func (m *Module) RegisterRoutes(r chi.Router) {
	r.Post("/", m.h.create)
	r.Get("/", m.h.list)
	r.Get("/{id}", m.h.getByID)
	r.Put("/{id}", m.h.update)
	r.Delete("/{id}", m.h.delete)
}
