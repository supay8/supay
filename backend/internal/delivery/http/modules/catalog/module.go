package catalog

import (
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
)

// Module expone las rutas de consulta de catálogos sincronizados.
// La sincronización sigue en el módulo SIAT; aquí solo se lee PostgreSQL.
type Module struct {
	h *handler
}

// NewModule construye el módulo catalog a partir del SiatUsecase.
func NewModule(siatUC *usecase.SiatUsecase) *Module {
	return &Module{h: newHandler(siatUC)}
}

func (m *Module) PathPrefix() string { return "" }

func (m *Module) RegisterRoutes(r chi.Router) {
	// Rutas anidadas bajo /companies/{id}.
	r.Get("/companies/{id}/actividades-economicas", m.h.listCompanyActividadesEconomicas)
	r.Route("/companies/{id}/catalogs", func(r chi.Router) {
		r.Get("/readiness", m.h.readiness)
		r.Get("/actividades-economicas", m.h.listActividadesEconomicas)
		r.Get("/documentos-sector", m.h.listDocumentosSector)
		r.Get("/leyendas-factura", m.h.listLeyendasFactura)
		r.Get("/productos-sin", m.h.listProductosSin)
		r.Get("/emision-bootstrap", m.h.emisionBootstrap)
		r.Get("/{catalogSlug}", m.h.listParametric)
	})

	// Rutas propias bajo /catalogs.
	r.Get("/catalogs/perfiles-documento-sector", m.h.listPerfilesDocumentoSector)
	r.Get("/catalogs/perfiles-documento-sector/{codigo}", m.h.getPerfilDocumentoSector)
}
