package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const certificateNotificationRetryDelay = time.Hour

type PostgresMaintenanceRepository struct {
	db *gorm.DB
}

func NewPostgresMaintenanceRepository(db *gorm.DB) domain.MaintenanceRepository {
	return &PostgresMaintenanceRepository{db: db}
}

func (r *PostgresMaintenanceRepository) ListActiveCredentialTargets(ctx context.Context) ([]domain.CredentialTarget, error) {
	var rows []models.PointOfSale
	if err := r.db.WithContext(ctx).
		Preload("Company.Config").
		Where("is_active = true").
		Order("tenant_id ASC, codigo_sucursal ASC, codigo_punto_venta ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]domain.CredentialTarget, 0, len(rows))
	for i := range rows {
		result = append(result, domain.CredentialTarget{
			Company:     *toDomainCompany(&rows[i].Company),
			PointOfSale: *toDomainPointOfSale(&rows[i]),
		})
	}
	return result, nil
}

func (r *PostgresMaintenanceRepository) ListCertificatesDue(ctx context.Context, dueBefore time.Time) ([]domain.CertificateAlertTarget, error) {
	var rows []models.Certificate
	if err := r.db.WithContext(ctx).
		Preload("Company.Config").
		Where("status = ? AND not_after <= ?", domain.CertificateActive, dueBefore).
		Order("not_after ASC, tenant_id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]domain.CertificateAlertTarget, 0, len(rows))
	for i := range rows {
		result = append(result, domain.CertificateAlertTarget{
			Certificate: *toDomainCertificate(&rows[i]),
			WebhookURL:  certificateWebhookURL(json.RawMessage(rows[i].Company.Config.Settings)),
		})
	}
	return result, nil
}

func certificateWebhookURL(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var settings struct {
		CertificateWebhookURL string `json:"certificate_webhook_url"`
		CertificateAlerts     struct {
			WebhookURL string `json:"webhook_url"`
		} `json:"certificate_alerts"`
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return ""
	}
	if value := strings.TrimSpace(settings.CertificateAlerts.WebhookURL); value != "" {
		return value
	}
	return strings.TrimSpace(settings.CertificateWebhookURL)
}

func (r *PostgresMaintenanceRepository) MarkCertificateExpired(ctx context.Context, certificateID string, now time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&models.Certificate{}).
		Where("id = ? AND status = ? AND not_after <= ?", certificateID, domain.CertificateActive, now).
		Updates(map[string]any{
			"status":     domain.CertificateExpired,
			"is_active":  false,
			"updated_at": now,
		})
	return result.RowsAffected == 1, result.Error
}

func (r *PostgresMaintenanceRepository) ClaimCertificateNotification(
	ctx context.Context,
	certificateID, tenantID string,
	thresholdDays int,
	now time.Time,
) (bool, error) {
	row := models.CertificateNotification{
		ID:            uuid.NewString(),
		TenantID:      tenantID,
		CertificateID: certificateID,
		ThresholdDays: thresholdDays,
		Channel:       domain.CertificateNotificationWebhook,
		Status:        domain.CertificateNotificationPending,
		Attempts:      1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	insert := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "certificate_id"}, {Name: "threshold_days"}, {Name: "channel"}},
		DoNothing: true,
	}).Create(&row)
	if insert.Error != nil {
		return false, insert.Error
	}
	if insert.RowsAffected == 1 {
		return true, nil
	}

	// Recupera entregas fallidas o PENDING abandonadas por un crash. La
	// condición sobre updated_at hace que solo una réplica gane el claim.
	retry := r.db.WithContext(ctx).Model(&models.CertificateNotification{}).
		Where("certificate_id = ? AND threshold_days = ? AND channel = ?", certificateID, thresholdDays, domain.CertificateNotificationWebhook).
		Where("status IN ? AND updated_at <= ?", []string{domain.CertificateNotificationPending, domain.CertificateNotificationFailed}, now.Add(-certificateNotificationRetryDelay)).
		Updates(map[string]any{
			"status":     domain.CertificateNotificationPending,
			"attempts":   gorm.Expr("attempts + 1"),
			"last_error": nil,
			"updated_at": now,
		})
	return retry.RowsAffected == 1, retry.Error
}

func (r *PostgresMaintenanceRepository) MarkCertificateNotificationSent(
	ctx context.Context,
	certificateID string,
	thresholdDays int,
	deliveredAt time.Time,
) error {
	return r.db.WithContext(ctx).Model(&models.CertificateNotification{}).
		Where("certificate_id = ? AND threshold_days = ? AND channel = ?", certificateID, thresholdDays, domain.CertificateNotificationWebhook).
		Updates(map[string]any{
			"status":       domain.CertificateNotificationSent,
			"last_error":   nil,
			"delivered_at": deliveredAt,
			"updated_at":   deliveredAt,
		}).Error
}

func (r *PostgresMaintenanceRepository) MarkCertificateNotificationFailed(
	ctx context.Context,
	certificateID string,
	thresholdDays int,
	message string,
	failedAt time.Time,
) error {
	return r.db.WithContext(ctx).Model(&models.CertificateNotification{}).
		Where("certificate_id = ? AND threshold_days = ? AND channel = ?", certificateID, thresholdDays, domain.CertificateNotificationWebhook).
		Updates(map[string]any{
			"status":     domain.CertificateNotificationFailed,
			"last_error": message,
			"updated_at": failedAt,
		}).Error
}
