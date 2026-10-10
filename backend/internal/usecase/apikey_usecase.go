package usecase

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
	"github.com/google/uuid"
)

type ApiKeyUsecase struct {
	companyRepo ports.CompanyRepository
	apiKeyRepo  ports.APIKeyRepository
	bootstrap   ports.CompanyBootstrap
	hasher      ports.SecretHasher
}

func NewApiKeyUsecase(companyRepo ports.CompanyRepository, apiKeyRepo ports.APIKeyRepository, bootstrap ports.CompanyBootstrap, hasher ports.SecretHasher) *ApiKeyUsecase {
	return &ApiKeyUsecase{companyRepo: companyRepo, apiKeyRepo: apiKeyRepo, bootstrap: bootstrap, hasher: hasher}
}

type CreateApiKeyRequest struct {
	Name string `json:"name"`
}

type CreateApiKeyResponse struct {
	APIKey    string `json:"api_key"`
	KeyPrefix string `json:"key_prefix"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type ListApiKeyResponse struct {
	ID         string  `json:"id"`
	KeyPrefix  string  `json:"key_prefix"`
	Name       string  `json:"name"`
	IsActive   bool    `json:"is_active"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

func (uc *ApiKeyUsecase) Generate(tenantID, name string) (string, *domain.ApiKey, error) {
	maxKeys, err := uc.apiKeyRepo.MaxActiveKeys(tenantID)
	if err != nil {
		return "", nil, err
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
	if activeCount >= maxKeys {
		return "", nil, domain.NewConflictError("límite de API keys alcanzado (" + strconv.Itoa(maxKeys) + ")")
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

	hash, err := uc.hasher.Hash(plain)
	if err != nil {
		return "", nil, err
	}

	key := &domain.ApiKey{
		CompanyId: tenantID,
		KeyHash:   hash,
		KeyPrefix: prefix,
		Name:      name,
		Scopes:    []string{"read", "write"},
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
	Company   *domain.Company `json:"company"`
	APIKey    string          `json:"api_key"`
	KeyPrefix string          `json:"key_prefix"`
	KeyID     string          `json:"key_id"`
}

func (uc *ApiKeyUsecase) BootstrapCompany(req BootstrapCompanyRequest) (BootstrapCompanyResponse, error) {
	if req.Nit == "" {
		return BootstrapCompanyResponse{}, domain.NewBadRequestError("el nit es obligatorio")
	}
	if req.BusinessName == "" {
		return BootstrapCompanyResponse{}, domain.NewBadRequestError("el nombre de la empresa es obligatorio")
	}

	company := &domain.Company{
		Nit:                   req.Nit,
		BusinessName:          req.BusinessName,
		AuthOrganizationID:    strings.TrimSpace(req.AuthOrganizationID),
		Ambiente:              req.Ambiente,
		Modalidad:             req.Modalidad,
		UsuarioSiat:           req.UsuarioSiat,
		Municipio:             req.Municipio,
		Direccion:             req.Direccion,
		Telefono:              req.Telefono,
		CodigoActividad:       req.CodigoActividad,
		PiePagina:             req.PiePagina,
		CertificateWebhookURL: strings.TrimSpace(req.CertificateWebhookURL),
		InvoiceEmailEnabled:   req.InvoiceEmailEnabled,
	}

	if company.Ambiente == "" {
		company.Ambiente = domain.EnvironmentPiloto
	}
	if company.UsuarioSiat == "" {
		company.UsuarioSiat = "SUPAY"
	}
	if company.Modalidad == 0 {
		company.Modalidad = 1
	}
	if company.AuthOrganizationID != "" {
		if err := uuid.Validate(company.AuthOrganizationID); err != nil {
			return BootstrapCompanyResponse{}, domain.NewBadRequestError("auth_organization_id debe ser un UUID válido")
		}
	}

	if company.Ambiente != domain.EnvironmentPiloto && company.Ambiente != domain.EnvironmentProduccion {
		return BootstrapCompanyResponse{}, domain.NewBadRequestError("el ambiente debe ser PILOTO o PRODUCCION")
	}
	if !validModalidad(company.Modalidad) {
		return BootstrapCompanyResponse{}, domain.NewBadRequestError("la modalidad debe ser 1 (electrónica) o 2 (computarizada)")
	}
	if !validWebhookURL(company.CertificateWebhookURL) {
		return BootstrapCompanyResponse{}, domain.NewBadRequestError("certificate_webhook_url debe ser una URL HTTP(S) válida")
	}

	var result BootstrapCompanyResponse

	err := uc.bootstrap.WithinTransaction(func(companyRepo ports.CompanyRepository, keyRepo ports.APIKeyRepository) error {
		if err := companyRepo.Create(company); err != nil {
			return err
		}

		result.Company = company

		plain, key, genErr := NewApiKeyUsecase(companyRepo, keyRepo, nil, uc.hasher).Generate(company.ID, "default")
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
