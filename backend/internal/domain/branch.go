package domain

import (
	"errors"
	"time"
)

// ErrBranchSucursalConflict indica que ya existe una sucursal con el mismo
// codigoSucursal para la empresa (violación del índice único compuesto).
var ErrBranchSucursalConflict = errors.New("ya existe una sucursal con este codigoSucursal para la empresa")

type Branch struct {
	ID             string    `json:"id"`
	CompanyID      string    `json:"company_id"`
	CodigoSucursal int       `json:"codigo_sucursal"`
	Name           string    `json:"name"`
	Address        string    `json:"address"`
	Active         bool      `json:"active"`
	CreatedAt      time.Time `json:"created_at"`
}

type BranchRepository interface {
	Create(branch *Branch) error
	GetByID(id string) (*Branch, error)
	GetByCompanyAndSucursal(companyID string, codigoSucursal int) (*Branch, error)
	List(companyID string) ([]*Branch, error)
	Update(branch *Branch) error
	Delete(id string) error
}
