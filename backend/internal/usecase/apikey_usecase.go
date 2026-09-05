package usecase

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/brandsrx/supay/internal/repository/postgres"
	"gorm.io/gorm"
)

type ApiKeyUsecase struct {
	companyRepo    domain.CompanyRepository
	apiKeyRepo     *postgres.PostgresApiKeyRepository
	db             *gorm.DB
}

func NewApiKeyUsecase(companyRepo domain.CompanyRepository, apiKeyRepo *postgres.PostgresApiKeyRepository, db *gorm.DB) *ApiKeyUsecase {
	return &ApiKeyUsecase{
		companyRepo: companyRepo,
		apiKeyRepo:  apiKeyRepo,
		db:          db,
	}
}

type CreateApiKeyRequest struct {
	Name string `json:"name"`
}

type CreateApiKeyResponse struct {
	APIKey     string          `json:"api_key"`
	KeyPrefix  string          `json:"key_prefix"`
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	CreatedAt  string          `json:"created_at"`
}

type ListApiKeyResponse struct {
	ID         string `json:"id"`
	KeyPrefix  string `json:"key_prefix"`
	Name       string `json:"name"`
	IsActive   bool   `json:"is_active"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
	CreatedAt  string `json:"created_at"`
}

func (uc *ApiKeyUsecase) Generate(tenantID, name string) (string, *models.ApiKey, error) {
	var config models.TenantConfig
	if err := uc.db.Where("tenant_id = ?", tenantID).First(&config).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil, err
		}
		config.MaxAPIKeys = 10
	}

	existingKeys, err := uc.apiKeyRepo.ListByTenant(tenantID)
	if err != nil {
		return "", nil, err
	}
	activeCount := 0
	for _, k := range existingKeys {
		if k.IsActive {
			activeCount++
		}
	}
	if activeCount >= config.MaxAPIKeys {
		return "", nil, domain.NewConflictError("límite de API keys alcanzado (" + string(rune(config.MaxAPIKeys+'0')) + ")")
	}

	env := "live"
	if strings.Contains(strings.ToLower(tenantID), "test") || strings.Contains(strings.ToLower(tenantID), "piloto") {
		env = "test"
	}

	prefixBytes := make([]byte, 4)
	if _, err := rand.Read(prefixBytes); err != nil {
		return "", nil, err
	}
	prefix := "sup_" + env + "_" + hex.EncodeToString(prefixBytes)

	secretBytes := make([]byte, 16)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", nil, err
	}
	secret := hex.EncodeToString(secretBytes)

	plain := prefix + "_" + secret

	hash, err := postgres.HashKey(plain)
	if err != nil {
		return "", nil, err
	}

	key := &models.ApiKey{
		CompanyId: tenantID,
		KeyHash:   hash,
		KeyPrefix: prefix,
		Name:      name,
		Scopes:    "read,write",
		IsActive:  true,
	}

	if err := uc.apiKeyRepo.Create(key); err != nil {
		return "", nil, err
	}

	return plain, key, nil
}

func (uc *ApiKeyUsecase) Create(req CreateApiKeyRequest, tenantID string) (CreateApiKeyResponse, error) {
	plain, key, err := uc.Generate(tenantID, req.Name)
	if err != nil {
		return CreateApiKeyResponse{}, err
	}
	return CreateApiKeyResponse{
		APIKey:    plain,
		KeyPrefix: key.KeyPrefix,
		ID:        key.ID,
		Name:      key.Name,
		CreatedAt: key.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

func (uc *ApiKeyUsecase) List(tenantID string) ([]ListApiKeyResponse, error) {
	keys, err := uc.apiKeyRepo.ListByTenant(tenantID)
	if err != nil {
		return nil, err
	}
	resp := make([]ListApiKeyResponse, 0, len(keys))
	for _, k := range keys {
		var lastUsed string
		if k.LastUsedAt != nil {
			lastUsed = k.LastUsedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		resp = append(resp, ListApiKeyResponse{
			ID:         k.ID,
			KeyPrefix:  k.KeyPrefix,
			Name:       k.Name,
			IsActive:   k.IsActive,
			LastUsedAt: &lastUsed,
			CreatedAt:  k.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return resp, nil
}

func (uc *ApiKeyUsecase) Revoke(tenantID, keyID string) error {
	return uc.apiKeyRepo.Deactivate(keyID, tenantID)
}

type BootstrapCompanyRequest struct {
	RegisterCompanyRequest
}

type BootstrapCompanyResponse struct {
	Company  *domain.Company `json:"company"`
	APIKey   string          `json:"api_key"`
	KeyPrefix string         `json:"key_prefix"`
	KeyID    string          `json:"key_id"`
}

func (uc *ApiKeyUsecase) BootstrapCompany(req BootstrapCompanyRequest) (BootstrapCompanyResponse, error) {
	if req.Nit == "" {
		return BootstrapCompanyResponse{}, domain.NewBadRequestError("el nit es obligatorio")
	}
	if req.BusinessName == "" {
		return BootstrapCompanyResponse{}, domain.NewBadRequestError("el nombre de la empresa es obligatorio")
	}

	company := &domain.Company{
		Nit:             req.Nit,
		BusinessName:    req.BusinessName,
		CodigoSistema:   req.CodigoSistema,
		Ambiente:        req.Ambiente,
		UsuarioSiat:     req.UsuarioSiat,
		Municipio:       req.Municipio,
		Direccion:       req.Direccion,
		Telefono:        req.Telefono,
		CodigoActividad: req.CodigoActividad,
		PiePagina:       req.PiePagina,
	}

	if company.Ambiente == "" {
		company.Ambiente = domain.EnvironmentPiloto
	}
	if company.UsuarioSiat == "" {
		company.UsuarioSiat = "SUPAY"
	}

	if company.Ambiente != domain.EnvironmentPiloto && company.Ambiente != domain.EnvironmentProduccion {
		return BootstrapCompanyResponse{}, domain.NewBadRequestError("el ambiente debe ser PILOTO o PRODUCCION")
	}

	var result BootstrapCompanyResponse

	err := uc.db.Transaction(func(tx *gorm.DB) error {
		if err := uc.companyRepo.Create(company); err != nil {
			return err
		}

		result.Company = company

		plain, key, genErr := uc.Generate(company.ID, "default")
		if genErr != nil {
			return genErr
		}

		result.APIKey = plain
		result.KeyPrefix = key.KeyPrefix
		result.KeyID = key.ID

		return nil
	})

	if err != nil {
		if errors.Is(err, domain.ErrCompanyNitConflict) {
			return BootstrapCompanyResponse{}, domain.NewConflictError("ya existe una empresa registrada con este nit")
		}
		return BootstrapCompanyResponse{}, err
	}

	return result, nil
}