package invoice

import (
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

// Module expone las rutas de gestión de facturas.
type Module struct {
	h *handler
}

// NewModule construye el módulo invoice a partir de su usecase.
func NewModule(uc *usecase.InvoiceUsecase) *Module {
	return &Module{h: newHandler(uc)}
}

func (m *Module) PathPrefix() string { return "/invoices" }

func (m *Module) RegisterRoutes(r chi.Router) {
	r.Post("/", m.h.create)
	r.Get("/", m.h.list)
	r.Get("/sectores", m.h.sectores)
	r.Get("/{id}", m.h.getByID)
	r.Get("/{id}/xml", m.h.downloadXML)
	r.Post("/{id}/emit", m.h.emit)
	r.Get("/{id}/siat-status", m.h.siatStatus)
	r.Post("/{id}/annul", m.h.annul)
	r.Post("/{id}/annul/revert", m.h.revertAnnul)
}
