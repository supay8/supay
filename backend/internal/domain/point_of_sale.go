package domain

import (
	"errors"
	"time"
)

// ErrPointOfSaleCodeConflict indica que ya existe un punto de venta con el mismo código
// para la empresa y sucursal (violación del índice único compuesto).
var ErrPointOfSaleCodeConflict = errors.New("ya existe un punto de venta con este código para la empresa y sucursal")

// ErrPointOfSaleHasDependencies indica que el punto de venta tiene registros
// asociados y no puede eliminarse (integridad referencial).
var ErrPointOfSaleHasDependencies = errors.New("no se puede eliminar: el punto de venta tiene CUFDs, facturas o eventos de contingencia asociados")

type PointOfSale struct {
	ID               string     `json:"id"`
	CompanyId        string     `json:"company_id"`
	CodigoSucursal   int        `json:"codigo_sucursal"`
	CodigoPuntoVenta int        `json:"codigo_punto_venta"`
	Description      string     `json:"description"`
	Cuis             *string    `json:"cuis,omitempty"`
	CuisCreatedAt    *time.Time `json:"cuis_created_at,omitempty"`
	IsActive         bool       `json:"is_active"`
	CreatedAt        time.Time  `json:"created_at"`
}

// PointOfSaleRepository define el contrato para la persistencia
type PointOfSaleRepository interface {
	// Create persiste un punto de venta calculando automáticamente el
	// codigoPuntoVenta (MAX+1) bajo advisory lock para evitar colisiones
	// bajo concurrencia.
	Create(pos *PointOfSale) error
	GetByID(id string) (*PointOfSale, error)
	List(companyID string) ([]*PointOfSale, error)
	Update(pos *PointOfSale) error
	Delete(id string) error
}
