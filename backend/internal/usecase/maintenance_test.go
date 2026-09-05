package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
)

type fakeMaintenanceRepo struct {
	credentialTargets  []domain.CredentialTarget
	certificateTargets []domain.CertificateAlertTarget
	claimed            map[string]bool
	expired            []string
	sent               []string
	failed             []string
}

func (f *fakeMaintenanceRepo) ListActiveCredentialTargets(context.Context) ([]domain.CredentialTarget, error) {
	return f.credentialTargets, nil
}

func (f *fakeMaintenanceRepo) ListCertificatesDue(context.Context, time.Time) ([]domain.CertificateAlertTarget, error) {
	return f.certificateTargets, nil
}

func (f *fakeMaintenanceRepo) MarkCertificateExpired(_ context.Context, certificateID string, _ time.Time) (bool, error) {
	f.expired = append(f.expired, certificateID)
	return true, nil
}

func (f *fakeMaintenanceRepo) ClaimCertificateNotification(_ context.Context, certificateID, _ string, thresholdDays int, _ time.Time) (bool, error) {
	if f.claimed == nil {
		f.claimed = make(map[string]bool)
	}
	key := notificationTestKey(certificateID, thresholdDays)
	if f.claimed[key] {
		return false, nil
	}
	f.claimed[key] = true
	return true, nil
}

func (f *fakeMaintenanceRepo) MarkCertificateNotificationSent(_ context.Context, certificateID string, thresholdDays int, _ time.Time) error {
	f.sent = append(f.sent, notificationTestKey(certificateID, thresholdDays))
	return nil
}

func (f *fakeMaintenanceRepo) MarkCertificateNotificationFailed(_ context.Context, certificateID string, thresholdDays int, _ string, _ time.Time) error {
	f.failed = append(f.failed, notificationTestKey(certificateID, thresholdDays))
	return nil
}

func notificationTestKey(certificateID string, thresholdDays int) string {
	return certificateID + "/" + time.Duration(thresholdDays).String()
}

type fakeCredentialMaintainer struct {
	cufd             *domain.Cufd
	ensureCuisErr    error
	ensureCufdErr    error
	refreshCuisErr   error
	refreshCufdErr   error
	ensureCuisCalls  int
	ensureCufdCalls  int
	refreshCuisCalls int
	refreshCufdCalls int
}

func (f *fakeCredentialMaintainer) EnsureCuis(context.Context, *domain.Company, *domain.PointOfSale) error {
	f.ensureCuisCalls++
	return f.ensureCuisErr
}

func (f *fakeCredentialMaintainer) EnsureCufd(context.Context, *domain.Company, *domain.PointOfSale) (*domain.Cufd, error) {
	f.ensureCufdCalls++
	return f.cufd, f.ensureCufdErr
}

func (f *fakeCredentialMaintainer) RefreshCuis(_ context.Context, _ *domain.Company, pos *domain.PointOfSale) (*ports.CuisResult, error) {
	f.refreshCuisCalls++
	if f.refreshCuisErr != nil {
		return nil, f.refreshCuisErr
	}
	cuis := "CUIS-RENOVADO"
	pos.Cuis = &cuis
	return &ports.CuisResult{Codigo: cuis}, nil
}

func (f *fakeCredentialMaintainer) RefreshCufd(context.Context, *domain.Company, *domain.PointOfSale) (*ports.CufdResult, *domain.Cufd, error) {
	f.refreshCufdCalls++
	if f.refreshCufdErr != nil {
		return nil, nil, f.refreshCufdErr
	}
	return &ports.CufdResult{Codigo: "CUFD-RENOVADO"}, &domain.Cufd{ID: "new-cufd"}, nil
}

type fakeNotifier struct {
	notifications []ports.Notification
	err           error
}

func (f *fakeNotifier) Notify(_ context.Context, _ string, notification ports.Notification) error {
	f.notifications = append(f.notifications, notification)
	return f.err
}

