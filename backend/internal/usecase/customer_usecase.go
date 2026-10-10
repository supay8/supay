package usecase

import (
	"errors"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
)

type CustomerUsecase struct {
	repo ports.CustomerRepository
}

func NewCustomerUsecase(repo ports.CustomerRepository) *CustomerUsecase {
	return &CustomerUsecase{repo: repo}
}

func (uc *CustomerUsecase) GetByID(id string) (*domain.Customer, error) {
	c, err := uc.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.NewNotFoundError("cliente no encontrado")
		}
		return nil, err
	}
	return c, nil
}

func (uc *CustomerUsecase) List(companyID string) ([]*domain.Customer, error) {
	return uc.repo.List(companyID)
}
