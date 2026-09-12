package usecase

import (
	"errors"

	"github.com/brandsrx/supay/internal/domain"
	"gorm.io/gorm"
)

type CustomerUsecase struct {
	repo domain.CustomerRepository
}

func NewCustomerUsecase(repo domain.CustomerRepository) *CustomerUsecase {
	return &CustomerUsecase{repo: repo}
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
