package usecase

import (
	"errors"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"gorm.io/gorm"
)

type PointOfSaleUsecase struct {
	repo   domain.PointOfSaleRepository
	branch domain.BranchRepository
}

func NewPointOfSaleUsecase(repo domain.PointOfSaleRepository, branch domain.BranchRepository) *PointOfSaleUsecase {
	return &PointOfSaleUsecase{repo: repo, branch: branch}
}

type RegisterPointOfSaleRequest struct {
	BranchId       string  `json:"branch_id"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	TipoPuntoVenta int     `json:"tipo_punto_venta"`
	Cuis           *string `json:"cuis,omitempty"`
	IsActive       *bool   `json:"is_active,omitempty"`
}

type UpdatePointOfSaleRequest struct {
	BranchId       *string `json:"branch_id,omitempty"`
	Name           *string `json:"name,omitempty"`
	Description    *string `json:"description,omitempty"`
	TipoPuntoVenta *int    `json:"tipo_punto_venta,omitempty"`
	Cuis           *string `json:"cuis,omitempty"`
	IsActive       *bool   `json:"is_active,omitempty"`
}

func (uc *PointOfSaleUsecase) Register(req RegisterPointOfSaleRequest) (*domain.PointOfSale, error) {
	if req.BranchId == "" {
		return nil, domain.NewBadRequestError("el branch_id es obligatorio")
	}
	if req.Description == "" {
		return nil, domain.NewBadRequestError("la descripción es obligatoria")
	}
	if req.Name == "" {
		return nil, domain.NewBadRequestError("el nombre del POS es obligatorio")
	}
	if req.TipoPuntoVenta == 0 {
		req.TipoPuntoVenta = 2 // default
	}
	// Regla de negocio: Verificar que la sucursal exista
	branch, err := uc.branch.GetByID(req.BranchId)
	if err != nil {
		return nil, domain.NewNotFoundError("sucursal no encontrada")
	}
	pos := &domain.PointOfSale{
		CompanyId:      branch.CompanyID,
		BranchId:       branch.ID,
		TipoPuntoVenta: &req.TipoPuntoVenta,
		Description:    req.Description,
		Name:           req.Name,
		CodigoSucursal: branch.CodigoSucursal,
		Cuis:           req.Cuis,
		IsActive:       true,
	}

	if req.IsActive != nil {
		pos.IsActive = *req.IsActive
	}

	if pos.Cuis != nil && *pos.Cuis != "" {
		now := time.Now()
		pos.CuisCreatedAt = &now
	}

	// El repositorio calcula codigoPuntoVenta (MAX+1) bajo advisory lock.
	if err := uc.repo.Create(pos); err != nil {
		return nil, err
	}

	return pos, nil
}

func (uc *PointOfSaleUsecase) GetByID(id string) (*domain.PointOfSale, error) {
	pos, err := uc.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.NewNotFoundError("punto de venta no encontrado")
		}
		return nil, err
	}
	return pos, nil
}

func (uc *PointOfSaleUsecase) List(companyID string) ([]*domain.PointOfSale, error) {
	return uc.repo.List(companyID)
}

func (uc *PointOfSaleUsecase) Update(req UpdatePointOfSaleRequest, id string) (*domain.PointOfSale, error) {
	if id == "" {
		return nil, domain.NewBadRequestError("el id es obligatorio")
	}

	// Verificar si el punto de venta existe
	existing, err := uc.repo.GetByID(id)
	if err != nil {
		return nil, domain.NewNotFoundError("punto de venta no encontrado")
	}

	if req.Description != nil {
		existing.Description = *req.Description
	}

	if req.BranchId != nil {
		branch, branchErr := uc.branch.GetByID(*req.BranchId)
		if branchErr != nil {
			return nil, domain.NewNotFoundError("sucursal no encontrada")
		}
		existing.BranchId = branch.ID
		existing.CompanyId = branch.CompanyID
		existing.CodigoSucursal = branch.CodigoSucursal
	}
	if req.TipoPuntoVenta != nil {
		existing.TipoPuntoVenta = req.TipoPuntoVenta
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Cuis != nil {
		if existing.Cuis == nil || *existing.Cuis != *req.Cuis {
			now := time.Now()
			existing.CuisCreatedAt = &now
		}
		existing.Cuis = req.Cuis
	}
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}

	if err := uc.repo.Update(existing); err != nil {
		return nil, err
	}

	return existing, nil
}

func (uc *PointOfSaleUsecase) Delete(id string) error {
	// Verificar si el punto de venta existe
	_, err := uc.repo.GetByID(id)
	if err != nil {
		return domain.NewNotFoundError("punto de venta no encontrado")
	}

	return uc.repo.Delete(id)
}
