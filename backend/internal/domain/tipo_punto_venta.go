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
