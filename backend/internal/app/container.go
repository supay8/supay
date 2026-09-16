package app

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/brandsrx/supay/internal/adapters/notification"
	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/adapters/siat/sandbox"
	"github.com/brandsrx/supay/internal/config"
	appconfig "github.com/brandsrx/supay/internal/config"
	"github.com/brandsrx/supay/internal/crypto"
	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	deliveryModules "github.com/brandsrx/supay/internal/delivery/http/modules"
	"github.com/brandsrx/supay/internal/delivery/http/modules/apikey"
	"github.com/brandsrx/supay/internal/delivery/http/modules/branch"
	"github.com/brandsrx/supay/internal/delivery/http/modules/catalog"
	"github.com/brandsrx/supay/internal/delivery/http/modules/certificate"
	"github.com/brandsrx/supay/internal/delivery/http/modules/company"
	"github.com/brandsrx/supay/internal/delivery/http/modules/customer"
	"github.com/brandsrx/supay/internal/delivery/http/modules/invoice"
	"github.com/brandsrx/supay/internal/delivery/http/modules/pos"
	siatModule "github.com/brandsrx/supay/internal/delivery/http/modules/siat"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/emissionqueue"
	"github.com/brandsrx/supay/internal/pdf"
	"github.com/brandsrx/supay/internal/ports"
	"github.com/brandsrx/supay/internal/repository/postgres"
	"github.com/brandsrx/supay/internal/storage"
	"github.com/brandsrx/supay/internal/usecase"
	"gorm.io/gorm"
)

// Container centraliza la creación perezosa de dependencias. Es el único lugar
// que debe tocarse para agregar un nuevo dominio/usecase/handler.
type Container struct {
	cfg appconfig.Config
	db  *gorm.DB

	// Repositorios
	companyRepo                domain.CompanyRepository
	apiKeyRepo                 *postgres.PostgresApiKeyRepository
	pointOfSaleRepo            domain.PointOfSaleRepository
	cufdRepo                   domain.CufdRepository
	contingencyRepo            domain.ContingencyEventRepository
	tipoPuntoVentaRepo         domain.TipoPuntoVentaRepository
	catalogRepo                domain.CatalogRepository
	sinProductRepo             domain.SinProductRepository
	syncStateRepo              domain.CatalogSyncStateRepository
	siatActividadRepo          domain.SiatActividadRepository
	siatLeyendaRepo            domain.SiatLeyendaRepository
	siatActividadDocSectorRepo domain.SiatActividadDocSectorRepository
	branchRepo                 domain.BranchRepository
	sentPackageRepo            domain.SentPackageRepository
	customerRepo               domain.CustomerRepository
	invoiceRepo                domain.InvoiceRepository
	invoiceEventRepo           domain.InvoiceEventRepository
	invoiceDocumentRepo        domain.InvoiceDocumentRepository
	certificateRepo            domain.CertificateRepository
	maintenanceRepo            domain.MaintenanceRepository
	outboxRepo                 domain.OutboxRepository

	// Servicios de infraestructura
	cryptoSvc    *crypto.Service
	certStorage  storage.CertStorage
	pdfStorage   pdf.Storage
	siatProvider siat.SiatClientProvider
	siatService  *siat.Service
	pdfService   *pdf.Service
	notifier     ports.Notifier

	// Usecases
	companyUsecase     *usecase.CompanyUsecase
	apiKeyUsecase      *usecase.ApiKeyUsecase
	pointOfSaleUsecase *usecase.PointOfSaleUsecase
	branchUsecase      *usecase.BranchUsecase
	customerUsecase    *usecase.CustomerUsecase
	invoiceUsecase     *usecase.InvoiceUsecase
	siatUsecase        *usecase.SiatUsecase
	certificateUsecase *usecase.CertificateUsecase
	credentialService  *usecase.CredentialService
	maintenanceService *usecase.MaintenanceService
	maintenanceMetrics *usecase.MaintenanceMetrics
	emissionQueue      *emissionqueue.Service

	// HTTP
	modules []deliveryModules.Module
	router  http.Handler
	server  *http.Server
}

// NewContainer construye un contenedor sin inicializar sus dependencias.
func NewContainer(cfg appconfig.Config, db *gorm.DB) *Container {
	return &Container{cfg: cfg, db: db}
}

func (c *Container) CompanyRepo() domain.CompanyRepository {
	if c.companyRepo == nil {
		c.companyRepo = postgres.NewPostgresCompanyRepository(c.db)
	}
	return c.companyRepo
}

