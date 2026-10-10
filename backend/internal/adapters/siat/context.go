package siat

import (
	"context"

	"github.com/brandsrx/supay/internal/domain/fiscal"
)

func WithCompanyID(ctx context.Context, id string) context.Context {
	return fiscal.WithCompanyID(ctx, id)
}
func CompanyIDFromContext(ctx context.Context) (string, bool) {
	return fiscal.CompanyIDFromContext(ctx)
}