func TestRunCredentialRenewalRenuevaCufdCuatroHorasAntes(t *testing.T) {
	now := time.Date(2026, time.September, 5, 9, 0, 0, 0, time.UTC)
	cuis := "CUIS-VIGENTE"
	cuisExpiry := now.Add(90 * 24 * time.Hour)
	repo := &fakeMaintenanceRepo{credentialTargets: []domain.CredentialTarget{{
		Company: domain.Company{ID: "tenant-1"},
		PointOfSale: domain.PointOfSale{
			ID: "pos-1", CompanyId: "tenant-1", Cuis: &cuis, CuisExpiresAt: &cuisExpiry, IsActive: true,
		},
	}}}
	credentials := &fakeCredentialMaintainer{cufd: &domain.Cufd{
		ID: "old-cufd", Active: true, ValidFrom: now.Add(-20 * time.Hour), ValidTo: now.Add(3 * time.Hour),
	}}
	metrics := &MaintenanceMetrics{}
	service := NewMaintenanceService(repo, credentials, &fakeNotifier{}, metrics, MaintenanceOptions{CufdRenewalLead: 4 * time.Hour})
	service.now = func() time.Time { return now }

	if err := service.RunCredentialRenewal(context.Background()); err != nil {
		t.Fatalf("RunCredentialRenewal: %v", err)
	}
	if credentials.refreshCufdCalls != 1 {
		t.Fatalf("renovaciones CUFD=%d, se esperaba 1", credentials.refreshCufdCalls)
	}
	if got := metrics.Snapshot().CufdRenewed; got != 1 {
		t.Fatalf("métrica cufd_renewed=%d, se esperaba 1", got)
	}
}

func TestRunCredentialRenewalRenuevaCuisYDespuesCufd(t *testing.T) {
	now := time.Date(2026, time.September, 5, 9, 0, 0, 0, time.UTC)
	cuis := "CUIS-VIEJO"
	cuisExpiry := now.Add(7 * 24 * time.Hour)
	repo := &fakeMaintenanceRepo{credentialTargets: []domain.CredentialTarget{{
		Company:     domain.Company{ID: "tenant-1"},
		PointOfSale: domain.PointOfSale{ID: "pos-1", CompanyId: "tenant-1", Cuis: &cuis, CuisExpiresAt: &cuisExpiry, IsActive: true},
	}}}
	credentials := &fakeCredentialMaintainer{cufd: &domain.Cufd{
		ID: "old-cufd", Active: true, ValidFrom: now.Add(-time.Hour), ValidTo: now.Add(20 * time.Hour),
	}}
	service := NewMaintenanceService(repo, credentials, &fakeNotifier{}, nil, MaintenanceOptions{CuisRenewalLead: 30 * 24 * time.Hour})
	service.now = func() time.Time { return now }

	if err := service.RunCredentialRenewal(context.Background()); err != nil {
		t.Fatalf("RunCredentialRenewal: %v", err)
	}
	if credentials.refreshCuisCalls != 1 || credentials.refreshCufdCalls != 1 {
		t.Fatalf("renovaciones CUIS/CUFD=%d/%d, se esperaba 1/1", credentials.refreshCuisCalls, credentials.refreshCufdCalls)
	}
}

func TestRunCredentialRenewalRegistraFallo(t *testing.T) {
	repo := &fakeMaintenanceRepo{credentialTargets: []domain.CredentialTarget{{
		Company: domain.Company{ID: "tenant-1"}, PointOfSale: domain.PointOfSale{ID: "pos-1", CompanyId: "tenant-1"},
	}}}
	credentials := &fakeCredentialMaintainer{refreshCuisErr: errors.New("siat fuera de línea")}
	metrics := &MaintenanceMetrics{}
	service := NewMaintenanceService(repo, credentials, &fakeNotifier{}, metrics, MaintenanceOptions{})

	if err := service.RunCredentialRenewal(context.Background()); err == nil {
		t.Fatal("se esperaba error agregado de renovación")
	}
	if got := metrics.Snapshot().CredentialRenewalFailures; got != 1 {
		t.Fatalf("métrica renewal_failures=%d, se esperaba 1", got)
	}
}

