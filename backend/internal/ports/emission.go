package ports

import (
	"context"
	"github.com/brandsrx/supay/internal/domain"
)

// InvoiceEmitter is the driving port used by authenticated HTTP requests.
type InvoiceEmitter interface {
	Emit(context.Context, string) (*domain.Invoice, error)
}

// InvoiceEmissionProcessor is the driving port used by background workers.
// A worker supplies an explicit tenant rather than an HTTP authentication context.
type InvoiceEmissionProcessor interface {
	ProcessEmission(context.Context, string, string) (*domain.Invoice, error)
}
