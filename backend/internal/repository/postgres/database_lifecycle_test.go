package postgres

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
)

func TestDatabaseCustomerContactPreservesInvoiceSnapshot(t *testing.T) {
	db := newTestDB(t)
	f := seedFixture(t, db)
	inv := nuevaFacturaPendiente(f)
	repo := NewPostgresInvoiceRepository(db)
	if err := repo.Create(inv); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE customers SET email = 'new@example.com' WHERE id = ?", f.customer.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE customers SET name = 'Changed' WHERE id = ?", f.customer.ID).Error; err == nil {
		t.Fatal("fiscal identity update succeeded")
	}
	got, err := repo.GetByID(f.companyID, inv.ID)
	if err != nil || got.Customer.Email == nil || *got.Customer.Email != *f.customer.Email {
		t.Fatalf("snapshot changed: %+v %v", got, err)
	}
	latest := *f.customer
	latest.Name = "Latest Receiver"
	latest.ID = ""
	customers := NewPostgresCustomerRepository(db)
	if err := customers.Create(&latest); err != nil {
		t.Fatal(err)
	}
	rows, err := customers.List(f.companyID)
	if err != nil || len(rows) != 1 || rows[0].ID != latest.ID {
		t.Fatalf("current receiver: %+v %v", rows, err)
	}
}

func TestDatabaseInvoiceFilesAndDocuments(t *testing.T) {
	db := newTestDB(t)
	a, b := seedFixture(t, db), seedFixture(t, db)
	inv := nuevaFacturaPendiente(a)
	if err := NewPostgresInvoiceRepository(db).Create(inv); err != nil {
		t.Fatal(err)
	}
	files := NewPostgresInvoiceFileRepository(db)
	file := &domain.InvoiceFile{CompanyID: a.companyID, InvoiceID: inv.ID, Kind: "xml", StorageKey: "invoice/test.xml", SHA256: fmt.Sprintf("%064d", 1), ContentType: "application/xml"}
	if err := files.CreateFile(context.Background(), file); err != nil {
		t.Fatal(err)
	}
	got, err := files.FindFile(context.Background(), a.companyID, inv.ID, "xml")
	if err != nil || got.CompanyID != a.companyID || got.ID != file.ID {
		t.Fatalf("file lookup: %+v %v", got, err)
	}
	if _, err := files.FindFile(context.Background(), b.companyID, inv.ID, "xml"); err == nil {
		t.Fatal("cross-tenant read succeeded")
	}
	file.CompanyID, file.Kind, file.StorageKey = b.companyID, "pdf", "wrong.pdf"
	if err := files.CreateFile(context.Background(), file); err == nil {
		t.Fatal("cross-tenant FK succeeded")
	}
	for v := 1; v <= 2; v++ {
		if err := db.Exec(`INSERT INTO invoice_documents(invoice_id, document_type, version, storage_ref) VALUES (?, 'PDF', ?, ?)`, inv.ID, v, fmt.Sprintf("pdf/%d", v)).Error; err != nil {
			t.Fatal(err)
		}
	}
	var current int
	if err := db.Raw(`SELECT version FROM invoice_documents WHERE invoice_id = ? AND is_current`, inv.ID).Scan(&current).Error; err != nil || current != 2 {
		t.Fatalf("current=%d %v", current, err)
	}
	if err := db.Exec(`UPDATE invoice_documents SET storage_ref = 'tampered' WHERE invoice_id = ?`, inv.ID).Error; err == nil {
		t.Fatal("document mutated")
	}
}

