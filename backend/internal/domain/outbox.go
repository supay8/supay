package domain

import (
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
