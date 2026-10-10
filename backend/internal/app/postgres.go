package app

import (
	"context"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/observability"
	"github.com/brandsrx/supay/internal/ports"
	"github.com/brandsrx/supay/internal/repository/postgres"
)

type repositories struct {
	// Repositorios
	companyRepo                ports.CompanyRepository
	apiKeyRepo                 *postgres.PostgresApiKeyRepository
	authRepo                   *postgres.PostgresAuthRepository
	betterAuthMembershipRepo   *postgres.BetterAuthMembershipRepository
	pointOfSaleRepo            ports.PointOfSaleRepository
	cufdRepo                   ports.CufdRepository
	contingencyRepo            ports.ContingencyEventRepository
	tipoPuntoVentaRepo         ports.TipoPuntoVentaRepository
	catalogRepo                ports.CatalogRepository
	sinProductRepo             ports.SinProductRepository
	syncStateRepo              ports.CatalogSyncStateRepository
	siatActividadRepo          ports.SiatActividadRepository
	siatLeyendaRepo            ports.SiatLeyendaRepository
	siatActividadDocSectorRepo ports.SiatActividadDocSectorRepository
	branchRepo                 ports.BranchRepository
	sentPackageRepo            ports.SentPackageRepository
	customerRepo               ports.CustomerRepository
	invoiceRepo                ports.InvoiceRepository
	invoiceEventRepo           ports.InvoiceEventRepository
	certificateRepo            ports.CertificateRepository
	maintenanceRepo            ports.MaintenanceRepository
	outboxRepo                 ports.OutboxRepository
	emailNotificationRepo      ports.InvoiceEmailNotificationRepository
}

func configureRepositories(c *App) {

	c.companyRepo = postgres.NewPostgresCompanyRepository(c.db)
	c.apiKeyRepo = postgres.NewPostgresApiKeyRepository(c.db)
	c.authRepo = postgres.NewPostgresAuthRepository(c.db)
	c.pointOfSaleRepo = postgres.NewPostgresPointOfSaleRepository(c.db)
	c.cufdRepo = postgres.NewPostgresCufdRepository(c.db)
	c.contingencyRepo = postgres.NewPostgresContingencyEventRepository(c.db)
	c.tipoPuntoVentaRepo = postgres.NewPostgresTipoPuntoVentaRepository(c.db)
	c.catalogRepo = postgres.NewPostgresCatalogRepository(c.db)
	c.sinProductRepo = postgres.NewPostgresSinProductRepository(c.db)
	c.syncStateRepo = postgres.NewPostgresCatalogSyncStateRepository(c.db)
	c.siatActividadRepo = postgres.NewPostgresSiatActividadRepository(c.db)
	c.siatLeyendaRepo = postgres.NewPostgresSiatLeyendaRepository(c.db)
	c.siatActividadDocSectorRepo = postgres.NewPostgresSiatActividadDocSectorRepository(c.db)
	c.branchRepo = postgres.NewPostgresBranchRepository(c.db)
	c.sentPackageRepo = postgres.NewPostgresSentPackageRepository(c.db)
	c.customerRepo = postgres.NewPostgresCustomerRepository(c.db)
	c.invoiceRepo = postgres.NewPostgresInvoiceRepository(c.db)
	c.emailNotificationRepo = postgres.NewPostgresInvoiceEmailNotificationRepository(c.db)
	c.invoiceEventRepo = postgres.NewPostgresInvoiceEventRepository(c.db)
	c.certificateRepo = postgres.NewPostgresCertificateRepository(c.db)
	c.maintenanceRepo = postgres.NewPostgresMaintenanceRepository(c.db)
	observability.DefaultMetrics().SetOutboxPendingProvider(func() (float64, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		var count int64
		err := c.db.WithContext(ctx).Table("outbox").Where("status = ?", domain.OutboxStatusPending).Count(&count).Error
		return float64(count), err
	})
	c.outboxRepo = postgres.NewPostgresOutboxRepository(c.db)
}
