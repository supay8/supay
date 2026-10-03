package domain

import (
	"context"
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

type InvoiceEmailNotificationRepository interface {
	ClaimPending(ctx context.Context, owner string, limit int, now time.Time, lockTimeout time.Duration) ([]InvoiceEmailNotification, error)
	MarkEnqueued(ctx context.Context, id, owner, taskName string, now time.Time) error
	MarkPublishFailed(ctx context.Context, id, owner, message string, nextAttempt time.Time) error
	ClaimDelivery(ctx context.Context, id string, now time.Time, lockTimeout time.Duration) (*InvoiceEmailNotification, bool, error)
	MarkSent(ctx context.Context, id string, now time.Time) error
	MarkDeliveryFailed(ctx context.Context, id, message string, now time.Time) error
}

type InvoiceEmailTaskDispatcher interface {
	DispatchOnce(context.Context) error
}