func TestDatabaseEmailAdditionalRecipientsAndResends(t *testing.T) {
	db := newTestDB(t)
	f := seedFixture(t, db)
	inv := nuevaFacturaPendiente(f)
	inv.Status = domain.InvoiceAccepted
	if err := NewPostgresInvoiceRepository(db).Create(inv); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE tenant_configs SET settings = '{"invoice_email":{"enabled":true}}' WHERE tenant_id = ?`, f.companyID).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := queueInvoiceEmailNotification(db, f.companyID, inv.ID); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewPostgresInvoiceEmailNotificationRepository(db)
	first, err := repo.QueueDelivery(context.Background(), f.companyID, inv.ID, " CLIENTE@EXAMPLE.COM ")
	if err != nil {
		t.Fatal(err)
	}
	same, err := repo.QueueDelivery(context.Background(), f.companyID, inv.ID, "cliente@example.com")
	if err != nil || same == first {
		t.Fatalf("resend requires another delivery ID: %s %s %v", first, same, err)
	}
	if _, err := repo.QueueDelivery(context.Background(), f.companyID, inv.ID, "second@example.com"); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Table("invoice_email_notifications").Where("invoice_id = ?", inv.ID).Count(&count)
	if count != 4 {
		t.Fatalf("deliveries=%d", count)
	}
	if _, err := repo.QueueDelivery(context.Background(), uuid.NewString(), inv.ID, "second@example.com"); err == nil {
		t.Fatal("cross tenant delivery succeeded")
	}
}

func TestDatabaseAuthAuthoritiesAndArchive(t *testing.T) {
	db := newTestDB(t)
	local, cloud := seedFixture(t, db), seedFixture(t, db)
	uid, org, cloudUID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users(id,email,name,password_hash) VALUES (?, ?, 'Local', 'hash')`, []any{uid, uid + "@example.com"}},
		{`INSERT INTO auth.users(id,email,name) VALUES (?, ?, 'Cloud')`, []any{cloudUID, " CLOUD-" + cloudUID + "@EXAMPLE.COM "}},
		{`INSERT INTO auth.organizations(id,name,slug) VALUES (?, 'Cloud', ?)`, []any{org, org}},
		{`UPDATE tenants SET auth_organization_id = ? WHERE id = ?`, []any{org, cloud.companyID}},
		{`INSERT INTO user_tenants(user_id,tenant_id) VALUES (?, ?), (?, ?)`, []any{uid, local.companyID, uid, cloud.companyID}},
		{`INSERT INTO auth.members(user_id,organization_id) VALUES (?, ?)`, []any{cloudUID, org}},
	} {
		if err := db.Exec(q.sql, q.args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	localRepo := NewPostgresAuthRepository(db)
	cloudRepo := NewBetterAuthMembershipRepository(db)
	if ok, err := localRepo.HasCompanyAccess(uid, cloud.companyID); err != nil || ok {
		t.Fatalf("local authority entered cloud: %v %v", ok, err)
	}
	if ok, err := cloudRepo.HasCompanyAccess(cloudUID, cloud.companyID); err != nil || !ok {
		t.Fatalf("cloud access: %v %v", ok, err)
	}
	key := &domain.ApiKey{CompanyId: cloud.companyID, KeyHash: uuid.NewString(), KeyPrefix: "test-key", Scopes: []string{"read", "write"}, IsActive: true}
	keys := NewPostgresApiKeyRepository(db)
	if err := keys.Create(key); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`SELECT archive_tenant(?, 'closed by administrator')`, cloud.companyID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := keys.FindByPrefix(key.KeyPrefix); err == nil {
		t.Fatal("archived key still authorized")
	}
	if ok, err := cloudRepo.HasCompanyAccess(cloudUID, cloud.companyID); err != nil || ok {
		t.Fatalf("archived cloud authorized: %v %v", ok, err)
	}
	if err := db.Exec(`DELETE FROM tenants WHERE id = ?`, cloud.companyID).Error; err == nil {
		t.Fatal("archived fiscal tenant deleted")
	}
	var stored models.ApiKey
	if err := db.First(&stored, "id = ?", key.ID).Error; err != nil || len(stored.Scopes) != 2 || stored.IsActive {
		t.Fatalf("archive / scopes: %+v %v", stored, err)
	}
	var email string
	db.Raw(`SELECT email FROM auth.users WHERE id = ?`, cloudUID).Scan(&email)
	if email != "cloud-"+cloudUID+"@example.com" {
		t.Fatalf("email=%q", email)
	}
}

func TestDatabaseOutboxAndCatalogConcurrency(t *testing.T) {
	db := newTestDB(t)
	f := seedFixture(t, db)
	inv := nuevaFacturaPendiente(f)
	if err := NewPostgresInvoiceRepository(db).Create(inv); err != nil {
		t.Fatal(err)
	}
	repo := NewPostgresOutboxRepository(db)
	results := make(chan *domain.OutboxEvent, 8)
	errs := make(chan error, 16)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e, err := repo.EnqueueInvoiceEmission(context.Background(), inv.ID, f.companyID, f.cufd.ID)
			if err != nil {
				errs <- err
			} else {
				results <- e
			}
			if err := replaceVersionedCatalog(db, f.companyID, "test-concurrent", time.Now(), nil); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	id := ""
	for event := range results {
		if id != "" && event.ID != id {
			t.Fatal("concurrent emission duplicated")
		}
		id = event.ID
	}
	var versions int64
	db.Model(&models.CatalogVersion{}).Where("tenant_id = ? AND tipo = 'test-concurrent'", f.companyID).Count(&versions)
	if versions != 8 {
		t.Fatalf("catalog versions=%d", versions)
	}
	for i := 0; i < 2; i++ {
		if err := db.Exec(`INSERT INTO outbox(tenant_id,aggregate_type,aggregate_id,event_type) VALUES (?, 'invoice', ?, 'invoice.status.changed')`, f.companyID, inv.ID).Error; err != nil {
			t.Fatal(err)
		}
	}
	var before, after time.Time
	db.Raw(`SELECT updated_at FROM tenants WHERE id = ?`, f.companyID).Scan(&before)
	if err := db.Exec(`UPDATE tenants SET business_name = 'Updated' WHERE id = ?`, f.companyID).Error; err != nil {
		t.Fatal(err)
	}
	db.Raw(`SELECT updated_at FROM tenants WHERE id = ?`, f.companyID).Scan(&after)
	if !after.After(before) {
		t.Fatal("updated_at was not maintained")
	}
}

func TestDatabaseRejectedPackagePreservesMembershipHistory(t *testing.T) {
	db := newTestDB(t)
	f := seedFixture(t, db)
	inv := nuevaFacturaPendiente(f)
	if err := NewPostgresInvoiceRepository(db).Create(inv); err != nil {
		t.Fatal(err)
	}
	packages := NewPostgresSentPackageRepository(db)
	first := &domain.SentPackage{CompanyId: f.companyID, PointOfSaleId: f.posID, Type: domain.PackageTypePaquete, CodigoRecepcion: uuid.NewString(), HashArchivo: "hash", CantidadFacturas: 1, CodigoDocumentoSector: 1, CodigoTipoFactura: 1, CodigoEmision: 2, Status: domain.PackageStatusSent, XmlHash: "hash", SentAt: time.Now()}
	if err := packages.Create(first); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.SentPackageInvoice{TenantID: f.companyID, InvoiceID: inv.ID, SentPackageID: first.ID}).Error; err != nil {
		t.Fatal(err)
	}
	second := *first
	second.ID = ""
	second.CodigoRecepcion = uuid.NewString()
	if err := packages.Create(&second); err != nil {
		t.Fatal(err)
	}
	membership := &models.SentPackageInvoice{TenantID: f.companyID, InvoiceID: inv.ID, SentPackageID: second.ID}
	if err := db.Create(membership).Error; err == nil {
		t.Fatal("two live packages reserved same invoice")
	}
	if err := db.Exec(`UPDATE sent_packages SET status = 'REJECTED' WHERE id = ?`, first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(membership).Error; err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&models.SentPackageInvoice{}).Where("invoice_id = ?", inv.ID).Count(&count)
	if err := db.Exec(`UPDATE sent_packages SET status = 'SENT' WHERE id = ?`, first.ID).Error; err == nil {
		t.Fatal("rejected package revived and invalidated reservation isolation")
	}
	if count != 2 {
		t.Fatalf("membership history=%d", count)
	}
	if err := db.Exec(`UPDATE sent_package_invoices SET position = 2 WHERE invoice_id = ?`, inv.ID).Error; err == nil {
		t.Fatal("membership history mutated")
	}
}

