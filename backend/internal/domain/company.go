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
	ID                    string          `json:"id"`
	AuthOrganizationID    string          `json:"auth_organization_id,omitempty"`
	Nit                   string          `json:"nit"`
	BusinessName          string          `json:"business_name"`
	Ambiente              SiatEnvironment `json:"ambiente"`
	Modalidad             int             `json:"modalidad"` // 1 electrónica, 2 computarizada (default 1)
	Municipio             string          `json:"municipio,omitempty"`
	Direccion             string          `json:"direccion,omitempty"`
	Telefono              string          `json:"telefono,omitempty"`
	CodigoActividad       *string         `json:"codigo_actividad,omitempty"`
	PiePagina             string          `json:"pie_pagina,omitempty"`
	UsuarioSiat           string          `json:"usuario_siat,omitempty"`
	CertificateWebhookURL string          `json:"certificate_webhook_url,omitempty"`
	// EncryptedTokenDelegado se persiste cifrado en tenant_configs y nunca se
	// expone en respuestas JSON.
	EncryptedTokenDelegado string    `json:"-"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// CompanyRepository define el contrato para la persistencia
type CompanyRepository interface {
	Create(company *Company) error
	GetByNit(nit string) (*Company, error)
	GetByID(id string) (*Company, error)
	Update(company *Company) error
	Delete(id string) error
}
