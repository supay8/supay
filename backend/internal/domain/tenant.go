package domain

import "errors"

// ErrMissingCompanyID indica que una operación requiere el identificador de
// empresa/tenant y no fue proporcionado. Se usa para reforzar el aislamiento
// multi-tenant en repositorios y usecases.
var ErrMissingCompanyID = errors.New("se requiere el identificador de empresa")
