package domain

import (
	"context"
	"encoding/json"
	"time"
)

const (
	OutboxAggregateInvoice = "invoice"
	OutboxEventInvoiceEmit = "invoice.emit.requested"
	OutboxStatusPending    = "PENDING"
	OutboxStatusProcessing = "PROCESSING"
	OutboxStatusPublished  = "PUBLISHED"
)

// InvoiceEmissionPayload is deliberately small: the invoice remains the
// source of truth and the worker reloads it before contacting SIAT.
type InvoiceEmissionPayload struct {
	InvoiceID string `json:"invoice_id"`
	TenantID  string `json:"tenant_id"`
}

type OutboxEvent struct {
	ID            string
	TenantID      string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       json.RawMessage
	Status        string
	Attempts      int
	AvailableAt   time.Time
	LockedAt      *time.Time
	LockedBy      *string
	PublishedAt   *time.Time
	LastError     *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// EmissionQueue is the application-facing port used by InvoiceUsecase. Its
// PostgreSQL implementation only writes to the outbox; it never calls SIAT.
type EmissionQueue interface {
	EnqueueInvoiceEmission(ctx context.Context, invoiceID, tenantID, cufdID string) (*OutboxEvent, error)
}

// OutboxRepository is consumed by the dispatcher that relays persistent
// outbox records into River.
type OutboxRepository interface {
	EmissionQueue
	ClaimPending(ctx context.Context, eventType, owner string, limit int, now time.Time, lockTimeout time.Duration) ([]OutboxEvent, error)
	MarkPublished(ctx context.Context, id, owner string, publishedAt time.Time) error
	MarkFailed(ctx context.Context, id, owner, lastError string, nextAttempt time.Time) error
}