func (c *Container) ApiKeyRepo() *postgres.PostgresApiKeyRepository {
	if c.apiKeyRepo == nil {
		c.apiKeyRepo = postgres.NewPostgresApiKeyRepository(c.db)
	}
	return c.apiKeyRepo
}

func (c *Container) PointOfSaleRepo() domain.PointOfSaleRepository {
	if c.pointOfSaleRepo == nil {
		c.pointOfSaleRepo = postgres.NewPostgresPointOfSaleRepository(c.db)
	}
	return c.pointOfSaleRepo
}

func (c *Container) CufdRepo() domain.CufdRepository {
	if c.cufdRepo == nil {
		c.cufdRepo = postgres.NewPostgresCufdRepository(c.db)
	}
	return c.cufdRepo
}

func (c *Container) ContingencyRepo() domain.ContingencyEventRepository {
	if c.contingencyRepo == nil {
		c.contingencyRepo = postgres.NewPostgresContingencyEventRepository(c.db)
	}
	return c.contingencyRepo
}

func (c *Container) TipoPuntoVentaRepo() domain.TipoPuntoVentaRepository {
	if c.tipoPuntoVentaRepo == nil {
		c.tipoPuntoVentaRepo = postgres.NewPostgresTipoPuntoVentaRepository(c.db)
	}
	return c.tipoPuntoVentaRepo
}

func (c *Container) CatalogRepo() domain.CatalogRepository {
	if c.catalogRepo == nil {
		c.catalogRepo = postgres.NewPostgresCatalogRepository(c.db)
	}
	return c.catalogRepo
}

func (c *Container) SinProductRepo() domain.SinProductRepository {
	if c.sinProductRepo == nil {
		c.sinProductRepo = postgres.NewPostgresSinProductRepository(c.db)
	}
	return c.sinProductRepo
}

func (c *Container) SyncStateRepo() domain.CatalogSyncStateRepository {
	if c.syncStateRepo == nil {
		c.syncStateRepo = postgres.NewPostgresCatalogSyncStateRepository(c.db)
	}
	return c.syncStateRepo
}

func (c *Container) SiatActividadRepo() domain.SiatActividadRepository {
	if c.siatActividadRepo == nil {
		c.siatActividadRepo = postgres.NewPostgresSiatActividadRepository(c.db)
	}
	return c.siatActividadRepo
}

func (c *Container) SiatLeyendaRepo() domain.SiatLeyendaRepository {
	if c.siatLeyendaRepo == nil {
		c.siatLeyendaRepo = postgres.NewPostgresSiatLeyendaRepository(c.db)
	}
	return c.siatLeyendaRepo
}

func (c *Container) SiatActividadDocSectorRepo() domain.SiatActividadDocSectorRepository {
	if c.siatActividadDocSectorRepo == nil {
		c.siatActividadDocSectorRepo = postgres.NewPostgresSiatActividadDocSectorRepository(c.db)
	}
	return c.siatActividadDocSectorRepo
}

func (c *Container) BranchRepo() domain.BranchRepository {
	if c.branchRepo == nil {
		c.branchRepo = postgres.NewPostgresBranchRepository(c.db)
	}
	return c.branchRepo
}

func (c *Container) SentPackageRepo() domain.SentPackageRepository {
	if c.sentPackageRepo == nil {
		c.sentPackageRepo = postgres.NewPostgresSentPackageRepository(c.db)
	}
	return c.sentPackageRepo
}

func (c *Container) CustomerRepo() domain.CustomerRepository {
	if c.customerRepo == nil {
		c.customerRepo = postgres.NewPostgresCustomerRepository(c.db)
	}
	return c.customerRepo
}

func (c *Container) InvoiceRepo() domain.InvoiceRepository {
	if c.invoiceRepo == nil {
		c.invoiceRepo = postgres.NewPostgresInvoiceRepository(c.db)
	}
	return c.invoiceRepo
}

func (c *Container) InvoiceEventRepo() domain.InvoiceEventRepository {
	if c.invoiceEventRepo == nil {
		c.invoiceEventRepo = postgres.NewPostgresInvoiceEventRepository(c.db)
	}
	return c.invoiceEventRepo
}

