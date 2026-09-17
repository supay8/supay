package usecase

import (
	"errors"
	"net/url"
	"strings"

	"github.com/brandsrx/supay/internal/crypto"
	"github.com/brandsrx/supay/internal/domain"
	"gorm.io/gorm"
)

type CompanyUsecase struct {
	repo                 domain.CompanyRepository
	crypto               *crypto.Service
	invalidateSiatClient func(companyID string)
}

func NewCompanyUsecase(repo domain.CompanyRepository, cryptoSvc *crypto.Service, invalidateSiatClient func(string)) *CompanyUsecase {
	return &CompanyUsecase{repo: repo, crypto: cryptoSvc, invalidateSiatClient: invalidateSiatClient}
}

type RegisterCompanyRequest struct {
	Nit          string                 `json:"nit"`
	BusinessName string                 `json:"business_name"`
	Ambiente     domain.SiatEnvironment `json:"ambiente"`
	Modalidad    int                    `json:"modalidad,omitempty"`
	UsuarioSiat  string                 `json:"usuario_siat,omitempty"`
	// Datos del emisor que viajan en la cabecera de la factura. Municipio y
	// dirección deben coincidir con el padrón del SIAT.
	Municipio             string  `json:"municipio,omitempty"`
	Direccion             string  `json:"direccion,omitempty"`
	Telefono              string  `json:"telefono,omitempty"`
	CodigoActividad       *string `json:"codigo_actividad,omitempty"`
	PiePagina             string  `json:"pie_pagina,omitempty"`
	CertificateWebhookURL string  `json:"certificate_webhook_url,omitempty"`
}

type UpdateCompanyRequest struct {
	Nit                   *string                 `json:"nit,omitempty"`
	BusinessName          *string                 `json:"business_name,omitempty"`
	Ambiente              *domain.SiatEnvironment `json:"ambiente,omitempty"`
	Modalidad             *int                    `json:"modalidad,omitempty"`
	UsuarioSiat           *string                 `json:"usuario_siat,omitempty"`
	Municipio             *string                 `json:"municipio,omitempty"`
	Direccion             *string                 `json:"direccion,omitempty"`
	Telefono              *string                 `json:"telefono,omitempty"`
	CodigoActividad       *string                 `json:"codigo_actividad,omitempty"`
	PiePagina             *string                 `json:"pie_pagina,omitempty"`
	CertificateWebhookURL *string                 `json:"certificate_webhook_url,omitempty"`
	TokenDelegado         *string                 `json:"token_delegado,omitempty"`
}

func (uc *CompanyUsecase) Register(req RegisterCompanyRequest) (*domain.Company, error) {
	if req.Nit == "" {
		return nil, domain.NewBadRequestError("el nit es obligatorio")
	}
	if req.BusinessName == "" {
		return nil, domain.NewBadRequestError("el nombre de la empresa es obligatorio")
	}

	// Regla de negocio: Verificar si ya existe una empresa con el mismo NIT
	existing, err := uc.repo.GetByNit(req.Nit)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	} else if existing != nil {
		return nil, domain.NewConflictError("ya existe una empresa registrada con este nit")
	}

	company := &domain.Company{
		Nit:                   req.Nit,
		BusinessName:          req.BusinessName,
		Ambiente:              req.Ambiente,
		Modalidad:             req.Modalidad,
		UsuarioSiat:           req.UsuarioSiat,
		Municipio:             req.Municipio,
		Direccion:             req.Direccion,
		Telefono:              req.Telefono,
		CodigoActividad:       req.CodigoActividad,
		PiePagina:             req.PiePagina,
		CertificateWebhookURL: strings.TrimSpace(req.CertificateWebhookURL),
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

	if !validEnvironment(company.Ambiente) {
		return nil, domain.NewBadRequestError("el ambiente debe ser PILOTO o PRODUCCION")
	}
	if !validModalidad(company.Modalidad) {
		return nil, domain.NewBadRequestError("la modalidad debe ser 1 (electrónica) o 2 (computarizada)")
	}
	if !validWebhookURL(company.CertificateWebhookURL) {
		return nil, domain.NewBadRequestError("certificate_webhook_url debe ser una URL HTTP(S) válida")
	}

	if err := uc.repo.Create(company); err != nil {
		return nil, err
	}

	return company, nil
}

