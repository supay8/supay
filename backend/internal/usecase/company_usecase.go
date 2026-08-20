package usecase

import (
	"errors"

	"github.com/brandsrx/supay/internal/domain"
	"gorm.io/gorm"
)

type CompanyUsecase struct {
	repo domain.CompanyRepository
}

func NewCompanyUsecase(repo domain.CompanyRepository) *CompanyUsecase {
	return &CompanyUsecase{repo: repo}
}

type RegisterCompanyRequest struct {
	Nit           string                 `json:"nit"`
	BusinessName  string                 `json:"business_name"`
	CodigoSistema string                 `json:"codigo_sistema"`
	Ambiente      domain.SiatEnvironment `json:"ambiente"`
	UsuarioSiat   string                 `json:"usuario_siat,omitempty"`
}

type UpdateCompanyRequest struct {
	Nit           *string                 `json:"nit,omitempty"`
	BusinessName  *string                 `json:"business_name,omitempty"`
	CodigoSistema *string                 `json:"codigo_sistema,omitempty"`
	Ambiente      *domain.SiatEnvironment `json:"ambiente,omitempty"`
	UsuarioSiat   *string                 `json:"usuario_siat,omitempty"`
}

func (uc *CompanyUsecase) Register(req RegisterCompanyRequest) (*domain.Company, error) {
	if req.Nit == "" {
		return nil, errors.New("el NIT es obligatorio")
	}
	if req.BusinessName == "" {
		return nil, errors.New("el nombre de la empresa es obligatorio")
	}

	// Regla de negocio: Verificar si ya existe una empresa con el mismo NIT
	existing, err := uc.repo.GetByNit(req.Nit)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	} else if existing != nil {
		return nil, errors.New("ya existe una empresa registrada con este NIT")
	}

	company := &domain.Company{
		Nit:           req.Nit,
		BusinessName:  req.BusinessName,
		CodigoSistema: req.CodigoSistema,
		Ambiente:      req.Ambiente,
		UsuarioSiat:   req.UsuarioSiat,
	}

	if company.Ambiente == "" {
		company.Ambiente = domain.EnvironmentPiloto
	}
	if company.UsuarioSiat == "" {
		company.UsuarioSiat = "SUPAY"
	}

	if !validEnvironment(company.Ambiente) {
		return nil, errors.New("el ambiente debe ser PILOTO o PRODUCCION")
	}

	if err := uc.repo.Create(company); err != nil {
		return nil, err
	}

	return company, nil
}

func validEnvironment(a domain.SiatEnvironment) bool {
	return a == domain.EnvironmentPiloto || a == domain.EnvironmentProduccion
}

func (uc *CompanyUsecase) GetByNit(nit string) (*domain.Company, error) {
	return uc.repo.GetByNit(nit)
}

func (uc *CompanyUsecase) Update(req UpdateCompanyRequest, id string) (*domain.Company, error) {
	if id == "" {
		return nil, errors.New("el ID es obligatorio")
	}

	// Verificar si la empresa existe
	existing, err := uc.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("empresa no encontrada")
	}

	if req.Nit != nil {
		if *req.Nit != existing.Nit {
			companyWithSameNit, err := uc.repo.GetByNit(*req.Nit)
			if err == nil && companyWithSameNit != nil && companyWithSameNit.ID != id {
				return nil, errors.New("el NIT ya está registrado por otra empresa")
			}
		}
		existing.Nit = *req.Nit
	}
	if req.BusinessName != nil {
		existing.BusinessName = *req.BusinessName
	}
	if req.CodigoSistema != nil {
		existing.CodigoSistema = *req.CodigoSistema
	}
	if req.Ambiente != nil {
		if *req.Ambiente == "" {
			existing.Ambiente = domain.EnvironmentPiloto
		} else {
			if !validEnvironment(*req.Ambiente) {
				return nil, errors.New("el ambiente debe ser PILOTO o PRODUCCION")
			}
			existing.Ambiente = *req.Ambiente
		}
	}
	if req.UsuarioSiat != nil {
		existing.UsuarioSiat = *req.UsuarioSiat
	}

	if err := uc.repo.Update(existing); err != nil {
		return nil, err
	}

	return existing, nil
}

func (uc *CompanyUsecase) Delete(id string) error {
	// Verificar si la empresa existe
	_, err := uc.repo.GetByID(id)
	if err != nil {
		return errors.New("empresa no encontrada")
	}

	// Eliminar la empresa de la base de datos
	return uc.repo.Delete(id)
}
