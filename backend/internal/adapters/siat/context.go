package siat

import "context"

type ctxKey string

const companyIDKey ctxKey = "siat_company_id"

// WithCompanyID inyecta el CompanyId en el contexto (middleware auth).
func WithCompanyID(ctx context.Context, companyID string) context.Context {
	return context.WithValue(ctx, companyIDKey, companyID)
}

// CompanyIDFromContext extrae el CompanyId del contexto.
func CompanyIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(companyIDKey).(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}
