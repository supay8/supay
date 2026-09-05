package domain

import "time"

type InvoiceEvent struct {
	ID        string
	InvoiceID string
	TenantID  string
	EventKey  *string
	Type      string
	Message   string
	Payload   []byte
	CreatedAt time.Time
}

type InvoiceEventRepository interface {
	Create(event *InvoiceEvent) error
	List(invoiceID string) ([]*InvoiceEvent, error)
}
