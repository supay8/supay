package emissionqueue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
)

type fakeOutboxRepo struct {
	events      []domain.OutboxEvent
	published   []string
	failed      []string
	nextAttempt time.Time
	lastError   string
}

func (f *fakeOutboxRepo) EnqueueInvoiceEmission(context.Context, string, string, string) (*domain.OutboxEvent, error) {
	return nil, nil
}

func (f *fakeOutboxRepo) ClaimPending(context.Context, string, string, int, time.Time, time.Duration) ([]domain.OutboxEvent, error) {
	return f.events, nil
}

func (f *fakeOutboxRepo) MarkPublished(_ context.Context, id, _ string, _ time.Time) error {
	f.published = append(f.published, id)
	return nil
}

func (f *fakeOutboxRepo) MarkFailed(_ context.Context, id, _, lastError string, nextAttempt time.Time) error {
	f.failed = append(f.failed, id)
	f.lastError = lastError
	f.nextAttempt = nextAttempt
	return nil
}

type fakePublisher struct {
	err error
}

func (f *fakePublisher) Publish(context.Context, domain.OutboxEvent) error { return f.err }

func TestDispatcherMarcaEventoPublicado(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	repo := &fakeOutboxRepo{events: []domain.OutboxEvent{{ID: "evt-1", Attempts: 1}}}
	dispatcher := NewDispatcher(repo, &fakePublisher{}, time.Second, 10, time.Minute)
	dispatcher.now = func() time.Time { return now }

	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce: %v", err)
	}
	if len(repo.published) != 1 || repo.published[0] != "evt-1" {
		t.Fatalf("publicados=%v", repo.published)
	}
}

func TestDispatcherReprogramaFalloConBackoff(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	repo := &fakeOutboxRepo{events: []domain.OutboxEvent{{ID: "evt-1", Attempts: 3}}}
	dispatcher := NewDispatcher(repo, &fakePublisher{err: errors.New("river no disponible")}, time.Second, 10, time.Minute)
	dispatcher.now = func() time.Time { return now }

	if err := dispatcher.DispatchOnce(context.Background()); err == nil {
		t.Fatal("se esperaba error agregado")
	}
	if len(repo.failed) != 1 || !repo.nextAttempt.Equal(now.Add(4*time.Second)) {
		t.Fatalf("fallidos=%v next=%s", repo.failed, repo.nextAttempt)
	}
}
