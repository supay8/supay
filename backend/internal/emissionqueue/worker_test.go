package emissionqueue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
	"github.com/brandsrx/supay/internal/usecase"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

type fakeProcessor struct {
	calls int
	err   error
}

func (f *fakeProcessor) ProcessEmission(context.Context, string) (*domain.Invoice, error) {
	f.calls++
	return &domain.Invoice{Status: domain.InvoiceAccepted}, f.err
}

func TestWorkerSnoozeLiberaSlotCuandoTenantSuperaRate(t *testing.T) {
	limiter := NewTenantRateLimiter(1, 1)
	if allowed, _ := limiter.TryAcquire("tenant-a"); !allowed {
		t.Fatal("precondición: consumir token inicial")
	}
	processor := &fakeProcessor{}
	worker := NewInvoiceEmissionWorker(processor, limiter, NewCircuitBreaker(2, time.Minute), time.Minute)
	err := worker.Work(context.Background(), &river.Job[InvoiceEmissionArgs]{Args: InvoiceEmissionArgs{InvoiceID: "inv-1", TenantID: "tenant-a"}})
	var snooze *rivertype.JobSnoozeError
	if !errors.As(err, &snooze) {
		t.Fatalf("se esperaba JobSnoozeError, se obtuvo %v", err)
	}
	if processor.calls != 0 {
		t.Fatalf("processor calls=%d", processor.calls)
	}
}

func TestWorkerCancelaRechazoSIATSinReintentar(t *testing.T) {
	processor := &fakeProcessor{err: &usecase.EmissionRejectedError{
		CodigoEstado: 902,
		Mensajes:     []ports.FiscalMessage{{Codigo: 123, Descripcion: "rechazo"}},
	}}
	worker := NewInvoiceEmissionWorker(processor, nil, NewCircuitBreaker(2, time.Minute), time.Minute)
	err := worker.Work(context.Background(), &river.Job[InvoiceEmissionArgs]{Args: InvoiceEmissionArgs{InvoiceID: "inv-1", TenantID: "tenant-a"}})
	var cancelled *rivertype.JobCancelError
	if !errors.As(err, &cancelled) {
		t.Fatalf("se esperaba JobCancelError, se obtuvo %v", err)
	}
}

func TestWorkerReprogramaFacturaEnProceso(t *testing.T) {
	processor := &fakeProcessor{err: &usecase.EmissionInProgressError{}}
	worker := NewInvoiceEmissionWorker(processor, nil, NewCircuitBreaker(2, time.Minute), time.Minute)
	err := worker.Work(context.Background(), &river.Job[InvoiceEmissionArgs]{Args: InvoiceEmissionArgs{InvoiceID: "inv-1", TenantID: "tenant-a"}})
	var snooze *rivertype.JobSnoozeError
	if !errors.As(err, &snooze) || snooze.Duration != 10*time.Second {
		t.Fatalf("se esperaba snooze de factura en progreso, se obtuvo %v", err)
	}
}

func TestExponentialRetryPolicyRespetaTope(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	policy := NewExponentialRetryPolicy(2*time.Second, 10*time.Second)
	policy.now = func() time.Time { return now }

	if got := policy.NextRetry(&rivertype.JobRow{Attempt: 1}); !got.Equal(now.Add(2 * time.Second)) {
		t.Fatalf("primer retry=%s", got)
	}
	if got := policy.NextRetry(&rivertype.JobRow{Attempt: 10}); !got.Equal(now.Add(10 * time.Second)) {
		t.Fatalf("retry con tope=%s", got)
	}
}
