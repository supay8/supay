package http

import (
	"context"
)

// contextKey es un tipo privado para evitar colisiones con otros paquetes.
type contextKey int

const (
	companyIDKey contextKey = iota
	userIDKey
)

// WithCompanyID inyecta el identificador de empresa/tenant en el contexto.
// Reemplaza la dependencia de siat.WithCompanyID en la capa HTTP.
func WithCompanyID(ctx context.Context, companyID string) context.Context {
	return context.WithValue(ctx, companyIDKey, companyID)
}

// CompanyIDFromContext extrae el identificador de empresa/tenant del contexto.
func CompanyIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(companyIDKey).(string)
	return id, ok && id != ""
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok && id != ""
}
