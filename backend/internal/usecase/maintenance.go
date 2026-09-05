package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync/atomic"
	"time"

	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
)

var certificateAlertThresholds = [...]int{7, 15, 30}

type CredentialMaintainer interface {
	EnsureCuis(ctx context.Context, company *domain.Company, pos *domain.PointOfSale) error
	EnsureCufd(ctx context.Context, company *domain.Company, pos *domain.PointOfSale) (*domain.Cufd, error)
	RefreshCuis(ctx context.Context, company *domain.Company, pos *domain.PointOfSale) (*ports.CuisResult, error)
	RefreshCufd(ctx context.Context, company *domain.Company, pos *domain.PointOfSale) (*ports.CufdResult, *domain.Cufd, error)
}

type MaintenanceOptions struct {
	CufdRenewalLead      time.Duration
	CuisRenewalLead      time.Duration
	CuisFallbackValidity time.Duration
	DefaultWebhookURL    string
}

type MaintenanceService struct {
	repo        domain.MaintenanceRepository
	credentials CredentialMaintainer
	notifier    ports.Notifier
	metrics     *MaintenanceMetrics
	options     MaintenanceOptions
	now         func() time.Time
}

func NewMaintenanceService(
	repo domain.MaintenanceRepository,
	credentials CredentialMaintainer,
	notifier ports.Notifier,
	metrics *MaintenanceMetrics,
	options MaintenanceOptions,
) *MaintenanceService {
	if options.CufdRenewalLead <= 0 {
		options.CufdRenewalLead = 4 * time.Hour
	}
	if options.CuisRenewalLead <= 0 {
		options.CuisRenewalLead = defaultCuisRenewalLead
	}
	if options.CuisFallbackValidity <= 0 {
		options.CuisFallbackValidity = defaultCuisFallbackValidity
	}
	if metrics == nil {
		metrics = &MaintenanceMetrics{}
	}
	return &MaintenanceService{
		repo: repo, credentials: credentials, notifier: notifier, metrics: metrics,
		options: options,
		now:     func() time.Time { return time.Now().In(siat.LaPaz) },
	}
}

// RunCredentialRenewal renueva CUFD con anticipación y CUIS próximos a vencer
// para todos los puntos de venta activos. Los fallos se aíslan por tenant/POS.
func (s *MaintenanceService) RunCredentialRenewal(ctx context.Context) error {
	s.metrics.credentialRuns.Add(1)
	targets, err := s.repo.ListActiveCredentialTargets(ctx)
	if err != nil {
		s.metrics.credentialRenewalFailures.Add(1)
		return fmt.Errorf("listar puntos de venta para renovación: %w", err)
	}

	now := s.now()
	var failures []error
	for i := range targets {
		if err := ctx.Err(); err != nil {
			failures = append(failures, err)
			break
		}
		target := &targets[i]
		company := &target.Company
		pos := &target.PointOfSale
		cuisRenewed := false

		if CuisNeedsRenewal(pos, now, s.options.CuisRenewalLead, s.options.CuisFallbackValidity) {
			if _, err := s.credentials.RefreshCuis(ctx, company, pos); err != nil {
				s.recordCredentialFailure("CUIS", company.ID, pos.ID, err)
				failures = append(failures, fmt.Errorf("tenant %s pos %s CUIS: %w", company.ID, pos.ID, err))
				continue
			}
			cuisRenewed = true
			s.metrics.cuisRenewed.Add(1)
		} else if err := s.credentials.EnsureCuis(ctx, company, pos); err != nil {
			s.recordCredentialFailure("CUIS", company.ID, pos.ID, err)
			failures = append(failures, fmt.Errorf("tenant %s pos %s CUIS: %w", company.ID, pos.ID, err))
			continue
		}

		cufd, err := s.credentials.EnsureCufd(ctx, company, pos)
		if err != nil {
			s.recordCredentialFailure("CUFD", company.ID, pos.ID, err)
			failures = append(failures, fmt.Errorf("tenant %s pos %s CUFD: %w", company.ID, pos.ID, err))
			continue
		}
		if !cuisRenewed && cufd.ValidFrom.After(now.Add(-time.Minute)) {
			// EnsureCufd tuvo que crear uno porque no había uno vigente.
			s.metrics.cufdRenewed.Add(1)
		}
		if cuisRenewed || !cufd.ValidTo.After(now.Add(s.options.CufdRenewalLead)) {
			if _, _, err := s.credentials.RefreshCufd(ctx, company, pos); err != nil {
				s.recordCredentialFailure("CUFD", company.ID, pos.ID, err)
				failures = append(failures, fmt.Errorf("tenant %s pos %s CUFD: %w", company.ID, pos.ID, err))
				continue
			}
			s.metrics.cufdRenewed.Add(1)
		}
	}
	return errors.Join(failures...)
}

func (s *MaintenanceService) recordCredentialFailure(kind, tenantID, posID string, err error) {
	s.metrics.credentialRenewalFailures.Add(1)
	slog.Error("alerta: falló la renovación de credenciales SIAT",
		"alert", true,
		"credential", kind,
		"tenant_id", tenantID,
		"point_of_sale_id", posID,
		"error", err,
	)
}

