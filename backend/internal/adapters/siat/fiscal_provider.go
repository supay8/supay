package siat

import (
	"context"

	"github.com/brandsrx/supay/internal/ports"
)

// FiscalProvider hides concrete cached SDK clients behind the application port.
type FiscalProvider struct{ clients SiatClientProvider }

func NewFiscalProvider(clients SiatClientProvider) *FiscalProvider {
	return &FiscalProvider{clients: clients}
}
func (p *FiscalProvider) GetForCompany(ctx context.Context, id string) (ports.FiscalService, error) {
	client, err := p.clients.GetForCompany(ctx, id)
	if err != nil {
		return nil, err
	}
	return NewFiscalAdapter(client), nil
}
func (p *FiscalProvider) Invalidate(id string) { p.clients.Invalidate(id) }

var _ ports.FiscalServiceProvider = (*FiscalProvider)(nil)
