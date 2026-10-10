package ports

import "github.com/brandsrx/supay/internal/domain"

type APIKeyRepository interface {
	Create(*domain.ApiKey) error
	ListByTenant(string) ([]domain.ApiKey, error)
	Deactivate(string, string) error
	MaxActiveKeys(string) (int, error)
}

// CompanyBootstrap runs all writes on the same transaction-bound repositories.
type CompanyBootstrap interface {
	WithinTransaction(func(CompanyRepository, APIKeyRepository) error) error
}
