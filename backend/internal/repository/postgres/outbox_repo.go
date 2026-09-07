package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"gorm.io/gorm"
)

type PostgresOutboxRepository struct {
	db *gorm.DB
}

func NewPostgresOutboxRepository(db *gorm.DB) *PostgresOutboxRepository {
	return &PostgresOutboxRepository{db: db}
}

func (r *PostgresOutboxRepository) EnqueueInvoiceEmission(ctx context.Context, invoiceID, tenantID, cufdID string) (*domain.OutboxEvent, error) {
	payload, err := json.Marshal(domain.InvoiceEmissionPayload{InvoiceID: invoiceID, TenantID: tenantID})
	if err != nil {
		return nil, fmt.Errorf("serializar evento de emisión: %w", err)
	}

	var event models.OutboxEvent
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var invoice struct {
			TenantID      string `gorm:"column:tenant_id"`
			PointOfSaleID string `gorm:"column:point_of_sale_id"`
			Status        string `gorm:"column:status"`
		}
		if err := tx.Raw(`
			SELECT tenant_id, point_of_sale_id, status
			FROM invoices
			WHERE id = ?
			FOR UPDATE`, invoiceID).Scan(&invoice).Error; err != nil {
			return err
		}
		if invoice.TenantID == "" {
			return gorm.ErrRecordNotFound
		}
		if invoice.TenantID != tenantID {
			return errors.New("la factura no pertenece al tenant indicado")
		}
		if domain.InvoiceStatus(invoice.Status) != domain.InvoicePending {
			return domain.NewConflictError("solo se pueden encolar facturas en estado PENDING")
		}
		if cufdID == "" {
			return domain.NewConflictError("no se puede encolar una factura sin CUFD vigente")
		}
		var validCufd bool
		if err := tx.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM cufd_history
				WHERE id = ? AND tenant_id = ? AND point_of_sale_id = ?
				  AND is_active = true AND valid_from <= now() AND valid_to >= now()
			)`, cufdID, tenantID, invoice.PointOfSaleID).Scan(&validCufd).Error; err != nil {
			return err
		}
		if !validCufd {
			return domain.NewConflictError("el CUFD seleccionado dejó de estar vigente antes de encolar")
		}
		if err := tx.Exec(`
			UPDATE invoices
			SET cufd_id = ?, updated_at = now()
			WHERE id = ? AND tenant_id = ?`, cufdID, invoiceID, tenantID).Error; err != nil {
			return err
		}

		return tx.Raw(`
			INSERT INTO outbox (
				tenant_id, aggregate_type, aggregate_id, event_type, payload,
				status, attempts, available_at
			)
			VALUES (?, ?, ?, ?, ?::jsonb, 'PENDING', 0, now())
			ON CONFLICT (event_type, aggregate_id) DO UPDATE SET
				payload = EXCLUDED.payload,
				status = CASE
					WHEN outbox.status = 'PUBLISHED' THEN 'PENDING'
					ELSE outbox.status
				END,
				attempts = CASE
					WHEN outbox.status = 'PUBLISHED' THEN 0
					ELSE outbox.attempts
				END,
				available_at = CASE
					WHEN outbox.status = 'PUBLISHED' THEN now()
					ELSE outbox.available_at
				END,
				published_at = CASE
					WHEN outbox.status = 'PUBLISHED' THEN NULL
					ELSE outbox.published_at
				END,
				last_error = CASE
					WHEN outbox.status = 'PUBLISHED' THEN NULL
					ELSE outbox.last_error
				END,
				updated_at = now()
			RETURNING *`,
			tenantID, domain.OutboxAggregateInvoice, invoiceID,
			domain.OutboxEventInvoiceEmit, string(payload),
		).Scan(&event).Error
	})
	if err != nil {
		return nil, err
	}
	return outboxToDomain(&event), nil
}

func (r *PostgresOutboxRepository) ClaimPending(ctx context.Context, eventType, owner string, limit int, now time.Time, lockTimeout time.Duration) ([]domain.OutboxEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	if lockTimeout <= 0 {
		lockTimeout = time.Minute
	}
	staleBefore := now.Add(-lockTimeout)
	var rows []models.OutboxEvent
	if err := r.db.WithContext(ctx).Raw(`
		WITH candidates AS (
			SELECT id
			FROM outbox
			WHERE event_type = ?
			  AND ((status = 'PENDING' AND available_at <= ?)
			   OR (status = 'PROCESSING' AND locked_at < ?))
			ORDER BY available_at, created_at
			FOR UPDATE SKIP LOCKED
			LIMIT ?
		)
		UPDATE outbox AS event
		SET status = 'PROCESSING',
			attempts = event.attempts + 1,
			locked_at = ?,
			locked_by = ?,
			updated_at = ?
		FROM candidates
		WHERE event.id = candidates.id
		RETURNING event.*`, eventType, now, staleBefore, limit, now, owner, now).Scan(&rows).Error; err != nil {
		return nil, err
	}
	events := make([]domain.OutboxEvent, 0, len(rows))
	for i := range rows {
		events = append(events, *outboxToDomain(&rows[i]))
	}
	return events, nil
}

func (r *PostgresOutboxRepository) MarkPublished(ctx context.Context, id, owner string, publishedAt time.Time) error {
	result := r.db.WithContext(ctx).Model(&models.OutboxEvent{}).
		Where("id = ? AND status = 'PROCESSING' AND locked_by = ?", id, owner).
		Updates(map[string]any{
			"status":       domain.OutboxStatusPublished,
			"published_at": publishedAt,
			"locked_at":    nil,
			"locked_by":    nil,
			"last_error":   nil,
			"updated_at":   publishedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("outbox %s no estaba reclamado por %s", id, owner)
	}
	return nil
}

func (r *PostgresOutboxRepository) MarkFailed(ctx context.Context, id, owner, lastError string, nextAttempt time.Time) error {
	if len(lastError) > 1000 {
		lastError = lastError[:1000]
	}
	result := r.db.WithContext(ctx).Model(&models.OutboxEvent{}).
		Where("id = ? AND status = 'PROCESSING' AND locked_by = ?", id, owner).
		Updates(map[string]any{
			"status":       domain.OutboxStatusPending,
			"available_at": nextAttempt,
			"locked_at":    nil,
			"locked_by":    nil,
			"last_error":   lastError,
			"updated_at":   time.Now().UTC(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("outbox %s no estaba reclamado por %s", id, owner)
	}
	return nil
}

func outboxToDomain(event *models.OutboxEvent) *domain.OutboxEvent {
	return &domain.OutboxEvent{
		ID: event.ID, TenantID: event.TenantID, AggregateType: event.AggregateType,
		AggregateID: event.AggregateID, EventType: event.EventType,
		Payload: append(json.RawMessage(nil), event.Payload...), Status: event.Status,
		Attempts: event.Attempts, AvailableAt: event.AvailableAt, LockedAt: event.LockedAt,
		LockedBy: event.LockedBy, PublishedAt: event.PublishedAt, LastError: event.LastError,
		CreatedAt: event.CreatedAt, UpdatedAt: event.UpdatedAt,
	}
}

var _ domain.OutboxRepository = (*PostgresOutboxRepository)(nil)
