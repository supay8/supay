package ports

import (
	"context"
	"time"
)

// Notification es el contrato estable entre los casos de uso y cualquier
// canal de entrega (webhook hoy; email puede agregarse sin tocar el dominio).
type Notification struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	TenantID   string         `json:"tenant_id"`
	OccurredAt time.Time      `json:"occurred_at"`
	Data       map[string]any `json:"data"`
}

type Notifier interface {
	Notify(ctx context.Context, target string, notification Notification) error
}
