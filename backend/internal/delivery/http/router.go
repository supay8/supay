package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// maxBodyBytes limita el tamaño del cuerpo de las peticiones (10 MB): los
// endpoints masivos aceptan hasta 500 facturas por paquete.
const maxBodyBytes int64 = 10 << 20

// Handlers agrupa todos los handlers HTTP de la aplicación.
type Handlers struct {
	Company  *CompanyHandler
	Pos      *PosHandler
	Branch   *BranchHandler
	Customer *CustomerHandler
	Product  *ProductHandler
	Invoice  *InvoiceHandler
	Siat     *SiatHandler
}

// NewRouter construye el router de Chi con todas las rutas de la API.
// apiKey protege todas las rutas con el header X-API-Key (excepto /health);
// una cadena vacía deja la API abierta.
func NewRouter(h Handlers, apiKey string) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(LimitBody(maxBodyBytes))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","message":"Supay API running"}`))
	})

	r.Group(func(r chi.Router) {
		if apiKey != "" {
			r.Use(RequireAPIKey(apiKey))
		}

		r.Route("/companies", func(r chi.Router) {
			r.Post("/", h.Company.Create)
			r.Get("/", h.Company.GetByNit)
			r.Patch("/{id}", h.Company.Update)
			r.Delete("/{id}", h.Company.Delete)
		})

		r.Route("/point-of-sale", func(r chi.Router) {
			r.Post("/", h.Pos.Create)
			r.Get("/", h.Pos.List)
			r.Get("/{id}", h.Pos.GetByID)
			r.Patch("/{id}", h.Pos.Update)
			r.Delete("/{id}", h.Pos.Delete)
		})

		r.Route("/branches", func(r chi.Router) {
			r.Post("/", h.Branch.Create)
			r.Get("/", h.Branch.List)
			r.Get("/{id}", h.Branch.GetByID)
			r.Put("/{id}", h.Branch.Update)
			r.Delete("/{id}", h.Branch.Delete)
		})

		r.Route("/siat", func(r chi.Router) {
			r.Post("/cuis/{companyId}/{pointOfSaleId}", h.Siat.SolicitarCUIS)
			r.Post("/cufd/{companyId}/{pointOfSaleId}", h.Siat.SolicitarCUFD)
			r.Post("/sincronizar/{companyId}/{pointOfSaleId}", h.Siat.Sincronizar)
			r.Post("/evento-significativo/{companyId}/{pointOfSaleId}", h.Siat.RegistrarEventoSignificativo)
			r.Post("/firma/{companyId}/{pointOfSaleId}", h.Siat.FirmarFactura)
			r.Post("/paquete/{companyId}/{pointOfSaleId}", h.Siat.EnviarPaquete)
			r.Post("/paquete/{companyId}/{pointOfSaleId}/validar", h.Siat.ValidarPaquete)
			r.Post("/masiva/{companyId}/{pointOfSaleId}", h.Siat.EnviarMasiva)
			r.Post("/masiva/{companyId}/{pointOfSaleId}/validar", h.Siat.ValidarMasiva)
			r.Post("/compras/{companyId}/{pointOfSaleId}", h.Siat.EnviarCompras)
			r.Post("/documento-ajuste/{companyId}/{pointOfSaleId}", h.Siat.EmitirDocumentoAjuste)
			r.Get("/invoice/{invoiceId}/pdf", h.Siat.DownloadPDF)
		})

		r.Route("/catalogs", func(r chi.Router) {
			r.Get("/products", h.Siat.ListSinProducts)
			r.Get("/readiness", h.Siat.CatalogReadiness)
			r.Get("/{companyId}", h.Siat.GetCatalog)
			r.Get("/{companyId}/{tipo}", h.Siat.GetCatalog)
		})

		r.Route("/customers", func(r chi.Router) {
			r.Post("/", h.Customer.Create)
			r.Get("/", h.Customer.List)
			r.Get("/{id}", h.Customer.GetByID)
		})

		r.Route("/products", func(r chi.Router) {
			r.Post("/", h.Product.Create)
			r.Get("/", h.Product.List)
			r.Post("/{id}/mappings", h.Product.AddMapping)
		})

		r.Route("/invoices", func(r chi.Router) {
			r.Post("/", h.Invoice.Create)
			r.Get("/", h.Invoice.ListByPointOfSale)
			r.Get("/sectores", h.Invoice.Sectores)
			r.Get("/{id}", h.Invoice.GetByID)
			r.Post("/{id}/emit", h.Invoice.Emit)
			r.Get("/{id}/siat-status", h.Invoice.SiatStatus)
			r.Post("/{id}/annul", h.Invoice.Annul)
			r.Post("/{id}/annul/revert", h.Invoice.RevertAnnul)
		})
	})

	return r
}
