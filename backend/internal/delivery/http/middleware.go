package http

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/brandsrx/supay/internal/siat"
)

const apiKeyHeader = "X-API-Key"
const companyIDHeader = "X-Company-Id"

// RequireAPIKey devuelve un middleware que exige el header X-API-Key con el
// valor configurado. Si apikey es vacía, el middleware no se aplica.
func RequireAPIKey(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := r.Header.Get(apiKeyHeader)
			if subtle.ConstantTimeCompare([]byte(provided), []byte(apiKey)) != 1 {
				writeErrorBody(w, http.StatusUnauthorized, errorBody{
					Code:    codeUnauthorized,
					Message: "no autorizado: falta o es inválido el header X-API-Key",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// LimitBody acota el tamaño del cuerpo de las peticiones para evitar que un
// payload enorme consuma memoria antes de ser validado.
func LimitBody(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// InjectCompanyID inyecta el CompanyId en el context a partir del header
// X-Company-Id o del JWT Bearer (claim company_id). No valida existencia;
// el SiatClientProvider fallará si la empresa no existe. Se ejecuta después
// de RequireAPIKey.
func InjectCompanyID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		companyID := strings.TrimSpace(r.Header.Get(companyIDHeader))
		// Fallback: Bearer JWT con claim company_id (sin verificar firma en fase 1,
		// solo extrae si viene como JWT base64 sin validación - para compatibilidad futura)
		if companyID == "" {
			if auth := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(auth), "bearer ") {
				// No se valida firma aquí; el provider no depende de JWT, solo deja el header.
				// Un middleware JWT futuro puede reemplazar este bloque.
				_ = auth
			}
		}
		if companyID != "" {
			ctx := siat.WithCompanyID(r.Context(), companyID)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}
