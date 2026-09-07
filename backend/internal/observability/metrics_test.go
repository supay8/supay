package observability

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func TestHTTPMiddlewareUsesNormalizedRoute(t *testing.T) {
	metrics := NewMetrics()
	router := chi.NewRouter()
	router.Use(HTTPMiddleware(metrics))
	router.Get("/customers/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/customers/customer-secret", nil))

	metricsRecorder := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(metricsRecorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := metricsRecorder.Body.String()
	if !strings.Contains(body, `supay_http_requests_total{method="GET",route="/customers/{id}",status="204"} 1`) {
		t.Fatalf("métrica normalizada ausente:\n%s", body)
	}
	if strings.Contains(body, "customer-secret") {
		t.Fatal("la métrica expuso un identificador concreto")
	}
}

func TestMetricsExposeEmissionAndOutboxResults(t *testing.T) {
	metrics := NewMetrics()
	metrics.ObserveEmission("success", 250*time.Millisecond)
	metrics.ObserveOutbox("published")

	recorder := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := recorder.Body.String()
	for _, expected := range []string{
		`supay_invoice_emissions_total{result="success"} 1`,
		`supay_outbox_events_total{result="published"} 1`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("métrica %q ausente", expected)
		}
	}
}

func TestRedactingHandlerRemovesPIIAndCredentials(t *testing.T) {
	var output strings.Builder
	handler := NewRedactingHandler(slog.NewJSONHandler(&output, nil))
	logger := slog.New(handler)
	logger.Error("operación fallida",
		"email", "ana@example.com",
		"api_key", "sup_live_secret",
		"path", "/invoices/invoice-123",
		"cufd", "fiscal-code-secret",
		"invoice_id", "invoice-123",
		"error", io.ErrUnexpectedEOF,
	)

	logLine := output.String()
	for _, secret := range []string{"ana@example.com", "sup_live_secret", "/invoices/invoice-123", "fiscal-code-secret", "invoice-123", io.ErrUnexpectedEOF.Error()} {
		if strings.Contains(logLine, secret) {
			t.Fatalf("el log expuso %q: %s", secret, logLine)
		}
	}
	if !strings.Contains(logLine, "[REDACTED]") || !strings.Contains(logLine, "sha256:") {
		t.Fatalf("el log no aplicó redacción/hash: %s", logLine)
	}
}
