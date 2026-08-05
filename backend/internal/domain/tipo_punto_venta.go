package domain

import (
	"errors"
	"time"
)

// ErrTipoPuntoVentaEmpty indica que el catálogo de tipos de punto de venta
// aún no ha sido sincronizado para la empresa.
var ErrTipoPuntoVentaEmpty = errors.New("catálogo de tipos de punto de venta vacío para la empresa")

type TipoPuntoVenta struct {
	ID                 string    `json:"id"`
	CompanyID          string    `json:"company_id"`
	CodigoClasificador int       `json:"codigo_clasificador"`
	Descripcion        string    `json:"descripcion"`
	SyncedAt           time.Time `json:"synced_at"`
	CreatedAt          time.Time `json:"created_at"`
}

type TipoPuntoVentaRepository interface {
	// Replace reemplaza el catálogo de tipos de punto de venta de la empresa
	// con los valores sincronizados del SIAT.
	Replace(companyID string, tipos []TipoPuntoVenta, syncedAt time.Time) error
	// List devuelve el catálogo vigente de la empresa ordenado por clasificador.
	List(companyID string) ([]*TipoPuntoVenta, error)
	// FindByClasificador busca un tipo por su código oficial.
	FindByClasificador(companyID string, codigoClasificador int) (*TipoPuntoVenta, error)
}
