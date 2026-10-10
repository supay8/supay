package usecase

import (
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
)

// PaqueteInput y MasivaInput expresan únicamente la selección del consumidor.
// Una selección omitida procesa las facturas pendientes aptas del punto de venta.
type PaqueteInput struct {
	FacturaIDs []string `json:"invoice_ids,omitempty"`
}

type MasivaInput struct {
	FacturaIDs []string `json:"invoice_ids,omitempty"`
}

type ComprasInput struct {
	Descripcion      string    `json:"descripcion"`
	TipoCompra       int       `json:"tipoCompra"`
	Archivo          string    `json:"archivo"`
	HashArchivo      string    `json:"hashArchivo"`
	CantidadFacturas int       `json:"cantidadFacturas"`
	Gestion          int       `json:"gestion"`
	Periodo          int       `json:"periodo"`
	FechaEnvio       time.Time `json:"fechaEnvio"`
}

type PaqueteValidacionInput struct {
	BatchID string `json:"-"`
}

type FirmaInput struct {
	Xml string `json:"xml"`
}

type BatchResultado struct {
	BatchID    string                     `json:"batch_id,omitempty"`
	InvoiceIDs []string                   `json:"invoice_ids"`
	Status     domain.SentPackageStatus   `json:"status"`
	Response   *ports.FiscalPackageResult `json:"response,omitempty"`
	Error      string                     `json:"error,omitempty"`
}

type PaqueteResultado struct {
	Batches     []BatchResultado
	Company     *domain.Company
	PointOfSale *domain.PointOfSale
	Response    *ports.FiscalPackageResult
}

type ComprasResultado struct {
	Company     *domain.Company
	PointOfSale *domain.PointOfSale
	Response    *ports.FiscalPurchaseResult
}

type FirmaResultado struct {
	Company     *domain.Company
	PointOfSale *domain.PointOfSale
	Response    *ports.FiscalSignResult
}
