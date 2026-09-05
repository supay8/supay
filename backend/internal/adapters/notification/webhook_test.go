package notification

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/ports"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestWebhookNotifierPostsNotification(t *testing.T) {
	var method, contentType, eventID string
	notifier := NewWebhookNotifier(time.Second)
	notifier.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		method = req.Method
		contentType = req.Header.Get("Content-Type")
		eventID = req.Header.Get("X-Supay-Event-ID")
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})

	err := notifier.Notify(context.Background(), "https://tenant.example.test/alerts", ports.Notification{
		ID: "certificate:cert-1:7", Type: "certificate.expiring", TenantID: "tenant-1", OccurredAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if method != http.MethodPost || contentType != "application/json" {
		t.Fatalf("request inesperado: method=%s content-type=%s", method, contentType)
	}
	if eventID != "certificate:cert-1:7" {
		t.Fatalf("X-Supay-Event-ID=%q", eventID)
	}
}

func TestWebhookNotifierRejectsInvalidTarget(t *testing.T) {
	notifier := NewWebhookNotifier(time.Second)
	if err := notifier.Notify(context.Background(), "file:///etc/passwd", ports.Notification{}); err == nil {
		t.Fatal("se esperaba rechazo de un target que no sea HTTP(S)")
	}
}
