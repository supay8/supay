package invoiceemail

import (
	"context"
	"errors"
	"fmt"
	"html"
	"time"

	"github.com/brandsrx/supay/internal/domain"
)

type PDFGenerator interface {
	GenerateInvoicePDFWithContext(context.Context, string) ([]byte, error)
}

type InvoiceFileReader interface {
	ReadAll(ctx context.Context, companyID, invoiceID, kind string) ([]byte, *domain.InvoiceFile, error)
}

type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

type Message struct {
	To          string
	Subject     string
	Text        string
	HTML        string
	Attachments []Attachment
}

type Sender interface {
	Send(context.Context, Message) error
}

type Processor struct {
	repo        domain.InvoiceEmailNotificationRepository
	invoices    domain.InvoiceRepository
	files       InvoiceFileReader
	pdf         PDFGenerator
	sender      Sender
	lockTimeout time.Duration
	now         func() time.Time
}

func NewProcessor(repo domain.InvoiceEmailNotificationRepository, invoices domain.InvoiceRepository, files InvoiceFileReader, pdf PDFGenerator, sender Sender, lockTimeout time.Duration) *Processor {
	if lockTimeout <= 0 {
		lockTimeout = 10 * time.Minute
	}
	return &Processor{repo: repo, invoices: invoices, files: files, pdf: pdf, sender: sender, lockTimeout: lockTimeout, now: func() time.Time { return time.Now().UTC() }}
}

func (p *Processor) Process(ctx context.Context, notificationID string) error {
	if p == nil || p.repo == nil || p.invoices == nil || p.files == nil || p.pdf == nil || p.sender == nil {
		return errors.New("procesador de email no configurado")
	}
	notification, claimed, err := p.repo.ClaimDelivery(ctx, notificationID, p.now(), p.lockTimeout)
	if err != nil {
		return err
	}
	if !claimed {
		if notification.Status == domain.InvoiceEmailSent {
			return nil
		}
		return fmt.Errorf("notificación %s ocupada o no encolada", notificationID)
	}
	fail := func(cause error) error {
		if markErr := p.repo.MarkDeliveryFailed(ctx, notificationID, cause.Error(), p.now()); markErr != nil {
			return errors.Join(cause, markErr)
		}
		return cause
	}
	invoice, err := p.invoices.GetByID(notification.TenantID, notification.InvoiceID)
	if err != nil {
		return fail(err)
	}
	if invoice.Status != domain.InvoiceAccepted && invoice.Status != domain.InvoiceObserved {
		return fail(fmt.Errorf("factura %s no tiene estado enviable: %s", invoice.ID, invoice.Status))
	}
	pdfBytes, err := p.pdf.GenerateInvoicePDFWithContext(ctx, invoice.ID)
	if err != nil {
		return fail(fmt.Errorf("generar PDF: %w", err))
	}
	xmlBytes, _, err := p.files.ReadAll(ctx, notification.TenantID, invoice.ID, "xml")
	if err != nil {
		return fail(fmt.Errorf("leer XML: %w", err))
	}
	filename := fmt.Sprintf("factura-%d", invoice.InvoiceNumber)
	message := Message{
		To:          notification.Recipient,
		Subject:     fmt.Sprintf("Factura %d - %s", invoice.InvoiceNumber, invoice.Company.BusinessName),
		Text:        fmt.Sprintf("Adjuntamos su factura N° %d en formatos PDF y XML.", invoice.InvoiceNumber),
		HTML:        fmt.Sprintf("<p>Adjuntamos su factura <strong>N° %d</strong> en formatos PDF y XML.</p><p>%s</p>", invoice.InvoiceNumber, html.EscapeString(invoice.Company.BusinessName)),
		Attachments: []Attachment{{Filename: filename + ".pdf", ContentType: "application/pdf", Data: pdfBytes}, {Filename: filename + ".xml", ContentType: "application/xml", Data: xmlBytes}},
	}
	if err := p.sender.Send(ctx, message); err != nil {
		return fail(fmt.Errorf("enviar email: %w", err))
	}
	return p.repo.MarkSent(ctx, notificationID, p.now())
}
