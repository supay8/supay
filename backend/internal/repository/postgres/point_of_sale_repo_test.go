package postgres

import (
	"errors"
	"testing"

	"github.com/brandsrx/supay/internal/domain"
)

func TestPointOfSaleGetByIDRechazaUUIDInvalido(t *testing.T) {
	repo := &PostgresPointOfSaleRepository{}

	_, err := repo.GetByID("pos_123")
	var validationErr *domain.BadRequestError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error=%T %v; se esperaba BadRequestError", err, err)
	}
	if got := err.Error(); got != "point_of_sale_id debe ser un UUID válido" {
		t.Fatalf("mensaje=%q", got)
	}
}