func (c *Container) InvoiceDocumentRepo() domain.InvoiceDocumentRepository {
	if c.invoiceDocumentRepo == nil {
		c.invoiceDocumentRepo = postgres.NewPostgresInvoiceDocumentRepository(c.db)
	}
	return c.invoiceDocumentRepo
}

func (c *Container) CertificateRepo() domain.CertificateRepository {
	if c.certificateRepo == nil {
		c.certificateRepo = postgres.NewPostgresCertificateRepository(c.db)
	}
	return c.certificateRepo
}

func (c *Container) MaintenanceRepo() domain.MaintenanceRepository {
	if c.maintenanceRepo == nil {
		c.maintenanceRepo = postgres.NewPostgresMaintenanceRepository(c.db)
	}
	return c.maintenanceRepo
}

func (c *Container) OutboxRepo() domain.OutboxRepository {
	if c.outboxRepo == nil {
		c.outboxRepo = postgres.NewPostgresOutboxRepository(c.db)
	}
	return c.outboxRepo
}

func (c *Container) CryptoService() *crypto.Service {
	if c.cryptoSvc == nil {
		if key := c.cfg.EncryptionKey; key != "" {
			var err error
			c.cryptoSvc, err = crypto.New(key)
			if err != nil {
				log.Fatalf("❌ ENCRYPTION_KEY inválida: %v", err)
			}
		}
	}
	return c.cryptoSvc
}

func (c *Container) CertStorage() storage.CertStorage {
	if c.certStorage == nil {
		var err error
		c.certStorage, err = storage.NewCertStorageFromConfig(c.cfg)
		if err != nil {
			log.Fatalf("❌ Cert storage no disponible (STORAGE_DRIVER=%s): %v", c.cfg.StorageDriver, err)
		}
	}
	return c.certStorage
}

func (c *Container) PdfStorage() pdf.Storage {
	if c.pdfStorage == nil {
		var err error
		c.pdfStorage, err = pdf.NewStorageFromConfig(c.cfg)
		if err != nil {
			log.Fatalf("PDF storage no disponible (STORAGE_DRIVER=%s): %v", c.cfg.StorageDriver, err)
		}
	}
	return c.pdfStorage
}

func (c *Container) SiatProvider() siat.SiatClientProvider {
	if c.siatProvider == nil {
		c.siatProvider = siat.NewSiatClientProviderWithStorage(
			c.CompanyRepo(),
			c.CertificateRepo(),
			c.CryptoService(),
			c.CertStorage(),
			siat.ProviderInfra{
				BaseURL:        c.cfg.SiatInfra.BaseURL,
				CodigoAmbiente: c.cfg.SiatInfra.CodigoAmbiente,
				CodigoSistema:  c.cfg.SiatInfra.CodigoSistema,
				Timeout:        c.cfg.SiatInfra.Timeout,
				TraceId:        c.cfg.SiatInfra.TraceId,
				UserAgent:      c.cfg.SiatInfra.UserAgent,
				Modalidad:      c.cfg.SiatInfra.Modalidad,
			},
		)
	}
	return c.siatProvider
}

func (c *Container) SiatService() *siat.Service {
	if c.siatService == nil {
		legacyCfg := c.cfg.SIAT
		if legacyCfg.Token != "" && legacyCfg.Nit != 0 && legacyCfg.CodigoSistema != "" {
			if err := legacyCfg.Validate(); err == nil {
				if svc, svcErr := siat.NewService(legacyCfg); svcErr == nil {
					c.siatService = svc
				}
			}
		}
	}
	return c.siatService
}

// FiscalService devuelve el sandbox únicamente cuando SIAT_SANDBOX=true. En
// cualquier otro caso, la falta de credenciales reales se representa con nil
// para que el provider devuelva un error y nunca CUIS/CUFD falsos.
func (c *Container) FiscalService() ports.FiscalService {
	if c.cfg.SiatSandbox {
		return sandbox.NewFiscalService()
	}
	if svc := c.SiatService(); svc != nil {
		return siat.NewFiscalAdapter(svc)
	}
	return nil
}

func (c *Container) PdfService() *pdf.Service {
	if c.pdfService == nil {
		c.pdfService = pdf.NewServiceWithStorage(c.db, c.PdfStorage())
	}
	return c.pdfService
}

func (c *Container) Notifier() ports.Notifier {
	if c.notifier == nil {
		c.notifier = notification.NewWebhookNotifier(c.cfg.Maintenance.NotificationTimeout)
	}
	return c.notifier
}

