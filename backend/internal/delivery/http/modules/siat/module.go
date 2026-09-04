package siat

import (
	"github.com/go-chi/chi/v5"
)

// Module expone las rutas de operaciones SIAT.
type Module struct {
	h *handler
}

// NewModule construye el módulo SIAT a partir de su usecase y servicio PDF.
func NewModule(siatUC siatService, pdfService pdfGenerator) *Module {
	return &Module{h: newHandler(siatUC, pdfService)}
}

func (m *Module) PathPrefix() string { return "" }

func (m *Module) RegisterRoutes(r chi.Router) {
	r.Post("/setup", m.h.setup)

	r.Route("/siat", func(r chi.Router) {
		r.Post("/cuis/{companyId}/{pointOfSaleId}", m.h.solicitarCUIS)
		r.Post("/cufd/{companyId}/{pointOfSaleId}", m.h.solicitarCUFD)
		r.Post("/sincronizar/{companyId}/{pointOfSaleId}", m.h.sincronizar)
		r.Post("/evento-significativo/{companyId}/{pointOfSaleId}", m.h.registrarEventoSignificativo)
		r.Post("/firma/{companyId}/{pointOfSaleId}", m.h.firmarFactura)
		r.Post("/paquete/{companyId}/{pointOfSaleId}", m.h.enviarPaquete)
		r.Post("/paquete/{companyId}/{pointOfSaleId}/validar", m.h.validarPaquete)
		r.Post("/masiva/{companyId}/{pointOfSaleId}", m.h.enviarMasiva)
		r.Post("/masiva/{companyId}/{pointOfSaleId}/validar", m.h.validarMasiva)
		r.Post("/compras/{companyId}/{pointOfSaleId}", m.h.enviarCompras)
		r.Post("/documento-ajuste/{companyId}/{pointOfSaleId}", m.h.emitirDocumentoAjuste)
		r.Get("/invoice/{invoiceId}/pdf", m.h.downloadPDF)
	})

	// Rutas legadas de catálogos (compatibilidad).
	r.Get("/catalogs/activites-document-sectors", m.h.listActivitesDocumentSectors)
	r.Get("/catalogs/products", m.h.listSinProducts)
	r.Get("/catalogs/readiness", m.h.catalogReadiness)
	r.Get("/catalogs/{companyId}", m.h.getCatalog)
	r.Get("/catalogs/{companyId}/{tipo}", m.h.getCatalog)

	// PDF de factura (también expuesto bajo /invoices por compatibilidad).
	r.Get("/invoices/{id}/pdf", m.h.downloadPDF)
}
