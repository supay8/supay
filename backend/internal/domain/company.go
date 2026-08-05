package domain

import (
	"errors"
	"time"
)

type SiatEnvironment string

const (
	EnvironmentPiloto     SiatEnvironment = "PILOTO"
	EnvironmentProduccion SiatEnvironment = "PRODUCCION"
)

// CodigoAmbiente devuelve el código numérico que el SIAT espera para el ambiente:
// 1 = producción, 2 = piloto/pruebas.
func (e SiatEnvironment) CodigoAmbiente() int {
	if e == EnvironmentProduccion {
		return 1
	}
	return 2
}

// ErrCompanyNitConflict indica que ya existe una empresa con el mismo NIT
var ErrCompanyNitConflict = errors.New("ya existe una empresa registrada con este NIT")

// ErrCompanyHasDependencies indica que la empresa tiene registros asociados
// y no puede eliminarse (integridad referencial).
var ErrCompanyHasDependencies = errors.New("no se puede eliminar: la empresa tiene puntos de venta, clientes o facturas asociados")

type Company struct {
	ID            string          `json:"id"`
	Nit           string          `json:"nit"`
	BusinessName  string          `json:"business_name"`
	CodigoSistema string          `json:"codigo_sistema"`
	Ambiente      SiatEnvironment `json:"ambiente"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// CompanyRepository define el contrato para la persistencia
type CompanyRepository interface {
	Create(company *Company) error
	GetByNit(nit string) (*Company, error)
	GetByID(id string) (*Company, error)
	Update(company *Company) error
	Delete(id string) error
}