func TestDatabaseCufUniqueWithoutRequestKeys(t *testing.T) {
	db := newTestDB(t)
	fixture := seedFixture(t, db)
	repo := NewPostgresInvoiceRepository(db)
	first := nuevaFacturaPendiente(fixture)
	cuf := "CUF-EXISTING-FISCAL-DOCUMENT"
	first.Cuf = &cuf
	if err := repo.Create(first); err != nil {
		t.Fatal(err)
	}
	duplicate := nuevaFacturaPendiente(fixture)
	duplicate.Cuf = &cuf
	if err := repo.Create(duplicate); err == nil {
		t.Fatal("duplicate fiscal CUF accepted")
	}
	stored, err := repo.GetByID(fixture.companyID, first.ID)
	if err != nil || stored.Cuf == nil || *stored.Cuf != cuf || stored.InvoiceNumber != first.InvoiceNumber {
		t.Fatalf("original identity changed: %+v %v", stored, err)
	}
	second := nuevaFacturaPendiente(fixture)
	if err := repo.Create(second); err != nil {
		t.Fatal(err)
	}
	if second.InvoiceNumber != first.InvoiceNumber+1 {
		t.Fatal("failed duplicate consumed the invoice sequence")
	}
	var keyColumns int64
	if err := db.Raw(`SELECT count(*) FROM information_schema.columns
  WHERE table_schema = 'public' AND table_name IN ('invoices', 'outbox', 'invoice_email_notifications')
    AND column_name = 'idempotency_key'`).Scan(&keyColumns).Error; err != nil || keyColumns != 0 {
		t.Fatalf("obsolete key columns=%d err=%v", keyColumns, err)
	}
}