// RunCertificateCheck expira certificados vencidos y entrega una alerta única
// por los umbrales de 30, 15 y 7 días. certificate_notifications hace la
// operación idempotente incluso con varias réplicas del scheduler.
func (s *MaintenanceService) RunCertificateCheck(ctx context.Context) error {
	s.metrics.certificateRuns.Add(1)
	now := s.now()
	targets, err := s.repo.ListCertificatesDue(ctx, now.Add(30*24*time.Hour))
	if err != nil {
		s.metrics.certificateAlertFailures.Add(1)
		return fmt.Errorf("listar certificados próximos a vencer: %w", err)
	}

	var failures []error
	for i := range targets {
		if err := ctx.Err(); err != nil {
			failures = append(failures, err)
			break
		}
		target := &targets[i]
		cert := &target.Certificate
		if !cert.NotAfter.After(now) {
			expired, err := s.repo.MarkCertificateExpired(ctx, cert.ID, now)
			if err != nil {
				s.metrics.certificateAlertFailures.Add(1)
				failures = append(failures, fmt.Errorf("expirar certificado %s: %w", cert.ID, err))
			} else if expired {
				s.metrics.certificatesExpired.Add(1)
				slog.Error("alerta interna: certificado ACTIVE vencido transicionado a EXPIRED",
					"alert", true,
					"tenant_id", cert.CompanyId,
					"certificate_id", cert.ID,
					"not_after", cert.NotAfter,
				)
			}
			continue
		}

		thresholdDays := certificateAlertThreshold(cert.NotAfter, now)
		if thresholdDays == 0 {
			continue
		}
		claimed, err := s.repo.ClaimCertificateNotification(ctx, cert.ID, cert.CompanyId, thresholdDays, now)
		if err != nil {
			s.metrics.certificateAlertFailures.Add(1)
			failures = append(failures, fmt.Errorf("reservar alerta de certificado %s/%dd: %w", cert.ID, thresholdDays, err))
			continue
		}
		if !claimed {
			continue
		}

		targetURL := strings.TrimSpace(target.WebhookURL)
		if targetURL == "" {
			targetURL = strings.TrimSpace(s.options.DefaultWebhookURL)
		}
		notification := ports.Notification{
			ID:         fmt.Sprintf("certificate:%s:%d", cert.ID, thresholdDays),
			Type:       "certificate.expiring",
			TenantID:   cert.CompanyId,
			OccurredAt: now,
			Data: map[string]any{
				"certificate_id": cert.ID,
				"name":           cert.Name,
				"not_after":      cert.NotAfter,
				"threshold_days": thresholdDays,
				"days_remaining": int(math.Ceil(cert.NotAfter.Sub(now).Hours() / 24)),
			},
		}
		if s.notifier == nil {
			err = errors.New("notificador de certificados no configurado")
		} else {
			err = s.notifier.Notify(ctx, targetURL, notification)
		}
		if err != nil {
			s.metrics.certificateAlertFailures.Add(1)
			message := truncateError(err, 2000)
			if markErr := s.repo.MarkCertificateNotificationFailed(ctx, cert.ID, thresholdDays, message, now); markErr != nil {
				err = errors.Join(err, markErr)
			}
			slog.Error("alerta: no se pudo notificar vencimiento de certificado",
				"alert", true,
				"tenant_id", cert.CompanyId,
				"certificate_id", cert.ID,
				"threshold_days", thresholdDays,
				"error", err,
			)
			failures = append(failures, fmt.Errorf("notificar certificado %s/%dd: %w", cert.ID, thresholdDays, err))
			continue
		}
		if err := s.repo.MarkCertificateNotificationSent(ctx, cert.ID, thresholdDays, now); err != nil {
			s.metrics.certificateAlertFailures.Add(1)
			failures = append(failures, fmt.Errorf("confirmar alerta de certificado %s/%dd: %w", cert.ID, thresholdDays, err))
			continue
		}
		s.metrics.certificateAlertsSent.Add(1)
	}
	return errors.Join(failures...)
}

func certificateAlertThreshold(notAfter, now time.Time) int {
	for _, thresholdDays := range certificateAlertThresholds {
		if !notAfter.After(now.Add(time.Duration(thresholdDays) * 24 * time.Hour)) {
			return thresholdDays
		}
	}
	return 0
}

func truncateError(err error, max int) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) <= max {
		return message
	}
	return message[:max]
}

type MaintenanceMetrics struct {
	credentialRuns            atomic.Uint64
	certificateRuns           atomic.Uint64
	cufdRenewed               atomic.Uint64
	cuisRenewed               atomic.Uint64
	credentialRenewalFailures atomic.Uint64
	certificateAlertsSent     atomic.Uint64
	certificateAlertFailures  atomic.Uint64
	certificatesExpired       atomic.Uint64
}

type MaintenanceMetricsSnapshot struct {
	CredentialRuns            uint64
	CertificateRuns           uint64
	CufdRenewed               uint64
	CuisRenewed               uint64
	CredentialRenewalFailures uint64
	CertificateAlertsSent     uint64
	CertificateAlertFailures  uint64
	CertificatesExpired       uint64
}

func (m *MaintenanceMetrics) Snapshot() MaintenanceMetricsSnapshot {
	if m == nil {
		return MaintenanceMetricsSnapshot{}
	}
	return MaintenanceMetricsSnapshot{
		CredentialRuns:            m.credentialRuns.Load(),
		CertificateRuns:           m.certificateRuns.Load(),
		CufdRenewed:               m.cufdRenewed.Load(),
		CuisRenewed:               m.cuisRenewed.Load(),
		CredentialRenewalFailures: m.credentialRenewalFailures.Load(),
		CertificateAlertsSent:     m.certificateAlertsSent.Load(),
		CertificateAlertFailures:  m.certificateAlertFailures.Load(),
		CertificatesExpired:       m.certificatesExpired.Load(),
	}
}
