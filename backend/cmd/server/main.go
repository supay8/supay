package main

import (
	"log"
	"net/http"

	appconfig "github.com/brandsrx/supay/internal/config"
	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	"github.com/brandsrx/supay/internal/pdf"
	"github.com/brandsrx/supay/internal/repository/database"
	"github.com/brandsrx/supay/internal/repository/postgres"
	"github.com/brandsrx/supay/internal/siat"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/joho/godotenv"
)

func main() {
	// Cargar .env antes de leer la configuración para que las credenciales
	// del SIAT estén disponibles.
	_ = godotenv.Load()
	log.Println("Iniciando Supay API...")
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

	cufdRepo := postgres.NewPostgresCufdRepository(database.DB)
	tipoPVRepo := postgres.NewPostgresTipoPuntoVentaRepository(database.DB)
	catalogRepo := postgres.NewPostgresCatalogRepository(database.DB)
	branchRepo := postgres.NewPostgresBranchRepository(database.DB)
	branchUsecase := usecase.NewBranchUsecase(branchRepo, companyRepo)
	branchHandler := deliveryHttp.NewBranchHandler(branchUsecase)

	customerRepo := postgres.NewPostgresCustomerRepository(database.DB)
	customerUsecase := usecase.NewCustomerUsecase(customerRepo, companyRepo)
	customerHandler := deliveryHttp.NewCustomerHandler(customerUsecase)

	invoiceRepo := postgres.NewPostgresInvoiceRepository(database.DB)

	// 3. Servicio SIAT sobre el SDK go-siat (CUIS y CUFD)
	var siatService *siat.Service
	if err := appCfg.SIAT.Validate(); err == nil {
		svc, svcErr := siat.NewService(appCfg.SIAT)
		if svcErr != nil {
			log.Printf("⚠️ No se pudo inicializar el servicio SIAT: %v", svcErr)
		} else {
			siatService = svc
			log.Printf("✅ Servicio SIAT listo: ambiente=%d base=%s", appCfg.SIAT.CodigoAmbiente, appCfg.SIAT.BaseURL)
		}
	} else {
		log.Printf("⚠️ Configuración SIAT inválida o incompleta: %v", err)
	}

	var emissionService usecase.SiatEmissionService
	if siatService != nil {
		emissionService = siatService
	}
	invoiceUsecase := usecase.NewInvoiceUsecase(invoiceRepo, customerRepo, companyRepo, posRepo, catalogRepo, cufdRepo, emissionService, appCfg.SiatModalidad)
	invoiceHandler := deliveryHttp.NewInvoiceHandler(invoiceUsecase)

	siatHandler := deliveryHttp.NewSiatHandler(companyRepo, posRepo, cufdRepo, tipoPVRepo, catalogRepo, siatService, pdf.NewService(database.DB), appCfg.SiatModalidad)

	// 4. Router
	router := deliveryHttp.NewRouter(deliveryHttp.Handlers{
		Company:  companyHandler,
		Pos:      posHandler,
		Branch:   branchHandler,
		Customer: customerHandler,
		Invoice:  invoiceHandler,
		Siat:     siatHandler,
	})

	port := ":" + appCfg.Port
	log.Printf("Servidor escuchando en el puerto %s", port)
	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}
