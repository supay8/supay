package pdf

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/brandsrx/supay/internal/models"
	"gorm.io/gorm"
)

// Service carga una factura con sus relaciones y genera el PDF.
// Usa un Storage pluggable: none (on-demand), local (filesystem) o r2 (Cloudflare).
type Service struct {
	db      *gorm.DB
	storage Storage
}

// NewService crea el servicio con storage noop (compatibilidad).
// Preferir NewServiceWithStorage para inyección explícita.
func NewService(db *gorm.DB) *Service {
	return &Service{db: db, storage: NewNoopStorage()}
}

// NewServiceWithStorage crea el servicio con el storage indicado.
func NewServiceWithStorage(db *gorm.DB, storage Storage) *Service {
	if storage == nil {
		storage = NewNoopStorage()
	}
	return &Service{db: db, storage: storage}
}

// GenerateInvoicePDF carga la factura (empresa, snapshot del receptor, punto de venta,
// CUFD e ítems) y devuelve el PDF como []byte. Implementa cache-aside:
// intenta storage.Get primero; en miss genera y hace Save best-effort.
func (s *Service) GenerateInvoicePDF(invoiceID string) ([]byte, error) {
	return s.GenerateInvoicePDFWithContext(context.Background(), invoiceID)
}

// GenerateInvoicePDFWithContext es la variante con contexto para storage.
func (s *Service) GenerateInvoicePDFWithContext(ctx context.Context, invoiceID string) ([]byte, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("pdf service: db no inicializado")
	}
	if s.storage == nil {
		s.storage = NewNoopStorage()
	}
	// Cache-aside: intentar servir desde storage sin tocar DB.
	if data, err := s.storage.Get(ctx, invoiceID); err == nil {
		return data, nil
	} else if !errors.Is(err, ErrNotFound) {
		slog.Warn("pdf storage: get falló, regenerando on-demand", "invoice_id", invoiceID, "error", err)
	}

	var inv models.Invoice
	if err := s.db.Preload("Items").Preload("PointOfSale").
		Preload("Company.Config").Preload("CufdRecord").
		First(&inv, "id = ?", invoiceID).Error; err != nil {
		return nil, fmt.Errorf("pdf: factura no encontrada: %w", err)
	}

	data := InvoicePDFData{
		Invoice: &inv,
		QRImage: DecodeQR(inv.CufdRecord.CodigoQR),
	}
	pdfBytes, err := Generate(data)
	if err != nil {
		return nil, err
	}
	// Best-effort persist: no falla la request si el storage falla.
	if err := s.storage.Save(ctx, invoiceID, pdfBytes); err != nil {
		slog.Warn("pdf storage: save falló (on-demand sigue funcionando)", "invoice_id", invoiceID, "error", err)
	}
	return pdfBytes, nil
}

// GenerateAndPersist genera y persiste sin devolver error al caller (útil para hook post-emisión).
func (s *Service) GenerateAndPersist(ctx context.Context, invoiceID string) {
	if _, err := s.GenerateInvoicePDFWithContext(ctx, invoiceID); err != nil {
		slog.Warn("pdf storage: GenerateAndPersist falló", "invoice_id", invoiceID, "error", err)
	}
}
