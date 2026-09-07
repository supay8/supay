package emissionqueue

import (
	"context"
	"testing"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

type fakeRiverInserter struct {
	args InvoiceEmissionArgs
	opts *river.InsertOpts
}

func (f *fakeRiverInserter) Insert(_ context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	f.args = args.(InvoiceEmissionArgs)
	f.opts = opts
	return &rivertype.JobInsertResult{}, nil
}

func TestRiverPublisherInsertaJobUnicoEnColaDeEmision(t *testing.T) {
	inserter := &fakeRiverInserter{}
	publisher := &RiverPublisher{client: inserter, maxAttempts: 8}
	event := domain.OutboxEvent{
		ID: "outbox-1", TenantID: "tenant-1", AggregateType: domain.OutboxAggregateInvoice,
		AggregateID: "invoice-1", EventType: domain.OutboxEventInvoiceEmit,
	}

	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if inserter.args.InvoiceID != "invoice-1" || inserter.args.TenantID != "tenant-1" || inserter.args.OutboxID != "outbox-1" {
		t.Fatalf("args=%+v", inserter.args)
	}
	if inserter.opts == nil || inserter.opts.Queue != InvoiceEmissionQueue || inserter.opts.MaxAttempts != 8 {
		t.Fatalf("opts=%+v", inserter.opts)
	}
	if !inserter.opts.UniqueOpts.ByArgs || !inserter.opts.UniqueOpts.ByQueue {
		t.Fatalf("el job debe ser único por factura y cola: %+v", inserter.opts.UniqueOpts)
	}
}
