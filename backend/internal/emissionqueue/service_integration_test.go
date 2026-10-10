package emissionqueue_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/emissionqueue"
	"github.com/brandsrx/supay/internal/models"
	"github.com/brandsrx/supay/internal/repository/database"
	repository "github.com/brandsrx/supay/internal/repository/postgres"
	"github.com/brandsrx/supay/internal/testutil"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type acceptingProcessor struct {
	db *gorm.DB
}

func (p *acceptingProcessor) ProcessEmission(_ context.Context, tenantID, invoiceID string) (*domain.Invoice, error) {
	if err := p.db.Model(&models.Invoice{}).
		Where("id = ? AND tenant_id = ?", invoiceID, tenantID).
		Update("status", models.StatusAccepted).Error; err != nil {
		return nil, err
	}
	return &domain.Invoice{ID: invoiceID, CompanyId: tenantID, Status: domain.InvoiceAccepted}, nil
}

func TestEmissionQueuePendingToAcceptedWithTraffic(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL no está definida; omitiendo integración River")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("conexión a BD de pruebas: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("pool de BD de pruebas: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	testutil.LockPostgres(t, sqlDB)
	if err := database.MigrateDB(db); err != nil {
		t.Fatalf("migraciones: %v", err)
	}

	company := &domain.Company{
		Nit: fmt.Sprintf("%d", time.Now().UnixNano()%10000000000), BusinessName: "River Integration", Ambiente: domain.EnvironmentPiloto,
	}
	if err := repository.NewPostgresCompanyRepository(db).Create(company); err != nil {
		t.Fatalf("crear empresa: %v", err)
	}
	pos := &domain.PointOfSale{CompanyId: company.ID, Description: "PV River", IsActive: true}
	if err := repository.NewPostgresPointOfSaleRepository(db).Create(pos); err != nil {
		t.Fatalf("crear POS: %v", err)
	}
	customer := &domain.Customer{CompanyId: company.ID, DocumentType: string(models.DocNIT), DocumentNumber: "123456789", CodigoCliente: "123456789", Name: "Cliente River"}
	if err := repository.NewPostgresCustomerRepository(db).Create(customer); err != nil {
		t.Fatalf("crear cliente: %v", err)
	}
	cufd := &models.Cufd{
		TenantId: company.ID, PointOfSaleId: pos.ID, Cufd: "CUFD-RIVER", Direccion: "Calle 1",
		CodigoControl: "CTRL-RIVER", ValidFrom: time.Now().Add(-time.Hour), ValidTo: time.Now().Add(time.Hour), Active: true,
	}
	if err := db.Create(cufd).Error; err != nil {
		t.Fatalf("crear CUFD: %v", err)
	}
	activity, productCode, unitCode := "101010", "5113100", 58
	invoice := &domain.Invoice{
		CompanyId: company.ID, CustomerId: customer.ID, PointOfSaleId: pos.ID, CufdId: cufd.ID,
		EmissionType: "EN_LINEA", IssueDate: time.Now(), Subtotal: 100, Total: 116,
		Status: domain.InvoicePending, Customer: *customer,
		Items: []domain.InvoiceItem{{
			Code: "ITEM-1", Description: "Item 1", CodigoActividad: &activity,
			CodigoProductoSin: &productCode, UnitCode: &unitCode, Quantity: 1, UnitPrice: 100, Subtotal: 100,
		}},
	}
	if err := repository.NewPostgresInvoiceRepository(db).Create(invoice); err != nil {
		t.Fatalf("crear factura: %v", err)
	}

	outbox := repository.NewPostgresOutboxRepository(db)
	queue, err := emissionqueue.NewService(db, outbox, &acceptingProcessor{db: db}, emissionqueue.Config{
		DispatchInterval: 10 * time.Millisecond, DispatchBatchSize: 10, OutboxLockTimeout: 30 * time.Second,
		MaxWorkers: 1, MaxAttempts: 3, JobTimeout: 5 * time.Second, SoftStopTimeout: time.Second,
		RetryBase: 10 * time.Millisecond, RetryMax: time.Second, RatePerSecond: 100, RateBurst: 1,
	})
	if err != nil {
		t.Fatalf("crear cola: %v", err)
	}
	if err := queue.Start(context.Background()); err != nil {
		t.Fatalf("iniciar cola: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := queue.Stop(ctx); err != nil {
			t.Errorf("detener cola: %v", err)
		}
	})

	event, err := outbox.EnqueueInvoiceEmission(context.Background(), invoice.ID, company.ID, cufd.ID)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		var storedInvoice models.Invoice
		var storedEvent models.OutboxEvent
		if db.First(&storedInvoice, "id = ?", invoice.ID).Error == nil &&
			db.First(&storedEvent, "id = ?", event.ID).Error == nil &&
			storedInvoice.Status == models.StatusAccepted && storedEvent.Status == domain.OutboxStatusPublished {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("la cola no llevó outbox PENDING a factura ACCEPTED dentro del plazo")
}
