package usecase

import (
	"errors"

	"github.com/brandsrx/supay/internal/domain"
	"gorm.io/gorm"
)

type CustomerUsecase struct {
	repo        domain.CustomerRepository
	companyRepo domain.CompanyRepository
}

func NewCustomerUsecase(repo domain.CustomerRepository, companyRepo domain.CompanyRepository) *CustomerUsecase {
	return &CustomerUsecase{repo: repo, companyRepo: companyRepo}
}

type CreateCustomerRequest struct {
	CompanyId      string  `json:"company_id"`
	DocumentType   string  `json:"document_type"`
	DocumentNumber string  `json:"document_number"`
	Complement     *string `json:"complement,omitempty"`
	Name           string  `json:"name"`
}

func validDocumentType(dt string) bool {
	switch dt {
	case "CI", "CEX", "PAS", "NIT", "OD":
		return true
	}
	return false
}

func (uc *CustomerUsecase) Create(req CreateCustomerRequest) (*domain.Customer, error) {
	if req.CompanyId == "" {
		return nil, domain.NewBadRequestError("el company_id es obligatorio")
	}
	if req.DocumentType == "" || req.DocumentNumber == "" {
		return nil, domain.NewBadRequestError("el tipo y número de documento son obligatorios")
	}
	if !validDocumentType(req.DocumentType) {
		return nil, domain.NewBadRequestError("tipo de documento inválido (CI, CEX, PAS, NIT, OD)")
	}
	if req.Name == "" {
		return nil, domain.NewBadRequestError("el nombre es obligatorio")
	}
	if _, err := uc.companyRepo.GetByID(req.CompanyId); err != nil {
		return nil, domain.NewNotFoundError("empresa no encontrada")
	}

	existing, err := uc.repo.GetByCompanyAndDocument(req.CompanyId, req.DocumentType, req.DocumentNumber)
	if err == nil && existing != nil {
		return nil, domain.ErrCustomerDocumentConflict
	}

	c := &domain.Customer{
		CompanyId:      req.CompanyId,
		DocumentType:   req.DocumentType,
		DocumentNumber: req.DocumentNumber,
		Complement:     req.Complement,
		Name:           req.Name,
	}
	if err := uc.repo.Create(c); err != nil {
		return nil, err
	}
	return c, nil
}

func (uc *CustomerUsecase) GetByID(id string) (*domain.Customer, error) {
	c, err := uc.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.NewNotFoundError("cliente no encontrado")
		}
		return nil, err
	}
	return c, nil
}

func (uc *CustomerUsecase) List(companyID string) ([]*domain.Customer, error) {
	return uc.repo.List(companyID)
}
