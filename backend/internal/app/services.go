package app

import (
	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	authn "github.com/brandsrx/supay/internal/auth"
	"github.com/brandsrx/supay/internal/emissionqueue"
	"github.com/brandsrx/supay/internal/invoiceemail"
	"github.com/brandsrx/supay/internal/repository/postgres"
	"github.com/brandsrx/supay/internal/usecase"
)

type services struct {
	// Usecases
	companyUsecase     *usecase.CompanyUsecase
	apiKeyUsecase      *usecase.ApiKeyUsecase
	authUsecase        *usecase.AuthUsecase
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
	emailTasksClient   *cloudtasks.Client
	emailDispatcher    *invoiceemail.Dispatcher
	emailProcessor     *invoiceemail.Processor
	emailTaskHandler   *invoiceemail.Handler
}

func configureServices(c *App) {

	c.companyUsecase = usecase.NewCompanyUsecase(c.companyRepo, c.cipher, func(companyID string) {
		if c.siatProvider != nil {
			c.siatProvider.Invalidate(companyID)
		}
	})
	c.apiKeyUsecase = usecase.NewApiKeyUsecase(c.companyRepo, c.apiKeyRepo, postgres.NewCompanyBootstrap(c.db), authn.BcryptHasher{})
	c.pointOfSaleUsecase = usecase.NewPointOfSaleUsecase(c.pointOfSaleRepo, c.branchRepo)
	c.branchUsecase = usecase.NewBranchUsecase(c.branchRepo, c.companyRepo)
	c.customerUsecase = usecase.NewCustomerUsecase(c.customerRepo)
	c.credentialService = usecase.NewCredentialService(c.pointOfSaleRepo, c.cufdRepo, c.fiscalService, c.cfg.SiatInfra.Modalidad)
	c.credentialService.SetProvider(c.siatProvider)

	c.credentialService.SetRenewalPolicy(
		c.cfg.Maintenance.CuisRenewalLead,
		c.cfg.Maintenance.CuisFallbackValidity,
	)
	c.maintenanceMetrics = &usecase.MaintenanceMetrics{}
	c.maintenanceService = usecase.NewMaintenanceService(
		c.maintenanceRepo,
		c.credentialService,
		c.notifier,
		c.maintenanceMetrics,
		usecase.MaintenanceOptions{
			CufdRenewalLead:      c.cfg.Maintenance.CufdRenewalLead,
			CuisRenewalLead:      c.cfg.Maintenance.CuisRenewalLead,
			CuisFallbackValidity: c.cfg.Maintenance.CuisFallbackValidity,
			DefaultWebhookURL:    c.cfg.Maintenance.CertificateWebhookURL,
		},
	)
	c.certificateUsecase = usecase.NewCertificateUsecase(
		c.certificateRepo, c.companyRepo, c.cipher, c.certStorage,
		func(companyID string) {
			if c.siatProvider != nil {
				c.siatProvider.Invalidate(companyID)
			}
		},
	)
	c.invoiceUsecase = usecase.NewInvoiceUsecase(
		c.invoiceRepo, c.customerRepo, c.companyRepo,
		c.pointOfSaleRepo, c.catalogRepo, c.cufdRepo,
		c.fiscalService, c.cfg.SiatInfra.Modalidad,
		c.syncStateRepo, c.siatLeyendaRepo,
		c.siatActividadDocSectorRepo, c.credentialService,
		c.pdfService, c.cfg.AllowCustomIssueDate, c.siatProvider,
	)
	c.invoiceUsecase.SetContingencyRepository(c.contingencyRepo)
	c.invoiceUsecase.SetFileService(c.invoiceFileService)
	if c.cfg.InvoiceEmail.Enabled {
		c.invoiceUsecase.SetEmailDispatcher(c.emailDispatcher)
	}
	c.siatUsecase = usecase.NewSiatUsecase(
		c.companyRepo, c.pointOfSaleRepo, c.cufdRepo,
		c.tipoPuntoVentaRepo, c.catalogRepo, c.contingencyRepo,
		c.sentPackageRepo, c.fiscalService, c.cfg.SiatInfra.Modalidad,
		c.sinProductRepo, c.syncStateRepo, c.siatActividadRepo,
		c.siatLeyendaRepo, c.siatActividadDocSectorRepo,
		c.invoiceRepo, c.siatProvider,
	)
	c.siatUsecase.SetFileService(c.invoiceFileService)
	if c.cfg.InvoiceEmail.Enabled {
		c.siatUsecase.SetEmailDispatcher(c.emailDispatcher)
	}
}
