package domain

import (
	"context"
	"time"
)

const (
	CertificateNotificationPending = "PENDING"
	CertificateNotificationSent    = "SENT"
	CertificateNotificationFailed  = "FAILED"
	CertificateNotificationWebhook = "WEBHOOK"
)

// CredentialTarget reúne el tenant y el punto de venta que un job de
// mantenimiento necesita para renovar credenciales sin perder el aislamiento
// multi-tenant.
type CredentialTarget struct {
	Company     Company
	PointOfSale PointOfSale
}

// CertificateAlertTarget contiene únicamente los datos necesarios para
// evaluar el vencimiento y entregar la notificación configurada por el tenant.
type CertificateAlertTarget struct {
	Certificate Certificate
	WebhookURL  string
}

// MaintenanceRepository concentra las consultas cross-tenant reservadas a
// jobs internos. Ningún handler HTTP debe usar este contrato.
type MaintenanceRepository interface {
	ListActiveCredentialTargets(ctx context.Context) ([]CredentialTarget, error)
	ListCertificatesDue(ctx context.Context, dueBefore time.Time) ([]CertificateAlertTarget, error)
	MarkCertificateExpired(ctx context.Context, certificateID string, now time.Time) (bool, error)
	ClaimCertificateNotification(ctx context.Context, certificateID, tenantID string, thresholdDays int, now time.Time) (bool, error)
	MarkCertificateNotificationSent(ctx context.Context, certificateID string, thresholdDays int, deliveredAt time.Time) error
	MarkCertificateNotificationFailed(ctx context.Context, certificateID string, thresholdDays int, message string, failedAt time.Time) error
}
