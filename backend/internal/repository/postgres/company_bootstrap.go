package postgres

import (
	"errors"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/brandsrx/supay/internal/ports"
	"gorm.io/gorm"
)

type CompanyBootstrap struct{ db *gorm.DB }

func NewCompanyBootstrap(db *gorm.DB) *CompanyBootstrap { return &CompanyBootstrap{db: db} }

var _ ports.CompanyBootstrap = (*CompanyBootstrap)(nil)

func (b *CompanyBootstrap) WithinTransaction(run func(ports.CompanyRepository, ports.APIKeyRepository) error) error {
	return repositoryError(b.db.Transaction(func(tx *gorm.DB) error {
		return run(NewPostgresCompanyRepository(tx), NewPostgresApiKeyRepository(tx))
	}))
}
func (r *PostgresApiKeyRepository) MaxActiveKeys(tenantID string) (int, error) {
	var config models.TenantConfig
	err := r.db.Where("tenant_id = ?", tenantID).First(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 10, nil
	}
	return config.MaxAPIKeys, repositoryError(err)
}

func apiKeyModel(key *domain.ApiKey) models.ApiKey {
	return models.ApiKey{ID: key.ID, CompanyId: key.CompanyId, KeyHash: key.KeyHash, KeyPrefix: key.KeyPrefix, Name: key.Name, Scopes: key.Scopes, IsActive: key.IsActive, LastUsedAt: key.LastUsedAt, ExpiresAt: key.ExpiresAt, CreatedAt: key.CreatedAt, UpdatedAt: key.UpdatedAt}
}
func apiKeyDomain(key models.ApiKey) domain.ApiKey {
	return domain.ApiKey{ID: key.ID, CompanyId: key.CompanyId, KeyHash: key.KeyHash, KeyPrefix: key.KeyPrefix, Name: key.Name, Scopes: key.Scopes, IsActive: key.IsActive, LastUsedAt: key.LastUsedAt, ExpiresAt: key.ExpiresAt, CreatedAt: key.CreatedAt, UpdatedAt: key.UpdatedAt}
}
