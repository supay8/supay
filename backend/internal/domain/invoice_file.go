package domain

import (
	"time"
)

type InvoiceFile struct {
	ID          string
	CompanyID   string
	InvoiceID   string
	Kind        string
	StorageKey  string
	SHA256      string
	Size        int64
	ContentType string
	CreatedAt   time.Time
}
