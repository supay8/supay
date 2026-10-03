package invoiceemail

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
)

type dispatcherRepoStub struct {
	pending          []domain.InvoiceEmailNotification
	enqueuedID       string
	enqueuedTaskName string
	failedID         string
	failedMessage    string
	nextAttempt      time.Time
}

func (r *dispatcherRepoStub) ClaimPending(context.Context, string, int, time.Time, time.Duration) ([]domain.InvoiceEmailNotification, error) {
	return r.pending, nil
}

func (r *dispatcherRepoStub) MarkEnqueued(_ context.Context, id, _ string, taskName string, _ time.Time) error {
	r.enqueuedID, r.enqueuedTaskName = id, taskName
	return nil
}

func (r *dispatcherRepoStub) MarkPublishFailed(_ context.Context, id, _ string, message string, next time.Time) error {
	r.failedID, r.failedMessage, r.nextAttempt = id, message, next
	return nil
}

func (*dispatcherRepoStub) ClaimDelivery(context.Context, string, time.Time, time.Duration) (*domain.InvoiceEmailNotification, bool, error) {
	panic("not used")
}

func (*dispatcherRepoStub) MarkSent(context.Context, string, time.Time) error { panic("not used") }

func (*dispatcherRepoStub) MarkDeliveryFailed(context.Context, string, string, time.Time) error {
	panic("not used")
}

type publisherStub struct {
	taskName string
	err      error
	seenID   string
}

func (p *publisherStub) Publish(_ context.Context, id string) (string, error) {
	p.seenID = id
	return p.taskName, p.err
}

func TestDispatcherMarksTaskEnqueuedAfterPublish(t *testing.T) {
	repo := &dispatcherRepoStub{pending: []domain.InvoiceEmailNotification{{ID: "notification-1"}}}
	publisher := &publisherStub{taskName: "projects/p/locations/l/queues/q/tasks/t"}
	dispatcher := NewDispatcher(repo, publisher, 10, time.Minute)

	count, err := dispatcher.DispatchBatch(context.Background())
	if err != nil {
		t.Fatalf("DispatchBatch: %v", err)
	}
	if count != 1 || publisher.seenID != "notification-1" {
		t.Fatalf("publicación inesperada: count=%d id=%q", count, publisher.seenID)
	}
	if repo.enqueuedID != "notification-1" || repo.enqueuedTaskName != publisher.taskName {
		t.Fatalf("no se confirmó la tarea: id=%q task=%q", repo.enqueuedID, repo.enqueuedTaskName)
	}
}

func TestDispatcherKeepsNotificationPendingWhenPublishFails(t *testing.T) {
	repo := &dispatcherRepoStub{pending: []domain.InvoiceEmailNotification{{ID: "notification-2", PublishAttempts: 1}}}
	publishErr := errors.New("cloud tasks unavailable")
	dispatcher := NewDispatcher(repo, &publisherStub{err: publishErr}, 10, time.Minute)
	fixedNow := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	dispatcher.now = func() time.Time { return fixedNow }

	count, err := dispatcher.DispatchBatch(context.Background())
	if count != 1 || !errors.Is(err, publishErr) {
		t.Fatalf("resultado inesperado: count=%d err=%v", count, err)
	}
	if repo.failedID != "notification-2" || repo.failedMessage != publishErr.Error() {
		t.Fatalf("el fallo no quedó persistido: id=%q error=%q", repo.failedID, repo.failedMessage)
	}
	if !repo.nextAttempt.After(fixedNow) {
		t.Fatalf("la republicación no fue reprogramada: %s", repo.nextAttempt)
	}
	if repo.enqueuedID != "" {
		t.Fatalf("una publicación fallida no debe marcarse encolada: %q", repo.enqueuedID)
	}
}