func (c *Container) CredentialService() *usecase.CredentialService {
	if c.credentialService == nil {
		c.credentialService = usecase.NewCredentialServiceWithProvider(
			c.PointOfSaleRepo(),
			c.CufdRepo(),
			c.SiatProvider(),
			c.cfg.SiatInfra.Modalidad,
		)
		if c.SiatProvider() == nil {
			c.credentialService = usecase.NewCredentialService(
				c.PointOfSaleRepo(),
				c.CufdRepo(),
				c.FiscalService(),
				c.cfg.SiatInfra.Modalidad,
			)
		}
		c.credentialService.SetRenewalPolicy(
			c.cfg.Maintenance.CuisRenewalLead,
			c.cfg.Maintenance.CuisFallbackValidity,
		)
	}
	return c.credentialService
}

func (c *Container) MaintenanceMetrics() *usecase.MaintenanceMetrics {
	if c.maintenanceMetrics == nil {
		c.maintenanceMetrics = &usecase.MaintenanceMetrics{}
	}
	return c.maintenanceMetrics
}

func (c *Container) MaintenanceService() *usecase.MaintenanceService {
	if c.maintenanceService == nil {
		c.maintenanceService = usecase.NewMaintenanceService(
			c.MaintenanceRepo(),
			c.CredentialService(),
			c.Notifier(),
			c.MaintenanceMetrics(),
			usecase.MaintenanceOptions{
				CufdRenewalLead:      c.cfg.Maintenance.CufdRenewalLead,
				CuisRenewalLead:      c.cfg.Maintenance.CuisRenewalLead,
				CuisFallbackValidity: c.cfg.Maintenance.CuisFallbackValidity,
				DefaultWebhookURL:    c.cfg.Maintenance.CertificateWebhookURL,
			},
		)
	}
	return c.maintenanceService
}

func (c *Container) CompanyUsecase() *usecase.CompanyUsecase {
	if c.companyUsecase == nil {
		c.companyUsecase = usecase.NewCompanyUsecase(c.CompanyRepo(), c.CryptoService(), func(companyID string) {
			if c.siatProvider != nil {
				c.siatProvider.Invalidate(companyID)
			}
		})
	}
	return c.companyUsecase
}

func (c *Container) ApiKeyUsecase() *usecase.ApiKeyUsecase {
	if c.apiKeyUsecase == nil {
		c.apiKeyUsecase = usecase.NewApiKeyUsecase(c.CompanyRepo(), c.ApiKeyRepo(), c.db)
	}
	return c.apiKeyUsecase
}

func (c *Container) PointOfSaleUsecase() *usecase.PointOfSaleUsecase {
	if c.pointOfSaleUsecase == nil {
		c.pointOfSaleUsecase = usecase.NewPointOfSaleUsecase(c.PointOfSaleRepo(), c.BranchRepo())
	}
	return c.pointOfSaleUsecase
}

func (c *Container) BranchUsecase() *usecase.BranchUsecase {
	if c.branchUsecase == nil {
		c.branchUsecase = usecase.NewBranchUsecase(c.BranchRepo(), c.CompanyRepo())
	}
	return c.branchUsecase
}

func (c *Container) CustomerUsecase() *usecase.CustomerUsecase {
	if c.customerUsecase == nil {
		c.customerUsecase = usecase.NewCustomerUsecase(c.CustomerRepo())
	}
	return c.customerUsecase
}

func (c *Container) InvoiceUsecase() *usecase.InvoiceUsecase {
	if c.invoiceUsecase == nil {
		c.invoiceUsecase = usecase.NewInvoiceUsecase(
			c.InvoiceRepo(), c.CustomerRepo(), c.CompanyRepo(),
			c.PointOfSaleRepo(), c.CatalogRepo(), c.CufdRepo(),
			c.FiscalService(), c.cfg.SiatInfra.Modalidad,
			c.SyncStateRepo(), c.SiatLeyendaRepo(),
			c.SiatActividadDocSectorRepo(), c.CredentialService(),
			c.PdfService(), c.cfg.AllowCustomIssueDate, c.SiatProvider(),
		)
		c.invoiceUsecase.SetContingencyRepository(c.ContingencyRepo())
	}
	return c.invoiceUsecase
}

