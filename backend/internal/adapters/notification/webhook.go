package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/ports"
)

var ErrMissingWebhookTarget = errors.New("el tenant no tiene un webhook de alertas de certificados configurado")

type WebhookNotifier struct {
	client *http.Client
}

func NewWebhookNotifier(timeout time.Duration) *WebhookNotifier {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &WebhookNotifier{client: &http.Client{Timeout: timeout}}
}

func (n *WebhookNotifier) Notify(ctx context.Context, target string, notification ports.Notification) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return ErrMissingWebhookTarget
	}
	parsed, err := url.ParseRequestURI(target)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil {
		return errors.New("webhook de alertas inválido")
	}
	body, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("serializar notificación: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("crear request de notificación: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "supay-certificate-monitor/1.0")
	if notification.ID != "" {
		req.Header.Set("X-Supay-Event-ID", notification.ID)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			// url.Error incluye el target completo; no lo propagamos porque el
			// webhook puede llevar un token en el query string.
			return fmt.Errorf("entregar notificación: %w", urlErr.Err)
		}
		return fmt.Errorf("entregar notificación: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("webhook respondió HTTP %d", resp.StatusCode)
	}
	return nil
}
