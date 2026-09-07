package emissionqueue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/observability"
	"github.com/google/uuid"
)

type OutboxPublisher interface {
	Publish(ctx context.Context, event domain.OutboxEvent) error
}

type Dispatcher struct {
	repo        domain.OutboxRepository
	publisher   OutboxPublisher
	owner       string
	interval    time.Duration
	batchSize   int
	lockTimeout time.Duration
	now         func() time.Time
	metrics     *observability.Metrics
}

func (d *Dispatcher) WithMetrics(metrics *observability.Metrics) *Dispatcher {
	d.metrics = metrics
	return d
}

func NewDispatcher(repo domain.OutboxRepository, publisher OutboxPublisher, interval time.Duration, batchSize int, lockTimeout time.Duration) *Dispatcher {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	if lockTimeout <= 0 {
		lockTimeout = time.Minute
	}
	return &Dispatcher{
		repo: repo, publisher: publisher, owner: "outbox-" + uuid.NewString(),
		interval: interval, batchSize: batchSize, lockTimeout: lockTimeout, now: timeNow(),
	}
}

func (d *Dispatcher) Run(ctx context.Context) {
	d.dispatchAndLog(ctx)
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.dispatchAndLog(ctx)
		}
	}
}

func (d *Dispatcher) DispatchOnce(ctx context.Context) error {
	if d.repo == nil || d.publisher == nil {
		return errors.New("dispatcher de outbox no configurado")
	}
	now := d.now()
	events, err := d.repo.ClaimPending(ctx, domain.OutboxEventInvoiceEmit, d.owner, d.batchSize, now, d.lockTimeout)
	if err != nil {
		return fmt.Errorf("reclamar outbox: %w", err)
	}
	var dispatchErrors []error
	for _, event := range events {
		if err := d.publisher.Publish(ctx, event); err != nil {
			d.metrics.ObserveOutbox("publish_failed")
			next := now.Add(outboxRetryDelay(event.Attempts))
			if markErr := d.repo.MarkFailed(ctx, event.ID, d.owner, err.Error(), next); markErr != nil {
				d.metrics.ObserveOutbox("mark_failed")
				dispatchErrors = append(dispatchErrors, errors.Join(err, markErr))
			} else {
				dispatchErrors = append(dispatchErrors, err)
			}
			continue
		}
		if err := d.repo.MarkPublished(ctx, event.ID, d.owner, d.now()); err != nil {
			d.metrics.ObserveOutbox("mark_failed")
			dispatchErrors = append(dispatchErrors, err)
		} else {
			d.metrics.ObserveOutbox("published")
		}
	}
	return errors.Join(dispatchErrors...)
}

func (d *Dispatcher) dispatchAndLog(ctx context.Context) {
	if err := d.DispatchOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("outbox: uno o más eventos no pudieron publicarse", "error_type", fmt.Sprintf("%T", err))
	}
}

func outboxRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	shift := min(attempt-1, 8)
	delay := time.Second * time.Duration(1<<shift)
	return min(delay, 5*time.Minute)
}
