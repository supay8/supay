package invoiceemail

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"google.golang.org/api/idtoken"
)

type NotificationProcessor interface {
	Process(context.Context, string) error
}

type TokenVerifier interface {
	Verify(context.Context, string, string) (map[string]any, error)
}

type GoogleTokenVerifier struct{}

func (GoogleTokenVerifier) Verify(ctx context.Context, token, audience string) (map[string]any, error) {
	payload, err := idtoken.Validate(ctx, token, audience)
	if err != nil {
		return nil, err
	}
	return payload.Claims, nil
}

type Handler struct {
	processor      NotificationProcessor
	verifier       TokenVerifier
	audience       string
	serviceAccount string
}

func NewHandler(processor NotificationProcessor, verifier TokenVerifier, audience, serviceAccount string) *Handler {
	if verifier == nil {
		verifier = GoogleTokenVerifier{}
	}
	return &Handler{processor: processor, verifier: verifier, audience: audience, serviceAccount: strings.ToLower(strings.TrimSpace(serviceAccount))}
}

func (h *Handler) SendInvoiceEmail(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.processor == nil || h.verifier == nil || h.audience == "" || h.serviceAccount == "" {
		http.Error(w, "email worker no configurado", http.StatusServiceUnavailable)
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	claims, err := h.verifier.Verify(r.Context(), token, h.audience)
	if err != nil || !strings.EqualFold(stringClaim(claims, "email"), h.serviceAccount) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var input struct {
		NotificationID string `json:"notificacion_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || strings.TrimSpace(input.NotificationID) == "" {
		http.Error(w, "payload inválido", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "payload inválido", http.StatusBadRequest)
		return
	}
	if err := h.processor.Process(r.Context(), input.NotificationID); err != nil {
		if errors.Is(err, context.Canceled) {
			http.Error(w, "cancelled", http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "email delivery failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"sent"}`))
}

func stringClaim(claims map[string]any, key string) string {
	value, _ := claims[key].(string)
	return value
}
