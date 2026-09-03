package pos

import (
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

// Module expone las rutas de gestión de puntos de venta.
type Module struct {
	h *handler
}

// NewModule construye el módulo POS a partir de su usecase.
func NewModule(uc *usecase.PointOfSaleUsecase) *Module {
	return &Module{h: newHandler(uc)}
}

func (m *Module) PathPrefix() string { return "/point-of-sales" }

func (m *Module) RegisterRoutes(r chi.Router) {
	r.Post("/", m.h.create)
	r.Get("/", m.h.list)
	r.Get("/{id}", m.h.getByID)
	r.Patch("/{id}", m.h.update)
	r.Delete("/{id}", m.h.delete)
}
