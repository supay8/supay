package certificate

import (
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

// Module expone las rutas de gestión de certificados.
type Module struct {
	h *handler
}

// NewModule construye el módulo certificate a partir de su usecase.
func NewModule(uc *usecase.CertificateUsecase) *Module {
	return &Module{h: newHandler(uc)}
}

func (m *Module) PathPrefix() string { return "" }

func (m *Module) RegisterRoutes(r chi.Router) {
	r.Route("/companies/{id}/certificates", func(r chi.Router) {
		r.Post("/", m.h.create)
		r.Get("/", m.h.list)
		r.Get("/active", m.h.getActive)
		r.Delete("/{certId}", m.h.delete)
	})
}
