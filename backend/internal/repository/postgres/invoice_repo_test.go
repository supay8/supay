package postgres

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/brandsrx/supay/internal/repository/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Los tests de integración requieren una base PostgreSQL dedicada definida en
// TEST_DATABASE_URL (p.ej. postgresql://postgres:pass@localhost:5432/supay_test).
// Sin esa variable, los tests se omiten (t.Skip).
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL no está definida; omitiendo tests de integración de repositorio")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("conexión a BD de pruebas: %v", err)
	}
	if err := database.MigrateDB(db); err != nil {
		t.Fatalf("migraciones: %v", err)
	}

	if err := db.Exec(`TRUNCATE TABLE
		invoice_items, invoice_events, invoices, sent_packages, contingency_events,
		cufd_history, cuis_history, catalog_items, catalog_versions,
		catalog_sync_states, product_mappings, products, sin_products,
		certificates, api_keys, points_of_sale, branches, customers,
		tenant_configs, tenants CASCADE`).Error; err != nil {
		t.Fatalf("limpieza de base de pruebas: %v", err)
	}
	return db
}

type repoFixture struct {
	companyID string
	posID     string
	customer  *domain.Customer
	cufd      *domain.Cufd
}

func seedFixture(t *testing.T, db *gorm.DB) repoFixture {
	t.Helper()
	company := &domain.Company{
		Nit:           fmt.Sprintf("%d", time.Now().UnixNano()%10000000000),
		BusinessName:  "Empresa Test",
		CodigoSistema: "SYS-TEST",
		Ambiente:      domain.EnvironmentPiloto,
	}
	if company.Nit == "" {
		t.Fatal("nit vacío")
	}
	companyRepo := NewPostgresCompanyRepository(db)
	if err := companyRepo.Create(company); err != nil {
		t.Fatalf("seed company: %v", err)
	}

	pos := &domain.PointOfSale{
		CompanyId:        company.ID,
		CodigoSucursal:   0,
		CodigoPuntoVenta: 0,
		Description:      "PV test",
		IsActive:         true,
	}
	posRepo := NewPostgresPointOfSaleRepository(db)
	if err := posRepo.Create(pos); err != nil {
		t.Fatalf("seed pos: %v", err)
	}

	customer := &domain.Customer{
		CompanyId:      company.ID,
		DocumentType:   string(models.DocNIT),
		DocumentNumber: "123456789",
		Name:           "Cliente Test",
	}
	customerRepo := NewPostgresCustomerRepository(db)
	if err := customerRepo.Create(customer); err != nil {
		t.Fatalf("seed customer: %v", err)
	}

	cufdModel := &models.Cufd{
		TenantId:      company.ID,
		PointOfSaleId: pos.ID,
		Cufd:          "CUFD-TEST",
		Direccion:     "Calle 1",
		CodigoControl: "CTRL-01",
		ValidFrom:     time.Now().Add(-time.Hour),
		ValidTo:       time.Now().Add(24 * time.Hour),
		Active:        true,
	}
	if err := db.Create(cufdModel).Error; err != nil {
		t.Fatalf("seed cufd: %v", err)
	}

	return repoFixture{companyID: company.ID, posID: pos.ID, customer: customer, cufd: toDomainCufd(cufdModel)}
}

func nuevaFacturaPendiente(f repoFixture) *domain.Invoice {
	return &domain.Invoice{
		CompanyId:     f.companyID,
		CustomerId:    f.customer.ID,
		PointOfSaleId: f.posID,
		CufdId:        f.cufd.ID,
		EmissionType:  "EN_LINEA",
		IssueDate:     time.Now(),
		Subtotal:      100,
		Total:         116,
		Status:        domain.InvoicePending,
		Items: []domain.InvoiceItem{
			{Description: "Item 1", Quantity: 1, UnitPrice: 100, Subtotal: 100},
		},
	}
}

func TestCreateAsignaCorrelativosPorPuntoDeVenta(t *testing.T) {
	db := newTestDB(t)
	repo := NewPostgresInvoiceRepository(db).(*PostgresInvoiceRepository)
	f := seedFixture(t, db)

	for i := 1; i <= 3; i++ {
		inv := nuevaFacturaPendiente(f)
		if err := repo.Create(inv); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		if inv.InvoiceNumber != i {
			t.Errorf("correlativo esperado %d, got %d", i, inv.InvoiceNumber)
		}
	}
}

func TestClaimForEmissionEsAtomica(t *testing.T) {
	db := newTestDB(t)
	repo := NewPostgresInvoiceRepository(db).(*PostgresInvoiceRepository)
	f := seedFixture(t, db)

	inv := nuevaFacturaPendiente(f)
	if err := repo.Create(inv); err != nil {
		t.Fatalf("create: %v", err)
	}

	claimed, err := repo.ClaimForEmission(inv.ID)
	if err != nil || !claimed {
		t.Fatalf("primer claim debe ser true, got claimed=%v err=%v", claimed, err)
	}
	claimed, err = repo.ClaimForEmission(inv.ID)
	if err != nil || claimed {
		t.Fatalf("segundo claim debe ser false, got claimed=%v err=%v", claimed, err)
	}
}

