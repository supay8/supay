package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"gorm.io/gorm"
)

type invoiceEmailNotificationRow struct {
	ID               string `gorm:"type:uuid;primaryKey"`
	TenantID         string `gorm:"column:tenant_id"`
	InvoiceID        string `gorm:"column:invoice_id"`
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

func (invoiceEmailNotificationRow) TableName() string { return "invoice_email_notifications" }

type PostgresInvoiceEmailNotificationRepository struct{ db *gorm.DB }

func NewPostgresInvoiceEmailNotificationRepository(db *gorm.DB) *PostgresInvoiceEmailNotificationRepository {
	return &PostgresInvoiceEmailNotificationRepository{db: db}
}

func queueInvoiceEmailNotification(tx *gorm.DB, tenantID, invoiceID string) error {
	return tx.Exec(`
		INSERT INTO invoice_email_notifications (tenant_id, invoice_id, recipient)
		SELECT i.tenant_id, i.id, lower(trim(i.customer_email))
		FROM invoices AS i
		JOIN tenant_configs AS tc ON tc.tenant_id = i.tenant_id
		WHERE i.tenant_id = ? AND i.id = ?
		  AND tc.settings @> '{"invoice_email":{"enabled":true}}'::jsonb
		  AND i.customer_email IS NOT NULL AND trim(i.customer_email) <> ''
		ON CONFLICT (invoice_id) DO NOTHING`, tenantID, invoiceID).Error
}

func (r *PostgresInvoiceEmailNotificationRepository) ClaimPending(ctx context.Context, owner string, limit int, now time.Time, lockTimeout time.Duration) ([]domain.InvoiceEmailNotification, error) {
	if limit <= 0 {
		limit = 100
	}
	if lockTimeout <= 0 {
		lockTimeout = 30 * time.Second
	}
	var rows []invoiceEmailNotificationRow
	err := r.db.WithContext(ctx).Raw(`
		WITH candidates AS (
			SELECT id FROM invoice_email_notifications
			WHERE (status = 'PENDING' AND available_at <= ?)
			   OR (status = 'PUBLISHING' AND locked_at < ?)
			ORDER BY available_at, created_at
			FOR UPDATE SKIP LOCKED LIMIT ?
		)
		UPDATE invoice_email_notifications AS n
		SET status = 'PUBLISHING', publish_attempts = n.publish_attempts + 1,
			locked_at = ?, locked_by = ?, updated_at = ?
		FROM candidates WHERE n.id = candidates.id RETURNING n.*`,
		now, now.Add(-lockTimeout), limit, now, owner, now).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make([]domain.InvoiceEmailNotification, 0, len(rows))
	for i := range rows {
		result = append(result, emailNotificationToDomain(rows[i]))
	}
	return result, nil
}

func (r *PostgresInvoiceEmailNotificationRepository) MarkEnqueued(ctx context.Context, id, owner, taskName string, now time.Time) error {
	return r.updateClaimed(ctx, id, owner, map[string]any{
		"status": domain.InvoiceEmailEnqueued, "task_name": taskName, "locked_at": nil,
		"locked_by": nil, "last_error": nil, "updated_at": now,
	})
}

func (r *PostgresInvoiceEmailNotificationRepository) MarkPublishFailed(ctx context.Context, id, owner, message string, nextAttempt time.Time) error {
	return r.updateClaimed(ctx, id, owner, map[string]any{
		"status": domain.InvoiceEmailPending, "available_at": nextAttempt, "locked_at": nil,
		"locked_by": nil, "last_error": truncateEmailError(message), "updated_at": time.Now().UTC(),
	})
}

func (r *PostgresInvoiceEmailNotificationRepository) updateClaimed(ctx context.Context, id, owner string, values map[string]any) error {
	result := r.db.WithContext(ctx).Model(&invoiceEmailNotificationRow{}).
		Where("id = ? AND status = ? AND locked_by = ?", id, domain.InvoiceEmailPublishing, owner).Updates(values)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("notificación %s no estaba reclamada por %s", id, owner)
	}
	return nil
}

func (r *PostgresInvoiceEmailNotificationRepository) ClaimDelivery(ctx context.Context, id string, now time.Time, lockTimeout time.Duration) (*domain.InvoiceEmailNotification, bool, error) {
	if lockTimeout <= 0 {
		lockTimeout = 10 * time.Minute
	}
	var row invoiceEmailNotificationRow
	result := r.db.WithContext(ctx).Raw(`
		UPDATE invoice_email_notifications
		SET status = 'SENDING', delivery_attempts = delivery_attempts + 1,
			locked_at = ?, locked_by = 'cloud-tasks', updated_at = ?
		WHERE id = ? AND (status = 'ENQUEUED' OR (status = 'SENDING' AND locked_at < ?))
		RETURNING *`, now, now, id, now.Add(-lockTimeout)).Scan(&row)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if row.ID != "" {
		n := emailNotificationToDomain(row)
		return &n, true, nil
	}
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return nil, false, err
	}
	n := emailNotificationToDomain(row)
	return &n, false, nil
}

func (r *PostgresInvoiceEmailNotificationRepository) MarkSent(ctx context.Context, id string, now time.Time) error {
	result := r.db.WithContext(ctx).Model(&invoiceEmailNotificationRow{}).
		Where("id = ? AND status = ?", id, domain.InvoiceEmailSending).
		Updates(map[string]any{"status": domain.InvoiceEmailSent, "sent_at": now, "locked_at": nil, "locked_by": nil, "last_error": nil, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("notificación %s no estaba en envío", id)
	}
	return nil
}

func (r *PostgresInvoiceEmailNotificationRepository) MarkDeliveryFailed(ctx context.Context, id, message string, now time.Time) error {
	result := r.db.WithContext(ctx).Model(&invoiceEmailNotificationRow{}).
		Where("id = ? AND status = ?", id, domain.InvoiceEmailSending).
		Updates(map[string]any{"status": domain.InvoiceEmailEnqueued, "locked_at": nil, "locked_by": nil, "last_error": truncateEmailError(message), "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("notificación %s no estaba en envío", id)
	}
	return nil
}

func truncateEmailError(message string) string {
	message = strings.TrimSpace(message)
	if len(message) > 1000 {
		return message[:1000]
	}
	return message
}

func emailNotificationToDomain(row invoiceEmailNotificationRow) domain.InvoiceEmailNotification {
	return domain.InvoiceEmailNotification{
		ID: row.ID, TenantID: row.TenantID, InvoiceID: row.InvoiceID, Recipient: row.Recipient,
		Status: row.Status, PublishAttempts: row.PublishAttempts, DeliveryAttempts: row.DeliveryAttempts,
		AvailableAt: row.AvailableAt, LockedAt: row.LockedAt, LockedBy: row.LockedBy,
		TaskName: row.TaskName, LastError: row.LastError, SentAt: row.SentAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

var _ domain.InvoiceEmailNotificationRepository = (*PostgresInvoiceEmailNotificationRepository)(nil)
