package app

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

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
	"github.com/brandsrx/supay/internal/delivery/http/modules/product"
	siatModule "github.com/brandsrx/supay/internal/delivery/http/modules/siat"
	"github.com/brandsrx/supay/internal/domain"
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
	productRepo                domain.ProductRepository
	branchRepo                 domain.BranchRepository
	sentPackageRepo            domain.SentPackageRepository
	customerRepo               domain.CustomerRepository
	invoiceRepo                domain.InvoiceRepository
	invoiceEventRepo           domain.InvoiceEventRepository
	invoiceDocumentRepo        domain.InvoiceDocumentRepository
	certificateRepo            domain.CertificateRepository

	// Servicios de infraestructura
	cryptoSvc    *crypto.Service
	certStorage  storage.CertStorage
	pdfStorage   pdf.Storage
	siatProvider siat.SiatClientProvider
	siatService  *siat.Service
	pdfService   *pdf.Service

	// Usecases
	companyUsecase     *usecase.CompanyUsecase
	apiKeyUsecase      *usecase.ApiKeyUsecase
	pointOfSaleUsecase *usecase.PointOfSaleUsecase
	productUsecase     *usecase.ProductUsecase
	branchUsecase      *usecase.BranchUsecase
	customerUsecase    *usecase.CustomerUsecase
	invoiceUsecase     *usecase.InvoiceUsecase
	siatUsecase        *usecase.SiatUsecase
	certificateUsecase *usecase.CertificateUsecase
	credentialService  *usecase.CredentialService

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

func (c *Container) ProductRepo() domain.ProductRepository {
	if c.productRepo == nil {
		c.productRepo = postgres.NewPostgresProductRepository(c.db)
	}
	return c.productRepo
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

// FiscalService devuelve el adaptador SIAT real si está configurado; en
// desarrollo/CI o cuando faltan credenciales legacy retorna el sandbox
// determinístico para que la aplicación pueda arrancar y testearse sin el
// SIAT real.
func (c *Container) FiscalService() ports.FiscalService {
	if svc := c.SiatService(); svc != nil {
		return siat.NewFiscalAdapter(svc)
	}
	return sandbox.NewFiscalService()
}

func (c *Container) PdfService() *pdf.Service {
	if c.pdfService == nil {
		c.pdfService = pdf.NewServiceWithStorage(c.db, c.PdfStorage())
	}
	return c.pdfService
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
	}
	return c.credentialService
}

func (c *Container) CompanyUsecase() *usecase.CompanyUsecase {
	if c.companyUsecase == nil {
		c.companyUsecase = usecase.NewCompanyUsecase(c.CompanyRepo())
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
		c.pointOfSaleUsecase = usecase.NewPointOfSaleUsecase(c.PointOfSaleRepo(), c.CompanyRepo())
	}
	return c.pointOfSaleUsecase
}

func (c *Container) ProductUsecase() *usecase.ProductUsecase {
	if c.productUsecase == nil {
		c.productUsecase = usecase.NewProductUsecase(
			c.ProductRepo(), c.CompanyRepo(), c.CatalogRepo(),
			c.SinProductRepo(), c.SiatActividadDocSectorRepo(),
		)
	}
	return c.productUsecase
}

func (c *Container) BranchUsecase() *usecase.BranchUsecase {
	if c.branchUsecase == nil {
		c.branchUsecase = usecase.NewBranchUsecase(c.BranchRepo(), c.CompanyRepo())
	}
	return c.branchUsecase
}

func (c *Container) CustomerUsecase() *usecase.CustomerUsecase {
	if c.customerUsecase == nil {
		c.customerUsecase = usecase.NewCustomerUsecase(c.CustomerRepo(), c.CompanyRepo())
	}
	return c.customerUsecase
}

func (c *Container) InvoiceUsecase() *usecase.InvoiceUsecase {
	if c.invoiceUsecase == nil {
		c.invoiceUsecase = usecase.NewInvoiceUsecase(
			c.InvoiceRepo(), c.CustomerRepo(), c.CompanyRepo(),
			c.PointOfSaleRepo(), c.CatalogRepo(), c.CufdRepo(),
			c.FiscalService(), c.cfg.SiatInfra.Modalidad,
			c.ProductRepo(), c.SyncStateRepo(), c.SiatLeyendaRepo(),
			c.SiatActividadDocSectorRepo(), c.CredentialService(),
			c.PdfService(), c.cfg.AllowCustomIssueDate, c.SiatProvider(),
		)
	}
	return c.invoiceUsecase
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
			product.NewModule(c.ProductUsecase()),
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
