package customer

import (
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

// Module expone únicamente consultas del historial de receptores.
type Module struct {
	h *handler
}

// NewModule construye el módulo customer a partir de su usecase.
func NewModule(uc *usecase.CustomerUsecase) *Module {
	return &Module{h: newHandler(uc)}
}

func (m *Module) PathPrefix() string { return "/customers" }

func (m *Module) RegisterRoutes(r chi.Router) {
	r.Get("/", m.h.list)
	r.Get("/{id}", m.h.getByID)
}
