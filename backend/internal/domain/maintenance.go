package domain

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
