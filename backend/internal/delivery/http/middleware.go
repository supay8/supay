package http

import (
	"net/http"
	"strings"

	"github.com/brandsrx/supay/internal/models"
	"github.com/brandsrx/supay/internal/siat"
)

const apiKeyHeader = "X-API-Key"

// ApiKeyLookup es el contrato mínimo que necesita el middleware para resolver
// una API key a un tenant. La implementación real vive en el repositorio.
type ApiKeyLookup interface {
	FindByPrefix(prefix string) (*models.ApiKey, error)
	TouchLastUsed(id string) error
}

// TenantMiddleware autentica la petición por X-API-Key e inyecta el company_id
// en el contexto. Es el único mecanismo de resolución de tenant; no se acepta
// header X-Company-Id.
func TenantMiddleware(lookup ApiKeyLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := strings.TrimSpace(r.Header.Get(apiKeyHeader))
			if provided == "" {
				writeErrorBody(w, http.StatusUnauthorized, errorBody{
					Code:    CodeUnauthorized,
					Message: "no autorizado: falta el header X-API-Key",
				})
				return
			}

			companyID, ok := resolveTenant(provided, lookup)
			if !ok {
				writeErrorBody(w, http.StatusUnauthorized, errorBody{
					Code:    CodeUnauthorized,
					Message: "no autorizado: API key inválida o inactiva",
				})
				return
			}

			ctx := WithCompanyID(r.Context(), companyID)
			// Fallback temporal para no romper el provider SIAT durante la transición.
			ctx = siat.WithCompanyID(ctx, companyID)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

// resolveTenant busca la API key en la tabla api_keys por prefijo y verifica
// el hash. Devuelve (companyID, true) si tiene éxito.
func resolveTenant(provided string, lookup ApiKeyLookup) (string, bool) {
	if lookup == nil {
		return "", false
	}
	prefix := extractKeyPrefix(provided)
	key, err := lookup.FindByPrefix(prefix)
	if err != nil {
		return "", false
	}
	if !verifyAPIKey(provided, key.KeyHash) {
		return "", false
	}
	// Actualizar last_used_at de forma fire-and-forget.
	_ = lookup.TouchLastUsed(key.ID)
	return key.CompanyId, true
}

// extractKeyPrefix extrae un prefijo identificador de una API key con formato
// sup_<prefijo>_<random>. Si el formato no coincide, devuelve la key completa
// (la búsqueda simplemente no encontrará coincidencia).
func extractKeyPrefix(key string) string {
	parts := strings.Split(key, "_")
	if len(parts) >= 3 && parts[0] != "" {
		return strings.Join(parts[:len(parts)-1], "_")
	}
	return key
}

// verifyAPIKey compara una API key plana con su hash almacenado.
// La implementación real se inyecta desde el contenedor (bcrypt).
var verifyAPIKey = func(plain, hash string) bool {
	return false
}

// SetVerifyAPIKey permite inyectar la función de verificación (usado en
// contenedor/router y tests).
func SetVerifyAPIKey(fn func(plain, hash string) bool) {
	verifyAPIKey = fn
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
