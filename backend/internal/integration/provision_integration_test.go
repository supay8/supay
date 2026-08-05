package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/config"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	repoPostgres "github.com/brandsrx/supay/internal/repository/postgres"
	"github.com/brandsrx/supay/internal/siat"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/joho/godotenv"
	gormpg "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Integration test for provisioning POS: creates company, branch, then provisions POS
func TestProvisionPOS_Integration(t *testing.T) {
	// load .env if present
	_ = godotenv.Load()

	// require env
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	siatNit := os.Getenv("SIAT_NIT")
	siatCodigoSistema := os.Getenv("SIAT_CODIGO_SISTEMA")
	if siatNit == "" || siatCodigoSistema == "" {
		t.Skip("SIAT_NIT or SIAT_CODIGO_SISTEMA not set; skipping SIAT integration test")
	}

	// connect DB
	db, err := gorm.Open(gormpg.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("cannot connect to database: %v", err)
	}

	// Ensure migrations for test
	if err := db.AutoMigrate(
		&models.Company{},
		&models.Branch{},
		&models.PointOfSale{},
		&models.Cuis{},
		&models.Cufd{},
		&models.TipoPuntoVenta{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	// build SIAT client from config
	cfg := config.Load()
	if err := cfg.SIAT.Validate(); err != nil {
		t.Skipf("SIAT config invalid: %v", err)
	}
	client, err := siat.NewClient(cfg.SIAT)
	if err != nil {
		t.Skipf("cannot create SIAT client: %v", err)
	}

	// repositories
	companyRepo := repoPostgres.NewPostgresCompanyRepository(db)
	branchRepo := repoPostgres.NewPostgresBranchRepository(db)
	posRepo := repoPostgres.NewPostgresPointOfSaleRepository(db)
	cuisRepo := repoPostgres.NewPostgresCuisRepository(db)
	cufdRepo := repoPostgres.NewPostgresCufdRepository(db)
	tipoPVRepo := repoPostgres.NewPostgresTipoPuntoVentaRepository(db)

	// services
	cuisSvc := siat.NewCuisService(client)
	cufdSvc := siat.NewCufdService(client)
	provisionSvc := usecase.NewPointOfSaleProvisionService(branchRepo, posRepo, companyRepo, client, cuisSvc, cufdSvc, cuisRepo, cufdRepo, tipoPVRepo, 1)

	// create company (or get existing)
	var company *domain.Company
	if c, err := companyRepo.GetByNit(siatNit); err == nil && c != nil {
		company = c
	} else {
		company = &domain.Company{
			Nit:           siatNit,
			BusinessName:  "Integration Test Co",
			CodigoSistema: siatCodigoSistema,
			Ambiente:      domain.EnvironmentPiloto,
		}
		if err := companyRepo.Create(company); err != nil {
			t.Fatalf("create company failed: %v", err)
		}
	}

	// create branch
	branch := &domain.Branch{
		CompanyID:      company.ID,
		CodigoSucursal: 1,
		Name:           "Test Branch",
		Address:        "Test address",
		Active:         true,
	}
	if err := branchRepo.Create(branch); err != nil {
		t.Fatalf("failed create branch: %v", err)
	}

	// provision POS
	input := usecase.CreatePOSInput{
		Description: "Integration POS",
		Name:        "Integration POS",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pos, err := provisionSvc.CreateOperationalPOS(ctx, branch.ID, input)
	if err != nil {
		t.Fatalf("provision failed: %v", err)
	}

	if pos.Status != "OPERATIVE" {
		t.Fatalf("expected POS to be OPERATIVE, got %s", pos.Status)
	}

	// ensure CUIS and CUFD exist
	if _, err := cuisRepo.GetActiveByPos(pos.ID); err != nil {
		t.Fatalf("missing active CUIS: %v", err)
	}
	if _, err := cufdRepo.GetActiveByPos(pos.ID); err != nil {
		t.Fatalf("missing active CUFD: %v", err)
	}
}
