package http

import (
	"crypto/subtle"
	"net/http"
)

const apiKeyHeader = "X-API-Key"

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