func TestCertificateAtSevenDaysTriggersAlertOnce(t *testing.T) {
	notAfter := time.Date(2026, time.October, 5, 9, 0, 0, 0, time.UTC)
	now := notAfter.Add(-30 * 24 * time.Hour)
	repo := &fakeMaintenanceRepo{certificateTargets: []domain.CertificateAlertTarget{{
		Certificate: domain.Certificate{
			ID: "cert-1", CompanyId: "tenant-1", Name: "firma principal", Status: domain.CertificateActive, NotAfter: notAfter,
		},
		WebhookURL: "https://tenant.example.test/certificate-alerts",
	}}}
	notifier := &fakeNotifier{}
	metrics := &MaintenanceMetrics{}
	service := NewMaintenanceService(repo, &fakeCredentialMaintainer{}, notifier, metrics, MaintenanceOptions{})
	service.now = func() time.Time { return now }

	if err := service.RunCertificateCheck(context.Background()); err != nil {
		t.Fatalf("RunCertificateCheck: %v", err)
	}
	now = notAfter.Add(-15 * 24 * time.Hour)
	if err := service.RunCertificateCheck(context.Background()); err != nil {
		t.Fatalf("RunCertificateCheck a 15 días: %v", err)
	}
	now = notAfter.Add(-7 * 24 * time.Hour)
	if err := service.RunCertificateCheck(context.Background()); err != nil {
		t.Fatalf("RunCertificateCheck a 7 días: %v", err)
	}
	if err := service.RunCertificateCheck(context.Background()); err != nil {
		t.Fatalf("RunCertificateCheck repetido a 7 días: %v", err)
	}
	if len(notifier.notifications) != 3 {
		t.Fatalf("notificaciones=%d, se esperaban los umbrales 30/15/7 una sola vez", len(notifier.notifications))
	}
	foundSevenDays := false
	for _, notification := range notifier.notifications {
		if notification.Data["threshold_days"] == 7 {
			foundSevenDays = true
		}
	}
	if !foundSevenDays {
		t.Fatal("el certificado a 7 días no disparó la alerta del umbral 7")
	}
	if got := metrics.Snapshot().CertificateAlertsSent; got != 3 {
		t.Fatalf("métrica certificate_alerts_sent=%d, se esperaba 3", got)
	}
}

func TestExpiredActiveCertificateTransitionsToExpired(t *testing.T) {
	now := time.Date(2026, time.September, 5, 9, 0, 0, 0, time.UTC)
	repo := &fakeMaintenanceRepo{certificateTargets: []domain.CertificateAlertTarget{{
		Certificate: domain.Certificate{
			ID: "cert-expired", CompanyId: "tenant-1", Status: domain.CertificateActive, NotAfter: now.Add(-time.Minute),
		},
		WebhookURL: "https://tenant.example.test/certificate-alerts",
	}}}
	metrics := &MaintenanceMetrics{}
	service := NewMaintenanceService(repo, &fakeCredentialMaintainer{}, &fakeNotifier{}, metrics, MaintenanceOptions{})
	service.now = func() time.Time { return now }

	if err := service.RunCertificateCheck(context.Background()); err != nil {
		t.Fatalf("RunCertificateCheck: %v", err)
	}
	if len(repo.expired) != 1 || repo.expired[0] != "cert-expired" {
		t.Fatalf("certificados expirados=%v", repo.expired)
	}
	if got := metrics.Snapshot().CertificatesExpired; got != 1 {
		t.Fatalf("métrica certificates_expired=%d, se esperaba 1", got)
	}
}
