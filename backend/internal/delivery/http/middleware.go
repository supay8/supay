package http

import (
	"crypto/subtle"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/models"
)

const apiKeyHeader = "X-API-Key"

// ApiKeyLookup es el contrato mínimo que necesita el middleware para resolver
// una API key a un tenant. La implementación real vive en el repositorio.
type ApiKeyLookup interface {
	FindByPrefix(prefix string) (*models.ApiKey, error)
	TouchLastUsed(id string) error
}

func InternalBootstrapMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Este log ahora sí se ejecuta en cada petición que llega a este grupo
			log.Println("Pasa por el middleware de bootstrap interno")

			token := r.Header.Get("X-Backend-Token")
			match := subtle.ConstantTimeCompare([]byte(token), []byte(secret)) == 1

			if !match {
				http.Error(w, "Unauthorized internal request", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
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

// RateLimiterIP implementa un token bucket por IP para limitar peticiones.
type RateLimiterIP struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rate    int
	burst   int
	cleanup *time.Ticker
}

type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
}

func newTokenBucket(rate, burst int) *tokenBucket {
	return &tokenBucket{
		tokens:     float64(burst),
		lastRefill: time.Now(),
	}
}
func (tb *tokenBucket) take(rate, burst int) bool {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * float64(rate)
	if tb.tokens > float64(burst) {
		tb.tokens = float64(burst)
	}
	tb.lastRefill = now
	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// RateLimitIP crea un middleware que limita las peticiones por IP.
// rate: peticiones permitidas por segundo (sustained rate).
// burst: máximo burst permitido.
// cleanupInterval: cada cuánto limpiar buckets inactivos.
func RateLimitIP(rate, burst int, cleanupInterval time.Duration) func(http.Handler) http.Handler {
	rl := &RateLimiterIP{
		buckets: make(map[string]*tokenBucket),
		rate:    rate,
		burst:   burst,
	}

	if cleanupInterval > 0 {
		rl.cleanup = time.NewTicker(cleanupInterval)
		go func() {
			for range rl.cleanup.C {
				rl.mu.Lock()
				now := time.Now()
				for ip, bucket := range rl.buckets {
					if now.Sub(bucket.lastRefill) > 10*time.Minute {
						delete(rl.buckets, ip)
					}
				}
				rl.mu.Unlock()
			}
		}()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			rl.mu.Lock()
			bucket, ok := rl.buckets[ip]
			if !ok {
				bucket = newTokenBucket(rate, burst)
				rl.buckets[ip] = bucket
			}
			allowed := bucket.take(rate, burst)
			rl.mu.Unlock()

			if !allowed {
				writeErrorBody(w, http.StatusTooManyRequests, errorBody{
					Code:    CodeRateLimited,
					Message: "demasiadas peticiones, intente más tarde",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxied requests)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	// Fall back to RemoteAddr
	return r.RemoteAddr
}