func (c *Container) EmissionQueue() (*emissionqueue.Service, error) {
	if c.emissionQueue != nil {
		return c.emissionQueue, nil
	}
	queue, err := emissionqueue.NewService(c.db, c.OutboxRepo(), c.InvoiceUsecase(), emissionqueue.Config{
		DispatchInterval:  c.cfg.Queue.DispatchInterval,
		DispatchBatchSize: c.cfg.Queue.DispatchBatchSize,
		OutboxLockTimeout: c.cfg.Queue.OutboxLockTimeout,
		MaxWorkers:        c.cfg.Queue.MaxWorkers,
		MaxAttempts:       c.cfg.Queue.MaxAttempts,
		JobTimeout:        c.cfg.Queue.JobTimeout,
		SoftStopTimeout:   c.cfg.Queue.SoftStopTimeout,
		RetryBase:         c.cfg.Queue.RetryBase,
		RetryMax:          c.cfg.Queue.RetryMax,
		RatePerSecond:     c.cfg.Queue.TenantRatePerSecond,
		RateBurst:         c.cfg.Queue.TenantRateBurst,
		CircuitThreshold:  c.cfg.Queue.CircuitThreshold,
		CircuitCooldown:   c.cfg.Queue.CircuitCooldown,
	})
	if err != nil {
		return nil, err
	}
	c.emissionQueue = queue
	return c.emissionQueue, nil
}

func (c *Container) SiatUsecase() *usecase.SiatUsecase {
	if c.siatUsecase == nil {
		c.siatUsecase = usecase.NewSiatUsecase(
			c.CompanyRepo(), c.PointOfSaleRepo(), c.CufdRepo(),
			c.TipoPuntoVentaRepo(), c.CatalogRepo(), c.ContingencyRepo(),
			c.SentPackageRepo(), c.FiscalService(), c.cfg.SiatInfra.Modalidad,
			c.SinProductRepo(), c.SyncStateRepo(), c.SiatActividadRepo(),
			c.SiatLeyendaRepo(), c.SiatActividadDocSectorRepo(),
			c.InvoiceRepo(), c.SiatProvider(),
		)
	}
	return c.siatUsecase
}

func (c *Container) CertificateUsecase() *usecase.CertificateUsecase {
	if c.certificateUsecase == nil {
		c.certificateUsecase = usecase.NewCertificateUsecase(
			c.CertificateRepo(), c.CompanyRepo(), c.CryptoService(), c.CertStorage(),
			func(companyID string) {
				if c.siatProvider != nil {
					c.siatProvider.Invalidate(companyID)
				}
			},
		)
	}
	return c.certificateUsecase
}

func (c *Container) Modules() []deliveryModules.Module {
	if c.modules == nil {
		c.modules = []deliveryModules.Module{
			company.NewModule(c.CompanyUsecase()),
			apikey.NewModule(c.ApiKeyUsecase()),
			pos.NewModule(c.PointOfSaleUsecase()),
			branch.NewModule(c.BranchUsecase()),
			customer.NewModule(c.CustomerUsecase()),
			invoice.NewModule(c.InvoiceUsecase()),
			siatModule.NewModule(c.SiatUsecase(), c.PdfService()),
			catalog.NewModule(c.SiatUsecase()),
			certificate.NewModule(c.CertificateUsecase()),
		}
	}
	return c.modules
}

func (c *Container) Router() http.Handler {
	config := config.Load()
	if c.router == nil {
		deliveryHttp.SetVerifyAPIKey(postgres.VerifyKey)
		companyCreateHandler := c.companyCreateHandler()
		c.router = deliveryHttp.NewRouter(config, c.Modules(), c.ApiKeyRepo(), companyCreateHandler)
	}
	return c.router
}

func (c *Container) companyCreateHandler() http.HandlerFunc {
	uc := c.ApiKeyUsecase()
	return func(w http.ResponseWriter, r *http.Request) {
		var req usecase.BootstrapCompanyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			deliveryHttp.RespondValidation(w, "payload JSON inválido")
			return
		}
		resp, err := uc.BootstrapCompany(req)
		if err != nil {
			deliveryHttp.RespondError(w, err)
			return
		}
		deliveryHttp.WriteJSON(w, http.StatusCreated, resp)
	}
}

func (c *Container) Server() *http.Server {
	if c.server == nil {
		c.server = &http.Server{
			Addr:              ":" + c.cfg.Port,
			Handler:           c.Router(),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
		}
	}
	return c.server
}

func (c *Container) DB() *gorm.DB {
	return c.db
}

func (c *Container) Config() appconfig.Config {
	return c.cfg
}
