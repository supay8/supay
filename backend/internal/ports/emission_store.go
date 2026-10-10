package ports

import (
	"context"

	"github.com/brandsrx/supay/internal/domain"
)

// EmissionPreparationStore atomically stores a prepared document while the
// invoice is still claimed (SENDING), without changing its business state.
type EmissionPreparationStore interface {
	StorePreparedEmission(context.Context, *domain.Invoice) error
}
