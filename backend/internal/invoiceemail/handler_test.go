package invoiceemail

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type verifierStub struct {
	claims map[string]any
	err    error
	token  string
	aud    string
}

func (v *verifierStub) Verify(_ context.Context, token, audience string) (map[string]any, error) {
	v.token, v.aud = token, audience
	return v.claims, v.err
}

type processorStub struct {
	id  string
	err error
}

func (p *processorStub) Process(_ context.Context, id string) error {
	p.id = id
	return p.err
}

func TestHandlerRequiresValidCloudTasksIdentity(t *testing.T) {
	tests := []struct {
		name       string
		auth       string
		claims     map[string]any
		verifyErr  error
		wantStatus int
	}{
		{name: "missing bearer", wantStatus: http.StatusUnauthorized},
		{name: "invalid token", auth: "Bearer bad", verifyErr: errors.New("invalid"), wantStatus: http.StatusUnauthorized},
		{name: "wrong service account", auth: "Bearer token", claims: map[string]any{"email": "other@example.iam.gserviceaccount.com"}, wantStatus: http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &processorStub{}
			handler := NewHandler(processor, &verifierStub{claims: tt.claims, err: tt.verifyErr}, "https://worker.example", "tasks@example.iam.gserviceaccount.com")
			req := httptest.NewRequest(http.MethodPost, "/internal/tasks/send-invoice-email", strings.NewReader(`{"notificacion_id":"n-1"}`))
			req.Header.Set("Authorization", tt.auth)
			response := httptest.NewRecorder()

			handler.SendInvoiceEmail(response, req)

			if response.Code != tt.wantStatus {
				t.Fatalf("status=%d want=%d", response.Code, tt.wantStatus)
			}
			if processor.id != "" {
				t.Fatalf("el procesador no debía ejecutarse: %q", processor.id)
			}
		})
	}
}

func TestHandlerProcessesAuthenticatedTask(t *testing.T) {
	processor := &processorStub{}
	verifier := &verifierStub{claims: map[string]any{"email": "tasks@example.iam.gserviceaccount.com"}}
	handler := NewHandler(processor, verifier, "https://worker.example", "tasks@example.iam.gserviceaccount.com")
	req := httptest.NewRequest(http.MethodPost, "/internal/tasks/send-invoice-email", strings.NewReader(`{"notificacion_id":"n-123"}`))
	req.Header.Set("Authorization", "Bearer signed-token")
	response := httptest.NewRecorder()

	handler.SendInvoiceEmail(response, req)

	if response.Code != http.StatusOK || processor.id != "n-123" {
		t.Fatalf("status=%d notification=%q", response.Code, processor.id)
	}
	if verifier.token != "signed-token" || verifier.aud != "https://worker.example" {
		t.Fatalf("verificación inesperada: token=%q audience=%q", verifier.token, verifier.aud)
	}
}

func TestHandlerReturnsServerErrorSoCloudTasksRetries(t *testing.T) {
	processor := &processorStub{err: errors.New("smtp down")}
	handler := NewHandler(processor, &verifierStub{claims: map[string]any{"email": "tasks@example.iam.gserviceaccount.com"}}, "https://worker.example", "tasks@example.iam.gserviceaccount.com")
	req := httptest.NewRequest(http.MethodPost, "/internal/tasks/send-invoice-email", strings.NewReader(`{"notificacion_id":"n-123"}`))
	req.Header.Set("Authorization", "Bearer signed-token")
	response := httptest.NewRecorder()

	handler.SendInvoiceEmail(response, req)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", response.Code)
	}
}