func validEnvironment(a domain.SiatEnvironment) bool {
	return a == domain.EnvironmentPiloto || a == domain.EnvironmentProduccion
}

func validModalidad(value int) bool {
	return value == 1 || value == 2
}

func validWebhookURL(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := url.ParseRequestURI(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}

func (uc *CompanyUsecase) GetByNit(nit string) (*domain.Company, error) {
	company, err := uc.repo.GetByNit(nit)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.NewNotFoundError("empresa no encontrada")
		}
		return nil, err
	}
	return company, nil
}

func (uc *CompanyUsecase) Update(req UpdateCompanyRequest, id string) (*domain.Company, error) {
	if id == "" {
		return nil, domain.NewBadRequestError("el id es obligatorio")
	}

	// Verificar si la empresa existe
	existing, err := uc.repo.GetByID(id)
	if err != nil {
		return nil, domain.NewNotFoundError("empresa no encontrada")
	}

	siatConfigChanged := false
	if req.Nit != nil {
		if *req.Nit != existing.Nit {
			companyWithSameNit, err := uc.repo.GetByNit(*req.Nit)
			if err == nil && companyWithSameNit != nil && companyWithSameNit.ID != id {
				return nil, domain.NewConflictError("el nit ya está registrado por otra empresa")
			}
		}
		existing.Nit = *req.Nit
		siatConfigChanged = true
	}
	if req.BusinessName != nil {
		existing.BusinessName = *req.BusinessName
	}
	if req.Ambiente != nil {
		if *req.Ambiente == "" {
			existing.Ambiente = domain.EnvironmentPiloto
		} else {
			if !validEnvironment(*req.Ambiente) {
				return nil, domain.NewBadRequestError("el ambiente debe ser PILOTO o PRODUCCION")
			}
			existing.Ambiente = *req.Ambiente
		}
		siatConfigChanged = true
	}
	if req.Modalidad != nil {
		if !validModalidad(*req.Modalidad) {
			return nil, domain.NewBadRequestError("la modalidad debe ser 1 (electrónica) o 2 (computarizada)")
		}
		existing.Modalidad = *req.Modalidad
		siatConfigChanged = true
	}
	if req.UsuarioSiat != nil {
		existing.UsuarioSiat = *req.UsuarioSiat
	}
	if req.Municipio != nil {
		existing.Municipio = *req.Municipio
	}
	if req.Direccion != nil {
		existing.Direccion = *req.Direccion
	}
	if req.Telefono != nil {
		existing.Telefono = *req.Telefono
	}
	if req.CodigoActividad != nil {
		existing.CodigoActividad = req.CodigoActividad
	}
	if req.PiePagina != nil {
		existing.PiePagina = *req.PiePagina
	}
	if req.CertificateWebhookURL != nil {
		webhookURL := strings.TrimSpace(*req.CertificateWebhookURL)
		if !validWebhookURL(webhookURL) {
			return nil, domain.NewBadRequestError("certificate_webhook_url debe ser una URL HTTP(S) válida")
		}
		existing.CertificateWebhookURL = webhookURL
	}
	if req.TokenDelegado != nil {
		token := strings.TrimSpace(*req.TokenDelegado)
		if token == "" {
			existing.EncryptedTokenDelegado = ""
		} else {
			if uc.crypto == nil {
				return nil, domain.NewConflictError("cifrado no configurado: defina ENCRYPTION_KEY")
			}
			encrypted, err := uc.crypto.EncryptString(token)
			if err != nil {
				return nil, err
			}
			existing.EncryptedTokenDelegado = encrypted
		}
		siatConfigChanged = true
	}

	if err := uc.repo.Update(existing); err != nil {
		return nil, err
	}
	if siatConfigChanged && uc.invalidateSiatClient != nil {
		uc.invalidateSiatClient(existing.ID)
	}

	return existing, nil
}

func (uc *CompanyUsecase) Delete(id string) error {
	// Verificar si la empresa existe
	_, err := uc.repo.GetByID(id)
	if err != nil {
		return domain.NewNotFoundError("empresa no encontrada")
	}

	// Eliminar la empresa de la base de datos
	return uc.repo.Delete(id)

}
