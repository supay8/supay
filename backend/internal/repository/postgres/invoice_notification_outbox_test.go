package postgres

import (
	"testing"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func enableTenantInvoiceEmail(t *testing.T, db *gorm.DB, tenantID string) {
	t.Helper()
	if err := db.Exec(`UPDATE tenant_configs SET settings = '{"invoice_email":{"enabled":true}}' WHERE tenant_id = ?`, tenantID).Error; err != nil {
		t.Fatal(err)
	}
}

func assertAutomaticDeliveries(t *testing.T, db *gorm.DB, invoiceID string, want int64) {
	t.Helper()
	var count int64
	if err := db.Table("invoice_email_notifications").Where("invoice_id = ? AND is_automatic", invoiceID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("automatic deliveries = %d, want %d", count, want)
	}
}

func TestInvoiceNotificationOutboxRollsBackWithFiscalTransition(t *testing.T) {
	db := newTestDB(t)
	f := seedFixture(t, db)
	enableTenantInvoiceEmail(t, db, f.companyID)
	repo := NewPostgresInvoiceRepository(db)
	inv := nuevaFacturaPendiente(f)
	if err := repo.Create(inv); err != nil {
		t.Fatal(err)
	}
	if claimed, err := repo.ClaimForEmission(f.companyID, inv.ID); err != nil || !claimed {
		t.Fatalf("claim = %v, %v", claimed, err)
	}

	// La auditoría de otro tenant falla después de escribir el outbox.
	event := &domain.InvoiceEvent{TenantID: uuid.NewString(), Type: "STATUS_TRANSITION"}
	if _, err := repo.TransitionStatus(f.companyID, inv.ID, domain.InvoiceSending, domain.InvoiceAccepted, domain.TransitionSIATAccepted, nil, event); err == nil {
		t.Fatal("expected audit failure")
	}
	stored, err := repo.GetByID(f.companyID, inv.ID)
	if err != nil || stored.Status != domain.InvoiceSending {
		t.Fatalf("fiscal transition was not rolled back: %+v, %v", stored, err)
	}
	assertAutomaticDeliveries(t, db, inv.ID, 0)

	if claimed, err := repo.TransitionStatus(f.companyID, inv.ID, domain.InvoiceSending, domain.InvoiceObserved, domain.TransitionSIATObserved, nil, nil); err != nil || !claimed {
		t.Fatalf("observe = %v, %v", claimed, err)
	}
	assertAutomaticDeliveries(t, db, inv.ID, 1)
	if claimed, err := repo.TransitionStatus(f.companyID, inv.ID, domain.InvoiceObserved, domain.InvoiceAccepted, domain.TransitionSIATReconciliation, nil, nil); err != nil || !claimed {
		t.Fatalf("accept = %v, %v", claimed, err)
	}
	assertAutomaticDeliveries(t, db, inv.ID, 1)
}

func TestAcceptedBatchPersistsNotificationOutboxWithoutProcessFlags(t *testing.T) {
	db := newTestDB(t)
	f := seedFixture(t, db)
	enableTenantInvoiceEmail(t, db, f.companyID)
	inv := nuevaFacturaPendiente(f)
	inv.Modalidad = 2
	if err := NewPostgresInvoiceRepository(db).Create(inv); err != nil {
		t.Fatal(err)
	}
	pkg := &domain.SentPackage{
		CompanyId: f.companyID, PointOfSaleId: f.posID, Type: domain.PackageTypeMasiva,
		CantidadFacturas: 1, CodigoDocumentoSector: 1, CodigoTipoFactura: 1,
		CodigoEmision: 3, Modalidad: 2, CufdID: f.cufd.ID, Cufd: f.cufd.Cufd, Cuis: "CUIS-TEST",
		Documents: []domain.BatchInvoiceDocument{{Cuf: "CUF-TEST", Xml: "<factura/>"}},
	}
	repo := NewPostgresSentPackageRepository(db)
	if err := repo.ReserveBatch(pkg, []string{inv.ID}, domain.InvoicePending); err != nil {
		t.Fatal(err)
	}
	assertAutomaticDeliveries(t, db, inv.ID, 0)
	pkg.Status, pkg.CodigoRecepcion = domain.PackageStatusAccepted, "RECEPTION-TEST"
	accepted := domain.InvoiceAccepted
	for i := 0; i < 2; i++ {
		if err := repo.UpdateBatch(pkg, &accepted); err != nil {
			t.Fatal(err)
		}
	}
	assertAutomaticDeliveries(t, db, inv.ID, 1)
	stored, err := NewPostgresInvoiceRepository(db).GetByID(f.companyID, inv.ID)
	if err != nil || stored.Status != domain.InvoiceAccepted {
		t.Fatalf("batch result = %+v, %v", stored, err)
	}
}
