package domain

import (
	"time"
)

const (
	InvoiceEmailPending    = "PENDING"
	InvoiceEmailPublishing = "PUBLISHING"
	InvoiceEmailEnqueued   = "ENQUEUED"
	InvoiceEmailSending    = "SENDING"
	InvoiceEmailSent       = "SENT"
)

// InvoiceEmailNotification is the durable hand-off between fiscal acceptance,
// Cloud Tasks and the email worker. PostgreSQL remains the source of truth.
type InvoiceEmailNotification struct {
	ID               string
	TenantID         string
	InvoiceID        string
	Recipient        string
	Status           string
	PublishAttempts  int
	DeliveryAttempts int
	AvailableAt      time.Time
	LockedAt         *time.Time
	LockedBy         *string
	TaskName         *string
	LastError        *string
	SentAt           *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
