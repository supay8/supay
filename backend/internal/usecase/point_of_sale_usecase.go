package usecase

import (
	"errors"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"gorm.io/gorm"
)

type PointOfSaleUsecase struct {
	repo        domain.PointOfSaleRepository
	companyRepo domain.CompanyRepository
}

func NewPointOfSaleUsecase(repo domain.PointOfSaleRepository, companyRepo domain.CompanyRepository) *PointOfSaleUsecase {
	return &PointOfSaleUsecase{repo: repo, companyRepo: companyRepo}
}

type RegisterPointOfSaleRequest struct {
	CompanyId      string  `json:"company_id"`
	CodigoSucursal int     `json:"codigo_sucursal"`
	Description    string  `json:"description"`
	Cuis           *string `json:"cuis,omitempty"`
	IsActive       *bool   `json:"is_active,omitempty"`
}

type UpdatePointOfSaleRequest struct {
	CodigoSucursal *int    `json:"codigo_sucursal,omitempty"`
	Description    *string `json:"description,omitempty"`
	Cuis           *string `json:"cuis,omitempty"`
	IsActive       *bool   `json:"is_active,omitempty"`
}

func (uc *PointOfSaleUsecase) Register(req RegisterPointOfSaleRequest) (*domain.PointOfSale, error) {
	if req.CompanyId == "" {
		return nil, errors.New("el company_id es obligatorio")
	}
	if req.Description == "" {
		return nil, errors.New("la descripción es obligatoria")
	}

	// Regla de negocio: Verificar que la empresa exista
	if _, err := uc.companyRepo.GetByID(req.CompanyId); err != nil {
		return nil, errors.New("empresa no encontrada")
	}

	pos := &domain.PointOfSale{
		CompanyId:      req.CompanyId,
		CodigoSucursal: req.CodigoSucursal,
		Description:    req.Description,
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
			return nil, errors.New("punto de venta no encontrado")
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
		return nil, errors.New("el ID es obligatorio")
	}

	// Verificar si el punto de venta existe
	existing, err := uc.repo.GetByID(id)
	if err != nil {
		return nil, errors.New("punto de venta no encontrado")
	}

	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.CodigoSucursal != nil {
		existing.CodigoSucursal = *req.CodigoSucursal
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
		return errors.New("punto de venta no encontrado")
	}

	return uc.repo.Delete(id)
}
