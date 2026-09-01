package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	appconfig "github.com/brandsrx/supay/internal/config"
	"github.com/brandsrx/supay/internal/crypto"
	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/pdf"
	"github.com/brandsrx/supay/internal/repository/database"
	"github.com/brandsrx/supay/internal/repository/postgres"
	"github.com/brandsrx/supay/internal/siat"
	"github.com/brandsrx/supay/internal/storage"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/joho/godotenv"
)

func main() {
	// Cargar .env antes de leer la configuración para que las credenciales
	// del SIAT estén disponibles.
	_ = godotenv.Load()
	setupLogging()
	log.Println("Iniciando Supay API...")
	appCfg := appconfig.Load()

	if appCfg.APIKey == "" {
		log.Fatal("API_KEY no está configurada: la API quedaría abierta sin autenticación. Define API_KEY en el entorno o en .env")
	}

	// 1. Conectar a PostgreSQL y migrar
	database.ConnectDB()

	if os.Getenv("AUTO_MIGRATE") == "true" {
		log.Println("AUTO_MIGRATE=true: ejecutando migraciones de esquema y datos...")
		err := database.Migrate()
		if err != nil {
			log.Fatalf("Error al ejecutar migraciones: %v", err)
		}
		log.Println("Migraciones completadas.")
	}

	// 2. Inyección de dependencias
	companyRepo := postgres.NewPostgresCompanyRepository(database.DB)
	companyUsecase := usecase.NewCompanyUsecase(companyRepo)
	companyHandler := deliveryHttp.NewCompanyHandler(companyUsecase)

	posRepo := postgres.NewPostgresPointOfSaleRepository(database.DB)
	posUsecase := usecase.NewPointOfSaleUsecase(posRepo, companyRepo)
	posHandler := deliveryHttp.NewPosHandler(posUsecase)

	cufdRepo := postgres.NewPostgresCufdRepository(database.DB)
	contingencyRepo := postgres.NewPostgresContingencyEventRepository(database.DB)
	tipoPVRepo := postgres.NewPostgresTipoPuntoVentaRepository(database.DB)
	catalogRepo := postgres.NewPostgresCatalogRepository(database.DB)
	sinProductRepo := postgres.NewPostgresSinProductRepository(database.DB)
	syncStateRepo := postgres.NewPostgresCatalogSyncStateRepository(database.DB)
	siatActividadRepo := postgres.NewPostgresSiatActividadRepository(database.DB)
	siatLeyendaRepo := postgres.NewPostgresSiatLeyendaRepository(database.DB)
	siatDocSectorRepo := postgres.NewPostgresSiatActividadDocSectorRepository(database.DB)
	productRepo := postgres.NewPostgresProductRepository(database.DB)
	productUsecase := usecase.NewProductUsecase(productRepo, companyRepo, catalogRepo, sinProductRepo, siatDocSectorRepo)
	productHandler := deliveryHttp.NewProductHandler(productUsecase)
	branchRepo := postgres.NewPostgresBranchRepository(database.DB)
	sentPackageRepo := postgres.NewPostgresSentPackageRepository(database.DB)
	branchUsecase := usecase.NewBranchUsecase(branchRepo, companyRepo)
	branchHandler := deliveryHttp.NewBranchHandler(branchUsecase)

	customerRepo := postgres.NewPostgresCustomerRepository(database.DB)
	customerUsecase := usecase.NewCustomerUsecase(customerRepo, companyRepo)
	customerHandler := deliveryHttp.NewCustomerHandler(customerUsecase)

	invoiceRepo := postgres.NewPostgresInvoiceRepository(database.DB)

	// 3. Capa criptográfica + SiatClientProvider (multi-tenant)
	var cryptoSvc *crypto.Service
	if strings.TrimSpace(appCfg.EncryptionKey) != "" {
		svc, err := crypto.New(appCfg.EncryptionKey)
		if err != nil {
			log.Fatalf("❌ ENCRYPTION_KEY inválida: %v", err)
		}
		cryptoSvc = svc
		log.Printf("🔐 Cifrado AES-GCM activo (ENCRYPTION_KEY configurada)")
	} else {
		log.Printf("⚠️ ENCRYPTION_KEY no configurada: credenciales SIAT por empresa no estarán cifradas (configure para producción)")
	}

	certificateRepo := postgres.NewPostgresCertificateRepository(database.DB)
	// CertStorage abstracto: local (self-hosted, ./storage/certs), r2 (SaaS bucket privado supay-certs, creación manual, SSE-S3), memory/none solo tests
	certStorage, err := storage.NewCertStorageFromConfig(appCfg)
	if err != nil {
		log.Fatalf("❌ Cert storage no disponible (STORAGE_DRIVER=%s): %v", appCfg.StorageDriver, err)
	}
	log.Printf("🔐 Cert storage: driver=%s bucket=%s", appCfg.StorageDriver, appCfg.R2.Bucket)
	siatProvider := siat.NewSiatClientProviderWithStorage(companyRepo, certificateRepo, cryptoSvc, certStorage, siat.ProviderInfra{
		BaseURL:        appCfg.SiatInfra.BaseURL,
		CodigoAmbiente: appCfg.SiatInfra.CodigoAmbiente,
		Timeout:        appCfg.SiatInfra.Timeout,
		TraceId:        appCfg.SiatInfra.TraceId,
		UserAgent:      appCfg.SiatInfra.UserAgent,
		Modalidad:      appCfg.SiatInfra.Modalidad,
	})
	log.Printf("✅ SiatClientProvider listo: base=%s ambiente=%d (multi-tenant por CompanyId)", appCfg.SiatInfra.BaseURL, appCfg.SiatInfra.CodigoAmbiente)

	// Legacy single-tenant service (fallback si no hay provider por empresa). Ya no bloquea arranque.
	var siatService *siat.Service
	var legacyCfg = appCfg.SIAT
	if legacyCfg.Token != "" && legacyCfg.Nit != 0 && legacyCfg.CodigoSistema != "" {
		if err := legacyCfg.Validate(); err == nil {
			if svc, svcErr := siat.NewService(legacyCfg); svcErr == nil {
				siatService = svc
				log.Printf("✅ Servicio SIAT legacy listo (fallback): ambiente=%d base=%s", legacyCfg.CodigoAmbiente, legacyCfg.BaseURL)
			}
		}
	}

	var emissionService usecase.SiatEmissionService
	if siatService != nil {
		emissionService = siatService
	}
	var credentialClient usecase.SiatCredentialClient
	if siatService != nil {
		credentialClient = siatService
	}
	pdfStorage, err := pdf.NewStorageFromConfig(appCfg)
	if err != nil {
		log.Fatalf("PDF storage no disponible (STORAGE_DRIVER=%s): %v", appCfg.StorageDriver, err)
	}
	log.Printf("📄 PDF storage: driver=%s deployment=%s path=%s", appCfg.StorageDriver, appCfg.DeploymentMode, appCfg.StoragePath)
	pdfService := pdf.NewServiceWithStorage(database.DB, pdfStorage)

	credentialService := usecase.NewCredentialServiceWithProvider(posRepo, cufdRepo, siatProvider, appCfg.SiatInfra.Modalidad)
	if credentialClient != nil && siatProvider == nil {
		credentialService = usecase.NewCredentialService(posRepo, cufdRepo, credentialClient, appCfg.SiatInfra.Modalidad)
	}
	invoiceUsecase := usecase.NewInvoiceUsecase(invoiceRepo, customerRepo, companyRepo, posRepo, catalogRepo, cufdRepo, emissionService, appCfg.SiatInfra.Modalidad, productRepo, syncStateRepo, siatLeyendaRepo, siatDocSectorRepo, credentialService, pdfService, appCfg.AllowCustomIssueDate, siatProvider)
	invoiceUsecase.SetAllowCustomIssueDate(appCfg.AllowCustomIssueDate)
	if appCfg.AllowCustomIssueDate {
		log.Printf("🧪 ALLOW_CUSTOM_ISSUE_DATE=true (PILOTO/dev) — POST /invoices acepta issue_date arbitrario")
	}
	invoiceHandler := deliveryHttp.NewInvoiceHandler(invoiceUsecase)

	siatUsecase := usecase.NewSiatUsecase(companyRepo, posRepo, cufdRepo, tipoPVRepo, catalogRepo, contingencyRepo, sentPackageRepo, siatService, appCfg.SiatInfra.Modalidad, sinProductRepo, syncStateRepo, siatActividadRepo, siatLeyendaRepo, siatDocSectorRepo, invoiceRepo, siatProvider)

	certificateUsecase := usecase.NewCertificateUsecase(certificateRepo, companyRepo, cryptoSvc, certStorage)
	certificateHandler := deliveryHttp.NewCertificateHandler(certificateUsecase)
	siatHandler := deliveryHttp.NewSiatHandler(siatUsecase, pdfService)
	catalogHandler := deliveryHttp.NewCatalogHandler(siatUsecase)

	// 4. Router
	router := deliveryHttp.NewRouter(deliveryHttp.Handlers{
		Company:     companyHandler,
		Pos:         posHandler,
		Branch:      branchHandler,
		Customer:    customerHandler,
		Product:     productHandler,
		Invoice:     invoiceHandler,
		Siat:        siatHandler,
		Catalog:     catalogHandler,
		Certificate: certificateHandler,
	}, appCfg.APIKey)

	srv := &http.Server{
		Addr:              ":" + appCfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// 5. Graceful shutdown: al recibir SIGINT/SIGTERM se detiene de forma
	// ordenada (drenando conexiones activas hasta el timeout del servidor).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 6. Reaper de facturas SENDING atascadas: cada 5 min libera las que llevan
	// más de 10 min sin completar la emisión (crash, fallo de red persistente).
	startStaleEmissionReaper(ctx, invoiceRepo, 5*time.Minute, 10*time.Minute)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("Servidor escuchando en el puerto :%s", appCfg.Port)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Error al iniciar el servidor: %v", err)
		}
	case <-ctx.Done():
		log.Println("Señal de apagado recibida, cerrando servidor...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("Apagado forzado por timeout: %v", err)
		}
		log.Println("Servidor detenido")
	}
}

// setupLogging configura slog como logger por defecto. LOG_LEVEL ajusta la
// verbosidad (debug|info|warn|error) y LOG_FORMAT elige json o texto.
func setupLogging() {
	var level slog.Level
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if strings.EqualFold(strings.TrimSpace(os.Getenv("LOG_FORMAT")), "json") {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}

// startStaleEmissionReaper libera periódicamente las facturas atascadas en
// SENDING (p.ej. crash del proceso entre el claim y el update final),
// devolviéndolas a PENDING para que puedan reemitirse.
func startStaleEmissionReaper(ctx context.Context, repo domain.InvoiceRepository, interval, staleAfter time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				released, err := repo.ReleaseStaleSending(staleAfter)
				if err != nil {
					slog.Error("reaper: no se pudo liberar facturas SENDING atascadas", "error", err)
					continue
				}
				if released > 0 {
					slog.Warn("reaper: facturas SENDING atascadas liberadas a PENDING", "cantidad", released, "stale_after", staleAfter.String())
				}
			}
		}
	}()
}