func TestReleaseStaleSendingSoloLiberaAntiguas(t *testing.T) {
	db := newTestDB(t)
	repo := NewPostgresInvoiceRepository(db).(*PostgresInvoiceRepository)
	f := seedFixture(t, db)

	vieja := nuevaFacturaPendiente(f)
	reciente := nuevaFacturaPendiente(f)
	for _, inv := range []*domain.Invoice{vieja, reciente} {
		if err := repo.Create(inv); err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := repo.ClaimForEmission(inv.ID); err != nil {
			t.Fatalf("claim: %v", err)
		}
	}
	// Envejecer solo una: updated_at fuera de la ventana.
	if err := db.Exec("UPDATE invoices SET updated_at = now() - interval '1 hour' WHERE id = ?", vieja.ID).Error; err != nil {
		t.Fatalf("envejecer: %v", err)
	}

	released, err := repo.ReleaseStaleSending(10 * time.Minute)
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if released != 1 {
		t.Fatalf("esperaba liberar exactamente 1 factura, got %d", released)
	}

	gotVieja, _ := repo.GetByID(vieja.ID)
	if gotVieja.Status != domain.InvoicePending {
		t.Errorf("la factura antigua debe volver a PENDING, got %s", gotVieja.Status)
	}
	gotReciente, _ := repo.GetByID(reciente.ID)
	if gotReciente.Status != domain.InvoiceSending {
		t.Errorf("la factura reciente debe seguir SENDING, got %s", gotReciente.Status)
	}
}

func TestClaimStatusTransicionCondicional(t *testing.T) {
	db := newTestDB(t)
	repo := NewPostgresInvoiceRepository(db).(*PostgresInvoiceRepository)
	f := seedFixture(t, db)

	inv := nuevaFacturaPendiente(f)
	inv.Status = domain.InvoiceAccepted
	if err := repo.Create(inv); err != nil {
		t.Fatalf("create: %v", err)
	}

	ok, err := repo.ClaimStatus(inv.ID, domain.InvoiceAccepted, domain.InvoiceCancelled, map[string]any{"motivo_anulacion": 1})
	if err != nil || !ok {
		t.Fatalf("primera transición debe ser true, got ok=%v err=%v", ok, err)
	}
	ok, err = repo.ClaimStatus(inv.ID, domain.InvoiceAccepted, domain.InvoiceCancelled, map[string]any{"motivo_anulacion": 1})
	if err != nil || ok {
		t.Fatalf("segunda transición debe ser false, got ok=%v err=%v", ok, err)
	}

	got, _ := repo.GetByID(inv.ID)
	if got.Status != domain.InvoiceCancelled {
		t.Errorf("estado esperado CANCELLED, got %s", got.Status)
	}
	if got.MotivoAnulacion == nil || *got.MotivoAnulacion != 1 {
		t.Errorf("motivo_anulacion esperado 1, got %v", got.MotivoAnulacion)
	}
}

func TestUpdateEsParcialYNoPisaCamposAjenos(t *testing.T) {
	db := newTestDB(t)
	repo := NewPostgresInvoiceRepository(db).(*PostgresInvoiceRepository)
	f := seedFixture(t, db)

	inv := nuevaFacturaPendiente(f)
	if err := repo.Create(inv); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Simular una lectura obsoleta: otra escritura cambió el estado en BD.
	stale := nuevaFacturaPendiente(f)
	stale.ID = inv.ID
	stale.InvoiceNumber = inv.InvoiceNumber

	inv.Cuf = ptrString("CUF-TEST-0001")
	inv.Status = domain.InvoiceAccepted
	if err := repo.Update(inv); err != nil {
		t.Fatalf("update real: %v", err)
	}

	// El update con datos obsoletos no debe revertir el CUF ni tocar montos.
	stale.Status = domain.InvoiceSending
	if err := repo.Update(stale); err != nil {
		t.Fatalf("update stale: %v", err)
	}

	got, err := repo.GetByID(inv.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Total != 116 {
		t.Errorf("el total no debe verse afectado por updates parciales, got %v", got.Total)
	}
	if got.InvoiceNumber != inv.InvoiceNumber {
		t.Errorf("el correlativo no debe cambiar, got %d", got.InvoiceNumber)
	}
	if got.Status != domain.InvoiceSending {
		t.Errorf("el status sí debe actualizarce con el último update, got %s", got.Status)
	}
}

func TestCreateCorrelativoBajoConcurrencia(t *testing.T) {
	db := newTestDB(t)
	repo := NewPostgresInvoiceRepository(db).(*PostgresInvoiceRepository)
	f := seedFixture(t, db)

	const n = 8
	numeros := make(chan int, n)
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			inv := nuevaFacturaPendiente(f)
			if err := repo.Create(inv); err != nil {
				errs <- err
				return
			}
			numeros <- inv.InvoiceNumber
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("create concurrente: %v", err)
	}

	vistos := map[int]bool{}
	for len(numeros) > 0 {
		n := <-numeros
		if vistos[n] {
			t.Errorf("correlativo duplicado: %d", n)
		}
		vistos[n] = true
	}
	if len(vistos) != n {
		t.Errorf("esperaba %d correlativos únicos, got %d", n, len(vistos))
	}
}

func ptrString(s string) *string { return &s }
