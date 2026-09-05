package postgres

import (
	"time"

	"github.com/brandsrx/supay/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// PostgresApiKeyRepository persiste y busca API keys por tenant.
type PostgresApiKeyRepository struct {
	db *gorm.DB
}

func NewPostgresApiKeyRepository(db *gorm.DB) *PostgresApiKeyRepository {
	return &PostgresApiKeyRepository{db: db}
}

// FindByPrefix busca una API key activa y no vencida por su prefijo identificador.
// El llamador debe verificar la key plana contra KeyHash con bcrypt.CompareHashAndPassword.
func (r *PostgresApiKeyRepository) FindByPrefix(prefix string) (*models.ApiKey, error) {
	var key models.ApiKey
	if err := r.db.Where("key_prefix = ? AND is_active = true", prefix).First(&key).Error; err != nil {
		return nil, err
	}
	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return nil, gorm.ErrRecordNotFound
	}
	return &key, nil
}

// TouchLastUsed actualiza el timestamp de último uso.
func (r *PostgresApiKeyRepository) TouchLastUsed(id string) error {
	return r.db.Model(&models.ApiKey{}).Where("id = ?", id).Update("last_used_at", time.Now()).Error
}

// HashKey genera un hash bcrypt de una API key plana.
func HashKey(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// VerifyKey compara una API key plana con su hash bcrypt.
func VerifyKey(plain, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// Create inserta una nueva API key.
func (r *PostgresApiKeyRepository) Create(key *models.ApiKey) error {
	return r.db.Create(key).Error
}

// ListByTenant devuelve todas las API keys de un tenant ordenadas por fecha de creación.
func (r *PostgresApiKeyRepository) ListByTenant(tenantID string) ([]models.ApiKey, error) {
	var keys []models.ApiKey
	err := r.db.Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&keys).Error
	return keys, err
}

// GetByID busca una API key por su ID.
func (r *PostgresApiKeyRepository) GetByID(id string) (*models.ApiKey, error) {
	var key models.ApiKey
	if err := r.db.First(&key, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// Deactivate desactiva una API key (soft delete por tenant).
func (r *PostgresApiKeyRepository) Deactivate(id, tenantID string) error {
	return r.db.Model(&models.ApiKey{}).Where("id = ? AND tenant_id = ?", id, tenantID).Update("is_active", false).Error
}
