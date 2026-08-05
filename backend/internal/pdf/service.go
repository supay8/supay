package pdf

import (
	"fmt"

	"github.com/brandsrx/supay/internal/models"
	"gorm.io/gorm"
)

// Service carga una factura con sus relaciones y genera el PDF.
type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// GenerateInvoicePDF carga la factura (empresa, cliente, punto de venta,
// CUFD e ítems) y devuelve el PDF como []byte.
func (s *Service) GenerateInvoicePDF(invoiceID string) ([]byte, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("pdf service: db no inicializado")
	}
	var inv models.Invoice
	if err := s.db.Preload("Items").Preload("PointOfSale").
		Preload("Company").Preload("Customer").Preload("CufdRecord").
		First(&inv, "id = ?", invoiceID).Error; err != nil {
		return nil, fmt.Errorf("pdf: factura no encontrada: %w", err)
	}

	data := InvoicePDFData{
		Invoice: &inv,
		QRImage: DecodeQR(inv.CufdRecord.CodigoQR),
	}
	return Generate(data)
}
