package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"gorm.io/datatypes"
)

func TestTenantSettingsWithCertificateWebhookPreservesOtherSettings(t *testing.T) {
	raw := datatypes.JSON([]byte(`{"feature_flag":true,"certificate_alerts":{"email":"ops@example.test","webhook_url":"https://old.example.test"}}`))
	updated, err := tenantSettingsWithCertificateWebhook(raw, "https://new.example.test")
	if err != nil {
		t.Fatalf("tenantSettingsWithCertificateWebhook: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(updated, &settings); err != nil {
		t.Fatalf("settings JSON inválido: %v", err)
	}
	if settings["feature_flag"] != true {
		t.Fatalf("se perdió feature_flag: %v", settings)
	}
	alerts, ok := settings["certificate_alerts"].(map[string]any)
	if !ok || alerts["email"] != "ops@example.test" || alerts["webhook_url"] != "https://new.example.test" {
		t.Fatalf("certificate_alerts inesperado: %v", settings["certificate_alerts"])
	}
}

func TestMaintenanceRepositoryTargetsClaimsAndExpiration(t *testing.T) {
	db := newTestDB(t)
	fixture := seedFixture(t, db)
	now := time.Now().UTC().Truncate(time.Second)
	cuis := "CUIS-TEST"
	cuisExpiry := now.Add(200 * 24 * time.Hour)
	if err := db.Model(&models.PointOfSale{}).Where("id = ?", fixture.posID).Updates(map[string]any{
		"cuis": cuis, "cuis_created_at": now, "cuis_expires_at": cuisExpiry, "is_active": true,
	}).Error; err != nil {
		t.Fatalf("preparar POS: %v", err)
	}
	settings := datatypes.JSON([]byte(`{"certificate_alerts":{"webhook_url":"https://tenant.example.test/alerts"}}`))
	if err := db.Model(&models.TenantConfig{}).Where("tenant_id = ?", fixture.companyID).Update("settings", settings).Error; err != nil {
		t.Fatalf("configurar webhook: %v", err)
	}
	certificate := &models.Certificate{
		CompanyId: fixture.companyID, Name: "firma", Type: "P12", Status: string(domain.CertificateActive),
		NotBefore: now.Add(-365 * 24 * time.Hour), NotAfter: now.Add(7 * 24 * time.Hour), IsActive: true,
	}
	if err := db.Create(certificate).Error; err != nil {
		t.Fatalf("crear certificado: %v", err)
	}

	repo := NewPostgresMaintenanceRepository(db)
	credentialTargets, err := repo.ListActiveCredentialTargets(context.Background())
	if err != nil || len(credentialTargets) != 1 {
		t.Fatalf("targets de credenciales=%d err=%v", len(credentialTargets), err)
	}
	if credentialTargets[0].PointOfSale.CuisExpiresAt == nil || !credentialTargets[0].PointOfSale.CuisExpiresAt.Equal(cuisExpiry) {
		t.Fatalf("cuis_expires_at no cargado: %v", credentialTargets[0].PointOfSale.CuisExpiresAt)
	}

	certificateTargets, err := repo.ListCertificatesDue(context.Background(), now.Add(30*24*time.Hour))
	if err != nil || len(certificateTargets) != 1 {
		t.Fatalf("targets de certificados=%d err=%v", len(certificateTargets), err)
	}
	if certificateTargets[0].WebhookURL != "https://tenant.example.test/alerts" {
		t.Fatalf("webhook=%q", certificateTargets[0].WebhookURL)
	}

	claimed, err := repo.ClaimCertificateNotification(context.Background(), certificate.ID, fixture.companyID, 7, now)
	if err != nil || !claimed {
		t.Fatalf("primer claim=%v err=%v", claimed, err)
	}
	claimed, err = repo.ClaimCertificateNotification(context.Background(), certificate.ID, fixture.companyID, 7, now)
	if err != nil || claimed {
		t.Fatalf("segundo claim=%v err=%v", claimed, err)
	}
	if err := repo.MarkCertificateNotificationSent(context.Background(), certificate.ID, 7, now); err != nil {
		t.Fatalf("marcar alerta enviada: %v", err)
	}

	if err := db.Model(certificate).Updates(map[string]any{"not_after": now.Add(-time.Minute)}).Error; err != nil {
		t.Fatalf("vencer certificado: %v", err)
	}
	expired, err := repo.MarkCertificateExpired(context.Background(), certificate.ID, now)
	if err != nil || !expired {
		t.Fatalf("expired=%v err=%v", expired, err)
	}
}

func TestCufdCreateRotatesActiveCredential(t *testing.T) {
	db := newTestDB(t)
	fixture := seedFixture(t, db)
	repo := NewPostgresCufdRepository(db)
	now := time.Now()
	newCufd := &domain.Cufd{
		PointOfSaleID: fixture.posID,
		Cufd:          "CUFD-RENOVADO",
		ControlCode:   "CTRL-RENOVADO",
		Direccion:     "Calle 2",
		ValidFrom:     now,
		ValidTo:       now.Add(24 * time.Hour),
		Active:        true,
	}
	if err := repo.Create(newCufd); err != nil {
		t.Fatalf("crear CUFD renovado: %v", err)
	}
	var activeCount int64
	if err := db.Model(&models.Cufd{}).
		Where("point_of_sale_id = ? AND is_active = true", fixture.posID).
		Count(&activeCount).Error; err != nil {
		t.Fatalf("contar CUFD activos: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("CUFD activos=%d, se esperaba 1", activeCount)
	}
}
