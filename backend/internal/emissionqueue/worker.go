package emissionqueue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/observability"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/riverqueue/river"
)

const (
	InvoiceEmissionJobKind = "invoice_emission"
	InvoiceEmissionQueue   = "invoice_emission"
)

type InvoiceEmissionArgs struct {
	InvoiceID string `json:"invoice_id" river:"unique"`
	TenantID  string `json:"tenant_id"`
	OutboxID  string `json:"outbox_id"`
}

func (InvoiceEmissionArgs) Kind() string { return InvoiceEmissionJobKind }

type InvoiceEmissionProcessor interface {
	ProcessEmission(ctx context.Context, invoiceID string) (*domain.Invoice, error)
}

type InvoiceEmissionWorker struct {
	river.WorkerDefaults[InvoiceEmissionArgs]
	processor InvoiceEmissionProcessor
	limiter   *TenantRateLimiter
	breaker   *CircuitBreaker
	timeout   time.Duration
	metrics   *observability.Metrics
}

func (w *InvoiceEmissionWorker) WithMetrics(metrics *observability.Metrics) *InvoiceEmissionWorker {
	w.metrics = metrics
	return w
}

func NewInvoiceEmissionWorker(processor InvoiceEmissionProcessor, limiter *TenantRateLimiter, breaker *CircuitBreaker, timeout time.Duration) *InvoiceEmissionWorker {
	return &InvoiceEmissionWorker{processor: processor, limiter: limiter, breaker: breaker, timeout: timeout}
}

func (w *InvoiceEmissionWorker) Timeout(*river.Job[InvoiceEmissionArgs]) time.Duration {
	return w.timeout
}

func (w *InvoiceEmissionWorker) Work(ctx context.Context, job *river.Job[InvoiceEmissionArgs]) error {
	started := time.Now()
	result := "unknown"
	defer func() { w.metrics.ObserveEmission(result, time.Since(started)) }()

	args := job.Args
	if args.InvoiceID == "" || args.TenantID == "" {
		result = "discarded"
		return river.JobCancel(errors.New("invoice_id y tenant_id son obligatorios"))
	}
	if w.processor == nil {
		result = "discarded"
		return river.JobCancel(errors.New("procesador de emisión no configurado"))
	}

	if w.breaker != nil {
		if allowed, retryAfter := w.breaker.Allow(args.TenantID); !allowed {
			result = "circuit_open"
			return river.JobSnooze(retryAfter)
		}
	}
	if w.limiter != nil {
		if allowed, retryAfter := w.limiter.TryAcquire(args.TenantID); !allowed {
			if w.breaker != nil {
				w.breaker.AbortProbe(args.TenantID)
			}
			result = "rate_limited"
			return river.JobSnooze(retryAfter)
		}
	}

	_, err := w.processor.ProcessEmission(ctx, args.InvoiceID)
	if err == nil {
		if w.breaker != nil {
			w.breaker.Success(args.TenantID)
		}
		result = "success"
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		if w.breaker != nil {
			w.breaker.AbortProbe(args.TenantID)
		}
		result = "retry"
		return err
	}

	var rejected *usecase.EmissionRejectedError
	if errors.As(err, &rejected) {
		// SIAT answered: this is a terminal business rejection, not an outage.
		if w.breaker != nil {
			w.breaker.Success(args.TenantID)
		}
		result = "rejected"
		return river.JobCancel(err)
	}
	var inProgress *usecase.EmissionInProgressError
	if errors.As(err, &inProgress) {
		if w.breaker != nil {
			w.breaker.AbortProbe(args.TenantID)
		}
		result = "in_progress"
		return river.JobSnooze(10 * time.Second)
	}
	if isPermanentEmissionError(err) {
		if w.breaker != nil {
			w.breaker.AbortProbe(args.TenantID)
		}
		result = "discarded"
		return river.JobCancel(err)
	}
	if w.breaker != nil {
		w.breaker.Failure(args.TenantID)
	}
	result = "retry"
	return fmt.Errorf("procesar factura %s: %w", args.InvoiceID, err)
}

func isPermanentEmissionError(err error) bool {
	var badRequest *domain.BadRequestError
	var notFound *domain.NotFoundError
	var conflict *domain.ConflictError
	return errors.As(err, &badRequest) || errors.As(err, &notFound) || errors.As(err, &conflict)
}

var _ river.Worker[InvoiceEmissionArgs] = (*InvoiceEmissionWorker)(nil)
