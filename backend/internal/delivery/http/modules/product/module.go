package product

import (
	"github.com/go-chi/chi/v5"
)

// Module expone las rutas de gestión de productos.
type Module struct {
	h *handler
}

// NewModule construye el módulo product a partir de su usecase.
func NewModule(uc productService) *Module {
	return &Module{h: newHandler(uc)}
}

func (m *Module) PathPrefix() string { return "/products" }

func (m *Module) RegisterRoutes(r chi.Router) {
	r.Post("/", m.h.create)
	r.Get("/", m.h.list)
	r.Post("/{id}/mappings", m.h.addMapping)
}
