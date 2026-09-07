package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
)

func TestOutboxEnqueueClaimRetryAndPublish(t *testing.T) {
	db := newTestDB(t)
	fixture := seedFixture(t, db)
	invoice := nuevaFacturaPendiente(fixture)
	invoiceRepo := NewPostgresInvoiceRepository(db)
	if err := invoiceRepo.Create(invoice); err != nil {
		t.Fatalf("crear factura: %v", err)
	}

	repo := NewPostgresOutboxRepository(db)
	first, err := repo.EnqueueInvoiceEmission(context.Background(), invoice.ID, fixture.companyID, fixture.cufd.ID)
	if err != nil {
		t.Fatalf("primer enqueue: %v", err)
	}
	second, err := repo.EnqueueInvoiceEmission(context.Background(), invoice.ID, fixture.companyID, fixture.cufd.ID)
	if err != nil {
		t.Fatalf("segundo enqueue: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("enqueue no idempotente: %s != %s", first.ID, second.ID)
	}
	var count int64
	if err := db.Model(&models.OutboxEvent{}).Where("aggregate_id = ?", invoice.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("eventos=%d err=%v", count, err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	events, err := repo.ClaimPending(context.Background(), domain.OutboxEventInvoiceEmit, "worker-a", 10, now, time.Minute)
	if err != nil || len(events) != 1 || events[0].Attempts != 1 {
		t.Fatalf("claim eventos=%+v err=%v", events, err)
	}
	next := now.Add(time.Minute)
	if err := repo.MarkFailed(context.Background(), first.ID, "worker-a", "river temporal", next); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	events, err = repo.ClaimPending(context.Background(), domain.OutboxEventInvoiceEmit, "worker-b", 10, now, time.Minute)
	if err != nil || len(events) != 0 {
		t.Fatalf("claim antes del backoff eventos=%+v err=%v", events, err)
	}
	events, err = repo.ClaimPending(context.Background(), domain.OutboxEventInvoiceEmit, "worker-b", 10, next, time.Minute)
	if err != nil || len(events) != 1 || events[0].Attempts != 2 {
		t.Fatalf("claim reintento eventos=%+v err=%v", events, err)
	}
	if err := repo.MarkPublished(context.Background(), first.ID, "worker-b", next); err != nil {
		t.Fatalf("MarkPublished: %v", err)
	}

	var stored models.OutboxEvent
	if err := db.First(&stored, "id = ?", first.ID).Error; err != nil {
		t.Fatalf("leer outbox: %v", err)
	}
	if stored.Status != domain.OutboxStatusPublished || stored.PublishedAt == nil {
		t.Fatalf("estado final=%s published_at=%v", stored.Status, stored.PublishedAt)
	}

	// Un replay vuelve a poner el mismo evento en PENDING. Si el dispatcher que
	// lo reclama cae, otro puede recuperar el lock cuando vence el lease.
	if _, err := repo.EnqueueInvoiceEmission(context.Background(), invoice.ID, fixture.companyID, fixture.cufd.ID); err != nil {
		t.Fatalf("reencolar evento publicado: %v", err)
	}
	events, err = repo.ClaimPending(context.Background(), domain.OutboxEventInvoiceEmit, "worker-caido", 10, next, time.Minute)
	if err != nil || len(events) != 1 {
		t.Fatalf("claim para simular crash eventos=%+v err=%v", events, err)
	}
	events, err = repo.ClaimPending(context.Background(), domain.OutboxEventInvoiceEmit, "worker-recuperacion", 10, next.Add(2*time.Minute), time.Minute)
	if err != nil || len(events) != 1 || events[0].LockedBy == nil || *events[0].LockedBy != "worker-recuperacion" {
		t.Fatalf("recuperación de lock eventos=%+v err=%v", events, err)
	}
	if !db.Migrator().HasTable("river_job") {
		t.Fatal("las migraciones deben instalar el esquema de River")
	}
}
