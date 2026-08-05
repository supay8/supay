package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	appconfig "github.com/brandsrx/supay/internal/config"
	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	"github.com/brandsrx/supay/internal/pdf"
	"github.com/brandsrx/supay/internal/repository/database"
	"github.com/brandsrx/supay/internal/repository/postgres"
	"github.com/brandsrx/supay/internal/siat"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
)

func main() {
	// Cargar .env antes de leer la configuración para que SIAT_TOKEN_DELEGADO
	// (y demás variables) estén disponibles para el cliente SOAP.
	_ = godotenv.Load()
	fmt.Println("🚀 Iniciando motor SIAT en Go con Chi Router...")
	appCfg := appconfig.Load()

	// 1. Conectar a PostgreSQL y migrar
	database.ConnectDB()

	// 2. Inyección de dependencias
	companyRepo := postgres.NewPostgresCompanyRepository(database.DB)
	companyUsecase := usecase.NewCompanyUsecase(companyRepo)
	companyHandler := deliveryHttp.NewCompanyHandler(companyUsecase)

	posRepo := postgres.NewPostgresPointOfSaleRepository(database.DB)
	posUsecase := usecase.NewPointOfSaleUsecase(posRepo, companyRepo)
	posHandler := deliveryHttp.NewPosHandler(posUsecase)
	// repos for cuis/cufd
	cuisRepo := postgres.NewPostgresCuisRepository(database.DB)
	cufdRepo := postgres.NewPostgresCufdRepository(database.DB)
	tipoPVRepo := postgres.NewPostgresTipoPuntoVentaRepository(database.DB)
	branchRepo := postgres.NewPostgresBranchRepository(database.DB)
	branchUsecase := usecase.NewBranchUsecase(branchRepo, companyRepo)
	branchHandler := deliveryHttp.NewBranchHandler(branchUsecase)

	customerRepo := postgres.NewPostgresCustomerRepository(database.DB)
	customerUsecase := usecase.NewCustomerUsecase(customerRepo, companyRepo)
	customerHandler := deliveryHttp.NewCustomerHandler(customerUsecase)

	invoiceRepo := postgres.NewPostgresInvoiceRepository(database.DB)
	invoiceUsecase := usecase.NewInvoiceUsecase(invoiceRepo, customerRepo, companyRepo, posRepo)
	invoiceHandler := deliveryHttp.NewInvoiceHandler(invoiceUsecase)

	var cuisService *siat.CuisService
	var cufdService *siat.CufdService
	var emissionService *siat.EmissionService
	var siATClient *siat.Client
	if err := appCfg.SIAT.Validate(); err == nil {
		var clientErr error
		siATClient, clientErr = siat.NewClient(appCfg.SIAT)
		if clientErr != nil {
			log.Printf("⚠️ No se pudo inicializar SIAT: %v", clientErr)
		} else {
			cuisService = siat.NewCuisService(siATClient)
			cufdService = siat.NewCufdService(siATClient)
			emissionService = siat.NewEmissionService(database.DB, siATClient, appCfg.SIAT, nil)
			log.Printf("✅ Cliente SIAT listo: WSDL=%s Endpoint=%s", siATClient.WSDLURL(), siATClient.Endpoint())
		}
	} else {
		log.Printf("⚠️ Configuración SIAT inválida o incompleta: %v", err)
	}
	siatHandler := deliveryHttp.NewSiatHandler(companyRepo, posRepo, cufdRepo, cuisService, cufdService, emissionService, pdf.NewService(database.DB), appCfg.SiatModalidad)

	// Provisioning service and handlers (create POS under branch and provision with SIAT)
	var branchPosHandler *deliveryHttp.BranchPosHandler
	if cuisService != nil && cufdService != nil && siATClient != nil {
		provisionSvc := usecase.NewPointOfSaleProvisionService(branchRepo, posRepo, companyRepo, siATClient, cuisService, cufdService, cuisRepo, cufdRepo, tipoPVRepo, appCfg.SiatModalidad)
		branchPosHandler = deliveryHttp.NewBranchPosHandler(provisionSvc)
	}

	// 3. Inicializar el Router de Chi
	r := chi.NewRouter()

	// Middlewares globales útiles
	r.Use(middleware.Logger)    // Muestra las peticiones HTTP en consola
	r.Use(middleware.Recoverer) // Evita que un panic tumbe el servidor
	r.Use(middleware.Timeout(60 * time.Second))

	// Ruta de salud
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok", "message": "SIAT Engine & Database running"}`))
	})

	// Agrupando rutas para Company
	r.Route("/companies", func(r chi.Router) {
		r.Post("/", companyHandler.Create)  // POST /companies
		r.Get("/", companyHandler.GetByNit) // GET /companies?nit=... o por query
		r.Patch("/{id}", companyHandler.Update)
		r.Delete("/{id}", companyHandler.Delete)
		r.Post("/{companyId}/siat/sync/tipo-punto-venta", func(w http.ResponseWriter, r *http.Request) {
			if branchPosHandler == nil {
				http.Error(w, "siat client not configured", http.StatusServiceUnavailable)
				return
			}
			branchPosHandler.SyncTipoPuntoVenta(w, r)
		})
	})

	// Agrupando rutas para PointOfSale
	r.Route("/point-of-sale", func(r chi.Router) {
		r.Post("/", posHandler.Create)       // POST /point-of-sale
		r.Get("/", posHandler.List)          // GET /point-of-sale?companyId=...
		r.Get("/{id}", posHandler.GetByID)   // GET /point-of-sale/{id}
		r.Patch("/{id}", posHandler.Update)  // PATCH /point-of-sale/{id}
		r.Delete("/{id}", posHandler.Delete) // DELETE /point-of-sale/{id}
	})

	r.Route("/branches", func(r chi.Router) {
		r.Post("/", branchHandler.Create)
		r.Get("/", branchHandler.List)
		r.Get("/{id}", branchHandler.GetByID)
		r.Put("/{id}", branchHandler.Update)
		r.Delete("/{id}", branchHandler.Delete)
		r.Get("/{branchId}/points-of-sale", func(w http.ResponseWriter, r *http.Request) {
			if branchPosHandler == nil {
				http.Error(w, "provisioning not configured", http.StatusServiceUnavailable)
				return
			}
			branchPosHandler.ListByBranch(w, r)
		})
		r.Post("/{branchId}/points-of-sale", func(w http.ResponseWriter, r *http.Request) {
			if branchPosHandler == nil {
				http.Error(w, "provisioning not configured", http.StatusServiceUnavailable)
				return
			}
			branchPosHandler.Create(w, r)
		})
	})

	// Alias plural para consulta de puntos de venta y estado tributario
	r.Route("/points-of-sale", func(r chi.Router) {
		r.Get("/{id}", posHandler.GetByID)
		r.Get("/{id}/status", func(w http.ResponseWriter, r *http.Request) {
			if branchPosHandler == nil {
				http.Error(w, "provisioning not configured", http.StatusServiceUnavailable)
				return
			}
			branchPosHandler.GetStatus(w, r)
		})
	})

	r.Route("/siat", func(r chi.Router) {
		r.Post("/cuis/{companyId}/{pointOfSaleId}", siatHandler.SolicitarCUIS)
		r.Post("/cufd/{companyId}/{pointOfSaleId}", siatHandler.SolicitarCUFD)
		r.Post("/emit/{invoiceId}", siatHandler.EmitInvoice)
		r.Get("/invoice/{invoiceId}/pdf", siatHandler.DownloadPDF)
	})

	r.Route("/customers", func(r chi.Router) {
		r.Post("/", customerHandler.Create)
		r.Get("/", customerHandler.List)
		r.Get("/{id}", customerHandler.GetByID)
	})

	r.Route("/invoices", func(r chi.Router) {
		r.Post("/", invoiceHandler.Create)
		r.Get("/", invoiceHandler.ListByPointOfSale)
		r.Get("/{id}", invoiceHandler.GetByID)
	})

	port := ":" + appCfg.Port
	log.Printf("Servidor escuchando en el puerto %s", port)
	if err := http.ListenAndServe(port, r); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}
