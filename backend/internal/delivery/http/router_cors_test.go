package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brandsrx/supay/internal/config"
)

func TestRouterUsesConfiguredCORSOrigins(t *testing.T) {
	router := NewRouter(
		config.Config{CORSAllowedOrigins: []string{"https://app.supay.test"}},
		nil,
		nil,
		func(http.ResponseWriter, *http.Request) {},
	)

	request := httptest.NewRequest(http.MethodOptions, "/v1/health", nil)
	request.Header.Set("Origin", "https://app.supay.test")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if origin := response.Header().Get("Access-Control-Allow-Origin"); origin != "https://app.supay.test" {
		t.Fatalf("origen CORS configurado no permitido: %q", origin)
	}
}

func TestRouterDoesNotReflectUnknownCORSOrigin(t *testing.T) {
	router := NewRouter(
		config.Config{CORSAllowedOrigins: []string{"https://app.supay.test"}},
		nil,
		nil,
		func(http.ResponseWriter, *http.Request) {},
	)

	request := httptest.NewRequest(http.MethodOptions, "/v1/health", nil)
	request.Header.Set("Origin", "https://attacker.example")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if origin := response.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Fatalf("origen CORS desconocido reflejado: %q", origin)
	}
}
