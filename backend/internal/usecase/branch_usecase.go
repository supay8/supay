package usecase

import (
	"errors"

	"github.com/brandsrx/supay/internal/domain"
	"time"
)

type BranchUsecase struct {
	repo        domain.BranchRepository
	companyRepo domain.CompanyRepository
}

func NewBranchUsecase(repo domain.BranchRepository, companyRepo domain.CompanyRepository) *BranchUsecase {
	return &BranchUsecase{repo: repo, companyRepo: companyRepo}
}

type CreateBranchRequest struct {
	CompanyID      string `json:"company_id"`
	CodigoSucursal int    `json:"codigo_sucursal"`
	Name           string `json:"name"`
	Address        string `json:"address"`
}

type UpdateBranchRequest struct {
	CodigoSucursal *int    `json:"codigo_sucursal,omitempty"`
	Name           *string `json:"name,omitempty"`
	Address        *string `json:"address,omitempty"`
	Active         *bool   `json:"active,omitempty"`
}

func (uc *BranchUsecase) Create(req CreateBranchRequest) (*domain.Branch, error) {
	if req.CompanyID == "" {
		return nil, errors.New("company_id es obligatorio")
	}
	if req.Name == "" {
		return nil, errors.New("name es obligatorio")
	}
	if req.CodigoSucursal < 0 {
		return nil, errors.New("codigo_sucursal debe ser mayor o igual a 0")
	}
	if _, err := uc.companyRepo.GetByID(req.CompanyID); err != nil {
		return nil, errors.New("empresa no encontrada")
	}
	if existing, err := uc.repo.GetByCompanyAndSucursal(req.CompanyID, req.CodigoSucursal); err == nil && existing != nil {
		return nil, domain.ErrBranchSucursalConflict
	}

	b := &domain.Branch{
		CompanyID:      req.CompanyID,
		CodigoSucursal: req.CodigoSucursal,
		Name:           req.Name,
		Address:        req.Address,
		Active:         true,
		CreatedAt:      time.Now(),
	}
	if err := uc.repo.Create(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (uc *BranchUsecase) GetByID(id string) (*domain.Branch, error) {
	return uc.repo.GetByID(id)
}

func (uc *BranchUsecase) List(companyID string) ([]*domain.Branch, error) {
	return uc.repo.List(companyID)
}

func (uc *BranchUsecase) Update(req UpdateBranchRequest, id string) (*domain.Branch, error) {
	b, err := uc.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if req.CodigoSucursal != nil {
		if *req.CodigoSucursal < 0 {
			return nil, errors.New("codigo_sucursal debe ser mayor o igual a 0")
		}
		b.CodigoSucursal = *req.CodigoSucursal
	}
	if req.Name != nil {
		b.Name = *req.Name
	}
	if req.Address != nil {
		b.Address = *req.Address
	}
	if req.Active != nil {
		b.Active = *req.Active
	}
	if err := uc.repo.Update(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (uc *BranchUsecase) Delete(id string) error {
	return uc.repo.Delete(id)
}
