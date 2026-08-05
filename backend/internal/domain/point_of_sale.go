package domain

import (
	"errors"
	"time"

	"gorm.io/datatypes"
)

// ErrPointOfSaleCodeConflict indica que ya existe un punto de venta con el mismo código
// para la empresa y sucursal (violación del índice único compuesto).
var ErrPointOfSaleCodeConflict = errors.New("ya existe un punto de venta con este código para la empresa y sucursal")

// ErrPointOfSaleHasDependencies indica que el punto de venta tiene registros
// asociados y no puede eliminarse (integridad referencial).
var ErrPointOfSaleHasDependencies = errors.New("no se puede eliminar: el punto de venta tiene CUFDs, facturas o eventos de contingencia asociados")

// ErrTipoPuntoVentaInvalido indica que el codigoTipoPuntoVenta solicitado no está
// en el catálogo oficial sincronizado del SIAT.
var ErrTipoPuntoVentaInvalido = errors.New("código de tipo de punto de venta no válido para esta empresa (sincronice el catálogo)")

type PointOfSale struct {
	ID               string     `json:"id"`
	CompanyId        string     `json:"company_id"`
	BranchId         *string    `json:"branch_id,omitempty"`
	CodigoSucursal   int        `json:"codigo_sucursal"`
	CodigoPuntoVenta int        `json:"codigo_punto_venta"`
	Description      string     `json:"description"`
	Cuis             *string    `json:"cuis,omitempty"`
	CuisCreatedAt    *time.Time `json:"cuis_created_at,omitempty"`
	IsActive         bool       `json:"is_active"`
	SiatCode         *int       `json:"siat_code,omitempty"`
	Status           string     `json:"status,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`

	// Datos del registro oficial ante el SIAT (operación registroPuntoVenta)
	TipoPuntoVenta   *int            `json:"tipo_punto_venta,omitempty"`
	SiatTransaccion  bool            `json:"siat_transaccion"`
	SiatRegisteredAt *time.Time      `json:"siat_registered_at,omitempty"`
	SiatResponse     *datatypes.JSON `json:"siat_response,omitempty"`
	SiatError        *string         `json:"siat_error,omitempty"`
}

// PointOfSaleRepository define el contrato para la persistencia
type PointOfSaleRepository interface {
	// Create persiste un punto de venta calculando automáticamente el
	// codigoPuntoVenta (MAX+1) bajo advisory lock para evitar colisiones
	// bajo concurrencia. Si pos.CodigoPuntoVenta ya es > 0 se respeta ese
	// código local (el índice único valida colisiones).
	Create(pos *PointOfSale) error
	GetByID(id string) (*PointOfSale, error)
	List(companyID string) ([]*PointOfSale, error)
	ListByBranch(branchID string) ([]*PointOfSale, error)
	Update(pos *PointOfSale) error
	Delete(id string) error
}
