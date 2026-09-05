package domain

import "time"

// InvoiceDocumentType identifica el formato persistido de un documento fiscal.
type InvoiceDocumentType string

const (
	InvoiceDocumentXML  InvoiceDocumentType = "XML"
	InvoiceDocumentFile InvoiceDocumentType = "FILE"
	InvoiceDocumentPDF  InvoiceDocumentType = "PDF"
)

// InvoiceDocument representa una versión inmutable de un documento de factura.
type InvoiceDocument struct {
	ID           string              `json:"id"`
	InvoiceID    string              `json:"invoice_id"`
	DocumentType InvoiceDocumentType `json:"document_type"`
	Version      int                 `json:"version"`
	Content      *string             `json:"content,omitempty"`
	StorageRef   *string             `json:"storage_ref,omitempty"`
	MIMEType     *string             `json:"mime_type,omitempty"`
	SHA256       *string             `json:"sha256,omitempty"`
	IsCurrent    bool                `json:"is_current"`
	CreatedAt    time.Time           `json:"created_at"`
}

type InvoiceDocumentRepository interface {
	Create(document *InvoiceDocument) error
	List(invoiceID string) ([]*InvoiceDocument, error)
	ListCurrent(invoiceID string) ([]*InvoiceDocument, error)
}
