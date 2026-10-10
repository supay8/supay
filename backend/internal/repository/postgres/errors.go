package postgres

import (
	"errors"

	"github.com/brandsrx/supay/internal/domain"
	"gorm.io/gorm"
)

// Repository consumers receive domain errors, independent of the ORM.
func repositoryError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) && !errors.Is(err, domain.ErrNotFound) {
		return errors.Join(domain.ErrNotFound, err)
	}
	return err
}
